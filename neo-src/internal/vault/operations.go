// Package vault 是凭据 + 备份域的用例层。
//
// 设计（见 .omo/plans/habit-vault-extraction.md 的 R4–R6）：
//   - Vault 拥有主密码、磁盘加密 blob、解锁状态、BackupManager 生命周期；
//   - 备份的所有变更都过 Operations 这一道闸门；
//   - 云端凭据闸门是单一私有的 requireCloudCredentials()，Phase 3b 实现。
//
// Phase 1（scaffold）：方法全部转发到既有 *App 逻辑，不含规则、不含凭据
// 闸门。规则在 Phase 3 迁移进来；本包当前不被任何 adapter 调用。
package vault

import (
	"context"

	"little-timer/internal/domain"
	"little-timer/internal/settings"
	"little-timer/internal/storage/backup"
)

// Error 是 vault 域的类型化错误。Adapter 用 errors.As 映射到 transport。
//
// Mode 只对"需要弹主密码 modal"的错误有意义（"setup" / "unlock"）；
// 其它错误留空。Adapter 把它塞进 action.params.mode。
type Error struct {
	Code    string
	Message string
	Mode    string
}

func (e *Error) Error() string { return e.Message }

// 哨兵错误。
var (
	ErrBackupNotConfigured = &Error{Code: "backup.not_configured", Message: "backup not configured"}
	ErrBackupNotEnabled    = &Error{Code: "backup.not_enabled", Message: "backup not enabled"}
	ErrPasswordTooShort    = &Error{Code: "password_too_short", Message: "password too short (minimum 4 characters)"}
)

// BackupRuntime 是备份用例需要的运行时依赖。*App 通过 vault_adapter.go
// 里的 appRuntime 满足它。
type BackupRuntime interface {
	// Manager 返回当前 BackupManager 快照；未配置时为 nil。
	Manager() *backup.BackupManager
	// Rebuild 从持久化 BackupConfig 重建 BackupManager。
	Rebuild(ctx context.Context) error
}

// MasterPasswordRuntime 是 Phase 1 的主密码依赖，由 *App 现有方法满足。
// Phase 3a 会把主密码逻辑搬进本包，此接口随之删除。
type MasterPasswordRuntime interface {
	HasMasterPassword() bool
	IsUnlocked() bool
	UnlockCredentials(password string) domain.UnlockResult
	SetMasterPassword(password string) error
	LockCredentials()
	GetMasterPasswordStatus() domain.MasterPasswordStatus
}

// Runtime 是 Operations 的运行时依赖并集。
type Runtime interface {
	BackupRuntime
	MasterPasswordRuntime
}

// Operations 是 vault 域的用例层。
type Operations struct {
	// secretPath 是加密 blob 在磁盘上的路径。Phase 3a 用它构造
	// crypto.SecretStorage；Phase 1 未使用。
	secretPath string
	settings   *settings.SettingsManager
	runtime    Runtime
}

// New 构造 Operations。secretPath 为空表示内存-only。
func New(secretPath string, settings *settings.SettingsManager, runtime Runtime) *Operations {
	return &Operations{secretPath: secretPath, settings: settings, runtime: runtime}
}

// ---- Master password（Phase 1 转发；Phase 3a 搬进来）----

// HasMasterPassword 转发到 runtime。
func (o *Operations) HasMasterPassword() bool { return o.runtime.HasMasterPassword() }

// IsUnlocked 转发到 runtime。
func (o *Operations) IsUnlocked() bool { return o.runtime.IsUnlocked() }

// UnlockCredentials 转发到 runtime。
func (o *Operations) UnlockCredentials(password string) domain.UnlockResult {
	return o.runtime.UnlockCredentials(password)
}

// SetMasterPassword 转发到 runtime。
func (o *Operations) SetMasterPassword(password string) error {
	return o.runtime.SetMasterPassword(password)
}

// LockCredentials 转发到 runtime。
func (o *Operations) LockCredentials() { o.runtime.LockCredentials() }

// GetMasterPasswordStatus 转发到 runtime。
func (o *Operations) GetMasterPasswordStatus() domain.MasterPasswordStatus {
	return o.runtime.GetMasterPasswordStatus()
}

// ---- Backup（Phase 1 转发；Phase 3b 加凭据闸门）----

// GetBackupConfig 返回持久化的 BackupConfig（secret 未打码，由 adapter 处理）。
func (o *Operations) GetBackupConfig() domain.BackupConfig {
	return o.settings.BackupConfig()
}

// UpdateBackupConfig 从 JSON 持久化新配置并重建 manager。
func (o *Operations) UpdateBackupConfig(ctx context.Context, json string) error {
	if err := o.settings.UpdateBackupConfigFromJSON(json); err != nil {
		return err
	}
	_ = o.runtime.Rebuild(ctx)
	return nil
}

// CreateBackup 触发一次备份。Phase 3b 会在最前面加 requireCloudCredentials()。
func (o *Operations) CreateBackup() (string, error) {
	mgr := o.runtime.Manager()
	if mgr == nil {
		return "", ErrBackupNotConfigured
	}
	if !o.settings.BackupConfig().Enabled {
		return "", ErrBackupNotEnabled
	}
	return mgr.CreateBackup()
}

// RestoreBackup 从指定名字的备份恢复。
func (o *Operations) RestoreBackup(name string) error {
	mgr := o.runtime.Manager()
	if mgr == nil {
		return ErrBackupNotConfigured
	}
	return mgr.RestoreFromBackup(name)
}

// ListBackups 列出现有备份；未配置时返回空切片。
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
	return mgr.TestConnection()
}

// BackupSummary 返回备份统计。
func (o *Operations) BackupSummary() (backup.BackupSummary, error) {
	mgr := o.runtime.Manager()
	if mgr == nil {
		return backup.BackupSummary{}, ErrBackupNotConfigured
	}
	return mgr.Summary()
}
