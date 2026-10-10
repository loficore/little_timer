// manager_secrets_test.go —— Task 4:WebDAV/S3 凭据经 crypto.SecretStorage
// (keystore envelope)托管、SQLite 只留 has_* 标记的读写分流测试。
//
// fakeKeystore 是从 internal/crypto/fake_keystore_test.go 复制的最小替身
// (跨包测试文件不可互相引用,见计划裁定):真实 SecretStorage + 替身 keystore
// 在临时目录做端到端 round-trip,不用 mock 断言。
package settings

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"little-timer/internal/crypto"
	"little-timer/internal/domain"
	"little-timer/internal/keystore"
)

// fakeMasterKeyItem 与 keystore 实现的 item 名对应。
const fakeMasterKeyItem = "master_key"

type fakeKeystore struct {
	items map[string][]byte
}

// 编译期断言:替身既满足完整 keystore.Keystore,也满足 SecretStorage 的
// 消费侧最小接口 crypto.Keystore。
var (
	_ keystore.Keystore = (*fakeKeystore)(nil)
	_ crypto.Keystore   = (*fakeKeystore)(nil)
)

func newFakeKeystore(t *testing.T) *fakeKeystore {
	t.Helper()
	f := &fakeKeystore{items: make(map[string][]byte)}
	if err := f.EnsureMasterKey(); err != nil {
		t.Fatalf("fake EnsureMasterKey: %v", err)
	}
	return f
}

func (f *fakeKeystore) Name() string { return "fake" }

func (f *fakeKeystore) Available() error { return nil }

func (f *fakeKeystore) EnsureMasterKey() error {
	if _, ok := f.items[fakeMasterKeyItem]; !ok {
		f.items[fakeMasterKeyItem] = crypto.GenerateKey()
	}
	return nil
}

func (f *fakeKeystore) HasMasterKey() (bool, error) {
	_, ok := f.items[fakeMasterKeyItem]
	return ok, nil
}

func (f *fakeKeystore) Seal(plaintext []byte) ([]byte, error) {
	key, ok := f.items[fakeMasterKeyItem]
	if !ok {
		return nil, fmt.Errorf("%w: fake master key 不存在", keystore.ErrNotFound)
	}
	return crypto.Encrypt(plaintext, key, crypto.GenerateNonce())
}

func (f *fakeKeystore) Unseal(blob []byte) ([]byte, error) {
	key, ok := f.items[fakeMasterKeyItem]
	if !ok {
		return nil, fmt.Errorf("%w: fake master key 不存在", keystore.ErrNotFound)
	}
	return crypto.Decrypt(blob, key)
}

func (f *fakeKeystore) DeleteMasterKey() error {
	delete(f.items, fakeMasterKeyItem)
	return nil
}

// secretHarness 把 fake keystore 与它托管的磁盘 secret 文件绑在一起。
// 同一 harness 内多次 newStore() 共享 master key —— 模拟进程重启后
// App 重新构造 SecretStorage 并从磁盘 Unseal 加载。
type secretHarness struct {
	ks       *fakeKeystore
	secretFP string
	store    *crypto.SecretStorage
}

func newSecretHarness(t *testing.T, dir string) *secretHarness {
	t.Helper()
	h := &secretHarness{
		ks:       newFakeKeystore(t),
		secretFP: filepath.Join(dir, "secret.db"),
	}
	h.store = crypto.NewSecretStorage(h.secretFP, h.ks)
	return h
}

// reloadStore 用同一 fake keystore + 同一磁盘路径新建 SecretStorage,
// 模拟重启加载(真实验证 envelope 的落盘/解密链路)。
func (h *secretHarness) reloadStore() *crypto.SecretStorage {
	return crypto.NewSecretStorage(h.secretFP, h.ks)
}

// newWiredTestManager 构造接好 SecretStorage 的 manager(测试专用)。
func newWiredTestManager(t *testing.T) (*SettingsManager, *secretHarness) {
	t.Helper()
	dir := t.TempDir()
	mgr, err := New(filepath.Join(dir, "settings.db"))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { _ = mgr.Close() })
	h := newSecretHarness(t, dir)
	mgr.SetSecretStore(h.store)
	return mgr, h
}

// queryBackupRowFlags 直接读 backup_config 行,验证列层面的真实状态:
// 三个 *_encrypted 列必须是 NULL,has_* 标记按期望返回。
func queryBackupRowFlags(t *testing.T, mgr *SettingsManager) (pwdEncNull, accessEncNull, secretEncNull bool, hasWebdav, hasAccess, hasSecret int64) {
	t.Helper()
	db := mgr.sqlite.DB()
	if db == nil {
		t.Fatal("sqlite not open")
	}
	err := db.QueryRow(`SELECT
	    webdav_password_encrypted IS NULL,
	    s3_access_key_encrypted IS NULL,
	    s3_secret_key_encrypted IS NULL,
	    has_webdav_password, has_s3_access_key, has_s3_secret_key
	FROM backup_config WHERE id = 1;`).
		Scan(&pwdEncNull, &accessEncNull, &secretEncNull, &hasWebdav, &hasAccess, &hasSecret)
	if err != nil {
		t.Fatalf("query backup_config flags: %v", err)
	}
	return
}

// retrieveSecret 断言 store 中 key 的当前值。
func retrieveSecret(t *testing.T, store *crypto.SecretStorage, key string) string {
	t.Helper()
	v, err := store.Retrieve([]byte(key))
	if err != nil {
		t.Fatalf("Retrieve(%q): %v", key, err)
	}
	return string(v)
}

func mustUpdateBackupJSON(t *testing.T, mgr *SettingsManager, fields map[string]any) {
	t.Helper()
	payload, err := json.Marshal(fields)
	if err != nil {
		t.Fatalf("marshal fields: %v", err)
	}
	if err := mgr.UpdateBackupConfigFromJSON(string(payload)); err != nil {
		t.Fatalf("UpdateBackupConfigFromJSON: %v", err)
	}
}

// --- Step 1 断言(计划 Task 4 Step 1)-----------------------------------

// Save 后:webdav_password_encrypted 写 NULL、has_webdav_password=1、
// Secrets 里 key "webdav_password" 值正确。三个凭据字段同构。
func TestSaveFansCredentialsToSecretStoreAndNullsEncryptedColumns(t *testing.T) {
	mgr, h := newWiredTestManager(t)

	mustUpdateBackupJSON(t, mgr, map[string]any{
		"webdav_password": "s3cret-pwd",
		"s3_access_key":   "AKIA-CANARY",
		"s3_secret_key":   "secret-CANARY",
	})

	pwdNull, accessNull, secretNull, hasWebdav, hasAccess, hasSecret := queryBackupRowFlags(t, mgr)
	if !pwdNull || !accessNull || !secretNull {
		t.Error("*_encrypted 列应已停止写入(为 NULL)")
	}
	if hasWebdav != 1 || hasAccess != 1 || hasSecret != 1 {
		t.Errorf("has_* = (%d,%d,%d), want (1,1,1)", hasWebdav, hasAccess, hasSecret)
	}
	if got := retrieveSecret(t, h.store, SecretWebDAVPassword); got != "s3cret-pwd" {
		t.Errorf("secret %s = %q", SecretWebDAVPassword, got)
	}
	if got := retrieveSecret(t, h.store, SecretS3AccessKey); got != "AKIA-CANARY" {
		t.Errorf("secret %s = %q", SecretS3AccessKey, got)
	}
	if got := retrieveSecret(t, h.store, SecretS3SecretKey); got != "secret-CANARY" {
		t.Errorf("secret %s = %q", SecretS3SecretKey, got)
	}

	// secret key 常量字面量必须是 spec §6 约定的三个名字。
	if SecretWebDAVPassword != "webdav_password" ||
		SecretS3AccessKey != "s3_access_key" ||
		SecretS3SecretKey != "s3_secret_key" {
		t.Error("secret key 常量与约定不符")
	}
}

// 凭据明文绝不能出现在 SQLite 文件字节里(替代旧的加密列 canary 测试)。
func TestNoCredentialPlaintextInSQLiteFile(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "settings.db")
	mgr, err := New(dbPath)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer mgr.Close()
	h := newSecretHarness(t, dir)
	mgr.SetSecretStore(h.store)

	mustUpdateBackupJSON(t, mgr, map[string]any{
		"s3_access_key":   "AKIA-PLAINTEXT-CANARY",
		"s3_secret_key":   "secret-PLAINTEXT-CANARY",
		"webdav_password": "pwd-PLAINTEXT-CANARY",
	})

	data, err := os.ReadFile(dbPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	for _, canary := range []string{"AKIA-PLAINTEXT-CANARY", "secret-PLAINTEXT-CANARY", "pwd-PLAINTEXT-CANARY"} {
		if strings.Contains(string(data), canary) {
			t.Errorf("凭据明文泄漏进 SQLite 文件: %s", canary)
		}
	}
}

// Load 反向:Secrets 有值 → BackupConfig 明文填充。跨 manager/SecretStorage
// 实例(模拟重启,磁盘 envelope 真解密)。
func TestLoadFillsCredentialsFromSecretStore(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "settings.db")

	mgr1, err := New(dbPath)
	if err != nil {
		t.Fatalf("New #1: %v", err)
	}
	h := newSecretHarness(t, dir)
	mgr1.SetSecretStore(h.store)
	mustUpdateBackupJSON(t, mgr1, map[string]any{
		"webdav_password": "s3cret-pwd",
		"s3_access_key":   "AKIA-ROTATE",
	})
	if err := mgr1.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	mgr2, err := New(dbPath)
	if err != nil {
		t.Fatalf("New #2: %v", err)
	}
	defer mgr2.Close()
	// 模拟重启:同一个 fake keystore + 同一个 secret 文件重建 store,
	// 接进 manager 后 Load 应把明文拉回来。
	mgr2.SetSecretStore(h.reloadStore())
	if err := mgr2.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}

	bc := mgr2.BackupConfig()
	if bc.WebDAVPassword != "s3cret-pwd" {
		t.Errorf("WebDAVPassword = %q, want 明文回填", bc.WebDAVPassword)
	}
	if bc.S3AccessKey != "AKIA-ROTATE" {
		t.Errorf("S3AccessKey = %q, want 明文回填", bc.S3AccessKey)
	}
	if bc.S3SecretKey != "" {
		t.Errorf("从未存过的 S3SecretKey = %q, want 空", bc.S3SecretKey)
	}
}

// 边界:has_*=1 但 Secrets 里对应 key 缺失 → 空串 + 不报错(其余字段照常)。
func TestLoadWithHasFlagButMissingSecretIsEmptyNoError(t *testing.T) {
	mgr, h := newWiredTestManager(t)
	mustUpdateBackupJSON(t, mgr, map[string]any{
		"webdav_password": "s3cret-pwd",
		"s3_access_key":   "AKIA-KEEP",
	})

	// 直接从 Secrets 抹掉 webdav_password,SQLite 的 has_webdav_password 仍为 1。
	if err := h.store.Delete([]byte(SecretWebDAVPassword)); err != nil {
		t.Fatalf("store.Delete: %v", err)
	}
	_, _, _, hasWebdav, _, _ := queryBackupRowFlags(t, mgr)
	if hasWebdav != 1 {
		t.Fatalf("前置条件:has_webdav_password 应为 1,得 %d", hasWebdav)
	}

	if err := mgr.Load(); err != nil {
		t.Fatalf("Load 不应报错,得 %v", err)
	}
	bc := mgr.BackupConfig()
	if bc.WebDAVPassword != "" {
		t.Errorf("缺失 secret 应降级为空串,得 %q", bc.WebDAVPassword)
	}
	if bc.S3AccessKey != "AKIA-KEEP" {
		t.Errorf("其余凭据不受影响, S3AccessKey = %q", bc.S3AccessKey)
	}
}

// Save 边界:值改为空 → Secrets 里 Delete + has_*=0。
func TestSaveEmptyCredentialDeletesSecretAndClearsFlag(t *testing.T) {
	mgr, h := newWiredTestManager(t)
	mustUpdateBackupJSON(t, mgr, map[string]any{"webdav_password": "s3cret-pwd"})

	mustUpdateBackupJSON(t, mgr, map[string]any{"webdav_password": ""})

	_, _, _, hasWebdav, _, _ := queryBackupRowFlags(t, mgr)
	if hasWebdav != 0 {
		t.Errorf("has_webdav_password = %d, want 0", hasWebdav)
	}
	if _, err := h.store.Retrieve([]byte(SecretWebDAVPassword)); !errors.Is(err, crypto.ErrSecretNotFound) {
		t.Errorf("Retrieve 应返回 ErrSecretNotFound,得 %v", err)
	}
}

// SetSecretStore(nil) 后任何用户触发的保存都报 settings: secret store not
// wired(防止漏接);接回后保存恢复正常。
func TestSaveWithoutSecretStoreWiredFails(t *testing.T) {
	dir := t.TempDir()
	mgr, err := New(filepath.Join(dir, "settings.db"))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer mgr.Close()

	err = mgr.Save()
	if err == nil || err.Error() != "settings: secret store not wired" {
		t.Fatalf("未接 store 的 Save 应报 settings: secret store not wired,得 %v", err)
	}
	if err := mgr.UpdateBackupConfigFromJSON(`{"webdav_password":"x"}`); err == nil {
		t.Fatal("未接 store 的 UpdateBackupConfigFromJSON 应报错")
	}

	h := newSecretHarness(t, dir)
	mgr.SetSecretStore(h.store)
	if err := mgr.Save(); err != nil {
		t.Fatalf("接回后 Save: %v", err)
	}

	mgr.SetSecretStore(nil)
	if err := mgr.Save(); err == nil || err.Error() != "settings: secret store not wired" {
		t.Fatalf("SetSecretStore(nil) 后 Save 应报 not wired,得 %v", err)
	}
}

// 构造路径不依赖 SecretStorage(setter 注入在构造之后),且 loadAll 恢复
// 分支的播种 initializeDefaultSettings 在未接线时也要能落默认行(凭据全
// 空、has_*=0、*_encrypted NULL)。播种成功后,用户触发的 Save 依旧要求
// 接线。
func TestNewAndSeedDefaultsWithoutSecretStore(t *testing.T) {
	dir := t.TempDir()
	mgr, err := New(filepath.Join(dir, "settings.db"))
	if err != nil {
		t.Fatalf("未接 store 的 New 不应失败: %v", err)
	}
	defer mgr.Close()

	if bc := mgr.BackupConfig(); bc.WebDAVPassword != "" || bc.S3AccessKey != "" || bc.S3SecretKey != "" {
		t.Errorf("默认 BackupConfig 凭据应为空: %+v", bc)
	}

	if err := mgr.initializeDefaultSettings(domain.NewDefaultSettingsConfig()); err != nil {
		t.Fatalf("未接 store 的播种不应失败: %v", err)
	}
	pwdNull, accessNull, secretNull, hasWebdav, hasAccess, hasSecret := queryBackupRowFlags(t, mgr)
	if !pwdNull || !accessNull || !secretNull {
		t.Error("播种行的 *_encrypted 列应为 NULL")
	}
	if hasWebdav != 0 || hasAccess != 0 || hasSecret != 0 {
		t.Errorf("播种行 has_* = (%d,%d,%d), want (0,0,0)", hasWebdav, hasAccess, hasSecret)
	}

	if err := mgr.Save(); err == nil || err.Error() != "settings: secret store not wired" {
		t.Fatalf("播种后未接 store 的 Save 应报 not wired,得 %v", err)
	}
}

// 三处同步之"列补齐":backup_config 的 CREATE(storage/migration.go)本
// 期不加列,manager 打开时幂等 ALTER 补三列;重复打开(老库路径)不报错。
func TestHasCredentialColumnsAreEnsuredIdempotently(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "settings.db")

	mgr, err := New(dbPath)
	if err != nil {
		t.Fatalf("New #1: %v", err)
	}
	if err := mgr.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// 重开:列已存在,ensure 步骤必须 no-op 而非报 duplicate column。
	mgr2, err := New(dbPath)
	if err != nil {
		t.Fatalf("New #2(列已存在): %v", err)
	}
	defer mgr2.Close()

	rows, err := mgr2.sqlite.DB().Query(`PRAGMA table_info(backup_config);`)
	if err != nil {
		t.Fatalf("PRAGMA: %v", err)
	}
	defer rows.Close()
	found := map[string]bool{}
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dflt any
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			t.Fatalf("scan PRAGMA: %v", err)
		}
		found[name] = true
	}
	for _, col := range []string{"has_webdav_password", "has_s3_access_key", "has_s3_secret_key"} {
		if !found[col] {
			t.Errorf("backup_config 缺少列 %s", col)
		}
	}
}
