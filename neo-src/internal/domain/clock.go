// Package domain —— 计时运行状态（倒计时/正计时）与
// ClockManager 事件驱动状态机。
package domain

import (
	"context"
	"fmt"
	"math"
	"sync"
	"sync/atomic"
)

// tickCount 是所有 ClockManager 实例共享的全局 tick 计数器。
var tickCount atomic.Uint64

// CountdownState 保存倒计时模式的内存运行状态。
type CountdownState struct {
	DurationMs  uint64
	RemainingMs int64
	Loop        bool
	// LoopIntervalSeconds 是每轮之间的休息时长（秒）。
	LoopIntervalSeconds uint64
	// LoopCount 是配置的循环总次数；0 表示无限。
	LoopCount uint32
	// LoopRemaining 是实时循环计数；0 = 无限。
	LoopRemaining   uint32
	LoopCompleted   bool
	InRest          bool
	RestRemainingMs int64
	// IsPaused 控制 Tick()；true 表示时钟被挂起。
	IsPaused   bool
	IsFinished bool
	// StartTimeMs 是时钟最近一次启动/恢复时的墙钟毫秒。
	StartTimeMs int64
	// PausedMs 是累计暂停的墙钟毫秒。
	PausedMs int64
	// ElapsedAtPause 是最近一次暂停时的已用毫秒。
	ElapsedAtPause int64
}

// Tick 将倒计时推进 deltaMs 毫秒。
func (c *CountdownState) Tick(deltaMs int64) {
	if c.IsPaused || c.IsFinished {
		return
	}
	if deltaMs < 0 {
		// 负增量直接忽略；日志归上层负责，让 domain 层保持无 I/O。
		return
	}
	if deltaMs == 0 {
		return
	}

	// 休息阶段：只有 RestRemainingMs 在倒数；主计数器在休息结束时重置。
	if c.InRest {
		c.RestRemainingMs -= deltaMs
		if c.RestRemainingMs <= 0 {
			c.InRest = false
			c.IsFinished = false
			c.RemainingMs = int64(c.DurationMs)
		}
		return
	}

	c.RemainingMs -= deltaMs
	if c.RemainingMs <= 0 {
		c.RemainingMs = 0
		c.IsFinished = true

		if c.Loop {
			// 有限循环：递减剩余次数，归零时停止。
			if c.LoopCount > 0 {
				if c.LoopRemaining > 0 {
					c.LoopRemaining--
				}
				if c.LoopRemaining == 0 {
					c.LoopCompleted = true
					return
				}
			}
			// 无限循环（LoopCount == 0）或仍有剩余轮次：
			// 要么进入休息阶段，要么直接进入下一轮。
			if c.LoopIntervalSeconds > 0 {
				c.InRest = true
				c.RestRemainingMs = int64(c.LoopIntervalSeconds * 1000)
				c.IsFinished = false
			} else {
				c.IsFinished = false
				c.RemainingMs = int64(c.DurationMs)
			}
		}
	}
}

// StopwatchState 保存正计时模式的内存运行状态。
type StopwatchState struct {
	ElapsedMs int64
	// MaxMs 是正计时的上限；ElapsedMs >= MaxMs 后 IsFinished 置真。
	MaxMs int64
	// IsPaused 控制 Tick()。
	IsPaused   bool
	IsFinished bool
	// StartTimeMs 是正计时最近一次启动时的墙钟毫秒。
	StartTimeMs int64
	// PausedMs 是累计暂停的墙钟毫秒。
	PausedMs int64
	// ElapsedAtPause 是最近一次暂停时的已用毫秒。
	ElapsedAtPause int64
}

// Tick 将正计时推进 deltaMs 毫秒，到达 MaxMs 时封顶并标记完成。
func (s *StopwatchState) Tick(deltaMs int64) {
	if s.IsPaused || s.IsFinished {
		return
	}
	if deltaMs < 0 {
		return
	}
	if deltaMs == 0 {
		return
	}

	s.ElapsedMs += deltaMs
	if s.ElapsedMs >= s.MaxMs {
		s.ElapsedMs = s.MaxMs
		s.IsFinished = true
	}
}

// ClockState 保存当前计时模式及其对应状态。
type ClockState struct {
	// Mode 是判别字段 —— 必须与 Countdown / Stopwatch 中非 nil 的那个保持同步。
	Mode ModeEnum
	// Countdown 非 nil 当且仅当 Mode == CountdownMode。
	Countdown *CountdownState
	// Stopwatch 非 nil 当且仅当 Mode == StopwatchMode。
	Stopwatch *StopwatchState
}

// GetTimeInfo 返回当前秒级时间信息：倒计时为剩余秒数，正计时为已用秒数。
func (s *ClockState) GetTimeInfo() int64 {
	switch s.Mode {
	case CountdownMode:
		if s.Countdown == nil {
			return 0
		}
		return s.Countdown.RemainingMs / 1000
	case StopwatchMode:
		if s.Stopwatch == nil {
			return 0
		}
		return s.Stopwatch.ElapsedMs / 1000
	default:
		return 0
	}
}

// GetMode 返回当前激活的计时模式。
func (s *ClockState) GetMode() ModeEnum { return s.Mode }

// IsPaused 返回当前状态是否处于暂停。
func (s *ClockState) IsPaused() bool {
	switch s.Mode {
	case CountdownMode:
		return s.Countdown != nil && s.Countdown.IsPaused
	case StopwatchMode:
		return s.Stopwatch != nil && s.Stopwatch.IsPaused
	default:
		return true
	}
}

// IsFinished 返回当前状态是否已结束。
func (s *ClockState) IsFinished() bool {
	switch s.Mode {
	case CountdownMode:
		return s.Countdown != nil && s.Countdown.IsFinished
	case StopwatchMode:
		return s.Stopwatch != nil && s.Stopwatch.IsFinished
	default:
		return false
	}
}

// InRest 返回倒计时是否处于循环间的休息阶段。正计时始终为 false。
func (s *ClockState) InRest() bool {
	if s.Mode != CountdownMode || s.Countdown == nil {
		return false
	}
	return s.Countdown.InRest
}

// GetRestRemainingTime 返回休息阶段剩余秒数。正计时始终返回 0。
func (s *ClockState) GetRestRemainingTime() int64 {
	if s.Mode != CountdownMode || s.Countdown == nil {
		return 0
	}
	return s.Countdown.RestRemainingMs / 1000
}

// GetLoopRemaining 返回实时剩余循环次数，0 表示无限循环。正计时始终返回 0。
func (s *ClockState) GetLoopRemaining() uint32 {
	if s.Mode != CountdownMode || s.Countdown == nil {
		return 0
	}
	return s.Countdown.LoopRemaining
}

// GetLoopTotal 返回配置的循环总次数，0 表示无限循环。正计时始终返回 0。
func (s *ClockState) GetLoopTotal() uint32 {
	if s.Mode != CountdownMode || s.Countdown == nil {
		return 0
	}
	return s.Countdown.LoopCount
}

// GetElapsedSeconds 计算已用时间（整秒），按 `(now - start - paused) / 1000` 公式。
func (s *ClockState) GetElapsedSeconds() int64 {
	now := NowMs()
	switch s.Mode {
	case CountdownMode:
		if s.Countdown == nil {
			return 0
		}
		c := s.Countdown
		return (now - c.StartTimeMs - c.PausedMs) / 1000
	case StopwatchMode:
		if s.Stopwatch == nil {
			return 0
		}
		w := s.Stopwatch
		return (now - w.StartTimeMs - w.PausedMs) / 1000
	default:
		return 0
	}
}

// GetRemainingSeconds 计算倒计时的实时剩余秒数，正计时返回 0。
func (s *ClockState) GetRemainingSeconds() int64 {
	if s.Mode != CountdownMode || s.Countdown == nil {
		return 0
	}
	now := NowMs()
	c := s.Countdown
	elapsedMs := now - c.StartTimeMs - c.PausedMs
	remainingMs := int64(c.DurationMs) - elapsedMs
	return remainingMs / 1000
}

// GetCurrentRound 返回循环倒计时的当前轮次。正计时始终返回 0。
func (s *ClockState) GetCurrentRound() int64 {
	if s.Mode != CountdownMode || s.Countdown == nil {
		return 0
	}
	return int64(s.Countdown.LoopCount) - int64(s.Countdown.LoopRemaining) + 1
}

// ClockManager 管理当前计时状态、复位用的初始配置快照以及事件通道。
//
// 状态由 `mu` 保护：Run() 在消费者 goroutine 上改写它，而调用方通过
// Update() 在其他 goroutine（http 层、UI、统计）读取它。跳过 mutex 会在
// 生产中留下真实的数据竞争 —— `-race` 能在下面的事件总线测试里抓到。
type ClockManager struct {
	mu            sync.Mutex
	state         ClockState
	initialConfig ClockTaskConfig

	// events 是异步事件总线。生产方向 Events() 发送 ClockEvent，
	// Run() 在独立 goroutine 中排空。测试改用同步的 HandleEvent
	// 路径以避免竞争。
	events chan ClockEvent

	closeOnce sync.Once
}

// NewClockManager 根据 ClockTaskConfig 构造（但不启动）一个 ClockManager。
// 它会校验时长是否溢出 int64，必要时回退到安全默认值。
func NewClockManager(cfg ClockTaskConfig) *ClockManager {
	if durationOverflows(cfg.Countdown.DurationSeconds) ||
		durationOverflows(cfg.Stopwatch.MaxSeconds) {
		// 静默替换为安全配置；日志归 log 层负责。
		cfg = ClockTaskConfig{
			Countdown: CountdownConfig{
				DurationSeconds:     25 * Minute,
				Loop:                false,
				LoopIntervalSeconds: 0,
				LoopCount:           0,
			},
			Stopwatch: StopwatchConfig{
				MaxSeconds: 24 * Hour,
			},
		}
	}
	return &ClockManager{
		state:         buildInitialState(cfg),
		initialConfig: cfg,
		events:        make(chan ClockEvent, 64),
	}
}

// Init 为兼容规范生命周期的空操作，实际构造在 NewClockManager 中完成。
func (m *ClockManager) Init() {}

// Update 返回当前 ClockState 的指针，仅供只读检查使用。
// 指针只在下一次 HandleEvent 调用前有效；需要稳定视图请把字段拷出。
// 并发场景下的调用方必须自己持有 m.mu —— Update 不会把锁随指针交还。
func (m *ClockManager) Update() *ClockState { return &m.state }

// Deinit 关闭事件通道，使用 sync.Once 保证可安全重复调用。
// 测试可以不带 `defer Deinit()` 手动关闭通道。
func (m *ClockManager) Deinit() {
	m.closeOnce.Do(func() {
		if m.events != nil {
			close(m.events)
		}
	})
}

// Events 返回事件总线的写入端，供生产方（http 层、用户操作等）发送 ClockEvent。
func (m *ClockManager) Events() chan<- ClockEvent { return m.events }

// Run 持续消费事件总线，直到 ctx 取消或通道关闭。
// 每次事件派发都持有状态 mutex，保证 Update() 的并发读者看到一致状态。
func (m *ClockManager) Run(ctx context.Context, events <-chan ClockEvent) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case ev, ok := <-events:
			if !ok {
				return nil
			}
			m.mu.Lock()
			m.handleEvent(ev)
			m.mu.Unlock()
		}
	}
}

// HandleEvent 同步处理单个事件，绕过事件通道直接派发。
// 适用于与消费者 goroutine 存在竞争问题的测试场景，也适用于生产方已经
// 拿到事件的地方。整个改写过程持有状态 mutex。
func (m *ClockManager) HandleEvent(ev ClockEvent) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.handleEvent(ev)
}

// 内部辅助函数。

// durationOverflows 在 `durationSeconds * 1000` 会溢出 int64 时返回 true。
func durationOverflows(durationSeconds uint64) bool {
	const maxSafeDuration = uint64(math.MaxInt64) / 1000
	return durationSeconds > maxSafeDuration
}

// buildInitialState 根据给定配置构造 ClockState。
func buildInitialState(cfg ClockTaskConfig) ClockState {
	switch cfg.DefaultMode {
	case CountdownMode:
		durMs := cfg.Countdown.DurationSeconds * 1000
		return ClockState{
			Mode: CountdownMode,
			Countdown: &CountdownState{
				DurationMs:          durMs,
				RemainingMs:         int64(durMs),
				Loop:                cfg.Countdown.Loop,
				LoopIntervalSeconds: cfg.Countdown.LoopIntervalSeconds,
				LoopCount:           cfg.Countdown.LoopCount,
				LoopRemaining:       cfg.Countdown.LoopCount,
				IsPaused:            true,
			},
		}
	case StopwatchMode:
		maxMs := cfg.Stopwatch.MaxSeconds * 1000
		return ClockState{
			Mode: StopwatchMode,
			Stopwatch: &StopwatchState{
				ElapsedMs: 0,
				MaxMs:     int64(maxMs),
				IsPaused:  true,
			},
		}
	default:
		// switch 已覆盖所有 ModeEnum 取值；这个兜底分支只为让函数
		// 可编译并安全降级。回退到暂停的 25 分钟倒计时。
		durMs := uint64(25 * Minute * 1000)
		return ClockState{
			Mode: CountdownMode,
			Countdown: &CountdownState{
				DurationMs:  durMs,
				RemainingMs: int64(durMs),
				IsPaused:    true,
			},
		}
	}
}

// handleEvent 把 ClockEvent 派发到对应的状态改写。
func (m *ClockManager) handleEvent(event ClockEvent) {
	switch ev := event.(type) {
	case TickEvent:
		// 递增共享 tick 计数；诊断信息不进 domain 层。
		newCount := tickCount.Add(1)
		_ = newCount // 为将来的诊断预留的占位
		m.onTick(ev.DeltaMs)

	case UserStartTimerEvent:
		now := NowMs()
		switch m.state.Mode {
		case CountdownMode:
			if c := m.state.Countdown; c.IsPaused {
				if c.StartTimeMs == 0 {
					c.StartTimeMs = now
				} else {
					c.StartTimeMs = now - c.ElapsedAtPause - c.PausedMs
				}
				c.IsPaused = false
			}
		case StopwatchMode:
			if w := m.state.Stopwatch; w.IsPaused {
				if w.StartTimeMs == 0 {
					w.StartTimeMs = now
				} else {
					w.StartTimeMs = now - w.ElapsedAtPause - w.PausedMs
				}
				w.IsPaused = false
			}
		}

	case UserPauseTimerEvent:
		now := NowMs()
		switch m.state.Mode {
		case CountdownMode:
			if c := m.state.Countdown; !c.IsPaused {
				c.ElapsedAtPause = now - c.StartTimeMs - c.PausedMs
				c.IsPaused = true
			}
		case StopwatchMode:
			if w := m.state.Stopwatch; !w.IsPaused {
				w.ElapsedAtPause = now - w.StartTimeMs - w.PausedMs
				w.IsPaused = true
			}
		}

	case UserResetTimerEvent:
		switch m.state.Mode {
		case CountdownMode:
			c := m.state.Countdown
			c.StartTimeMs = 0
			c.PausedMs = 0
			c.ElapsedAtPause = 0
			c.RemainingMs = int64(m.initialConfig.Countdown.DurationSeconds * 1000)
			c.LoopRemaining = m.initialConfig.Countdown.LoopCount
			c.LoopCompleted = false
			c.InRest = false
			c.RestRemainingMs = 0
			c.IsPaused = true
			c.IsFinished = false
		case StopwatchMode:
			w := m.state.Stopwatch
			w.StartTimeMs = 0
			w.PausedMs = 0
			w.ElapsedAtPause = 0
			w.ElapsedMs = 0
			w.IsPaused = true
			w.IsFinished = false
		}

	case UserFinishTimerEvent:
		switch m.state.Mode {
		case CountdownMode:
			c := m.state.Countdown
			c.IsPaused = true
			c.IsFinished = true
		case StopwatchMode:
			w := m.state.Stopwatch
			w.IsPaused = true
			w.IsFinished = true
		}

	case UserChangeModeEvent:
		// 这里有意忽略 initialConfig；使用硬编码默认值：
		// 25 分钟倒计时 / 24 小时正计时。
		switch ev.Mode {
		case CountdownMode:
			const durSec uint64 = 25 * Minute
			durMs := durSec * 1000
			m.state = ClockState{
				Mode: CountdownMode,
				Countdown: &CountdownState{
					DurationMs:          durMs,
					RemainingMs:         int64(durMs),
					Loop:                false,
					LoopIntervalSeconds: 0,
					LoopCount:           0,
					LoopRemaining:       0,
					LoopCompleted:       false,
					IsPaused:            true,
					StartTimeMs:         0,
					PausedMs:            0,
					ElapsedAtPause:      0,
				},
			}
			m.initialConfig = ClockTaskConfig{
				Countdown: CountdownConfig{
					DurationSeconds:     durSec,
					Loop:                false,
					LoopIntervalSeconds: 0,
					LoopCount:           0,
				},
				Stopwatch: StopwatchConfig{
					MaxSeconds: 24 * Hour,
				},
			}
		case StopwatchMode:
			const maxSec uint64 = 24 * Hour
			maxMs := maxSec * 1000
			m.state = ClockState{
				Mode: StopwatchMode,
				Stopwatch: &StopwatchState{
					ElapsedMs:      0,
					MaxMs:          int64(maxMs),
					IsPaused:       true,
					StartTimeMs:    0,
					PausedMs:       0,
					ElapsedAtPause: 0,
				},
			}
			m.initialConfig = ClockTaskConfig{
				Countdown: CountdownConfig{
					DurationSeconds:     25 * Minute,
					Loop:                false,
					LoopIntervalSeconds: 0,
					LoopCount:           0,
				},
				Stopwatch: StopwatchConfig{
					MaxSeconds: maxSec,
				},
			}
		}

	case UserChangeConfigEvent:
		newCfg := ev.Config
		m.state = buildInitialState(newCfg)
		m.initialConfig = newCfg

	default:
		// sealed interface 保证我们认识每一种变体；走到这里的 panic
		// 说明生产方存在编程错误，而不是值得吞掉的运行时状况。
		panic(fmt.Sprintf("clock: unknown ClockEvent variant %T", event))
	}
}

// onTick 把 TickEvent 的增量转交给当前激活变体的 Tick 方法。
func (m *ClockManager) onTick(deltaMs int64) {
	switch m.state.Mode {
	case CountdownMode:
		m.state.Countdown.Tick(deltaMs)
	case StopwatchMode:
		m.state.Stopwatch.Tick(deltaMs)
	}
}
