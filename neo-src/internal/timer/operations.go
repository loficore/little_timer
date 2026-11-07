package timer

import "little-timer/internal/domain"

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
