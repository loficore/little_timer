package timer

import (
	"testing"

	"little-timer/internal/domain"
	"little-timer/internal/storage"
)

func TestStart_FreshStart(t *testing.T) {
	rt := newFakeRuntime(domain.StopwatchMode)
	o := New(rt)

	res, err := o.Start(StartInput{})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	if res.Status != "started" {
		t.Errorf("status = %q, want started", res.Status)
	}
	if len(rt.store.created) != 1 {
		t.Fatalf("created = %d, want 1", len(rt.store.created))
	}
	if got := rt.store.created[0].mode; got != "stopwatch" {
		t.Errorf("mode = %q, want stopwatch (default)", got)
	}
	if got := rt.store.created[0].work; got != 1500 {
		t.Errorf("work = %d, want 1500 (default)", got)
	}
	if res.SessionID != rt.store.created[0].returnID {
		t.Errorf("result session id = %d, want %d", res.SessionID, rt.store.created[0].returnID)
	}
	if rt.SessionID() == nil || *rt.SessionID() != res.SessionID {
		t.Errorf("runtime session pointer = %v, want %d", rt.SessionID(), res.SessionID)
	}
	if rt.clock.Update().IsPaused() {
		t.Error("clock still paused after start")
	}
}

func TestStart_ResumePaused(t *testing.T) {
	rt := newFakeRuntime(domain.StopwatchMode)
	oldID := rt.store.seed(storage.TimerSessionRow{
		Mode:               "stopwatch",
		IsPaused:           true,
		ElapsedSeconds:     10,
		PausedTotalSeconds: 5,
		PauseStartedAt:     ptrInt64(100),
		WorkDuration:       1500,
	})
	rt.SetSessionID(&oldID)
	habitID := int64(7)
	rt.store.rows[oldID] = withHabit(rt.store.rows[oldID], &habitID)
	o := New(rt)

	res, err := o.Start(StartInput{})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	if len(rt.store.updated) != 1 {
		t.Fatalf("updated = %d, want 1", len(rt.store.updated))
	}
	if len(rt.store.created) != 0 {
		t.Errorf("created = %d, want 0 (resume must not create)", len(rt.store.created))
	}
	if res.Status != "started" {
		t.Errorf("status = %q, want started", res.Status)
	}
	if res.SessionID != oldID {
		t.Errorf("session id = %d, want old %d", res.SessionID, oldID)
	}
	if *rt.SessionID() != oldID {
		t.Errorf("runtime session pointer = %d, want %d", *rt.SessionID(), oldID)
	}
	if rt.store.updated[0].isPaused {
		t.Error("UpdateTimerSession called with isPaused=true, want false")
	}
	if rt.HabitID() == nil || *rt.HabitID() != habitID {
		t.Errorf("habit id = %v, want fallback %d", rt.HabitID(), habitID)
	}
	if rt.clock.Update().IsPaused() {
		t.Error("clock still paused after resume")
	}
}

func TestStart_ConvergesRunningResidue(t *testing.T) {
	rt := newFakeRuntime(domain.StopwatchMode)
	oldID := rt.store.seed(storage.TimerSessionRow{
		Mode:           "stopwatch",
		IsRunning:      true,
		ElapsedSeconds: 42,
		WorkDuration:   1500,
	})
	rt.SetSessionID(&oldID)
	rt.clock.HandleEvent(domain.UserStartTimerEvent{})
	o := New(rt)

	res, err := o.Start(StartInput{})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	if len(rt.store.finished) != 1 || rt.store.finished[0] != oldID {
		t.Errorf("finished = %v, want [%d]", rt.store.finished, oldID)
	}
	if len(rt.store.deleted) != 0 {
		t.Errorf("deleted = %v, want none (elapsed > 0)", rt.store.deleted)
	}
	if len(rt.store.created) != 1 {
		t.Fatalf("created = %d, want 1", len(rt.store.created))
	}
	if res.SessionID == oldID {
		t.Errorf("session id = %d, want a new session", res.SessionID)
	}
}

func TestStart_ConvergesZeroElapsed(t *testing.T) {
	rt := newFakeRuntime(domain.StopwatchMode)
	oldID := rt.store.seed(storage.TimerSessionRow{
		Mode:         "stopwatch",
		IsRunning:    true,
		WorkDuration: 1500,
	})
	rt.SetSessionID(&oldID)
	rt.clock.HandleEvent(domain.UserStartTimerEvent{})
	o := New(rt)

	res, err := o.Start(StartInput{})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	if len(rt.store.deleted) != 1 || rt.store.deleted[0] != oldID {
		t.Errorf("deleted = %v, want [%d] (elapsed == 0)", rt.store.deleted, oldID)
	}
	if len(rt.store.created) != 1 {
		t.Fatalf("created = %d, want 1", len(rt.store.created))
	}
	if res.SessionID == oldID {
		t.Errorf("session id = %d, want a new session", res.SessionID)
	}
}

func TestStart_CountdownMode(t *testing.T) {
	rt := newFakeRuntime(domain.StopwatchMode)
	o := New(rt)

	if _, err := o.Start(StartInput{Mode: "countdown", WorkDuration: 300}); err != nil {
		t.Fatalf("Start: %v", err)
	}

	if len(rt.store.created) != 1 {
		t.Fatalf("created = %d, want 1", len(rt.store.created))
	}
	if got := rt.store.created[0].mode; got != "countdown" {
		t.Errorf("mode = %q, want countdown", got)
	}
	if got := rt.store.created[0].work; got != 300 {
		t.Errorf("work = %d, want 300", got)
	}
}

func TestStart_ConvergesMissingRow(t *testing.T) {
	rt := newFakeRuntime(domain.StopwatchMode)
	danglingID := int64(99) // 指针指向一行已不存在的 session
	habitID := int64(5)
	rt.SetSessionID(&danglingID)
	rt.SetHabitID(&habitID)
	rt.clock.HandleEvent(domain.UserStartTimerEvent{})
	o := New(rt)

	res, err := o.Start(StartInput{})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	if len(rt.store.created) != 1 {
		t.Fatalf("created = %d, want 1", len(rt.store.created))
	}
	if res.SessionID == danglingID {
		t.Errorf("session id = %d, want a new session", res.SessionID)
	}
	if rt.HabitID() != nil {
		t.Errorf("habit id = %v, want nil for fresh start without habit", rt.HabitID())
	}
}

func TestStart_ConvergesFinishedNotReset(t *testing.T) {
	rt := newFakeRuntime(domain.StopwatchMode)
	oldID := rt.store.seed(storage.TimerSessionRow{
		Mode:           "stopwatch",
		IsFinished:     true,
		ElapsedSeconds: 30,
		WorkDuration:   1500,
	})
	rt.SetSessionID(&oldID)
	rt.clock.HandleEvent(domain.UserStartTimerEvent{})
	rt.clock.HandleEvent(domain.UserFinishTimerEvent{})
	o := New(rt)

	res, err := o.Start(StartInput{})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	if len(rt.store.finished) != 1 || rt.store.finished[0] != oldID {
		t.Errorf("finished = %v, want [%d]", rt.store.finished, oldID)
	}
	if len(rt.store.created) != 1 || res.SessionID == oldID {
		t.Errorf("created = %d, session id = %d, want a new session", len(rt.store.created), res.SessionID)
	}
}

// withHabit 返回带 habit 指针的副本。
func withHabit(row storage.TimerSessionRow, habitID *int64) storage.TimerSessionRow {
	row.HabitID = habitID
	return row
}
