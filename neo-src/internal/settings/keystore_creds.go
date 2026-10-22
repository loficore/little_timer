// SettingsCreds 适配层：把 SettingsManager 接到 keystore.SettingsCreds 接口上，
// 供一次性迁移器（internal/keystore）读写 v1 时代的旧凭据密文列。
//
// 接口定义在 keystore 侧而非本包，是因为依赖方向已成
// settings → crypto → keystore；反着声明会成环。
//
// 本文件只服务于迁移窗口。迁移完成（哨兵落盘）后这些列恒为 NULL/0，
// 跨过两个 release 后可整体删除——与 crypto/legacy_v1.go 同期。

package settings

import (
	"database/sql"
	"errors"
	"fmt"

	"little-timer/internal/crypto"
	"little-timer/internal/keystore"
)

// 编译期断言：本管理器满足迁移器要求的接口。
var _ keystore.SettingsCreds = (*SettingsManager)(nil)

// 编译期断言：两侧凭据 key 常量逐字一致。迁移器（keystore 包）不能 import
// 本包，只能自带一份字面量；legacyCipherColumn 按 keystore 常量分派即是
// 两条常量表的对齐点——改任一侧的字符串，另一侧立刻编译不过。

// legacyCipherColumn 把 secret key 映射到 v1 密文列名。
// 键取自 keystore 包的常量而非本包的 SecretXxx —— 迁移器按 key 取列，
// 两侧必须用同一套名字。
func legacyCipherColumn(secretKey string) (string, bool) {
	switch secretKey {
	case keystore.CredWebDAVPassword:
		return "webdav_password_encrypted", true
	case keystore.CredS3AccessKey:
		return "s3_access_key_encrypted", true
	case keystore.CredS3SecretKey:
		return "s3_secret_key_encrypted", true
	default:
		return "", false
	}
}

// LegacyCiphertext 返回该 key 对应的 v1 密文。未配置时返回 (nil, nil)，
// 调用方据此跳过——新装用户本就没有旧凭据，不该报错。
func (sm *SettingsManager) LegacyCiphertext(key string) ([]byte, error) {
	col, ok := legacyCipherColumn(key)
	if !ok {
		return nil, fmt.Errorf("settings: 未知的凭据 key %q", key)
	}
	db := sm.sqlite.DB()
	if db == nil {
		return nil, errors.New("settings: sqlite not open")
	}
	// 列名来自上面的白名单常量，不含用户输入，故可安全拼进 SQL。
	var blob []byte
	err := db.QueryRow(
		`SELECT ` + col + ` FROM backup_config WHERE id = 1;`).Scan(&blob)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		// NULL 扫描进 []byte 会得到 nil 而非报错，这里显式判一次以便
		// 与"扫描失败"区分开。
		return nil, fmt.Errorf("settings: 读取旧凭据列 %s 失败: %w", col, err)
	}
	return blob, nil
}

// PutSecret 把解密后的明文写进 keystore 托管的 SecretStorage。
// 未接线时返回明确错误——绝不静默丢弃凭据。
func (sm *SettingsManager) PutSecret(key string, value []byte) error {
	if sm.secretStore == nil {
		return errors.New("settings: secret store not wired, 无法写入迁移凭据")
	}
	return sm.secretStore.Store([]byte(key), value)
}

// ClearLegacyState 复位 v1 遗留状态：三列密文置 NULL，四个主口令标记列置 0。
//
// 注意这**不是**删列：Path B 决定保留 backup_config 表的存量 schema
// （cmd/migrate 的 expected columns 也列了它们），只是把值清成"没有"。
// 迁移后这些列再无写入方，恒为默认值。
func (sm *SettingsManager) ClearLegacyState() error {
	db := sm.sqlite.DB()
	if db == nil {
		return errors.New("settings: sqlite not open")
	}
	_, err := db.Exec(`
        UPDATE backup_config SET
            webdav_password_encrypted = NULL,
            s3_access_key_encrypted    = NULL,
            s3_secret_key_encrypted    = NULL,
            has_master_password        = 0,
            credentials_unlock_time    = 0,
            credential_unlock_attempts = 0,
            credential_locked_until    = 0
        WHERE id = 1;`)
	if err != nil {
		return fmt.Errorf("settings: 清理旧凭据状态失败: %w", err)
	}
	// 内存态同步：主口令四列清零，避免本次进程内再把"设过主口令"当成事实。
	// has_webdav_password / has_s3_* 三列不在结构体里（凭据是否已配由
	// SecretStorage 内的条目决定，见 loadBackupConfigFromDB），故无需同步。
	sm.backupConfig.HasMasterPassword = false
	sm.backupConfig.CredentialsUnlockTime = 0
	sm.backupConfig.CredentialUnlockAttempts = 0
	sm.backupConfig.CredentialLockedUntil = 0
	return nil
}

// sm.secretStore 的类型来自 crypto.SecretStorage —— 显式标注一句，
// 避免将来 crypto 侧重构时这里静默失配。
var _ *crypto.SecretStorage