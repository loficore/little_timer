package timer

import (
	"errors"
	"testing"

	"little-timer/internal/domain"
	"little-timer/internal/storage"
)

// startClockWithElapsed 让时钟处于运行态且已走过 elapsedSeconds 秒。
func startClockWithElapsed(rt *fakeRuntime, elapsedSeconds int64) {
	rt.clock.HandleEvent(domain.UserStartTimerEvent{})
	rt.clock.Update().Stopwatch.StartTimeMs = domain.NowMs() - elapsedSeconds*1000
}

func TestFinish_SuccessWritesDailySession(t *testing.T) {
	rt := newFakeRuntime(domain.StopwatchMode)
	habitID := int64(3)
	id := rt.store.seed(storage.TimerSessionRow{
		Mode:         "stopwatch",
		IsRunning:    true,
		WorkDuration: 1500,
	})
	rt.SetSessionID(&id)
	rt.SetHabitID(&habitID)
	startClockWithElapsed(rt, 5)
	o := New(rt)

	res, err := o.Finish()
	if err != nil {
		t.Fatalf("Finish: %v", err)
	}

	if res.Status != "finished" {
		t.Errorf("status = %q, want finished", res.Status)
	}
	if res.SessionID == nil || *res.SessionID != id {
		t.Errorf("session id = %v, want %d", res.SessionID, id)
	}
	if res.ElapsedSeconds < 5 {
		t.Errorf("elapsed = %d, want >= 5", res.ElapsedSeconds)
	}
	if len(rt.store.dailySessions) != 1 {
		t.Fatalf("daily sessions = %d, want 1", len(rt.store.dailySessions))
	}
	if got := rt.store.dailySessions[0]; got.habitID != habitID || got.duration != res.ElapsedSeconds {
		t.Errorf("daily session = %+v, want habit %d duration %d", got, habitID, res.ElapsedSeconds)
	}
	if rt.SessionID() != nil || rt.HabitID() != nil {
		t.Errorf("pointers after finish = (%v, %v), want nil", rt.SessionID(), rt.HabitID())
	}
}

func TestFinish_StoreErrorFallsBackToClock(t *testing.T) {
	rt := newFakeRuntime(domain.StopwatchMode)
	rt.store.finishErr = errors.New("store down")
	habitID := int64(9)
	id := rt.store.seed(storage.TimerSessionRow{
		Mode:         "stopwatch",
		IsRunning:    true,
		WorkDuration: 1500,
	})
	rt.SetSessionID(&id)
	rt.SetHabitID(&habitID)
	startClockWithElapsed(rt, 7)
	o := New(rt)

	res, err := o.Finish()
	if err != nil {
		t.Fatalf("Finish: %v", err)
	}

	if res.ElapsedSeconds < 7 {
		t.Errorf("elapsed = %d, want clock fallback >= 7", res.ElapsedSeconds)
	}
	if res.SessionID != nil {
		t.Errorf("session id = %v, want nil on fallback", res.SessionID)
	}
	if len(rt.store.dailySessions) != 1 || rt.store.dailySessions[0].duration != res.ElapsedSeconds {
		t.Errorf("daily sessions = %+v, want 1 row using clock elapsed", rt.store.dailySessions)
	}
}

func TestFinish_NoHabitSkipsDailySession(t *testing.T) {
	rt := newFakeRuntime(domain.StopwatchMode)
	id := rt.store.seed(storage.TimerSessionRow{
		Mode:         "stopwatch",
		IsRunning:    true,
		WorkDuration: 1500,
	})
	rt.SetSessionID(&id)
	startClockWithElapsed(rt, 5)
	o := New(rt)

	res, err := o.Finish()
	if err != nil {
		t.Fatalf("Finish: %v", err)
	}

	if len(rt.store.dailySessions) != 0 {
		t.Errorf("daily sessions = %d, want 0 (no habit)", len(rt.store.dailySessions))
	}
	if res.SessionID == nil || *res.SessionID != id {
		t.Errorf("session id = %v, want %d", res.SessionID, id)
	}
}

func TestFinish_ZeroElapsedSkipsDailySession(t *testing.T) {
	rt := newFakeRuntime(domain.StopwatchMode)
	habitID := int64(4)
	id := rt.store.seed(storage.TimerSessionRow{
		Mode:         "stopwatch",
		IsRunning:    true,
		WorkDuration: 1500,
	})
	rt.SetSessionID(&id)
	rt.SetHabitID(&habitID)
	rt.clock.HandleEvent(domain.UserStartTimerEvent{})
	o := New(rt)

	if _, err := o.Finish(); err != nil {
		t.Fatalf("Finish: %v", err)
	}

	if len(rt.store.dailySessions) != 0 {
		t.Errorf("daily sessions = %d, want 0 (elapsed == 0)", len(rt.store.dailySessions))
	}
}
