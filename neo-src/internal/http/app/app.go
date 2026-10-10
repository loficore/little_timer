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
	"os"
	"path/filepath"

	"sync"
	"time"

	"little-timer/internal/crypto"
	"little-timer/internal/domain"
	"little-timer/internal/keystore"
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

	// Keystore 是注入的 master key 托管(供 Secrets()/CredentialStatus 惰性
	// 建 store 时使用)。为 nil 时 ensureSecrets 回退到 keystore.Current(),
	// 便于测试替换、同时不让生产引导路径遗漏接线;启动期 fail-fast 校验由
	// Task 6 落地。
	Keystore keystore.Keystore

	// CurrentHabitID 是当前计时关联的习惯 ID。
	CurrentHabitID *int64
	// CurrentTimerSessionID 是当前计时会话 ID。
	CurrentTimerSessionID *int64

	// secrets 是进程内凭据存储，由 secretsOnce 惰性创建，因此调用方构造时
	// 可以省略它。每个 App 持有自己的 Once：新建 App 会重新解析 keystore。
	secretsOnce sync.Once
	secrets     *crypto.SecretStorage
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
//
// 创建前先幂等收敛既有残留会话(ConvergeStaleSessionIfAny),行为契约 =
// 「调用后 CurrentTimerSessionID 指向且仅指向新 session」。这挡住跨项目
// 共享后端进程时 App 内存态泄漏导致新 start 复用/误收敛旧 session
// (spec §6 H1)。
func (a *App) CreateTimerSession(habitID *int64, mode string, work, rest, loop int64) (int64, error) {
	a.ConvergeStaleSessionIfAny()

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

// ConvergeStaleSessionIfAny 收敛内存中残留的 timer_session(spec §6 H1
// 跨项目泄漏)。语义:
//   - 无残留指针时无操作。
//   - 有残留时先取回行读取 elapsed,再把它标记为已结束
//     (SQLite.Timers().FinishTimerSession);若 elapsed==0(该 session 从未
//     被真正使用)则直接 DeleteTimerSession 删除,避免以「已完成」形态
//     残留在 timer_sessions。
//   - 收敛只清 timer_sessions 状态,**绝不写入 sessions(今日统计)表** ——
//     残留会话不是用户主动完成,不应计入统计。
//   - 最后清空 CurrentTimerSessionID / CurrentHabitID。
//
// 调用方必须持有 a.mu 写锁。
func (a *App) ConvergeStaleSessionIfAny() {
	sessionID := a.CurrentTimerSessionID
	if sessionID == nil {
		a.CurrentHabitID = nil
		return
	}

	row, err := a.SQLite.Timers().GetTimerSessionByID(*sessionID)
	if err != nil {
		// 行取不到(可能已被删除):仅清指针,无需 DB 操作。
		a.CurrentTimerSessionID = nil
		a.CurrentHabitID = nil
		log.Info("timer.session.converge", "session_id", *sessionID, "result", "missing_row")
		return
	}

	elapsed := row.ElapsedSeconds
	if err := a.SQLite.Timers().FinishTimerSession(*sessionID); err != nil {
		// 不返回:仍需清指针,避免继续复用残留 session。
		log.Error("timer.session.converge finish failed", "session_id", *sessionID, "error", err.Error())
	}
	if elapsed == 0 {
		if err := a.SQLite.Timers().DeleteTimerSession(*sessionID); err != nil {
			log.Error("timer.session.converge delete failed", "session_id", *sessionID, "error", err.Error())
		}
	}

	a.CurrentTimerSessionID = nil
	a.CurrentHabitID = nil
	log.Info("timer.session.converge", "session_id", *sessionID, "elapsed", elapsed)
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

// 凭据辅助方法。SecretStorage 由平台 keystore 托管 master key,不再有
// 主口令 / 锁的概念;SettingsManager 的 WebDAV/S3 凭据分流依赖这里的接线。

// secretStorePath 返回与 DB 同级的 secret blob 路径。裸文件名 DB 路径
// （Dir 为 "" 或 "."）解析为当前目录下的 secret.db。
func secretStorePath(dbPath string) string {
	return filepath.Join(filepath.Dir(dbPath), "secret.db")
}

// secretFileExists 报告磁盘上是否已存在 secret blob（用于区分"新装"与
// "有历史凭据但读不出来"）。
func secretFileExists(dbPath string) bool {
	_, err := os.Stat(secretStorePath(dbPath))
	return err == nil
}

// ensureSecrets 惰性构造 SecretStorage，并把同一个实例接线到 SettingsManager
// （后者据此把 WebDAV/S3 凭据分流到 keystore Secrets，见 Task 4）。keystore
// 取自 a.Keystore，未注入时回退 keystore.Current()。构造由 sync.Once 保护：
// 并发首调只建一次、只接线一次，且 happens-before 保证后续读 a.secrets 无竞争。
func (a *App) ensureSecrets() *crypto.SecretStorage {
	a.secretsOnce.Do(func() {
		ks := a.Keystore
		if ks == nil {
			ks = keystore.Current()
		}
		a.secrets = crypto.NewSecretStorage(secretStorePath(a.DBPath), ks)
		if a.Settings != nil {
			a.Settings.SetSecretStore(a.secrets)
		}
	})
	return a.secrets
}

// Secrets 返回 SecretStorage（惰性初始化 + 接线 settings）。供需要存 / 取受
// keystore 保护的 secret 的调用方使用（例如 LLM API Key、备份凭据）。
func (a *App) Secrets() *crypto.SecretStorage {
	return a.ensureSecrets()
}

// activeKeystore 返回当前生效的 keystore：注入优先，否则回退平台实现。
func (a *App) activeKeystore() keystore.Keystore {
	if a.Keystore != nil {
		return a.Keystore
	}
	return keystore.Current()
}

// CredentialStatus 概览凭据子系统健康度。载入失败态（磁盘有 blob 但 master
// key 缺失 / 解封失败）透出 Backend="unavailable" + 非空 Detail，绝不假装
// "没有凭据"；否则取生效 keystore 的 Name()，HasSecret 反映 store 内是否
// 已有 secret。Detail / 日志只带错误文本，绝不含凭据值。
func (a *App) CredentialStatus() domain.CredentialStatus {
	store := a.ensureSecrets()
	if lf := store.LoadFailure(); lf != nil {
		return domain.CredentialStatus{
			Backend:   "unavailable",
			Detail:    lf.Error(),
			HasSecret: secretFileExists(a.DBPath),
		}
	}
	ks := a.activeKeystore()
	backend := "unavailable"
	detail := ""
	if ks != nil {
		backend = ks.Name()
	} else {
		// 维持 "unavailable ⇒ Detail 非空" 不变式:平台 keystore 缺失时
		// 也要透出原因,绝不留下空的不可用说明。
		detail = "平台 keystore 未注册"
	}
	items, err := store.Items()
	return domain.CredentialStatus{
		Backend:   backend,
		Detail:    detail,
		HasSecret: err == nil && len(items) > 0,
	}
}

// 认证辅助方法。

// GenerateToken 生成 32 字节随机令牌并以 base64 字符串形式返回。
// 44 字符的 base64 对 bearer token 绰绰有余，且比 hex 编码更省。
func GenerateToken() string {
	return base64Raw(crypto.GenerateKey())
}
