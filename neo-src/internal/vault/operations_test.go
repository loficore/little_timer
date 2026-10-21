package vault

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"little-timer/internal/crypto"
	"little-timer/internal/keystore/keystoretest"
	"little-timer/internal/settings"
	"little-timer/internal/storage"
	"little-timer/internal/storage/backup"
)

// fakeRuntime 满足 BackupRuntime。secrets 是 *App 惰性构造的凭据存储在
// 测试里的替身——Vault 不自己建凭据存储（去锁化后由 App.ensureSecrets 单点
// 接线），所以这里用 fakeRuntime 顶替那个入口。
type fakeRuntime struct {
	manager *backup.BackupManager
	secrets *crypto.SecretStorage
}

func (f fakeRuntime) Manager() *backup.BackupManager {
	if f.manager != nil {
		return f.manager
	}
	// gate 测试需要非 nil manager 才能走到 requireCloudCredentials。
	// CreateBackup 在 gate 返错时不会调 mgr.CreateBackup，dummy 安全。
	return &backup.BackupManager{}
}

func (f fakeRuntime) Rebuild(ctx context.Context) error { return nil }

// Secrets 返回 fakeRuntime 持有的凭据存储；nil 表示未接线
// （真实 App 侧 ensureSecrets 会保证非 nil，见 TestUpdateBackupConfig_*）。
func (f fakeRuntime) Secrets() *crypto.SecretStorage { return f.secrets }

// newTestVault 构造一个带临时 SQLite + 真 SettingsManager + fake Runtime 的 Vault。
// 凭据 store 用内存 fake keystore 接线，使 WebDAV/S3 凭据真能经 secrets 往返
// （否则 Save 会因 store 未接线而丢弃凭据，见 settings 的 ErrSecretStoreNotWired）。
func newTestVault(t *testing.T) *Operations {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	sqlite := storage.NewSqliteManager().Init(dbPath)
	if err := sqlite.Open(); err != nil {
		t.Fatalf("sqlite open: %v", err)
	}
	t.Cleanup(func() { _ = sqlite.Close() })
	if err := sqlite.Migrate(); err != nil {
		t.Fatalf("sqlite migrate: %v", err)
	}
	sm, err := settings.NewFromSqliteManager(sqlite, dbPath)
	if err != nil {
		t.Fatalf("settings: %v", err)
	}
	store := crypto.NewSecretStorage(
		filepath.Join(filepath.Dir(dbPath), "secret.db"),
		keystoretest.New("secret-service", true),
	)
	sm.SetSecretStore(store)
	return New(sm, fakeRuntime{manager: nil, secrets: store})
}

// ---- 凭据存储接线 ----

// TestUpdateBackupConfig_TriggersSecretsWiring 回归：UpdateBackupConfig 在写
// 配置前必须先触达 runtime.Secrets()，否则未接线的 SettingsManager 会把
// WebDAV/S3 凭据当普通字段丢掉。这里用一个"接线前 / 接线后"两段式 runtime
// 证明 vault 主动调了 Secrets()。
func TestUpdateBackupConfig_TriggersSecretsWiring(t *testing.T) {
	v := newTestVault(t)
	// 拆掉 store，模拟"App 还没接线的初始态"。
	v.settings.SetSecretStore(nil)

	calls := 0
	v.runtime = &countingRuntime{inner: v.runtime, calls: &calls, settings: v.settings}

	if err := v.UpdateBackupConfig(context.Background(),
		`{"target_type":"webdav","webdav_url":"http://x","webdav_username":"u","webdav_password":"p"}`); err != nil {
		t.Fatalf("UpdateBackupConfig: %v", err)
	}
	if calls != 1 {
		t.Errorf("runtime.Secrets() 调用 %d 次, want 1（写配置前必须先接线）", calls)
	}
	// 接线后凭据必须真的落到 secret store（而非被当普通字段丢弃）。
	if got := v.settings.BackupConfig().WebDAVPassword; got != "p" {
		t.Errorf("WebDAVPassword = %q, want p", got)
	}
}

// countingRuntime 记录 Secrets() 被调了几次后转发给内层。它同时镜像真实
// App.ensureSecrets 的副作用——除了返回 store，还把 store 接线到 SettingsManager；
// 只计数不接线就不是忠实的替身（Vault 正是靠这个副作用保证凭据不被丢弃）。
type countingRuntime struct {
	inner    BackupRuntime
	settings *settings.SettingsManager
	calls    *int
}

func (c countingRuntime) Manager() *backup.BackupManager { return c.inner.Manager() }
func (c countingRuntime) Rebuild(ctx context.Context) error {
	return c.inner.Rebuild(ctx)
}
func (c countingRuntime) Secrets() *crypto.SecretStorage {
	*c.calls++
	store := c.inner.Secrets()
	if store != nil && c.settings != nil {
		c.settings.SetSecretStore(store)
	}
	return store
}

// ---- 凭据闸门 ----

func TestCreateBackup_LocalTarget_BypassesGate(t *testing.T) {
	v := newTestVault(t)
	_, err := v.CreateBackup()
	// default cfg: target=local, Enabled=false → gate 跳过，enabled 返错
	if !errors.Is(err, ErrBackupNotEnabled) {
		t.Fatalf("err = %v, want ErrBackupNotEnabled", err)
	}
}

// TestCreateBackup_CloudTarget_NoCreds 覆盖去锁化后的闸门：云端目标没配凭据
// 就返 credentials_not_available，且不再有 Mode（原本用于区分"弹 setup 还是
// unlock modal"——主口令删除后两者合并为一种提示）。
func TestCreateBackup_CloudTarget_NoCreds(t *testing.T) {
	v := newTestVault(t)
	if err := v.settings.UpdateBackupConfigFromJSON(`{"target_type":"webdav","enabled":true}`); err != nil {
		t.Fatalf("set cfg: %v", err)
	}
	_, err := v.CreateBackup()
	var ve *Error
	if !errors.As(err, &ve) {
		t.Fatalf("err = %v, not *Error", err)
	}
	if ve.Code != "credentials_not_available" {
		t.Errorf("Code = %q, want credentials_not_available", ve.Code)
	}
	if ve.Mode != "" {
		t.Errorf("Mode = %q, want 空（去锁化后无 setup/unlock 之分）", ve.Mode)
	}
}

// TestCreateBackup_CloudTarget_S3PartialCreds 半套凭据同样拦下（缺 secret key）。
func TestCreateBackup_CloudTarget_S3PartialCreds(t *testing.T) {
	v := newTestVault(t)
	if err := v.settings.UpdateBackupConfigFromJSON(
		`{"target_type":"s3","enabled":true,"s3_access_key":"AKIA","s3_secret_key":""}`); err != nil {
		t.Fatalf("set cfg: %v", err)
	}
	_, err := v.CreateBackup()
	var ve *Error
	if !errors.As(err, &ve) {
		t.Fatalf("err = %v, not *Error", err)
	}
	if ve.Code != "credentials_not_available" {
		t.Errorf("Code = %q, want credentials_not_available", ve.Code)
	}
}

// TestCreateBackup_CloudTarget_HasCreds_PassesGate 配了凭据就该过闸门。
// 之后会调到 dummy manager 的 CreateBackup 而 panic，用 recover 兜住——
// 只关心"闸门没拦"。
func TestCreateBackup_CloudTarget_HasCreds_PassesGate(t *testing.T) {
	v := newTestVault(t)
	if err := v.settings.UpdateBackupConfigFromJSON(
		`{"target_type":"webdav","webdav_username":"u","webdav_password":"p","enabled":true}`); err != nil {
		t.Fatalf("set cfg: %v", err)
	}
	defer func() {
		if r := recover(); r != nil {
			t.Logf("expected panic past gate: %v", r)
		}
	}()
	_, err := v.CreateBackup()
	if err == nil {
		t.Fatal("expected panic or err; got nil")
	}
	var ve *Error
	if errors.As(err, &ve) && ve.Code == "credentials_not_available" {
		t.Error("gate 拦下了已配凭据的云端备份")
	}
}

// ---- UpdateBackupConfig：去锁化后不再有授权闸门 ----

// TestUpdateBackupConfig_CloudNoMasterPassword_Succeeds 回归（#34 曾拦这道）：
// 去锁化前切换到云端 target 需要"先设主密码"作为授权证明；凭据改由平台
// keystore 托管后保护边界是 OS 登录会话，闸门删除，带凭据的更新必须直接成功。
func TestUpdateBackupConfig_CloudNoMasterPassword_Succeeds(t *testing.T) {
	v := newTestVault(t)
	if err := v.settings.UpdateBackupConfigFromJSON(`{"target_type":"webdav","enabled":true}`); err != nil {
		t.Fatalf("set cfg: %v", err)
	}
	err := v.UpdateBackupConfig(context.Background(),
		`{"target_type":"webdav","webdav_url":"http://x","webdav_username":"u","webdav_password":"p"}`)
	if err != nil {
		t.Fatalf("带凭据的云端配置更新应放行（无主密码概念）: %v", err)
	}
	bc := v.settings.BackupConfig()
	if bc.WebDAVURL != "http://x" {
		t.Errorf("URL not updated: %q", bc.WebDAVURL)
	}
	// 凭据必须落进 keystore secrets（而不是被当普通字段丢进 SQLite）。
	if bc.WebDAVPassword != "p" {
		t.Errorf("WebDAVPassword = %q, want p（应经 secret store 往返）", bc.WebDAVPassword)
	}
}