package app

import (
	"path/filepath"
	"testing"

	"little-timer/internal/domain"
	"little-timer/internal/keystore/keystoretest"
	"little-timer/internal/settings"
	"little-timer/internal/storage"
	"little-timer/internal/storage/backup"
)

// wailsBackupTestApp 构造一个带真 backup manager 的 App，供 Wails backup
// service 测试用（gate 需要非 nil Manager 才能走到凭据检查）。
func wailsBackupTestApp(t *testing.T) *App {
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
	backupDir := t.TempDir()
	bm, err := backup.NewLocal(sqlite, dbPath, backupDir)
	if err != nil {
		t.Fatalf("backup manager: %v", err)
	}
	a := NewApp(domain.NewClockManager(domain.NewDefaultClockTaskConfig()), sm, sqlite, bm, dbPath)
	// 去锁化（issue #13）：SettingsManager 未接线凭据 store 时保存云端配置会直接
	// 报错（避免凭据被静默丢弃），因此测试 App 也要注入内存 keystore 并完成接线。
	a.Keystore = keystoretest.New("secret-service", true)
	_ = a.Secrets()
	return a
}

// setCloudTarget 把持久化 BackupConfig 切到 cloud target（无凭据）。
func setCloudTarget(t *testing.T, a *App, target string) {
	t.Helper()
	body := `{"enabled":true,"target_type":"` + target + `"}`
	if err := a.Settings.UpdateBackupConfigFromJSON(body); err != nil {
		t.Fatalf("set cloud target: %v", err)
	}
}

// TestWailsCreateBackup_CloudNoCreds_ReturnsMasterPasswordAction 覆盖 candidate 2
// 修的第二个 drift：Wails BackupService.CreateBackup 之前完全没凭据闸门，
// Android 端拿锁着的凭据也能起云端备份。Phase 3b 后 Wails 和 HTTP 共用
// a.Vault.CreateBackup，闸门自动同步。
func TestWailsCreateBackup_CloudNoCreds_ReturnsMasterPasswordAction(t *testing.T) {
	a := wailsBackupTestApp(t)
	setCloudTarget(t, a, "webdav")
	svc := NewBackupService(a)

	res, err := svc.CreateBackup()
	if err != nil {
		t.Fatalf("CreateBackup returned Go error: %v", err)
	}
	m, ok := res.(map[string]any)
	if !ok {
		t.Fatalf("res = %T, want map[string]any", res)
	}
	if success, _ := m["success"].(bool); success {
		t.Fatalf("success = true, want false; map = %+v", m)
	}
	if code, _ := m["code"].(string); code != "credentials_not_available" {
		t.Errorf("code = %q, want credentials_not_available", code)
	}
	// 去锁化（issue #13）后没有"弹主口令 modal"这回事：凭据由平台 keystore
	// 托管，缺凭据时错误形状退化为 success/code/message 三元组。
	if _, ok := m["action"]; ok {
		t.Errorf("action 应随主口令 UX 一并删除; map = %+v", m)
	}
}

// TestWailsRestoreBackup_CloudNoCreds_ReturnsMasterPasswordAction 覆盖
// RestoreBackup 的同类 drift。
func TestWailsRestoreBackup_CloudNoCreds_ReturnsMasterPasswordAction(t *testing.T) {
	a := wailsBackupTestApp(t)
	setCloudTarget(t, a, "s3")
	svc := NewBackupService(a)

	res, err := svc.RestoreBackup("any-name")
	if err != nil {
		t.Fatalf("RestoreBackup returned Go error: %v", err)
	}
	m, ok := res.(map[string]any)
	if !ok {
		t.Fatalf("res = %T, want map[string]any", res)
	}
	if code, _ := m["code"].(string); code != "credentials_not_available" {
		t.Errorf("code = %q, want credentials_not_available", code)
	}
	if _, ok := m["action"]; ok {
		t.Errorf("action 应随主口令 UX 一并删除; map = %+v", m)
	}
}

// TestWailsUpdateBackupConfig_CloudNoMasterPassword_Succeeds 覆盖 config update
// 路径。去锁化（issue #13）前：切到云端 target 若库里还没有该 target 的凭据，
// vault 会要求"先设主密码"作为授权证明（setup 闸门）；凭据改由平台 keystore
// 托管后保护边界是 OS 登录会话，该闸门整体删除——带凭据的更新必须直接成功。
func TestWailsUpdateBackupConfig_CloudNoMasterPassword_Succeeds(t *testing.T) {
	a := wailsBackupTestApp(t)
	svc := NewBackupService(a)

	res, err := svc.UpdateBackupConfig(`{"enabled":true,"target_type":"webdav","webdav_url":"http://example.com","webdav_username":"u","webdav_password":"p"}`)
	if err != nil {
		t.Fatalf("UpdateBackupConfig returned Go error: %v", err)
	}
	m, ok := res.(map[string]any)
	if !ok {
		t.Fatalf("res = %T, want map[string]any", res)
	}
	if success, _ := m["success"].(bool); !success {
		t.Fatalf("success = false, want true; map = %+v", m)
	}
	if _, ok := m["action"]; ok {
		t.Errorf("action 应随主口令 UX 一并删除; map = %+v", m)
	}
	// 凭据必须真的经 keystore secrets 往返回来，而不是被当普通字段丢弃。
	if got := a.Settings.BackupConfig().WebDAVPassword; got != "p" {
		t.Errorf("WebDAVPassword = %q, want p", got)
	}
}
