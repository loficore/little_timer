// Package vault 是凭据 + 备份域的用例层。
//
// 设计（见 .omo/plans/habit-vault-extraction.md 的 R4–R6）：
//   - Vault 拥有主密码、磁盘加密 blob、解锁状态、BackupManager 生命周期；
//   - 备份的所有变更都过 Operations 这一道闸门；
//   - 云端凭据闸门是单一私有的 requireCloudCredentials()，Phase 3b 实现。
//
// Phase 3a：主密码 6 个方法从 *App 搬进 vault，vault 持有 SecretStorage。
// *App 通过 Secrets() / 6 个 master password 方法薄委托；3c 删 *App 转发。
package vault

import (
	"context"
	"encoding/json"
	"time"

	"little-timer/internal/crypto"
	"little-timer/internal/domain"
	"little-timer/internal/log"
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
	ErrBackupNotConfigured   = &Error{Code: "backup.not_configured", Message: "backup not configured"}
	ErrBackupNotEnabled      = &Error{Code: "backup.not_enabled", Message: "backup not enabled"}
	ErrPasswordTooShort      = &Error{Code: "password_too_short", Message: "password too short (minimum 4 characters)"}
	ErrCredentialsLocked     = &Error{Code: "master_password_not_unlocked", Message: "凭证已过期，请重新解锁", Mode: "unlock"}
	ErrCredentialsUnavailable = &Error{Code: "credentials_not_available", Message: "凭证不可用，请先设置主密码", Mode: ""} // Mode 动态
)

// BackupRuntime 是备份用例需要的运行时依赖。*App 通过 vault_adapter.go
// 里的 appRuntime 满足它。
type BackupRuntime interface {
	Manager() *backup.BackupManager
	Rebuild(ctx context.Context) error
}

// Operations 是 vault 域的用例层。
type Operations struct {
	// secret 是进程内主密码存储 + 加密 blob；构造期由 New 一次性创建。
	secret *crypto.SecretStorage
	// settings 用来同步 BackupConfig 里的 HasMasterPassword / CredentialLockedUntil 等字段。
	settings *settings.SettingsManager
	// runtime 给备份用例使用；master password 方法不依赖它。
	runtime BackupRuntime
}

// New 构造 Operations。secretPath 为空表示内存-only。
func New(secretPath string, settings *settings.SettingsManager, runtime BackupRuntime) *Operations {
	return &Operations{
		secret:   crypto.New(secretPath),
		settings: settings,
		runtime:  runtime,
	}
}

// Secrets 返回底层的 SecretStorage，供需要存/取受主密码保护的 secret
// 的调用方使用（如 LLM API Key）。3a 时仍开放此出口；3c 后应改为命名空间
// 化的 Store/Retrieve 方法。
func (o *Operations) Secrets() *crypto.SecretStorage {
	return o.secret
}

// ---- 主密码 ----

// HasMasterPassword 返回是否已设置主密码（基于磁盘上的凭据或 BackupConfig 标志）。
func (o *Operations) HasMasterPassword() bool {
	cfg := o.settings.BackupConfig()
	if cfg.HasMasterPassword {
		return true
	}
	return o.secret.HasMasterPassword()
}

// IsUnlocked 返回凭据是否已解锁且锁定期已过。
//
// 无主密码时直接返 true：fresh SecretStorage 的 masterPassword == nil
// 会让 secret.IsLocked() == true，但“没设主密码”不是“锁定”——没有
// 任何可锁的东西，也不需要 Unlock。直接返 false 会导致
// requireCloudCredentials 在“有凭据 + 无主密码”时弹无意义的解锁 modal。
func (o *Operations) IsUnlocked() bool {
	if !o.HasMasterPassword() {
		return true
	}
	cfg := o.settings.BackupConfig()
	if !o.secret.IsLocked() && cfg.CredentialLockedUntil <= time.Now().Unix() {
		return true
	}
	return false
}

// UnlockCredentials 使用给定密码解锁凭据并返回结构化结果。
// 未设置主密码时 `Success` 恒为 true。
func (o *Operations) UnlockCredentials(password string) domain.UnlockResult {
	cfg := o.settings.BackupConfig()
	if !o.HasMasterPassword() {
		log.Info("UnlockCredentials: no master password")
		cfg.CredentialLockedUntil = 0
		cfg.CredentialsUnlockTime = time.Now().Unix()
		_ = o.settings.UpdateBackupConfigFromJSON(backupConfigToJSON(cfg))
		return domain.UnlockResult{Success: true, LockedUntil: 0}
	}
	err := o.secret.Unlock([]byte(password))
	if err != nil {
		log.Error("UnlockCredentials: failed", "error", err.Error())
		return domain.UnlockResult{Success: false, LockedUntil: o.secret.LockoutUntil()}
	}
	log.Info("UnlockCredentials: success")
	cfg.CredentialLockedUntil = 0
	cfg.CredentialsUnlockTime = time.Now().Unix()
	_ = o.settings.UpdateBackupConfigFromJSON(backupConfigToJSON(cfg))
	return domain.UnlockResult{Success: true, LockedUntil: 0}
}

// SetMasterPassword 设置主密码并同步更新 BackupConfig 中的标志。
// 密码短于 4 字符 → ErrPasswordTooShort。
func (o *Operations) SetMasterPassword(password string) error {
	if len(password) < 4 {
		return ErrPasswordTooShort
	}
	if err := o.secret.SetMasterPassword([]byte(password)); err != nil {
		log.Error("SetMasterPassword: failed", "error", err.Error())
		return err
	}
	log.Info("SetMasterPassword: success")
	cfg := o.settings.BackupConfig()
	cfg.HasMasterPassword = true
	cfg.CredentialLockedUntil = 0
	return o.settings.UpdateBackupConfigFromJSON(backupConfigToJSON(cfg))
}

// GetMasterPasswordStatus 返回主密码相关的状态信息。
func (o *Operations) GetMasterPasswordStatus() domain.MasterPasswordStatus {
	cfg := o.settings.BackupConfig()
	return domain.MasterPasswordStatus{
		HasPassword: o.HasMasterPassword(),
		Unlocked:    o.IsUnlocked(),
		LockedUntil: cfg.CredentialLockedUntil,
		UnlockTime:  cfg.CredentialsUnlockTime,
	}
}

// LockCredentials 立即锁定凭据：将锁定期设为当前时间并清空内存中的密钥缓存。
func (o *Operations) LockCredentials() {
	log.Info("LockCredentials: success")
	cfg := o.settings.BackupConfig()
	cfg.CredentialLockedUntil = time.Now().Unix() + 1
	_ = o.settings.UpdateBackupConfigFromJSON(backupConfigToJSON(cfg))
	o.secret.Lock()
}

// ---- Backup（Phase 3b 加凭据闸门）----

// GetBackupConfig 返回持久化的 BackupConfig（secret 未打码，由 adapter 处理）。
func (o *Operations) GetBackupConfig() domain.BackupConfig {
	return o.settings.BackupConfig()
}

// UpdateBackupConfig 从 JSON 持久化新配置并重建 manager。
// 云端 target 切换前鉴权：跟原始 handler 一致——
//   - inbound target 是 webdav/s3 且持久化 cfg 已有该 target 凭据：需 unlocked
//   - inbound target 是 webdav/s3 但持久化 cfg 没该 target 凭据：需主密码 + unlocked
//     （inbound JSON 会带新凭据进来）
//   - local target：不走闸门
func (o *Operations) UpdateBackupConfig(ctx context.Context, jsonStr string) error {
	var probe struct {
		TargetType string `json:"target_type"`
	}
	_ = json.Unmarshal([]byte(jsonStr), &probe)
	if probe.TargetType == "webdav" || probe.TargetType == "s3" {
		if err := o.requireAuthForConfigUpdate(probe.TargetType); err != nil {
			return err
		}
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
//   - cloud target + 持久化 cfg 没存该 target 凭据：
//     mode = "setup"（无主密码时）| "unlock"（有主密码但锁着）
//   - cloud target + 持久化 cfg 有凭据但 IsUnlocked()=false：
//     mode = "unlock"
//
// backup 操作必须有凭据 + 已解锁；不能仅靠主密码鉴权（inbound 没新凭据）。
//
// Adapter 拿 Mode 拼 action.params.mode。
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
		mode := "setup"
		if o.HasMasterPassword() {
			mode = "unlock"
		}
		return &Error{
			Code:    ErrCredentialsUnavailable.Code,
			Message: ErrCredentialsUnavailable.Message,
			Mode:    mode,
		}
	}
	if !o.IsUnlocked() {
		return &Error{
			Code:    ErrCredentialsLocked.Code,
			Message: ErrCredentialsLocked.Message,
			Mode:    ErrCredentialsLocked.Mode,
		}
	}
	return nil
}

// requireAuthForConfigUpdate 是 config update 闸门（比 requireCloudCredentials 宽松）。
//
//   - 当前 cfg 已有该 target 凭据：只需 unlocked
//   - 当前 cfg 没该 target 凭据：需主密码 + unlocked
//     （inbound JSON 会带新凭据，所以仅鉴权就够）
//   - 无主密码：返 setup modal
//   - 已设主密码但锁着：返 unlock modal
func (o *Operations) requireAuthForConfigUpdate(target string) *Error {
	cfg := o.settings.BackupConfig()
	var hasCreds bool
	switch target {
	case "webdav":
		hasCreds = cfg.WebDAVPassword != ""
	case "s3":
		hasCreds = cfg.S3AccessKey != "" && cfg.S3SecretKey != ""
	}
	if hasCreds {
		// 原 handler：有凭据就不检查（无论锁定与否，inbound 不一定在改凭据）
		return nil
	}
	if !o.HasMasterPassword() {
		return &Error{
			Code:    ErrCredentialsUnavailable.Code,
			Message: ErrCredentialsUnavailable.Message,
			Mode:    "setup",
		}
	}
	if !o.IsUnlocked() {
		return &Error{
			Code:    ErrCredentialsLocked.Code,
			Message: ErrCredentialsLocked.Message,
			Mode:    ErrCredentialsLocked.Mode,
		}
	}
	return nil
}

// backupConfigToJSON 把 BackupConfig 序列化成可直接喂给
// SettingsManager.UpdateBackupConfigFromJSON 的 JSON 字符串。BackupConfig
// 的 JSON tag 跟解析器期望的 wire 形状一致，直接 marshal 即可。
func backupConfigToJSON(cfg domain.BackupConfig) string {
	b, err := json.Marshal(cfg)
	if err != nil {
		return "{}"
	}
	return string(b)
}


