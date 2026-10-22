package keystore

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"little-timer/internal/crypto"
)

// ---- fixture 支撑 ----

// testKeystore 是包内最小内存 keystore。不能用 internal/keystore/keystoretest
// ——它 import 本包，在包内测试里会形成导入环。
type testKeystore struct{ hasKey bool }

func newTestKeystore() *testKeystore { return &testKeystore{hasKey: true} }

func (k *testKeystore) Name() string                { return "test" }
func (k *testKeystore) Available() error            { return nil }
func (k *testKeystore) EnsureMasterKey() error      { k.hasKey = true; return nil }
func (k *testKeystore) HasMasterKey() (bool, error) { return k.hasKey, nil }
func (k *testKeystore) DeleteMasterKey() error      { k.hasKey = false; return nil }

func (k *testKeystore) Seal(pt []byte) ([]byte, error) {
	if !k.hasKey {
		return nil, ErrNotFound
	}
	return append([]byte("k:"), pt...), nil
}

func (k *testKeystore) Unseal(blob []byte) ([]byte, error) {
	if !k.hasKey {
		return nil, ErrNotFound
	}
	if len(blob) < 2 || blob[0] != 'k' || blob[1] != ':' {
		return nil, ErrUnseal
	}
	return append([]byte(nil), blob[2:]...), nil
}

// writeSecretV1 用 v1 wire 造一个 secret.db：
// "LTMSECv1"(8) || salt(16) || nonce(12) || sealed。
// crypto.EncryptWithPassword 返回的正是 salt||nonce||sealed，前缀 magic 由本
// helper 补齐——与 v1 时代 persistLocked 的落盘布局一致。
func writeSecretV1(t *testing.T, path string, password string, entries map[string]string) {
	t.Helper()
	plaintext, err := json.Marshal(entries)
	if err != nil {
		t.Fatalf("marshal entries: %v", err)
	}
	sealed, err := crypto.EncryptWithPassword(plaintext, []byte(password))
	if err != nil {
		t.Fatalf("EncryptWithPassword: %v", err)
	}
	out := append([]byte("LTMSECv1"), sealed...)
	if err := os.WriteFile(path, out, 0o600); err != nil {
		t.Fatalf("write v1 secret: %v", err)
	}
}

// const testDBPath 是测试用的“旧 settings DB 路径”——旧凭据密钥由它派生，
// fixture 与 MigrateAll 必须用同一个值。
const testDBPath = "/fake/dbpath"

// fakeCreds 是 SettingsCreds 的测试替身：旧密文存内存，明文经 PutSecret 落进
// 真实的 SecretStorage v2（fake keystore 驱动，磁盘 envelope 真跑一遍）。
//
// store 刻意**惰性**构造：v1 文件还在盘上时就 new SecretStorage 会因 magic 不
// 匹配进入 loadFailure 态并拒绝一切写入——那是迁移器要处理的真实处境，
// 测试替身不能提前绕过它。
type fakeCreds struct {
	legacy   map[string][]byte
	path     string
	ks       Keystore
	store    *crypto.SecretStorage
	cleared  bool
	clearCnt int
}

func newFakeCreds(secretPath string, ks Keystore) *fakeCreds {
	return &fakeCreds{legacy: map[string][]byte{}, path: secretPath, ks: ks}
}

func (f *fakeCreds) LegacyCiphertext(key string) ([]byte, error) {
	return f.legacy[key], nil
}

// PutSecret 复刻 App.ensureSecrets 的惰性构造：首次写才建 store。
func (f *fakeCreds) PutSecret(key string, value []byte) error {
	if f.store == nil {
		f.store = crypto.NewSecretStorage(f.path, f.ks)
	}
	return f.store.Store([]byte(key), value)
}

func (f *fakeCreds) ClearLegacyState() error {
	f.cleared = true
	f.clearCnt++
	return nil
}

// reopen 造一个全新的 store（模拟重启），用来验证"迁移后的磁盘状态真的可读"
// 而不是复用迁移过程中那个已被污染的 cache。
func (f *fakeCreds) reopen() *crypto.SecretStorage {
	return crypto.NewSecretStorage(f.path, f.ks)
}

// ---- 测试 ----

// TestMigrateAll_FullFlow 走通完整迁移：settings 旧密文列（免口令）与
// secret.db v1（需口令）都搬进 keystore，写下 sentinel，抹掉旧状态标记。
func TestMigrateAll_FullFlow(t *testing.T) {
	dir := t.TempDir()
	secretPath := filepath.Join(dir, "secret.db")
	sentinelPath := filepath.Join(dir, SentinelName)
	writeSecretV1(t, secretPath, "old-pass", map[string]string{
		"llm_api_key": base64.StdEncoding.EncodeToString([]byte("sk-legacy-key")),
	})

	ks := newTestKeystore()
	creds := newFakeCreds(secretPath, ks)
	// settings 侧旧密文：照 deriveCredentialKey 公式派生 key 再 AES-GCM。
	creds.legacy["webdav_password"] = mustSeal(t, testDBPath, []byte("webdav-secret"))

	rep, err := MigrateAll(ks, MigrateOptions{
		DBPath:       testDBPath,
		SecretPath:   secretPath,
		SentinelPath: sentinelPath,
		Password:     []byte("old-pass"),
		Creds:        creds,
	})
	if err != nil {
		t.Fatalf("MigrateAll: %v", err)
	}

	// 迁移后 secretPath 上必须是 v2 wire（magic 换成 LTMSEKv2），v1 blob 消失。
	raw, err := os.ReadFile(secretPath)
	if err != nil {
		t.Fatalf("read secret after migrate: %v", err)
	}
	if string(raw[:8]) == "LTMSECv1" {
		t.Error("secret.db 仍是 v1 wire，迁移未覆盖写")
	}
	if string(raw[:8]) != "LTMSEKv2" {
		t.Errorf("secret.db magic = %q, want LTMSEKv2", raw[:8])
	}

	// 明文必须能经新 v2 store 取回。
	after := creds.reopen()
	if lf := after.LoadFailure(); lf != nil {
		t.Fatalf("迁移后的 store 处于载入失败态: %v", lf)
	}
	if got, err := after.Retrieve([]byte("llm_api_key")); err != nil || string(got) != "sk-legacy-key" {
		t.Errorf("llm_api_key = %q, err = %v; want sk-legacy-key", got, err)
	}
	if got, err := after.Retrieve([]byte("webdav_password")); err != nil || string(got) != "webdav-secret" {
		t.Errorf("webdav_password = %q, err = %v; want webdav-secret", got, err)
	}

	if !rep.SentinelWritten {
		t.Error("SentinelWritten = false, want true")
	}
	if _, err := os.Stat(sentinelPath); err != nil {
		t.Errorf("sentinel 未落盘: %v", err)
	}
	if !creds.cleared {
		t.Error("旧状态标记未清理")
	}
}

// TestMigrate_SentinelSkips 覆盖 Review Focus #1：迁移半途崩溃后重入。
// sentinel 存在时必须零副作用——不重复解密、不重复写 blob、不重复清标记。
// 同时覆盖 Review Focus #3 的残留态：即使 has_master_password 之类旧标记
// 仍是 true，二次调用也不该再去碰它们（否则会把迁移重跑一遍）。
func TestMigrate_SentinelSkips(t *testing.T) {
	dir := t.TempDir()
	secretPath := filepath.Join(dir, "secret.db")
	sentinelPath := filepath.Join(dir, SentinelName)
	writeSecretV1(t, secretPath, "old-pass", map[string]string{
		"llm_api_key": base64.StdEncoding.EncodeToString([]byte("sk-legacy-key")),
	})
	// 预置 sentinel：模拟"上次迁移已写完哨兵"。
	if err := os.WriteFile(sentinelPath, nil, 0o600); err != nil {
		t.Fatalf("seed sentinel: %v", err)
	}
	before, err := os.ReadFile(secretPath)
	if err != nil {
		t.Fatalf("read before: %v", err)
	}

	ks := newTestKeystore()
	creds := newFakeCreds(secretPath, ks)
	creds.legacy["webdav_password"] = mustSeal(t, testDBPath, []byte("webdav-secret"))

	// 口令故意给错：若真去解密就会失败，从而证明它根本没走解密路径。
	rep, err := MigrateAll(ks, MigrateOptions{
		DBPath:       testDBPath,
		SecretPath:   secretPath,
		SentinelPath: sentinelPath,
		Password:     []byte("WRONG"),
		Creds:        creds,
	})
	if err != nil {
		t.Fatalf("sentinel 存在时应直接返回而非报错: %v", err)
	}
	if !rep.Skipped {
		t.Error("Skipped = false, want true")
	}

	after, err := os.ReadFile(secretPath)
	if err != nil {
		t.Fatalf("read after: %v", err)
	}
	if string(before) != string(after) {
		t.Error("secret.db 被改写，sentinel 命中路径必须零副作用")
	}
	if creds.cleared {
		t.Error("旧标记被重复清理，sentinel 命中路径必须零副作用")
	}
}

// TestMigrate_V1Reentry_KeystoreKeyAlreadyThere 覆盖 Review Focus #1 的另一半：
// keystore 里 master key 已存在（用户先前在别的机器/别的入口建过）时再迁移，
// 必须仍然成功——EnsureMasterKey 是幂等的，不能因为"已有 key"就报错或跳过。
func TestMigrate_V1Reentry_KeystoreKeyAlreadyThere(t *testing.T) {
	dir := t.TempDir()
	secretPath := filepath.Join(dir, "secret.db")
	writeSecretV1(t, secretPath, "old-pass", map[string]string{
		"llm_api_key": base64.StdEncoding.EncodeToString([]byte("sk-legacy-key")),
	})

	ks := newTestKeystore()
	if err := ks.EnsureMasterKey(); err != nil {
		t.Fatalf("EnsureMasterKey: %v", err)
	}
	has, err := ks.HasMasterKey()
	if err != nil || !has {
		t.Fatalf("HasMasterKey = %v, %v; want true", has, err)
	}

	creds := newFakeCreds(secretPath, ks)
	if _, err := MigrateAll(ks, MigrateOptions{
		DBPath:       testDBPath,
		SecretPath:   secretPath,
		SentinelPath: filepath.Join(dir, SentinelName),
		Password:     []byte("old-pass"),
		Creds:        creds,
	}); err != nil {
		t.Fatalf("master key 已存在时迁移仍应成功: %v", err)
	}
	if got, err := creds.reopen().Retrieve([]byte("llm_api_key")); err != nil || string(got) != "sk-legacy-key" {
		t.Errorf("llm_api_key = %q, err = %v; want sk-legacy-key", got, err)
	}
}

// TestMigrate_WrongPassword 口令错必须返回包裹 ErrUnseal 的错误，且**文件
// 原样不动**——迁移是破坏性操作，不能在失败中途留下半份数据。
func TestMigrate_WrongPassword(t *testing.T) {
	dir := t.TempDir()
	secretPath := filepath.Join(dir, "secret.db")
	writeSecretV1(t, secretPath, "right-pass", map[string]string{
		"llm_api_key": base64.StdEncoding.EncodeToString([]byte("sk-legacy-key")),
	})
	before, err := os.ReadFile(secretPath)
	if err != nil {
		t.Fatalf("read before: %v", err)
	}

	ks := newTestKeystore()
	creds := newFakeCreds(secretPath, ks)

	_, err = MigrateAll(ks, MigrateOptions{
		DBPath:       testDBPath,
		SecretPath:   secretPath,
		SentinelPath: filepath.Join(dir, SentinelName),
		Password:     []byte("WRONG"),
		Creds:        creds,
	})
	if !errors.Is(err, ErrUnseal) {
		t.Fatalf("err = %v, want errors.Is(err, ErrUnseal)", err)
	}

	after, err := os.ReadFile(secretPath)
	if err != nil {
		t.Fatalf("read after: %v", err)
	}
	if string(before) != string(after) {
		t.Error("口令错时 secret.db 被改写，迁移必须失败即无副作用")
	}
	if _, err := os.Stat(filepath.Join(dir, SentinelName)); err == nil {
		t.Error("迁移失败却写了 sentinel")
	}
}

// TestMigrate_DryRun_NoWrites --dry-run 不产生任何写：blob 原样、无 sentinel、
// 旧标记未清。它仍需完成全部解密以证明"读路径通"，所以返回成功。
func TestMigrate_DryRun_NoWrites(t *testing.T) {
	dir := t.TempDir()
	secretPath := filepath.Join(dir, "secret.db")
	sentinelPath := filepath.Join(dir, SentinelName)
	writeSecretV1(t, secretPath, "old-pass", map[string]string{
		"llm_api_key": base64.StdEncoding.EncodeToString([]byte("sk-legacy-key")),
	})
	before, err := os.ReadFile(secretPath)
	if err != nil {
		t.Fatalf("read before: %v", err)
	}

	ks := newTestKeystore()
	creds := newFakeCreds(secretPath, ks)
	creds.legacy["webdav_password"] = mustSeal(t, testDBPath, []byte("webdav-secret"))

	rep, err := MigrateAll(ks, MigrateOptions{
		DBPath:       testDBPath,
		SecretPath:   secretPath,
		SentinelPath: sentinelPath,
		Password:     []byte("old-pass"),
		Creds:        creds,
		DryRun:       true,
	})
	if err != nil {
		t.Fatalf("dry-run 应能跑通: %v", err)
	}
	if !rep.DryRun {
		t.Error("Report.DryRun = false, want true")
	}
	if rep.SentinelWritten {
		t.Error("dry-run 不应写 sentinel")
	}

	after, err := os.ReadFile(secretPath)
	if err != nil {
		t.Fatalf("read after: %v", err)
	}
	if string(before) != string(after) {
		t.Error("dry-run 改写了 secret.db")
	}
	if _, err := os.Stat(sentinelPath); err == nil {
		t.Error("dry-run 写了 sentinel")
	}
	if creds.cleared {
		t.Error("dry-run 清理了旧标记")
	}
}

// TestMigrate_NoV1File_StillMigratesSettings 全新安装没有 v1 secret.db：
// settings 侧凭据仍要搬、sentinel 仍要写（否则每次启动都重试）。
func TestMigrate_NoV1File_StillMigratesSettings(t *testing.T) {
	dir := t.TempDir()
	secretPath := filepath.Join(dir, "secret.db")
	sentinelPath := filepath.Join(dir, SentinelName)

	ks := newTestKeystore()
	creds := newFakeCreds(secretPath, ks)
	creds.legacy["s3_access_key"] = mustSeal(t, testDBPath, []byte("AKIA-OLD"))

	rep, err := MigrateAll(ks, MigrateOptions{
		DBPath:       testDBPath,
		SecretPath:   secretPath,
		SentinelPath: sentinelPath,
		Password:     nil, // 无 v1 文件，不需要口令
		Creds:        creds,
	})
	if err != nil {
		t.Fatalf("MigrateAll: %v", err)
	}
	if got, err := creds.reopen().Retrieve([]byte("s3_access_key")); err != nil || string(got) != "AKIA-OLD" {
		t.Errorf("s3_access_key = %q, err = %v; want AKIA-OLD", got, err)
	}
	if !rep.SentinelWritten {
		t.Error("无 v1 文件也应写 sentinel")
	}
}

// mustSeal 用 settings 旧公式派生 key 做 AES-GCM，造出"旧密文列"的字节。
// dbPath 必须与 MigrateOptions.DBPath 一致——旧密钥就是按它派生的。
func mustSeal(t *testing.T, dbPath string, plaintext []byte) []byte {
	t.Helper()
	key := deriveLegacyCredentialKey(dbPath)
	// crypto.Encrypt 的输出本身就是 nonce||ct||tag，与旧列布局一致，不再手工拼 nonce。
	ct, err := crypto.Encrypt(plaintext, key, crypto.GenerateNonce())
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	return ct
}
