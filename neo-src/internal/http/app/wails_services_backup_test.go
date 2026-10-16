package app

import (
	"path/filepath"
	"testing"

	"little-timer/internal/domain"
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
	return NewApp(domain.NewClockManager(domain.NewDefaultClockTaskConfig()), sm, sqlite, bm, dbPath)
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
	action, ok := m["action"].(map[string]any)
	if !ok {
		t.Fatalf("action missing or wrong type; map = %+v", m)
	}
	if action["type"] != "show_modal" {
		t.Errorf("action.type = %v, want show_modal", action["type"])
	}
	if action["target"] != "master_password" {
		t.Errorf("action.target = %v, want master_password", action["target"])
	}
	params, _ := action["params"].(map[string]any)
	if params["mode"] != "setup" {
		t.Errorf("action.params.mode = %v, want setup", params["mode"])
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
	if _, ok := m["action"].(map[string]any); !ok {
		t.Errorf("action missing; map = %+v", m)
	}
}

// TestWailsUpdateBackupConfig_CloudNoMasterPassword_ReturnsSetupMode 覆盖
// config update 路径的闸门（之前 Wails 完全不检查）。
func TestWailsUpdateBackupConfig_CloudNoMasterPassword_ReturnsSetupMode(t *testing.T) {
	a := wailsBackupTestApp(t)
	svc := NewBackupService(a)

	res, err := svc.UpdateBackupConfig(`{"enabled":true,"target_type":"webdav","webdav_url":"http://example.com"}`)
	if err != nil {
		t.Fatalf("UpdateBackupConfig returned Go error: %v", err)
	}
	m, ok := res.(map[string]any)
	if !ok {
		t.Fatalf("res = %T, want map[string]any", res)
	}
	if code, _ := m["code"].(string); code != "credentials_not_available" {
		t.Errorf("code = %q, want credentials_not_available", code)
	}
	action, _ := m["action"].(map[string]any)
	params, _ := action["params"].(map[string]any)
	if params["mode"] != "setup" {
		t.Errorf("action.params.mode = %v, want setup", params["mode"])
	}
}
