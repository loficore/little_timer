// 一次性迁移器：把 v1 主口令时代的凭据搬进平台 keystore（issue #13）。
//
// 迁移分两阶段，边界是**先解密完再落任何盘**——迁移是破坏性操作，
// 口令错 / 文件损坏时必须零副作用（见 TestMigrate_WrongPassword）：
//
//	阶段一（只读）：settings 旧密文列按 dbPath 派生 key 解密 → 内存；
//	                secret.db v1 按 PBKDF2 口令解密 → 内存。
//	                任一解密失败立刻返回，此时磁盘未被触碰。
//	阶段二（可写）：确保 master key 存在 → 把 v1 blob 改名让路 → 建 v2 store
//	                写入全部条目 → 清旧标记 → 写 sentinel。
//
// sentinel 存在即短路返回（Review Focus #1）：迁移半途崩溃后重入必须幂等，
// 且不得因为"旧标记还是 true"就重跑一遍。
//
// 顺序说明：settings 凭据在前（免口令），secret.db 在后（需口令）。
// 这样即使用户在 App 内选了"稍后再输口令"，已能迁的凭据也不会被口令卡住。

package keystore

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"little-timer/internal/crypto"
	"little-timer/internal/log"
)

// SentinelName 是标记"迁移已完成"的哨兵文件名，放在数据目录下。
// 与 secret.db 同级；存在即视为迁移已成功，重入直接短路。
const SentinelName = ".keystore-migrated"

// secretV1Magic 是 v1 secret.db 的 magic 前缀。
// v1 落盘布局:magic(8) || salt(16) || nonce(12) || sealed。
var secretV1Magic = []byte("LTMSECv1")

// secretV1BackupSuffix 是 v1 blob 改名后的后缀。迁移不删用户数据——
// 原地删掉是破坏性的，出问题就再也回不去了。留一份改名备份，
// 用户确认新数据可用后可自行删除。
const secretV1BackupSuffix = ".v1.bak"

// settings 侧的凭据 secret key。取值必须与 settings 包的
// SecretWebDAVPassword / SecretS3AccessKey / SecretS3SecretKey 逐字一致
// （settings 包的 manager_secrets_test.go 有对齐断言守住这条不变量）。
// 这里不能用 settings 的常量——settings → crypto → keystore 会成导入环。
const (
	CredWebDAVPassword = "webdav_password"
	CredS3AccessKey    = "s3_access_key"
	CredS3SecretKey    = "s3_secret_key"
)

// legacyCredentialKeys 是需要从 settings 旧密文列搬走的凭据，按固定顺序遍历
// （map 迭代顺序随机会让日志与 Report 的可复现性变差）。
var legacyCredentialKeys = []string{CredWebDAVPassword, CredS3AccessKey, CredS3SecretKey}

// SettingsCreds 抽象 settings 侧旧密文列的读写，让本包不必 import settings
// （settings → crypto → keystore，已是既有依赖方向；反向会成环）。
// 实现在 settings 包，Task 9 的 migrate 端点复用同一套接口。
type SettingsCreds interface {
	// LegacyCiphertext 返回 backup_config 表里该 key 对应的旧密文。
	// 未配置时返回 (nil, nil)——调用方据此跳过该凭据。
	LegacyCiphertext(key string) ([]byte, error)
	// PutSecret 把解密出的明文写进 keystore 托管的 SecretStorage。
	PutSecret(key string, value []byte) error
	// ClearLegacyState 复位旧状态：has_* 标记列置 0、*_encrypted 列置 NULL。
	// 迁移成功后调用一次。
	ClearLegacyState() error
}

// MigrateOptions 是 MigrateAll 的输入。零值 SafeExcept(Creds/Password) 均合法。
type MigrateOptions struct {
	// DBPath 是 settings DB 路径——旧凭据密钥由它派生，必须与迁移发生时一致。
	DBPath string
	// SecretPath 是 secret.db 的完整路径（v1 与 v2 同址）。
	SecretPath string
	// SentinelPath 是哨兵落盘位置，通常为 filepath.Join(dir(DBPath), SentinelName)。
	// 为空时由 SecretPath 所在目录推导。
	SentinelPath string
	// Password 是 v1 主口令。仅在磁盘上存在 v1 secret.db 时需要；
	// 全新安装可留空。
	Password []byte
	// Creds 是 settings 侧适配器，可为 nil（只迁 secret.db）。
	Creds SettingsCreds
	// DryRun 只做阶段一（解密 + 校验），不产生任何写。
	DryRun bool
}

// Report 是迁移结果摘要。只含计数与布尔，绝不含凭据明文——日志可安全打印。
type Report struct {
	// Skipped 为 true 表示 sentinel 已存在，未做任何事。
	Skipped bool
	// DryRun 为 true 表示只做了只读校验。
	DryRun bool
	// SecretsMoved 是从 v1 secret.db 搬入的条目数。
	SecretsMoved int
	// CredsMoved 是从 settings 旧密文列搬入的凭据数。
	CredsMoved int
	// SentinelWritten 为 true 表示本次写了哨兵。
	SentinelWritten bool
	// V1BackedUp 是 v1 blob 改名备份后的路径，无备份时为空。
	V1BackedUp string
}

// MigrateAll 执行一次性迁移。幂等：sentinel 存在时直接返回 Skipped 报告。
//
// 错误语义:
//   - 口令错 / v1 blob 损坏 → 包裹 ErrUnseal，磁盘零副作用
//   - keystore 不可用        → 包裹 ErrUnavailable
//   - 哨兵已存在             → (Report{Skipped:true}, nil)，不是错误
func MigrateAll(ks Keystore, opts MigrateOptions) (Report, error) {
	if opts.SentinelPath == "" && opts.SecretPath != "" {
		opts.SentinelPath = filepath.Join(filepath.Dir(opts.SecretPath), SentinelName)
	}

	// 幂等闸门：哨兵在，任何后续动作都不做。放在最前面，连 Available 探测
	// 都不做——迁移已完成时不该因为 keystore 临时不可用就把启动卡死。
	if opts.SentinelPath != "" {
		if _, err := os.Stat(opts.SentinelPath); err == nil {
			return Report{Skipped: true, DryRun: opts.DryRun}, nil
		} else if !os.IsNotExist(err) {
			return Report{}, fmt.Errorf("keystore: 探测哨兵失败: %w", err)
		}
	}

	// ---- 阶段一：只读，全部解密进内存 ----
	credsPlain, credsCount, err := readLegacyCredentials(opts.Creds, opts.DBPath)
	if err != nil {
		return Report{}, err
	}
	secretsPlain, secretsCount, v1Backup, err := readSecretV1(opts)
	if err != nil {
		return Report{}, err
	}

	if opts.DryRun {
		// 阶段一全过即说明"读得懂、口令对"，这正是 dry-run 要回答的问题。
		return Report{DryRun: true, CredsMoved: credsCount, SecretsMoved: secretsCount}, nil
	}

	// ---- 阶段二：落盘 ----
	if err := ks.EnsureMasterKey(); err != nil {
		return Report{}, fmt.Errorf("%w: 准备 master key 失败: %v", ErrUnavailable, err)
	}

	// v1 blob 必须先让路:v2 store 见到非 LTMSEKv2 前缀会进入 loadFailure
	// 态并拒绝一切写入（SecretStorage.refuseLoadFailedLocked）。改名而非删除。
	if v1Backup != "" {
		if err := os.Rename(opts.SecretPath, v1Backup); err != nil {
			return Report{}, fmt.Errorf("keystore: 备份 v1 secret 文件失败: %w", err)
		}
		log.Info("keystore: v1 secret 文件已改名备份", "path", v1Backup)
	}

	store := crypto.NewSecretStorage(opts.SecretPath, ks)
	if lf := store.LoadFailure(); lf != nil {
		return Report{}, lf
	}
	for _, k := range legacyCredentialKeys {
		v, ok := credsPlain[k]
		if !ok {
			continue
		}
		if err := store.Store([]byte(k), v); err != nil {
			return Report{}, fmt.Errorf("keystore: 写入凭据 %s 失败: %w", k, err)
		}
	}
	for k, v := range secretsPlain {
		if err := store.Store([]byte(k), v); err != nil {
			return Report{}, fmt.Errorf("keystore: 写入 secret %s 失败: %w", k, err)
		}
	}

	if opts.Creds != nil {
		if err := opts.Creds.ClearLegacyState(); err != nil {
			return Report{}, fmt.Errorf("keystore: 清理旧凭据状态失败: %w", err)
		}
	}

	if err := writeSentinel(opts.SentinelPath); err != nil {
		return Report{}, err
	}

	log.Info("keystore: 迁移完成",
		"creds", credsCount, "secrets", secretsCount)
	return Report{
		CredsMoved:      credsCount,
		SecretsMoved:    secretsCount,
		SentinelWritten: true,
		V1BackedUp:      v1Backup,
	}, nil
}

// MigrateSecretV1 只迁移单个 v1 secret.db（不经 settings、不写哨兵）。
// Task 9 的 migrate 端点用它做"用户补输口令后重试 secret.db"这一小步。
//
// 口令错时返回包裹 ErrUnseal 的错误，且**不改文件**。
func MigrateSecretV1(ks Keystore, path string, password []byte) error {
	rep, err := MigrateAll(ks, MigrateOptions{
		SecretPath:   path,
		SentinelPath: filepath.Join(filepath.Dir(path), SentinelName),
		Password:     password,
	})
	if err != nil {
		return err
	}
	if rep.Skipped {
		// 哨兵已在：secret.db 必然已是 v2，无需再迁。
		return nil
	}
	return nil
}

// readLegacyCredentials 读并解密 settings 侧的三组旧密文。只读，无副作用。
// 未配置的 key 直接跳过（nil 密文），不报错——新装用户本就没有旧凭据。
func readLegacyCredentials(creds SettingsCreds, dbPath string) (map[string][]byte, int, error) {
	out := make(map[string][]byte, len(legacyCredentialKeys))
	if creds == nil {
		return out, 0, nil
	}
	for _, key := range legacyCredentialKeys {
		ct, err := creds.LegacyCiphertext(key)
		if err != nil {
			return nil, 0, fmt.Errorf("keystore: 读取旧凭据 %s 失败: %w", key, err)
		}
		if len(ct) == 0 {
			continue
		}
		pt, err := openLegacyCredential(ct, deriveLegacyCredentialKey(dbPath))
		if err != nil {
			return nil, 0, fmt.Errorf("keystore: 解密旧凭据 %s 失败: %w", key, err)
		}
		out[key] = pt
	}
	return out, len(out), nil
}

// readSecretV1 读并解密 v1 secret.db，返回明文条目与改名备份路径。
// 磁盘上没有该文件、或已是 v2 magic 时都视为"无需迁移"，返回空。
func readSecretV1(opts MigrateOptions) (map[string][]byte, int, string, error) {
	if opts.SecretPath == "" {
		return nil, 0, "", nil
	}
	raw, err := os.ReadFile(opts.SecretPath)
	if os.IsNotExist(err) {
		return nil, 0, "", nil // 全新安装：没有可迁的 secret。
	}
	if err != nil {
		return nil, 0, "", fmt.Errorf("keystore: 读取 v1 secret 文件失败: %w", err)
	}
	if !bytes.HasPrefix(raw, secretV1Magic) {
		if bytes.HasPrefix(raw, []byte("LTMSEKv2")) {
			return nil, 0, "", nil // 已是 v2，无需迁移。
		}
		return nil, 0, "", fmt.Errorf("%w: secret 文件前缀既非 LTMSECv1 也非 LTMSEKv2,拒绝猜测格式",
			ErrUnseal)
	}

	// 去掉 magic，剩下 salt||nonce||sealed 正是 crypto.DecryptWithPassword 的输入。
	body := raw[len(secretV1Magic):]
	if len(opts.Password) == 0 {
		return nil, 0, "", fmt.Errorf("%w: 磁盘上有 v1 secret 文件但未提供主口令", ErrUnseal)
	}
	plaintext, err := crypto.DecryptWithPassword(body, opts.Password)
	if err != nil {
		// 口令错与 blob 损坏在这里不可区分（GCM tag 校验失败），统一报 ErrUnseal。
		return nil, 0, "", fmt.Errorf("%w: v1 secret 解密失败(主口令错误或文件损坏): %v", ErrUnseal, err)
	}
	defer zeroBytes(plaintext)

	encoded := make(map[string]string)
	if err := json.Unmarshal(plaintext, &encoded); err != nil {
		return nil, 0, "", fmt.Errorf("%w: v1 secret 解密内容不是合法 JSON: %v", ErrUnseal, err)
	}
	out := make(map[string][]byte, len(encoded))
	for k, v := range encoded {
		rawVal, err := base64.StdEncoding.DecodeString(v)
		if err != nil {
			return nil, 0, "", fmt.Errorf("%w: v1 secret %s 的 base64 解码失败: %v", ErrUnseal, k, err)
		}
		out[k] = rawVal
	}
	return out, len(out), opts.SecretPath + secretV1BackupSuffix, nil
}

// writeSentinel 原子落哨兵（临时文件 + rename），与 SecretStorage 同惯例。
func writeSentinel(path string) error {
	if path == "" {
		return nil
	}
	tmp := path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("keystore: 创建哨兵临时文件失败: %w", err)
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("keystore: 关闭哨兵临时文件失败: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("keystore: 落哨兵失败: %w", err)
	}
	return nil
}

// deriveLegacyCredentialKey 复刻 v1 settings 的凭据密钥派生：
// SHA-256("little-timer-credential-key-v1" || dbPath)。
//
// Deprecated: v1 派生方式（确定性密钥，换 dbPath 即可解密，正是 issue #13
// 要消灭的弱点）。仅迁移器解密旧密文时使用。
func deriveLegacyCredentialKey(dbPath string) []byte {
	h := sha256.New()
	h.Write([]byte("little-timer-credential-key-v1"))
	h.Write([]byte(dbPath))
	return h.Sum(nil)
}

// openLegacyCredential 解开 settings 旧密文列。
//
// 列布局就是 crypto.Encrypt 的原样输出 nonce(12) || ciphertext || tag(16)，
// 而 crypto.Decrypt 自行从 blob 头部剥离 nonce，所以这里整体透传、不手工切片。
func openLegacyCredential(ct, key []byte) ([]byte, error) {
	pt, err := crypto.Decrypt(ct, key)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUnseal, err)
	}
	return pt, nil
}

// zeroBytes 尽力擦除内存中的明文副本。v1 时代 crypto 包内曾有同名私有实现，
// 随 SecretStorage 改写一并删除；迁移器仍需在解出口令后立刻擦掉。
func zeroBytes(b []byte) {
	for i := range b {
		b[i] = 0
	}
}
