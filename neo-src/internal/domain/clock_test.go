package domain

import (
	"context"
	"reflect"
	"testing"
	"time"
)

func TestTodayStringOffsetArithmetic(t *testing.T) {
	tests := []struct {
		name   string
		offset int8
		want   string
	}{
		{name: "UTC-8", offset: -8, want: "2024-01-14"},
		{name: "UTC", offset: 0, want: "2024-01-15"},
		{name: "UTC+8", offset: 8, want: "2024-01-15"},
		{name: "boundary crosses into next date", offset: 8, want: "2024-01-16"},
	}
	times := []time.Time{
		time.Date(2024, time.January, 15, 0, 30, 0, 0, time.UTC),
		time.Date(2024, time.January, 15, 12, 0, 0, 0, time.UTC),
		time.Date(2024, time.January, 15, 0, 30, 0, 0, time.UTC),
		time.Date(2024, time.January, 15, 23, 30, 0, 0, time.UTC),
	}
	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			previousNow := now
			now = func() time.Time { return times[i] }
			defer func() { now = previousNow }()

			got := TodayString(tt.offset)
			if got != tt.want {
				t.Errorf("offset %d: got %q, want %q", tt.offset, got, tt.want)
			}
		})
	}
}

// 小型断言辅助函数 —— 只用标准库，省去每行
// `if got != want { t.Errorf(...) }` 的噪音。

func mustEqual(t *testing.T, name string, got, want any) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("%s: got %v, want %v", name, got, want)
	}
}

func mustTrue(t *testing.T, name string, got bool) {
	t.Helper()
	if !got {
		t.Errorf("%s: expected true", name)
	}
}

func mustFalse(t *testing.T, name string, got bool) {
	t.Helper()
	if got {
		t.Errorf("%s: expected false", name)
	}
}

// 常用配置构造函数。

func cfgCountdownOnly(durationSeconds uint64) ClockTaskConfig {
	return ClockTaskConfig{
		DefaultMode: CountdownMode,
		Countdown: CountdownConfig{
			DurationSeconds:     durationSeconds,
			Loop:                false,
			LoopIntervalSeconds: 0,
			LoopCount:           0,
		},
		Stopwatch: StopwatchConfig{
			MaxSeconds: 3600,
		},
	}
}

func cfgCountdownLoop(durationSeconds uint64, loopCount uint32, loopInterval uint64) ClockTaskConfig {
	return ClockTaskConfig{
		DefaultMode: CountdownMode,
		Countdown: CountdownConfig{
			DurationSeconds:     durationSeconds,
			Loop:                true,
			LoopIntervalSeconds: loopInterval,
			LoopCount:           loopCount,
		},
		Stopwatch: StopwatchConfig{
			MaxSeconds: 3600,
		},
	}
}

func cfgStopwatch(maxSeconds uint64) ClockTaskConfig {
	return ClockTaskConfig{
		DefaultMode: StopwatchMode,
		Countdown:   NewDefaultCountdownConfig(),
		Stopwatch: StopwatchConfig{
			MaxSeconds: maxSeconds,
		},
	}
}

// 倒计时测试。

func TestCountdownInit(t *testing.T) {
	m := NewClockManager(cfgCountdownOnly(60))
	defer m.Deinit()

	c := m.state.Countdown
	if c == nil {
		t.Fatal("expected non-nil Countdown after init")
	}
	mustEqual(t, "RemainingMs", c.RemainingMs, int64(60000))
	mustTrue(t, "IsPaused", c.IsPaused)
	mustFalse(t, "IsFinished", c.IsFinished)
}

func TestCountdownBasicTick(t *testing.T) {
	m := NewClockManager(cfgCountdownOnly(60))
	defer m.Deinit()

	m.HandleEvent(UserStartTimerEvent{})
	m.HandleEvent(TickEvent{DeltaMs: 1000})

	c := m.state.Countdown
	mustEqual(t, "RemainingMs after 1s tick", c.RemainingMs, int64(59000))
	mustFalse(t, "IsFinished", c.IsFinished)
}

func TestCountdownPauseAndResume(t *testing.T) {
	m := NewClockManager(cfgCountdownOnly(10))
	defer m.Deinit()

	m.HandleEvent(UserStartTimerEvent{})
	m.HandleEvent(TickEvent{DeltaMs: 5000}) // 已用 5s

	m.HandleEvent(UserPauseTimerEvent{})
	mustTrue(t, "IsPaused after pause", m.state.Countdown.IsPaused)

	beforePause := m.state.Countdown.RemainingMs
	m.HandleEvent(TickEvent{DeltaMs: 3000}) // 暂停期间被忽略
	mustEqual(t, "RemainingMs unchanged while paused", m.state.Countdown.RemainingMs, beforePause)

	m.HandleEvent(UserStartTimerEvent{})
	mustFalse(t, "IsPaused after resume", m.state.Countdown.IsPaused)
	m.HandleEvent(TickEvent{DeltaMs: 2000})
	mustEqual(t, "RemainingMs after resume + tick", m.state.Countdown.RemainingMs, beforePause-2000)
}

func TestCountdownCompletion(t *testing.T) {
	m := NewClockManager(cfgCountdownOnly(5))
	defer m.Deinit()

	m.HandleEvent(UserStartTimerEvent{})
	m.HandleEvent(TickEvent{DeltaMs: 5000})

	mustTrue(t, "IsFinished", m.state.Countdown.IsFinished)
	mustEqual(t, "RemainingMs", m.state.Countdown.RemainingMs, int64(0))
}

func TestCountdownNeverGoesNegative(t *testing.T) {
	m := NewClockManager(cfgCountdownOnly(5))
	defer m.Deinit()

	m.HandleEvent(UserStartTimerEvent{})
	m.HandleEvent(TickEvent{DeltaMs: 10000})

	mustEqual(t, "RemainingMs clamps to zero", m.state.Countdown.RemainingMs, int64(0))
	mustTrue(t, "IsFinished", m.state.Countdown.IsFinished)
}

func TestCountdownReset(t *testing.T) {
	m := NewClockManager(cfgCountdownOnly(30))
	defer m.Deinit()

	m.HandleEvent(UserStartTimerEvent{})
	m.HandleEvent(TickEvent{DeltaMs: 10000})

	m.HandleEvent(UserResetTimerEvent{})

	c := m.state.Countdown
	mustEqual(t, "RemainingMs after reset", c.RemainingMs, int64(30000))
	mustTrue(t, "IsPaused after reset", c.IsPaused)
	mustFalse(t, "IsFinished after reset", c.IsFinished)
}

func TestCountdownLoopFiniteNoRest(t *testing.T) {
	m := NewClockManager(cfgCountdownLoop(5, 2, 0))
	defer m.Deinit()

	mustEqual(t, "initial LoopRemaining", m.state.Countdown.LoopRemaining, uint32(2))

	m.HandleEvent(UserStartTimerEvent{})
	m.HandleEvent(TickEvent{DeltaMs: 5000})

	// 无休息间隔的循环：第 1 轮结束后立即以相同时长
	// 重置进入第 2 轮。
	mustEqual(t, "LoopRemaining after round 1", m.state.Countdown.LoopRemaining, uint32(1))
	mustFalse(t, "IsFinished mid-loop", m.state.Countdown.IsFinished)
	mustEqual(t, "RemainingMs reset for round 2", m.state.Countdown.RemainingMs, int64(5000))

	m.HandleEvent(TickEvent{DeltaMs: 5000}) // 第 2 轮完成
	mustTrue(t, "LoopCompleted after last round", m.state.Countdown.LoopCompleted)
}

func TestCountdownLoopFiniteWithRest(t *testing.T) {
	m := NewClockManager(cfgCountdownLoop(5, 2, 3))
	defer m.Deinit()

	mustEqual(t, "initial LoopRemaining", m.state.Countdown.LoopRemaining, uint32(2))

	m.HandleEvent(UserStartTimerEvent{})
	m.HandleEvent(TickEvent{DeltaMs: 5000}) // 第 1 轮结束 → 开始休息

	mustEqual(t, "LoopRemaining after round 1", m.state.Countdown.LoopRemaining, uint32(1))
	mustFalse(t, "IsFinished during rest", m.state.Countdown.IsFinished)
	mustEqual(t, "RemainingMs during rest", m.state.Countdown.RemainingMs, int64(0))
	mustTrue(t, "InRest", m.state.Countdown.InRest)
	mustTrue(t, "state.InRest()", m.state.InRest())
	mustEqual(t, "GetRestRemainingTime", m.state.GetRestRemainingTime(), int64(3))

	m.HandleEvent(TickEvent{DeltaMs: 1500}) // 休息中
	mustTrue(t, "InRest mid-rest", m.state.Countdown.InRest)
	mustEqual(t, "GetRestRemainingTime mid-rest", m.state.GetRestRemainingTime(), int64(1))

	m.HandleEvent(TickEvent{DeltaMs: 1500}) // 休息完成 → 第 2 轮
	mustFalse(t, "InRest after rest", m.state.Countdown.InRest)
	mustEqual(t, "RemainingMs reset for round 2", m.state.Countdown.RemainingMs, int64(5000))
	mustEqual(t, "LoopRemaining", m.state.Countdown.LoopRemaining, uint32(1))
	mustFalse(t, "IsFinished in round 2", m.state.Countdown.IsFinished)

	m.HandleEvent(TickEvent{DeltaMs: 5000}) // 第 2 轮完成
	mustTrue(t, "LoopCompleted", m.state.Countdown.LoopCompleted)
	mustEqual(t, "RemainingMs", m.state.Countdown.RemainingMs, int64(0))
	mustTrue(t, "IsFinished", m.state.Countdown.IsFinished)
	mustFalse(t, "InRest after finish", m.state.Countdown.InRest)
}

func TestCountdownLoopInfinite(t *testing.T) {
	m := NewClockManager(cfgCountdownLoop(3, 0, 2))
	defer m.Deinit()

	mustEqual(t, "LoopRemaining infinite", m.state.Countdown.LoopRemaining, uint32(0))
	mustEqual(t, "GetLoopTotal infinite", m.state.GetLoopTotal(), uint32(0))

	m.HandleEvent(UserStartTimerEvent{})
	m.HandleEvent(TickEvent{DeltaMs: 3000}) // 第 1 轮结束
	mustTrue(t, "InRest after round 1", m.state.Countdown.InRest)
	mustEqual(t, "LoopRemaining never decremented", m.state.Countdown.LoopRemaining, uint32(0))

	m.HandleEvent(TickEvent{DeltaMs: 2000}) // 休息结束
	mustFalse(t, "InRest after rest", m.state.Countdown.InRest)
	mustEqual(t, "RemainingMs reset", m.state.Countdown.RemainingMs, int64(3000))

	m.HandleEvent(TickEvent{DeltaMs: 3000}) // 第 2 轮结束
	mustTrue(t, "InRest after round 2", m.state.Countdown.InRest)
	mustEqual(t, "LoopRemaining still infinite", m.state.Countdown.LoopRemaining, uint32(0))
	mustFalse(t, "LoopCompleted never true", m.state.Countdown.LoopCompleted)

	m.HandleEvent(TickEvent{DeltaMs: 2000})
	mustFalse(t, "InRest", m.state.Countdown.InRest)
	mustEqual(t, "RemainingMs reset again", m.state.Countdown.RemainingMs, int64(3000))
}

func TestCountdownLoopQueryHelpers(t *testing.T) {
	m := NewClockManager(cfgCountdownLoop(5, 3, 2))
	defer m.Deinit()

	mustEqual(t, "GetLoopRemaining initial", m.state.GetLoopRemaining(), uint32(3))
	mustEqual(t, "GetLoopTotal initial", m.state.GetLoopTotal(), uint32(3))
	mustFalse(t, "InRest initial", m.state.InRest())
	mustEqual(t, "GetRestRemainingTime initial", m.state.GetRestRemainingTime(), int64(0))

	m.HandleEvent(UserStartTimerEvent{})
	m.HandleEvent(TickEvent{DeltaMs: 5000})

	mustTrue(t, "InRest after round 1", m.state.InRest())
	mustEqual(t, "GetRestRemainingTime after round 1", m.state.GetRestRemainingTime(), int64(2))
	mustEqual(t, "GetLoopRemaining after round 1", m.state.GetLoopRemaining(), uint32(2))
	mustEqual(t, "GetLoopTotal unchanged", m.state.GetLoopTotal(), uint32(3))
}

// 正计时测试。

func TestStopwatchInit(t *testing.T) {
	m := NewClockManager(cfgStopwatch(3600))
	defer m.Deinit()

	w := m.state.Stopwatch
	if w == nil {
		t.Fatal("expected non-nil Stopwatch after init")
	}
	mustEqual(t, "ElapsedMs", w.ElapsedMs, int64(0))
	mustTrue(t, "IsPaused", w.IsPaused)
}

func TestStopwatchBasicTick(t *testing.T) {
	m := NewClockManager(cfgStopwatch(3600))
	defer m.Deinit()

	m.HandleEvent(UserStartTimerEvent{})
	m.HandleEvent(TickEvent{DeltaMs: 1000})

	mustEqual(t, "ElapsedMs", m.state.Stopwatch.ElapsedMs, int64(1000))
	mustFalse(t, "IsFinished", m.state.Stopwatch.IsFinished)
}

func TestStopwatchReachesMax(t *testing.T) {
	m := NewClockManager(cfgStopwatch(5))
	defer m.Deinit()

	m.HandleEvent(UserStartTimerEvent{})
	m.HandleEvent(TickEvent{DeltaMs: 5000})

	mustEqual(t, "ElapsedMs caps at MaxMs", m.state.Stopwatch.ElapsedMs, int64(5000))
	mustTrue(t, "IsFinished", m.state.Stopwatch.IsFinished)
}

func TestStopwatchCannotExceedMax(t *testing.T) {
	m := NewClockManager(cfgStopwatch(5))
	defer m.Deinit()

	m.HandleEvent(UserStartTimerEvent{})
	m.HandleEvent(TickEvent{DeltaMs: 6000})

	mustEqual(t, "ElapsedMs clamps at MaxMs", m.state.Stopwatch.ElapsedMs, int64(5000))
}

func TestStopwatchReset(t *testing.T) {
	m := NewClockManager(cfgStopwatch(3600))
	defer m.Deinit()

	m.HandleEvent(UserStartTimerEvent{})
	m.HandleEvent(TickEvent{DeltaMs: 5000})
	m.HandleEvent(UserResetTimerEvent{})

	w := m.state.Stopwatch
	mustEqual(t, "ElapsedMs reset", w.ElapsedMs, int64(0))
	mustTrue(t, "IsPaused", w.IsPaused)
	mustFalse(t, "IsFinished cleared", w.IsFinished)
}

func TestStopwatchPauseIgnoresTicks(t *testing.T) {
	m := NewClockManager(cfgStopwatch(3600))
	defer m.Deinit()

	m.HandleEvent(UserStartTimerEvent{})
	m.HandleEvent(TickEvent{DeltaMs: 2000})

	m.HandleEvent(UserPauseTimerEvent{})
	mustTrue(t, "IsPaused", m.state.Stopwatch.IsPaused)

	before := m.state.Stopwatch.ElapsedMs
	m.HandleEvent(TickEvent{DeltaMs: 3000}) // 被忽略
	mustEqual(t, "ElapsedMs unchanged while paused", m.state.Stopwatch.ElapsedMs, before)
}

func TestStopwatchFinishedIgnoresFurtherTicks(t *testing.T) {
	m := NewClockManager(cfgStopwatch(5))
	defer m.Deinit()

	m.HandleEvent(UserStartTimerEvent{})
	m.HandleEvent(TickEvent{DeltaMs: 5000})
	mustTrue(t, "IsFinished at cap", m.state.Stopwatch.IsFinished)

	m.HandleEvent(TickEvent{DeltaMs: 2000})
	mustEqual(t, "ElapsedMs does not grow past cap", m.state.Stopwatch.ElapsedMs, int64(5000))
	mustTrue(t, "IsFinished still true", m.state.Stopwatch.IsFinished)
}

// 模式切换测试。

func TestModeSwitchCountdownToStopwatch(t *testing.T) {
	m := NewClockManager(cfgCountdownOnly(60))
	defer m.Deinit()

	newCfg := cfgStopwatch(3600)
	m.HandleEvent(UserChangeConfigEvent{Config: newCfg})

	w := m.state.Stopwatch
	if w == nil {
		t.Fatal("expected non-nil Stopwatch after config change")
	}
	mustEqual(t, "ElapsedMs", w.ElapsedMs, int64(0))
}

func TestUserChangeModeUsesHardcodedDefaults(t *testing.T) {
	m := NewClockManager(cfgCountdownOnly(60))
	defer m.Deinit()

	m.HandleEvent(UserChangeModeEvent{Mode: StopwatchMode})
	mustEqual(t, "Stopwatch MaxMs uses 24h default",
		m.state.Stopwatch.MaxMs, int64(24*Hour*1000))
	mustEqual(t, "Stopwatch ElapsedMs", m.state.Stopwatch.ElapsedMs, int64(0))

	m.HandleEvent(UserChangeModeEvent{Mode: CountdownMode})
	mustEqual(t, "Countdown DurationMs uses 25m default",
		m.state.Countdown.DurationMs, uint64(25*Minute*1000))
}

func TestUserChangeConfigUpdatesInitialConfig(t *testing.T) {
	m := NewClockManager(cfgCountdownOnly(60))
	defer m.Deinit()

	m.HandleEvent(UserStartTimerEvent{})
	m.HandleEvent(TickEvent{DeltaMs: 30000})

	newCfg := cfgCountdownOnly(120)
	m.HandleEvent(UserChangeConfigEvent{Config: newCfg})

	mustEqual(t, "RemainingMs after config change",
		m.state.Countdown.RemainingMs, int64(120000))

	// 复位后应回到新的初始配置（120 秒），
	// 而不是最初的 60 秒。
	m.HandleEvent(UserStartTimerEvent{})
	m.HandleEvent(TickEvent{DeltaMs: 10000})
	m.HandleEvent(UserResetTimerEvent{})

	mustEqual(t, "RemainingMs after reset uses new config",
		m.state.Countdown.RemainingMs, int64(120000))
}

// 状态查询测试。

func TestGetTimeInfoCountdown(t *testing.T) {
	m := NewClockManager(cfgCountdownOnly(100))
	defer m.Deinit()

	m.HandleEvent(UserStartTimerEvent{})
	m.HandleEvent(TickEvent{DeltaMs: 23000})

	mustEqual(t, "GetTimeInfo countdown",
		m.Update().GetTimeInfo(), int64(77))
}

func TestGetTimeInfoStopwatch(t *testing.T) {
	m := NewClockManager(cfgStopwatch(3600))
	defer m.Deinit()

	m.HandleEvent(UserStartTimerEvent{})
	m.HandleEvent(TickEvent{DeltaMs: 42000})

	mustEqual(t, "GetTimeInfo stopwatch",
		m.Update().GetTimeInfo(), int64(42))
}

func TestIsPausedAndIsFinishedHelpers(t *testing.T) {
	m := NewClockManager(cfgCountdownOnly(10))
	defer m.Deinit()

	mustTrue(t, "IsPaused initial", m.state.IsPaused())
	mustFalse(t, "IsFinished initial", m.state.IsFinished())

	m.HandleEvent(UserStartTimerEvent{})
	mustFalse(t, "IsPaused after start", m.state.IsPaused())
}

// 边界条件测试。

func TestCountdownZeroSeconds(t *testing.T) {
	m := NewClockManager(cfgCountdownOnly(0))
	defer m.Deinit()

	m.HandleEvent(UserStartTimerEvent{})
	m.HandleEvent(TickEvent{DeltaMs: 1})

	mustEqual(t, "RemainingMs", m.state.Countdown.RemainingMs, int64(0))
	mustTrue(t, "IsFinished", m.state.Countdown.IsFinished)
}

func TestStopwatchOneMillisecond(t *testing.T) {
	m := NewClockManager(cfgStopwatch(3600))
	defer m.Deinit()

	m.HandleEvent(UserStartTimerEvent{})
	m.HandleEvent(TickEvent{DeltaMs: 1})

	mustEqual(t, "ElapsedMs at 1ms granularity",
		m.state.Stopwatch.ElapsedMs, int64(1))
}

func TestManySmallTicksAccumulate(t *testing.T) {
	m := NewClockManager(cfgCountdownOnly(100))
	defer m.Deinit()

	m.HandleEvent(UserStartTimerEvent{})
	for i := 0; i < 10; i++ {
		m.HandleEvent(TickEvent{DeltaMs: 100})
	}

	mustEqual(t, "RemainingMs after 10x100ms ticks",
		m.state.Countdown.RemainingMs, int64(99000))
}

// Smoke 测试。

func TestCountdown25MinSingleTickDecrements(t *testing.T) {
	cfg := cfgCountdownOnly(25 * Minute)
	m := NewClockManager(cfg)
	defer m.Deinit()

	mustEqual(t, "RemainingMs initial 25 min",
		m.state.Countdown.RemainingMs, int64(25*Minute*1000))

	m.HandleEvent(UserStartTimerEvent{})
	m.HandleEvent(TickEvent{DeltaMs: 60000})

	mustEqual(t, "RemainingMs after 60s tick",
		m.state.Countdown.RemainingMs, int64(24*Minute*1000))
}

func TestCountdownPauseBlocksTick(t *testing.T) {
	cfg := cfgCountdownOnly(25 * Minute)
	m := NewClockManager(cfg)
	defer m.Deinit()

	m.HandleEvent(UserStartTimerEvent{})
	m.HandleEvent(UserPauseTimerEvent{})
	before := m.state.Countdown.RemainingMs
	m.HandleEvent(TickEvent{DeltaMs: 60000})
	mustEqual(t, "Pause blocks ticks", m.state.Countdown.RemainingMs, before)
}

func TestCountdownResetClearsState(t *testing.T) {
	cfg := cfgCountdownOnly(25 * Minute)
	m := NewClockManager(cfg)
	defer m.Deinit()

	m.HandleEvent(UserStartTimerEvent{})
	m.HandleEvent(TickEvent{DeltaMs: 60000})
	m.HandleEvent(UserResetTimerEvent{})

	mustEqual(t, "RemainingMs after reset",
		m.state.Countdown.RemainingMs, int64(25*Minute*1000))
	mustTrue(t, "IsPaused after reset", m.state.Countdown.IsPaused)
}

func TestStopwatchStartTickIncrements(t *testing.T) {
	m := NewClockManager(cfgStopwatch(3600))
	defer m.Deinit()

	m.HandleEvent(UserStartTimerEvent{})
	m.HandleEvent(TickEvent{DeltaMs: 1000})

	mustEqual(t, "Stopwatch ElapsedMs (ms)", m.state.Stopwatch.ElapsedMs, int64(1000))
}

func TestStopwatchPauseBlocksTick(t *testing.T) {
	m := NewClockManager(cfgStopwatch(3600))
	defer m.Deinit()

	m.HandleEvent(UserStartTimerEvent{})
	m.HandleEvent(UserPauseTimerEvent{})
	before := m.state.Stopwatch.ElapsedMs
	m.HandleEvent(TickEvent{DeltaMs: 1000})
	mustEqual(t, "Stopwatch pause blocks tick", m.state.Stopwatch.ElapsedMs, before)
}

func TestModeSwitchResetsState(t *testing.T) {
	m := NewClockManager(cfgCountdownOnly(25 * Minute))
	defer m.Deinit()

	m.HandleEvent(UserStartTimerEvent{})
	m.HandleEvent(TickEvent{DeltaMs: 60000})
	mustEqual(t, "pre-switch RemainingMs",
		m.state.Countdown.RemainingMs, int64(24*Minute*1000))

	m.HandleEvent(UserChangeModeEvent{Mode: StopwatchMode})

	w := m.state.Stopwatch
	if w == nil {
		t.Fatal("expected non-nil Stopwatch after mode switch")
	}
	mustEqual(t, "Stopwatch ElapsedMs after switch", w.ElapsedMs, int64(0))
	mustEqual(t, "Stopwatch MaxMs after switch", w.MaxMs, int64(24*Hour*1000))
	mustTrue(t, "Stopwatch IsPaused after switch", w.IsPaused)
}

// 事件总线测试 —— 覆盖基于 channel 的派发路径。

func TestRunReadsFromEventChannel(t *testing.T) {
	m := NewClockManager(cfgStopwatch(3600))

	m.events = make(chan ClockEvent, 4)

	done := make(chan error, 1)
	go func() { done <- m.Run(context.Background(), m.events) }()

	m.events <- UserStartTimerEvent{}
	m.events <- TickEvent{DeltaMs: 100}
	m.events <- TickEvent{DeltaMs: 100}

	close(m.events)
	if err := <-done; err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if got := m.state.Stopwatch.ElapsedMs; got != int64(200) {
		t.Fatalf("channel-driven ticks: got ElapsedMs=%d, want 200", got)
	}
}
