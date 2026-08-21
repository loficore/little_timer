package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"little-timer/internal/domain"
	"little-timer/internal/settings"
	"little-timer/internal/storage"
	"little-timer/internal/storage/backup"
)

// TestRebuildBackupSwitchesTargetType 验证 RebuildBackup 能从持久化的
// BackupConfig 重建 BackupManager，并随 target_type 变化换掉活动 adapter。
// webdav/s3 配置是纯构造 —— NewWebDAVAdapter / NewS3Adapter 不做网络 I/O ——
// 所以不需要 httptest server。
func TestRebuildBackupSwitchesTargetType(t *testing.T) {
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

	a := NewApp(domain.NewClockManager(domain.NewDefaultClockTaskConfig()), sm, sqlite, nil, dbPath)

	assertTarget := func(want backup.BackupTarget) *backup.BackupManager {
		t.Helper()
		got := a.BackupManager()
		if got == nil {
			t.Fatalf("BackupManager() = nil, want adapter target %q", want)
		}
		if got.Adapter().Target() != want {
			t.Fatalf("adapter target = %q, want %q", got.Adapter().Target(), want)
		}
		return got
	}

	update := func(json string) {
		t.Helper()
		if err := sm.UpdateBackupConfigFromJSON(json); err != nil {
			t.Fatalf("update backup config: %v", err)
		}
		if err := a.RebuildBackup(context.Background()); err != nil {
			t.Fatalf("RebuildBackup: %v", err)
		}
	}

	// Local —— 默认 target；adapter 根在推导出的“DB 同级”备份目录
	//（LocalPath 为空）。
	update(`{"target_type": "local"}`)
	localMgr := assertTarget(backup.TargetLocal)

	// WebDAV —— 假 URL/凭据就够；构造不做 I/O。
	update(`{"target_type": "webdav", "webdav_url": "https://example.com/dav", "webdav_username": "u", "webdav_password": "p"}`)
	webdavMgr := assertTarget(backup.TargetWebDAV)
	if webdavMgr == localMgr {
		t.Fatal("BackupManager() still references the pre-rebuild manager after webdav switch")
	}

	// S3 —— NewS3Adapter 要求 bucket + region；仅静态凭据，不联网。
	update(`{"target_type": "s3", "s3_endpoint": "https://minio.example:9000", "s3_bucket": "bkt", "s3_region": "us-east-1", "s3_access_key": "ak", "s3_secret_key": "sk"}`)
	s3Mgr := assertTarget(backup.TargetS3)
	if s3Mgr == webdavMgr {
		t.Fatal("BackupManager() still references the pre-rebuild manager after s3 switch")
	}
	if s3Mgr == localMgr {
		t.Fatal("BackupManager() still references the original manager after s3 switch")
	}

	// 切回 local —— 从云端 target 重建后也必须落回 local adapter。
	update(`{"target_type": "local"}`)
	backAgain := assertTarget(backup.TargetLocal)
	if backAgain == s3Mgr {
		t.Fatal("BackupManager() still references the s3 manager after switching back to local")
	}
}

// TestRebuildBackupDisablesOnCloudConfigError 验证云端 target 的失败契约：
// 当云端 target 的 NewFromConfig 失败时（这里是缺 bucket/region 的 s3，
// NewS3Adapter 会拒绝），RebuildBackup 记日志、把 a.Backup 置 nil 并返回
// 错误 —— 绝不能回退到 local adapter，因为设置声明的是云端 target。
func TestRebuildBackupDisablesOnCloudConfigError(t *testing.T) {
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

	a := NewApp(domain.NewClockManager(domain.NewDefaultClockTaskConfig()), sm, sqlite, nil, dbPath)

	if err := sm.UpdateBackupConfigFromJSON(`{"target_type": "s3"}`); err != nil {
		t.Fatalf("update backup config: %v", err)
	}
	if err := a.RebuildBackup(context.Background()); err == nil {
		t.Fatal("RebuildBackup with broken s3 config should return an error")
	}
	if mgr := a.BackupManager(); mgr != nil {
		t.Fatalf("BackupManager() = %v after cloud config error, want nil (disabled)", mgr)
	}
}

// 关于 local-target 回退路径的说明：local 的宽松回退测试构造不出来。
// local target 的 NewFromConfig 只可能失败在 NewLocal 内部（对推导出的
// backupDir 做 os.MkdirAll —— buildAdapter 的 local 分支永不失败），而
// RebuildBackup 会用同一个推导目录重试 NewLocal，回退必然同样失败、总是
// 落到下方的 disable 路径。因此 local 的 disable 契约由
// TestRebuildBackupDisablesOnDoubleFailure 钉住。

// TestRebuildBackupDisablesOnDoubleFailure 验证终态路径：当 NewFromConfig
// 与 NewLocal 回退双双失败时，RebuildBackup 把 a.Backup 置 nil 并返回错误。
// 备份目录通过在其父目录位置放一个普通文件来强制无法创建，让 os.MkdirAll
// 以 ENOTDIR 失败。
func TestRebuildBackupDisablesOnDoubleFailure(t *testing.T) {
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

	blocker := filepath.Join(t.TempDir(), "not_a_dir")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatalf("write blocker file: %v", err)
	}

	a := NewApp(domain.NewClockManager(domain.NewDefaultClockTaskConfig()), sm, sqlite, nil, filepath.Join(blocker, "test.db"))

	err = a.RebuildBackup(context.Background())
	if err == nil {
		t.Fatal("RebuildBackup with uncreatable backup dir should return an error")
	}
	if mgr := a.BackupManager(); mgr != nil {
		t.Fatalf("BackupManager() = %v after double failure, want nil", mgr)
	}
}
