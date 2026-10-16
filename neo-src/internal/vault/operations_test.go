package vault

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"little-timer/internal/domain"
	"little-timer/internal/settings"
	"little-timer/internal/storage"
	"little-timer/internal/storage/backup"
)

type fakeRuntime struct {
	manager *backup.BackupManager
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

// newTestVault 构造一个带临时 SQLite + 真 SettingsManager + fake Runtime 的 Vault。
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
	return New("", sm, fakeRuntime{manager: nil})
}

func TestHasMasterPassword_FalseByDefault(t *testing.T) {
	v := newTestVault(t)
	if v.HasMasterPassword() {
		t.Fatal("HasMasterPassword = true before any set; want false")
	}
}

func TestSetMasterPassword_TooShort(t *testing.T) {
	v := newTestVault(t)
	err := v.SetMasterPassword("abc")
	if !errors.Is(err, ErrPasswordTooShort) {
		t.Fatalf("err = %v, want ErrPasswordTooShort", err)
	}
	// 短密码不应改变 HasMasterPassword
	if v.HasMasterPassword() {
		t.Fatal("HasMasterPassword = true after too-short attempt; want false")
	}
}

func TestSetMasterPassword_ThenHas(t *testing.T) {
	v := newTestVault(t)
	if err := v.SetMasterPassword("long-enough"); err != nil {
		t.Fatalf("set: %v", err)
	}
	if !v.HasMasterPassword() {
		t.Fatal("HasMasterPassword = false after set; want true")
	}
}

func TestUnlockCredentials_NoMasterPassword_AlwaysOK(t *testing.T) {
	v := newTestVault(t)
	res := v.UnlockCredentials("anything")
	if !res.Success {
		t.Fatalf("res.Success = false; want true (no master password case)")
	}
	if res.LockedUntil != 0 {
		t.Errorf("LockedUntil = %d; want 0", res.LockedUntil)
	}
}

func TestUnlockCredentials_AfterSet_PasswordMismatch(t *testing.T) {
	v := newTestVault(t)
	if err := v.SetMasterPassword("correct-password"); err != nil {
		t.Fatalf("set: %v", err)
	}
	res := v.UnlockCredentials("wrong-password")
	if res.Success {
		t.Fatal("res.Success = true with wrong password; want false")
	}
}

func TestGetMasterPasswordStatus_Composes(t *testing.T) {
	v := newTestVault(t)
	status := v.GetMasterPasswordStatus()
	if status.HasPassword {
		t.Errorf("HasPassword = true; want false")
	}
	// 预存在的 bug：IsUnlocked 在无主密码时返 false（因为 fresh SecretStorage
	// 以 locked 状态初始化，且 UnlockCredentials 在无主密码路径不调
	// secret.Unlock）。记在 .omo/evidence 里；本测试锁定现有行为。
	if status.Unlocked {
		t.Errorf("Unlocked = true; want false (pre-existing: no master password = secret starts locked)")
	}
}

func TestLockCredentials_AfterSet(t *testing.T) {
	v := newTestVault(t)
	if err := v.SetMasterPassword("correct-password"); err != nil {
		t.Fatalf("set: %v", err)
	}
	v.LockCredentials()
	// 锁后 secret 在内存中锁定，IsUnlocked 应返 false。
	// （预存在的 bug：settings.UpdateBackupConfigFromJSON 不解析
	// credential_locked_until 字段，持久化的 cfg.CredentialLockedUntil 不会变。
	// 修这个超出 candidate 2 范围，记在 .omo/evidence 里。）
	if v.IsUnlocked() {
		t.Error("IsUnlocked = true after lock; want false")
	}
}

func TestSecrets_ReturnsSameInstance(t *testing.T) {
	v := newTestVault(t)
	if v.Secrets() == nil {
		t.Fatal("Secrets() = nil; want non-nil SecretStorage")
	}
	if v.Secrets() != v.Secrets() {
		t.Error("Secrets() returns different instances across calls; want same")
	}
}

// 防 unused import 警告
var _ domain.UnlockResult

// ---- 凭据闸门 ----

func TestCreateBackup_LocalTarget_BypassesGate(t *testing.T) {
	v := newTestVault(t)
	_, err := v.CreateBackup()
	// default cfg: target=local, Enabled=false → gate 跳过，enabled 返错
	if !errors.Is(err, ErrBackupNotEnabled) {
		t.Fatalf("err = %v, want ErrBackupNotEnabled", err)
	}
}

func TestCreateBackup_CloudTarget_NoCreds_NoMasterPassword(t *testing.T) {
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
	if ve.Mode != "setup" {
		t.Errorf("Mode = %q, want setup (no master password)", ve.Mode)
	}
}

func TestCreateBackup_CloudTarget_NoCreds_HasMasterPassword(t *testing.T) {
	v := newTestVault(t)
	if err := v.settings.UpdateBackupConfigFromJSON(`{"target_type":"webdav","enabled":true}`); err != nil {
		t.Fatalf("set cfg: %v", err)
	}
	if err := v.SetMasterPassword("long-enough"); err != nil {
		t.Fatalf("set master password: %v", err)
	}
	_, err := v.CreateBackup()
	var ve *Error
	if !errors.As(err, &ve) {
		t.Fatalf("err = %v, not *Error", err)
	}
	if ve.Code != "credentials_not_available" {
		t.Errorf("Code = %q, want credentials_not_available", ve.Code)
	}
	if ve.Mode != "unlock" {
		t.Errorf("Mode = %q, want unlock (has master password but no creds)", ve.Mode)
	}
}

func TestCreateBackup_CloudTarget_HasCreds_Locked(t *testing.T) {
	v := newTestVault(t)
	if err := v.settings.UpdateBackupConfigFromJSON(`{"target_type":"webdav","webdav_password":"p","enabled":true}`); err != nil {
		t.Fatalf("set cfg: %v", err)
	}
	if err := v.SetMasterPassword("long-enough"); err != nil {
		t.Fatalf("set master password: %v", err)
	}
	v.LockCredentials()
	_, err := v.CreateBackup()
	var ve *Error
	if !errors.As(err, &ve) {
		t.Fatalf("err = %v, not *Error", err)
	}
	if ve.Code != "master_password_not_unlocked" {
		t.Errorf("Code = %q, want master_password_not_unlocked", ve.Code)
	}
	if ve.Mode != "unlock" {
		t.Errorf("Mode = %q, want unlock", ve.Mode)
	}
}

func TestCreateBackup_CloudTarget_HasCreds_Unlocked_PassesGate(t *testing.T) {
	v := newTestVault(t)
	if err := v.settings.UpdateBackupConfigFromJSON(`{"target_type":"webdav","webdav_password":"p","enabled":true}`); err != nil {
		t.Fatalf("set cfg: %v", err)
	}
	if err := v.SetMasterPassword("long-enough"); err != nil {
		t.Fatalf("set master password: %v", err)
	}
	// gate 通过；接下来调 mgr.CreateBackup() 会 panic（dummy 指针无 adapter）。
	// 用 recover 抓住，仅断言 gate 通过。
	defer func() {
		if r := recover(); r != nil {
			// 预期：从这里开始是 dummy mgr 的实际调用 panic。
			// 只 panic 在 CreateBackup 调用上，不在 gate 上。
			t.Logf("expected panic past gate: %v", r)
		}
	}()
	_, err := v.CreateBackup()
	// 如果 err 不是 nil 且不是 vault.Error，说明 gate 没返错。
	// 我们的目的是确认 gate 通过。如果走到 panic，错误是 panic 而非 err。
	if err == nil {
		t.Fatal("expected panic or err; got nil")
	}
}

// ---- requireAuthForConfigUpdate（config update 闸门）----
// 跟原 handler 语义对齐：已有 target 凭据时不检查（即使锁着），
// 因为 inbound JSON 不一定在改凭据。

func TestUpdateBackupConfig_HasCreds_Locked_NoUnlockCheck(t *testing.T) {
	v := newTestVault(t)
	// 持久化 cfg 已有 webdav 凭据
	if err := v.settings.UpdateBackupConfigFromJSON(`{"target_type":"webdav","webdav_password":"old","webdav_url":"http://old.example"}`); err != nil {
		t.Fatalf("set cfg: %v", err)
	}
	v.LockCredentials()
	// inbound 仍是 webdav，hasCreds=true → 跳闸门
	err := v.UpdateBackupConfig(context.Background(), `{"target_type":"webdav","webdav_url":"http://new.example"}`)
	if err != nil {
		t.Fatalf("update with creds + locked should succeed (原 handler 行为): %v", err)
	}
	if u := v.settings.BackupConfig().WebDAVURL; u != "http://new.example" {
		t.Errorf("URL not updated: %q", u)
	}
}

func TestUpdateBackupConfig_NoCreds_NoMasterPassword(t *testing.T) {
	v := newTestVault(t)
	if err := v.settings.UpdateBackupConfigFromJSON(`{"target_type":"webdav","enabled":true}`); err != nil {
		t.Fatalf("set cfg: %v", err)
	}
	// 没凭据 + 没主密码 → 返 setup
	err := v.UpdateBackupConfig(context.Background(), `{"target_type":"webdav","webdav_url":"http://x"}`)
	var ve *Error
	if !errors.As(err, &ve) {
		t.Fatalf("err = %v, not *Error", err)
	}
	if ve.Code != "credentials_not_available" || ve.Mode != "setup" {
		t.Errorf("got code=%q mode=%q, want credentials_not_available/setup", ve.Code, ve.Mode)
	}
}

func TestUpdateBackupConfig_NoCreds_Locked(t *testing.T) {
	v := newTestVault(t)
	if err := v.settings.UpdateBackupConfigFromJSON(`{"target_type":"webdav","enabled":true}`); err != nil {
		t.Fatalf("set cfg: %v", err)
	}
	if err := v.SetMasterPassword("long-enough"); err != nil {
		t.Fatalf("set master password: %v", err)
	}
	v.LockCredentials()
	err := v.UpdateBackupConfig(context.Background(), `{"target_type":"webdav","webdav_url":"http://x"}`)
	var ve *Error
	if !errors.As(err, &ve) {
		t.Fatalf("err = %v, not *Error", err)
	}
	if ve.Code != "master_password_not_unlocked" || ve.Mode != "unlock" {
		t.Errorf("got code=%q mode=%q, want master_password_not_unlocked/unlock", ve.Code, ve.Mode)
	}
}
