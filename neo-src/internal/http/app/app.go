// Package app 承载 App 结构体——应用的依赖与运行时状态束。
// 每个处理器通过 Gin 上下文的 `MustGet("app")` 槽位收到 `*App`，
// 因此处理器无需导入包级全局变量即可访问时钟、设置、SQLite 管理器、
// 备份管理器和主密码状态。
//
// 将 `App` 拆到独立子包打破了本会形成的导入环
// （router → handlers → http）。router 与 handlers 都导入本包。
package app

import (
	"context"
	"path/filepath"

	"sync"
	"time"

	"little-timer/internal/crypto"
	"little-timer/internal/domain"
	"little-timer/internal/log"
	"little-timer/internal/settings"
	"little-timer/internal/storage"
	"little-timer/internal/storage/backup"
)

// App 汇集 HTTP 处理器所需的应用依赖与运行时状态。
//
// 每个服务器进程对应一个 App，由调用方构造（目前是
// `cmd/server/main.go` 中的服务器引导流程），再传给 `NewRouter`。
//
// `Backup` 可选——为 nil 时备份 endpoint 返回类 503 响应
// （处理器将 nil 视为"未配置备份"）。这让 http 层不依赖备份管理器是否已接线。
type App struct {
	mu sync.RWMutex

	// Clock 管理当前计时状态。
	Clock *domain.ClockManager
	// Settings 管理应用设置。
	Settings *settings.SettingsManager
	// SQLite 管理 SQLite 连接与存储子模块。
	SQLite *storage.SqliteManager
	// Backup 管理备份操作；未配置时可为 nil。
	Backup *backup.BackupManager

	// DBPath 是数据库文件路径，构造时捕获，供备份处理器使用。
	DBPath string

	// CurrentHabitID 是当前计时关联的习惯 ID。
	CurrentHabitID *int64
	// CurrentTimerSessionID 是当前计时会话 ID。
	CurrentTimerSessionID *int64

	// secrets 是进程内主密码存储，由辅助方法惰性创建，
	// 因此调用方构造时可以省略它。
	secrets *crypto.SecretStorage
}

// NewApp 使用给定的依赖与默认内存状态构造一个 App。
// 捕获 `dbPath` 是为了让需要磁盘路径的处理器（目前只有临时构建适配器的
// 备份处理器）能直接读取，不必从 SQLite 管理器重新推导。
func NewApp(
	clk *domain.ClockManager,
	sm *settings.SettingsManager,
	sqlite *storage.SqliteManager,
	bm *backup.BackupManager,
	dbPath string,
) *App {
	return &App{
		Clock:    clk,
		Settings: sm,
		SQLite:   sqlite,
		Backup:   bm,
		DBPath:   dbPath,
	}
}

// mutex 便捷访问器——以 Lock/Unlock 形式暴露而非藏进辅助方法，
// 让处理器的加锁保持显式。

// Lock 获取 App 的写锁。
func (a *App) Lock() { a.mu.Lock() }

// Unlock 释放 App 的写锁。
func (a *App) Unlock() { a.mu.Unlock() }

// RLock 获取 App 的读锁，供 SSE 等只读消费者使用。
func (a *App) RLock() { a.mu.RLock() }

// RUnlock 释放 App 的读锁。
func (a *App) RUnlock() { a.mu.RUnlock() }

// Backup 管理器访问器。持久化的 BackupConfig 一变（切换 target、轮换
// 凭据）就重建管理器——BackupManager 上没有 SetTarget 可变方法。
// 处理器和 Wails 服务通过 BackupManager()（RLock 快照）读取当前管理器，
// 配置变更时调用 RebuildBackup（写锁）。

// BackupManager 返回当前 BackupManager 快照。未配置备份时可能为 nil
// （处理器将 nil 视为"未配置备份"）。
func (a *App) BackupManager() *backup.BackupManager {
	a.RLock()
	defer a.RUnlock()
	return a.Backup
}

// RebuildBackup 从持久化的 BackupConfig 重建 BackupManager，并在 App 写锁保护下
// 换入。云端 target（webdav/s3）构建失败时，本进程禁用备份:记录带 target
// 类型的错误日志、将 a.Backup 置 nil 并返回错误——绝不静默回退到 local，
// 因为 UI/设置宣传的是云端 target，本地备份会偏离配置的行为。仅 local
// target 构建失败时，记录日志并回退到以默认备份目录为根的 local 适配器;
// 只有该回退也失败时才将 a.Backup 置 nil（本进程禁用备份）。
func (a *App) RebuildBackup(ctx context.Context) error {
	backupDir := defaultBackupDir(a.DBPath)
	cfg := a.Settings.BackupConfig()
	mgr, err := backup.NewFromConfig(ctx, a.SQLite, a.DBPath, backupDir, cfg)
	if err != nil {
		if cfg.TargetType == domain.BackupTargetWebDAV || cfg.TargetType == domain.BackupTargetS3 {
			log.Error("RebuildBackup: NewFromConfig failed for cloud target, disabling backup", "target_type", cfg.TargetType.String(), "error", err.Error())
			a.Lock()
			a.Backup = nil
			a.Unlock()
			return err
		}
		log.Error("RebuildBackup: NewFromConfig failed, falling back to local", "error", err.Error())
		mgr, err = backup.NewLocal(a.SQLite, a.DBPath, backupDir)
		if err != nil {
			log.Error("RebuildBackup: NewLocal fallback failed, disabling backup", "error", err.Error())
			a.Lock()
			a.Backup = nil
			a.Unlock()
			return err
		}
	}
	a.Lock()
	a.Backup = mgr
	a.Unlock()
	log.Info("RebuildBackup: ok", "target_type", cfg.TargetType.String())
	return nil
}

// defaultBackupDir 返回与 DB 同级的备份目录。裸文件名 DB 路径
// （Dir 为 "" 或 "."）解析为 `./backups`。
func defaultBackupDir(dbPath string) string {
	dir := filepath.Dir(dbPath)
	if dir == "" || dir == "." {
		return "backups"
	}
	return filepath.Join(dir, "backups")
}

// 计时器会话辅助方法。
//
// 这些辅助方法假定调用方持有 App mutex（Lock 或 RLock）——它们只变更
// 内存指针和数据库，绝不触碰锁本身，也不重新加锁。

// CreateTimerSession 插入一条 timer_sessions 行并更新内存中的当前会话指针。调用方必须持有 a.mu 写锁。
func (a *App) CreateTimerSession(habitID *int64, mode string, work, rest, loop int64) (int64, error) {
	id, err := a.SQLite.Timers().CreateTimerSession(habitID, mode, work, rest, loop)
	if err != nil {
		log.Error("timer.session.create failed", "habit_id", habitID, "mode", mode, "error", err.Error())
		return 0, err
	}
	a.CurrentTimerSessionID = &id
	a.CurrentHabitID = habitID
	log.Info("timer.session.create", "session_id", id, "habit_id", habitID, "mode", mode)
	return id, nil
}

// FinishTimerSession 将当前 timer_session 标记为结束并返回已用秒数。调用方必须持有 a.mu 写锁。
func (a *App) FinishTimerSession() (int64, error) {
	sessionID := a.CurrentTimerSessionID
	if sessionID == nil {
		return 0, nil
	}
	if err := a.SQLite.Timers().FinishTimerSession(*sessionID); err != nil {
		log.Error("timer.session.finish failed", "session_id", *sessionID, "error", err.Error())
		return 0, err
	}
	state := a.Clock.Update()
	elapsed := state.GetElapsedSeconds()
	log.Info("timer.session.finish", "session_id", *sessionID, "elapsed", elapsed)
	return elapsed, nil
}

// ResetTimerSession 清除内存中的当前会话指针并删除对应的 timer_session 行。调用方必须持有 a.mu 写锁。
func (a *App) ResetTimerSession() {
	sessionID := a.CurrentTimerSessionID
	a.CurrentTimerSessionID = nil
	habitID := a.CurrentHabitID
	a.CurrentHabitID = nil
	if sessionID != nil {
		_ = a.SQLite.Timers().DeleteTimerSession(*sessionID)
	}
	log.Info("timer.session.reset", "session_id", sessionID, "habit_id", habitID)
}

// LoadTimerProgress 重新读取最近未结束的 timer_session 到内存指针。调用方必须持有 a.mu 写锁。
func (a *App) LoadTimerProgress() {
	row, err := a.SQLite.Timers().GetActiveTimerSession()
	if err != nil {
		log.Error("timer.session.load_progress failed", "error", err.Error())
		return
	}
	id := row.ID
	a.CurrentTimerSessionID = &id
	if row.HabitID != nil {
		hid := *row.HabitID
		a.CurrentHabitID = &hid
	}
	log.Info("timer.session.load_progress", "session_id", row.ID, "habit_id", row.HabitID, "elapsed", row.ElapsedSeconds)
}

// SaveProgressLocked 将当前时钟状态持久化到活动的 timer_session 行。调用方必须持有 a.mu 写锁。
func (a *App) SaveProgressLocked() {
	if a.CurrentTimerSessionID == nil {
		return
	}
	state := a.Clock.Update()
	now := time.Now().Unix()
	row, err := a.SQLite.Timers().GetTimerSessionByID(*a.CurrentTimerSessionID)
	if err != nil {
		log.Error("timer.session.save_progress failed", "session_id", *a.CurrentTimerSessionID, "error", err.Error())
		return
	}
	pausedTotal := row.PausedTotalSeconds
	pauseStarted := row.PauseStartedAt
	isPaused := state.IsPaused()
	isRunning := !isPaused
	if isPaused && pauseStarted == nil {
		pauseStarted = &now
	} else if !isPaused && pauseStarted != nil {
		if now > *pauseStarted {
			pausedTotal += now - *pauseStarted
		}
		pauseStarted = nil
	}
	remaining := state.GetRemainingSeconds()
	_ = a.SQLite.Timers().UpdateTimerSession(
		row.ID, state.GetElapsedSeconds(), &remaining,
		pausedTotal, pauseStarted, &now,
		isRunning, isPaused, state.IsFinished(),
		row.CurrentRound, state.InRest(),
	)
	log.Info("timer.session.save_progress", "session_id", row.ID, "elapsed", state.GetElapsedSeconds())
}

// 主密码辅助方法。状态存放在 SettingsManager 的 BackupConfig（已带锁定
// 字段）加上用于保存解锁密码本身的 SecretStorage。
//
// 所有辅助方法都容忍 Secrets 为 nil（视为"未设置主密码"）。

func (a *App) ensureSecrets() *crypto.SecretStorage {
	if a.secrets == nil {
		a.secrets = crypto.New(filepath.Join(filepath.Dir(a.DBPath), "secret.db"))
	}
	return a.secrets
}

// HasMasterPassword 返回是否已设置主密码（基于磁盘上的凭据或 BackupConfig 标志）。
func (a *App) HasMasterPassword() bool {
	cfg := a.Settings.BackupConfig()
	if cfg.HasMasterPassword {
		return true
	}
	return a.ensureSecrets().HasMasterPassword()
}

// IsUnlocked 返回凭据是否已解锁且锁定期已过。
func (a *App) IsUnlocked() bool {
	cfg := a.Settings.BackupConfig()
	if !a.ensureSecrets().IsLocked() && cfg.CredentialLockedUntil <= time.Now().Unix() {
		return true
	}
	return false
}

// UnlockCredentials 使用给定密码解锁凭据并返回结构化结果。
// 未设置主密码时 `Success` 恒为 true。
func (a *App) UnlockCredentials(password string) domain.UnlockResult {
	cfg := a.Settings.BackupConfig()
	if !a.HasMasterPassword() {
		log.Info("UnlockCredentials: no master password")
		// 无主密码:总是成功。
		cfg.CredentialLockedUntil = 0
		cfg.CredentialsUnlockTime = time.Now().Unix()
		_ = a.Settings.UpdateBackupConfigFromJSON(backupConfigToJSON(cfg))
		return domain.UnlockResult{Success: true, LockedUntil: 0}
	}
	err := a.ensureSecrets().Unlock([]byte(password))
	if err != nil {
		log.Error("UnlockCredentials: failed", "error", err.Error())
		return domain.UnlockResult{Success: false, LockedUntil: a.ensureSecrets().LockoutUntil()}
	}
	log.Info("UnlockCredentials: success")
	cfg.CredentialLockedUntil = 0
	cfg.CredentialsUnlockTime = time.Now().Unix()
	_ = a.Settings.UpdateBackupConfigFromJSON(backupConfigToJSON(cfg))
	return domain.UnlockResult{Success: true, LockedUntil: 0}
}

// SetMasterPassword 设置主密码并同步更新 BackupConfig 中的标志。
func (a *App) SetMasterPassword(password string) error {
	if len(password) < 4 {
		return errPasswordTooShort
	}
	if err := a.ensureSecrets().SetMasterPassword([]byte(password)); err != nil {
		log.Error("SetMasterPassword: failed", "error", err.Error())
		return err
	}
	log.Info("SetMasterPassword: success")
	cfg := a.Settings.BackupConfig()
	cfg.HasMasterPassword = true
	cfg.CredentialLockedUntil = 0
	return a.Settings.UpdateBackupConfigFromJSON(backupConfigToJSON(cfg))
}

// GetMasterPasswordStatus 返回主密码相关的状态信息。
func (a *App) GetMasterPasswordStatus() domain.MasterPasswordStatus {
	cfg := a.Settings.BackupConfig()
	return domain.MasterPasswordStatus{
		HasPassword: a.HasMasterPassword(),
		Unlocked:    a.IsUnlocked(),
		LockedUntil: cfg.CredentialLockedUntil,
		UnlockTime:  cfg.CredentialsUnlockTime,
	}
}

// LockCredentials 立即锁定凭据：将锁定期设为当前时间并清空内存中的密钥缓存。
func (a *App) LockCredentials() {
	log.Info("LockCredentials: success")
	cfg := a.Settings.BackupConfig()
	cfg.CredentialLockedUntil = time.Now().Unix() + 1
	_ = a.Settings.UpdateBackupConfigFromJSON(backupConfigToJSON(cfg))
	a.ensureSecrets().Lock()
}

// 认证辅助方法。

// GenerateToken 生成 32 字节随机令牌并以 base64 字符串形式返回。
// 44 字符的 base64 对 bearer token 绰绰有余，且比 hex 编码更省。
func GenerateToken() string {
	return base64Raw(crypto.GenerateKey())
}

// 错误。

// errPasswordTooShort 在 SetMasterPassword 接收到不足 4 个字符的密码时返回。
var errPasswordTooShort = &httpError{code: "password_too_short", message: "password too short (minimum 4 characters)"}

// httpError 是供内部使用的轻量错误类型，便于在 JSON 响应中生成稳定的错误字符串。
type httpError struct {
	code, message string
}

func (e *httpError) Error() string { return e.message }
