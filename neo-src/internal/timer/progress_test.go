package timer

import (
	"testing"
	"time"

	"little-timer/internal/domain"
	"little-timer/internal/storage"
)

func TestProgress_NoSessionLoadsActive(t *testing.T) {
	rt := newFakeRuntime(domain.StopwatchMode)
	habitID := int64(7)
	rt.store.active = &storage.TimerSessionRow{
		ID:             55,
		HabitID:        &habitID,
		Mode:           "stopwatch",
		IsPaused:       true,
		ElapsedSeconds: 12,
	}
	o := New(rt)

	res := o.Progress()

	if res.SessionID == nil || *res.SessionID != 55 {
		t.Errorf("session id = %v, want 55", res.SessionID)
	}
	if res.HabitID == nil || *res.HabitID != habitID {
		t.Errorf("habit id = %v, want %d", res.HabitID, habitID)
	}
	if rt.SessionID() == nil || *rt.SessionID() != 55 {
		t.Errorf("runtime session pointer = %v, want 55", rt.SessionID())
	}
}

func TestProgress_ReturnsClockFields(t *testing.T) {
	rt := newFakeRuntime(domain.StopwatchMode)
	id := rt.store.seed(storage.TimerSessionRow{Mode: "stopwatch", IsRunning: true})
	rt.SetSessionID(&id)
	startClockWithElapsed(rt, 3)
	o := New(rt)

	res := o.Progress()

	if res.Mode != domain.StopwatchMode {
		t.Errorf("mode = %v, want stopwatch", res.Mode)
	}
	if !res.IsRunning || res.IsPaused {
		t.Errorf("running/paused = %v/%v, want true/false", res.IsRunning, res.IsPaused)
	}
	if res.IsFinished {
		t.Error("is_finished = true, want false")
	}
	if res.InRest {
		t.Error("in_rest = true, want false")
	}
	if res.ElapsedSeconds < 3 {
		t.Errorf("elapsed = %d, want >= 3", res.ElapsedSeconds)
	}
	if res.RemainingSeconds != 0 {
		t.Errorf("remaining = %d, want 0 for stopwatch", res.RemainingSeconds)
	}
}

func TestSaveProgress_NoSessionNoop(t *testing.T) {
	rt := newFakeRuntime(domain.StopwatchMode)
	o := New(rt)

	o.SaveProgress()

	if len(rt.store.updated) != 0 {
		t.Errorf("updated = %d, want 0 (no session)", len(rt.store.updated))
	}
}

func TestSaveProgress_PausedComputesPausedTotal(t *testing.T) {
	rt := newFakeRuntime(domain.StopwatchMode)
	pauseStarted := time.Now().Unix() - 5
	id := rt.store.seed(storage.TimerSessionRow{
		Mode:               "stopwatch",
		IsRunning:          true,
		PausedTotalSeconds: 10,
		PauseStartedAt:     &pauseStarted,
	})
	rt.SetSessionID(&id)
	rt.clock.HandleEvent(domain.UserStartTimerEvent{})
	o := New(rt)

	o.SaveProgress()

	if len(rt.store.updated) != 1 {
		t.Fatalf("updated = %d, want 1", len(rt.store.updated))
	}
	got := rt.store.updated[0]
	if got.pausedTotal < 15 {
		t.Errorf("paused total = %d, want >= 15 (10 + 5)", got.pausedTotal)
	}
	if got.pauseStartedAt != nil {
		t.Errorf("pause_started_at = %v, want nil after unpause", got.pauseStartedAt)
	}
	if !got.isRunning || got.isPaused {
		t.Errorf("flags = running:%v paused:%v, want running:true paused:false", got.isRunning, got.isPaused)
	}
}
