// Package vault 是凭据 + 备份域的用例层。
//
// 设计（见 .omo/plans/habit-vault-extraction.md 的 R4–R6）：
//   - Vault 拥有主密码、磁盘加密 blob、解锁状态、BackupManager 生命周期；
//   - 备份的所有变更都过 Operations 这一道闸门；
//   - 云端凭据闸门是单一私有的 requireCloudCredentials()，Phase 3b 实现。
//
// issue #13 去锁化后：主口令与锁模型整体删除，凭据改由平台 keystore 托管
// （Linux Secret Service / Android Keystore）。Vault 不再持有 SecretStorage，
// 而是经 BackupRuntime.Secrets() 拿 *App 的惰性实例——后者负责 keystore 解析
// 与 SettingsManager 接线，保证「谁建谁接线」只有一处。
package vault

import (
	"context"

	"little-timer/internal/crypto"
	"little-timer/internal/domain"
	"little-timer/internal/settings"
	"little-timer/internal/storage/backup"
)

// Error 是 vault 域的类型化错误。Adapter 用 errors.As 映射到 transport。
//
// Mode 保留给"需要前端弹窗"的错误；去锁化后已无 setup/unlock 两种
// 主口令弹窗，仅在确有其它交互需求时使用，其余留空。
type Error struct {
	Code    string
	Message string
	Mode    string
}

func (e *Error) Error() string { return e.Message }

// 哨兵错误。
var (
	ErrBackupNotConfigured   = &Error{Code: "backup.not_configured", Message: "backup not configured"}
	ErrBackupNotEnabled      = &Error{Code: "backup.not_enabled", Message: "backup not enabled"}
	ErrCredentialsUnavailable = &Error{Code: "credentials_not_available", Message: "未配置该目标的备份凭据，请在设置中填写后重试"}
)

// BackupRuntime 是备份用例需要的运行时依赖。*App 通过 vault_adapter.go
// 里的 appRuntime 满足它。
type BackupRuntime interface {
	Manager() *backup.BackupManager
	Rebuild(ctx context.Context) error
	// Secrets 返回 App 惰性构造的 SecretStorage。Vault 不自己建凭据存储
	// （keystore 解析与 SettingsManager 接线只有 App.ensureSecrets 一处），
	// 写配置前调它确保 store 已接线，否则凭据会被静默丢弃。
	Secrets() *crypto.SecretStorage
}

// Operations 是 vault 域的用例层。
type Operations struct {
	// settings 是备份配置的持久化出口，凭据经 SetSecretStore 分流到
	// keystore Secrets（见 settings.SecretWebDAVPassword 等常量）。
	settings *settings.SettingsManager
	// runtime 给备份用例使用，并经 Secrets() 触达凭据存储。
	runtime BackupRuntime
}

// New 构造 Operations。凭据存储不在此创建——由 runtime.Secrets() 惰性提供，
// 使 keystore 解析与 SettingsManager 接线保持单一入口。
func New(settings *settings.SettingsManager, runtime BackupRuntime) *Operations {
	return &Operations{settings: settings, runtime: runtime}
}

// ---- Backup（Phase 3b 加凭据闸门）----

// GetBackupConfig 返回持久化的 BackupConfig（secret 未打码，由 adapter 处理）。
func (o *Operations) GetBackupConfig() domain.BackupConfig {
	return o.settings.BackupConfig()
}

// UpdateBackupConfig 从 JSON 持久化新配置并重建 manager。
//
// 去锁化（issue #13）：原先切换到云端 target 前要过"主密码 setup / unlock"
//闸门，那是用来证明"操作者知道主口令"的。凭据改由平台 keystore 托管后，
//保护边界是 OS 登录会话（issue #13 的前提），不再有需要证明的口令，
//故闸门整体删除；缺凭据由运行时的连接失败与 Task 9 的 credentials 端点体现。
func (o *Operations) UpdateBackupConfig(ctx context.Context, jsonStr string) error {
	// SettingsManager 未接线 secret store 时会把云端凭据当普通字段丢掉，
	// 先触达 Secrets() 保证接线（App.ensureSecrets 内部按 Once 幂等）。
	if o.runtime != nil {
		o.runtime.Secrets()
	}
	if err := o.settings.UpdateBackupConfigFromJSON(jsonStr); err != nil {
		return err
	}
	_ = o.runtime.Rebuild(ctx)
	return nil
}

// CreateBackup 触发一次备份。云端 target 需要凭据可用 + 未锁。
func (o *Operations) CreateBackup() (string, error) {
	mgr := o.runtime.Manager()
	if mgr == nil {
		return "", ErrBackupNotConfigured
	}
	if !o.settings.BackupConfig().Enabled {
		return "", ErrBackupNotEnabled
	}
	if err := o.requireCloudCredentials(o.settings.BackupConfig().TargetType.String()); err != nil {
		return "", err
	}
	return mgr.CreateBackup()
}

// RestoreBackup 从指定名字的备份恢复。
func (o *Operations) RestoreBackup(name string) error {
	mgr := o.runtime.Manager()
	if mgr == nil {
		return ErrBackupNotConfigured
	}
	if err := o.requireCloudCredentials(o.settings.BackupConfig().TargetType.String()); err != nil {
		return err
	}
	return mgr.RestoreFromBackup(name)
}

// ListBackups 列出现有备份；未配置时返回空切片。读操作不需闸门。
func (o *Operations) ListBackups() ([]backup.BackupInfo, error) {
	mgr := o.runtime.Manager()
	if mgr == nil {
		return []backup.BackupInfo{}, nil
	}
	items, err := mgr.ListBackups()
	if err != nil {
		return []backup.BackupInfo{}, nil
	}
	return items, nil
}

// DeleteBackup 删除指定名字的备份。
func (o *Operations) DeleteBackup(name string) error {
	mgr := o.runtime.Manager()
	if mgr == nil {
		return ErrBackupNotConfigured
	}
	if err := o.requireCloudCredentials(o.settings.BackupConfig().TargetType.String()); err != nil {
		return err
	}
	return mgr.DeleteBackup(name)
}

// VerifyBackup 测一次目标可达性。
func (o *Operations) VerifyBackup() error {
	mgr := o.runtime.Manager()
	if mgr == nil {
		return ErrBackupNotConfigured
	}
	if !o.settings.BackupConfig().Enabled {
		return ErrBackupNotEnabled
	}
	if err := o.requireCloudCredentials(o.settings.BackupConfig().TargetType.String()); err != nil {
		return err
	}
	return mgr.TestConnection()
}

// BackupSummary 返回备份统计。读操作不需闸门。
func (o *Operations) BackupSummary() (backup.BackupSummary, error) {
	mgr := o.runtime.Manager()
	if mgr == nil {
		return backup.BackupSummary{}, ErrBackupNotConfigured
	}
	return mgr.Summary()
}

// requireCloudCredentials 是 backup 操作的闸门。
//
//   - target 不是 webdav/s3：返 nil（local 不需要云端凭据）
//   - cloud target 但持久化 cfg 没有该 target 的凭据：返 ErrCredentialsUnavailable
//
// 去锁化后闸门只剩"凭据配了没"这一件事：原 IsUnlocked 分支（决定弹 setup
// 还是 unlock modal）随主口令删除。适配层把本错误映射成普通 {success:false}，
// 不再有 mode 字段。
func (o *Operations) requireCloudCredentials(target string) *Error {
	if target != "webdav" && target != "s3" {
		return nil
	}
	cfg := o.settings.BackupConfig()
	var hasCreds bool
	switch target {
	case "webdav":
		hasCreds = cfg.WebDAVPassword != ""
	case "s3":
		hasCreds = cfg.S3AccessKey != "" && cfg.S3SecretKey != ""
	}
	if !hasCreds {
		return ErrCredentialsUnavailable
	}
	return nil
}
