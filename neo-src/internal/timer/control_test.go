package timer

import (
	"testing"

	"little-timer/internal/domain"
	"little-timer/internal/storage"
)

func TestPause_SavesProgress(t *testing.T) {
	rt := newFakeRuntime(domain.StopwatchMode)
	id := rt.store.seed(storage.TimerSessionRow{
		Mode:         "stopwatch",
		IsRunning:    true,
		WorkDuration: 1500,
	})
	rt.SetSessionID(&id)
	rt.clock.HandleEvent(domain.UserStartTimerEvent{})
	o := New(rt)

	o.Pause()

	if !rt.clock.Update().IsPaused() {
		t.Error("clock not paused after Pause")
	}
	if len(rt.store.updated) != 1 {
		t.Fatalf("updated = %d, want 1", len(rt.store.updated))
	}
	got := rt.store.updated[0]
	if got.sessionID != id {
		t.Errorf("updated session = %d, want %d", got.sessionID, id)
	}
	if !got.isPaused || got.isRunning {
		t.Errorf("flags = running:%v paused:%v, want running:false paused:true", got.isRunning, got.isPaused)
	}
	if got.pauseStartedAt == nil {
		t.Error("pause_started_at = nil, want a timestamp")
	}
}

func TestPause_NoSession(t *testing.T) {
	rt := newFakeRuntime(domain.StopwatchMode)
	o := New(rt)

	o.Pause()

	if len(rt.store.updated) != 0 {
		t.Errorf("updated = %d, want 0 (no session)", len(rt.store.updated))
	}
}

func TestReset_DeletesSessionAndClearsPointers(t *testing.T) {
	rt := newFakeRuntime(domain.StopwatchMode)
	id := rt.store.seed(storage.TimerSessionRow{
		Mode:           "stopwatch",
		IsRunning:      true,
		ElapsedSeconds: 12,
		WorkDuration:   1500,
	})
	habitID := int64(3)
	rt.SetSessionID(&id)
	rt.SetHabitID(&habitID)
	rt.clock.HandleEvent(domain.UserStartTimerEvent{})
	o := New(rt)

	o.Reset()

	if len(rt.store.deleted) != 1 || rt.store.deleted[0] != id {
		t.Errorf("deleted = %v, want [%d]", rt.store.deleted, id)
	}
	if rt.SessionID() != nil {
		t.Errorf("session pointer = %v, want nil", rt.SessionID())
	}
	if rt.HabitID() != nil {
		t.Errorf("habit pointer = %v, want nil", rt.HabitID())
	}
	state := rt.clock.Update()
	if !state.IsPaused() || state.IsFinished() {
		t.Errorf("clock state after reset = paused:%v finished:%v, want paused:true finished:false",
			state.IsPaused(), state.IsFinished())
	}
	// domain 的 UserResetTimerEvent 把 StartTimeMs 归零（GetElapsedSeconds 在
	// StartTimeMs==0 时的取值是 domain 既有语义，timer 不改动它）。
	if state.Stopwatch == nil || state.Stopwatch.StartTimeMs != 0 {
		t.Errorf("stopwatch StartTimeMs not reset: %+v", state.Stopwatch)
	}
}
