package timer

import (
	"time"

	"little-timer/internal/domain"
	"little-timer/internal/log"
)

// 计时的默认值：两个 adapter 的请求体缺省时共用同一组常量。
const (
	defaultMode         = "stopwatch"
	defaultWorkDuration = 1500 // 秒
)

// StartInput 是 Start 的参数——两个 adapter 的请求体都映射到这里。
type StartInput struct {
	HabitID      *int64
	Mode         string
	WorkDuration int64
	RestDuration int64
	LoopCount    int64
}

// StartResult 是 Start 的稳定结果。
type StartResult struct {
	Status    string
	HabitID   *int64
	SessionID int64
}

// FinishResult 是 Finish 的稳定结果。兜底路径没有可报告的 session id，
// 此时 SessionID 为 nil。
type FinishResult struct {
	Status         string
	ElapsedSeconds int64
	SessionID      *int64
}

// ProgressResult 是 Progress 的稳定结果：持久化的 session/habit 指针
// 加上时钟的实时字段。
type ProgressResult struct {
	SessionID        *int64
	HabitID          *int64
	Mode             domain.ModeEnum
	IsRunning        bool
	IsPaused         bool
	IsFinished       bool
	InRest           bool
	ElapsedSeconds   int64
	RemainingSeconds int64
}

// Operations 持有计时规则。调用方（HTTP handler / Wails service）负责
// 持有 App 写锁并把握结果映射成各自的响应外壳。
type Operations struct {
	rt Runtime
}

// New 构造绑定到给定 Runtime 的 Operations。
func New(rt Runtime) *Operations {
	return &Operations{rt: rt}
}

// Start 启动或恢复计时。调用方必须持有 App 写锁。
//
// 分支顺序（与旧 HTTP handler 一致）：
//  1. 归一化 mode / 默认工作时长；
//  2. 已有 session 且时钟处于暂停态（未结束，或行本身标记为暂停）→ 恢复同一
//     session，不新建；
//  3. 其余情况（运行中残留、已 finished 未 reset、行取不到）→ 先收敛旧
//     session，再创建全新 session（spec §6 H1）。
func (o *Operations) Start(in StartInput) (StartResult, error) {
	mode := defaultMode
	if in.Mode == "countdown" {
		mode = "countdown"
	}
	work := in.WorkDuration
	if work == 0 {
		work = defaultWorkDuration
	}

	if sessionID := o.rt.SessionID(); sessionID != nil {
		row, err := o.rt.TimerStore().GetTimerSessionByID(*sessionID)
		if err == nil {
			clock := o.rt.ClockManager()
			state := clock.Update()
			if state.IsPaused() && (!state.IsFinished() || row.IsPaused) {
				pausedTotal := row.PausedTotalSeconds
				now := time.Now().Unix()
				if row.PauseStartedAt != nil && now > *row.PauseStartedAt {
					pausedTotal += now - *row.PauseStartedAt
				}
				clock.HandleEvent(domain.UserStartTimerEvent{})
				_ = o.rt.TimerStore().UpdateTimerSession(
					row.ID, row.ElapsedSeconds, row.RemainingSeconds,
					pausedTotal, nil, &now,
					true, false, false,
					row.CurrentRound, row.InRest,
				)
				habitID := in.HabitID
				if habitID == nil && row.HabitID != nil {
					h := *row.HabitID
					habitID = &h
				}
				o.rt.SetHabitID(habitID)
				return StartResult{Status: "started", HabitID: habitID, SessionID: row.ID}, nil
			}
		}
		// 运行中残留 / 过期会话 / 行取不到：收敛后落到下面的新建路径。
		o.convergeStale()
	}

	id, err := o.rt.TimerStore().CreateTimerSession(in.HabitID, mode, work, in.RestDuration, in.LoopCount)
	if err != nil {
		log.Error("timer.session.create failed", "habit_id", in.HabitID, "mode", mode, "error", err.Error())
		return StartResult{}, err
	}
	o.rt.SetSessionID(&id)
	o.rt.SetHabitID(in.HabitID)
	o.rt.ClockManager().HandleEvent(domain.UserStartTimerEvent{})
	log.Info("timer.session.create", "session_id", id, "habit_id", in.HabitID, "mode", mode)
	return StartResult{Status: "started", HabitID: in.HabitID, SessionID: id}, nil
}

// Pause 暂停时钟并把当前进度持久化到活动的 timer_session。调用方必须
// 持有 App 写锁。
func (o *Operations) Pause() {
	o.rt.ClockManager().HandleEvent(domain.UserPauseTimerEvent{})
	o.saveProgress()
}

// Reset 清除当前 session/habit 指针、删除 timer_session 行，并把时钟复位。
// 调用方必须持有 App 写锁。
func (o *Operations) Reset() {
	o.resetSession()
	o.rt.ClockManager().HandleEvent(domain.UserResetTimerEvent{})
}

// resetSession 清指针并删除当前 timer_session 行。调用方必须持有 App 写锁。
func (o *Operations) resetSession() {
	sessionID := o.rt.SessionID()
	o.rt.SetSessionID(nil)
	habitID := o.rt.HabitID()
	o.rt.SetHabitID(nil)
	if sessionID != nil {
		_ = o.rt.TimerStore().DeleteTimerSession(*sessionID)
	}
	log.Info("timer.session.reset", "session_id", sessionID, "habit_id", habitID)
}

// saveProgress 把当前时钟状态写回活动的 timer_session 行。调用方必须持有
// App 写锁。
func (o *Operations) saveProgress() {
	sessionID := o.rt.SessionID()
	if sessionID == nil {
		return
	}
	store := o.rt.TimerStore()
	state := o.rt.ClockManager().Update()
	now := time.Now().Unix()
	row, err := store.GetTimerSessionByID(*sessionID)
	if err != nil {
		log.Error("timer.session.save_progress failed", "session_id", *sessionID, "error", err.Error())
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
	_ = store.UpdateTimerSession(
		row.ID, state.GetElapsedSeconds(), &remaining,
		pausedTotal, pauseStarted, &now,
		isRunning, isPaused, state.IsFinished(),
		row.CurrentRound, state.InRest(),
	)
	log.Info("timer.session.save_progress", "session_id", row.ID, "elapsed", state.GetElapsedSeconds())
}

// convergeStale 收敛内存中残留的 timer_session（spec §6 H1 跨项目泄漏）。//
//   - 无残留指针时只清 habit 指针。
//   - 行取不到（可能已被删除）时只清指针，不做 DB 操作。
//   - 能读到行则标记为已结束；elapsed == 0（从未真正使用）时直接删除，避免
//     以「已完成」形态残留。
//   - 收敛绝不写入 sessions（今日统计）表——残留不是用户主动完成。
//   - 最后清空两个指针。
//
// 调用方必须持有 App 写锁。
func (o *Operations) convergeStale() {
	store := o.rt.TimerStore()
	sessionID := o.rt.SessionID()
	if sessionID == nil {
		o.rt.SetHabitID(nil)
		return
	}

	row, err := store.GetTimerSessionByID(*sessionID)
	if err != nil {
		o.rt.SetSessionID(nil)
		o.rt.SetHabitID(nil)
		log.Info("timer.session.converge", "session_id", *sessionID, "result", "missing_row")
		return
	}

	elapsed := row.ElapsedSeconds
	if err := store.FinishTimerSession(*sessionID); err != nil {
		// 不返回：仍需清指针，避免继续复用残留 session。
		log.Error("timer.session.converge finish failed", "session_id", *sessionID, "error", err.Error())
	}
	if elapsed == 0 {
		if err := store.DeleteTimerSession(*sessionID); err != nil {
			log.Error("timer.session.converge delete failed", "session_id", *sessionID, "error", err.Error())
		}
	}

	o.rt.SetSessionID(nil)
	o.rt.SetHabitID(nil)
	log.Info("timer.session.converge", "session_id", *sessionID, "elapsed", elapsed)
}
