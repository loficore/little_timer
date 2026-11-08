// Package timer 承载计时生命周期的用例规则——启动、恢复、暂停、结束、
// 重置与残留 session 收敛。
//
// 两条 transport（HTTP handler 与 Wails service）只做参数绑定与响应外壳
// 映射；「何时恢复、何时收敛、默认值是多少」只在本包出现一次。
//
// 调用约束：Operations 的所有变更方法都由调用方持有 App 写锁，本包内部
// 不获取该锁，也不回调 App 的加锁方法，避免重复加锁导致死锁。
package timer

import (
	"little-timer/internal/domain"
	"little-timer/internal/storage"
)

// Store 是计时用例需要的存储能力。签名逐字复制 *storage.TimerSessionCrud，
// 让 *storage.TimerSessionCrud 直接满足接口、测试可替换为内存实现。
type Store interface {
	GetTimerSessionByID(sessionID int64) (storage.TimerSessionRow, error)
	CreateTimerSession(habitID *int64, mode string, workDuration, restDuration, loopCount int64) (int64, error)
	UpdateTimerSession(
		sessionID int64,
		elapsedSeconds int64,
		remainingSeconds *int64,
		pausedTotalSeconds int64,
		pauseStartedAt *int64,
		lastSyncedAt *int64,
		isRunning, isPaused, isFinished bool,
		currentRound int64,
		inRest bool,
	) error
	FinishTimerSession(sessionID int64) error
	DeleteTimerSession(sessionID int64) error
	GetActiveTimerSession() (storage.TimerSessionRow, error)
	CreateSession(habitID, durationSeconds, count int64, date string) (int64, error)
}

// Runtime 是 Operations 需要从宿主（*app.App）读取的其余状态：时钟、
// 存储、时区与当前 session/habit 指针。
//
// 指针访问器只读写内存字段；实现方不得在其中加锁——Operations 的调用方
// 已经持有 App 写锁。
type Runtime interface {
	ClockManager() *domain.ClockManager
	TimerStore() Store
	TimezoneOffset() int8
	SessionID() *int64
	SetSessionID(*int64)
	HabitID() *int64
	SetHabitID(*int64)
}
