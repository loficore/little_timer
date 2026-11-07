package timer

import (
	"time"

	"little-timer/internal/domain"
	"little-timer/internal/storage"
)

// fakeStore 是 Store 的内存实现，记录每次写入供断言。
type fakeStore struct {
	rows map[int64]storage.TimerSessionRow
	// active 是 GetActiveTimerSession 返回的行；nil 表示没有活动会话。
	active *storage.TimerSessionRow

	nextID int64

	created       []createCall
	updated       []updateCall
	finished      []int64
	deleted       []int64
	dailySessions []dailyCall

	// finishErr 注入后 FinishTimerSession 返回该错误。
	finishErr error
	// updateErr 注入后 UpdateTimerSession 返回该错误。
	updateErr error
}

type createCall struct {
	habitID  *int64
	mode     string
	work     int64
	rest     int64
	loop     int64
	returnID int64
}

type updateCall struct {
	sessionID      int64
	elapsedSeconds int64
	remaining      *int64
	pausedTotal    int64
	pauseStartedAt *int64
	lastSyncedAt   *int64
	isRunning      bool
	isPaused       bool
	isFinished     bool
	currentRound   int64
	inRest         bool
}

type dailyCall struct {
	habitID  int64
	duration int64
	count    int64
	date     string
}

func newFakeStore() *fakeStore {
	return &fakeStore{rows: map[int64]storage.TimerSessionRow{}, nextID: 1}
}

// seed 插入一行既有 timer_session，返回其 id。
func (s *fakeStore) seed(row storage.TimerSessionRow) int64 {
	if row.ID == 0 {
		row.ID = s.nextID
		s.nextID++
	}
	s.rows[row.ID] = row
	return row.ID
}

func (s *fakeStore) GetTimerSessionByID(sessionID int64) (storage.TimerSessionRow, error) {
	row, ok := s.rows[sessionID]
	if !ok {
		return storage.TimerSessionRow{}, storage.ErrHabitNotFound
	}
	return row, nil
}

func (s *fakeStore) CreateTimerSession(habitID *int64, mode string, workDuration, restDuration, loopCount int64) (int64, error) {
	id := s.nextID
	s.nextID++
	now := time.Now().Unix()
	row := storage.TimerSessionRow{
		ID:               id,
		HabitID:          habitID,
		Mode:             mode,
		StartedAt:        now,
		UpdatedAt:        now,
		IsRunning:        true,
		WorkDuration:     workDuration,
		RestDuration:     restDuration,
		LoopCount:        loopCount,
		RemainingSeconds: ptrInt64(workDuration),
	}
	s.rows[id] = row
	s.created = append(s.created, createCall{
		habitID: habitID, mode: mode, work: workDuration,
		rest: restDuration, loop: loopCount, returnID: id,
	})
	return id, nil
}

func (s *fakeStore) UpdateTimerSession(
	sessionID int64,
	elapsedSeconds int64,
	remainingSeconds *int64,
	pausedTotalSeconds int64,
	pauseStartedAt *int64,
	lastSyncedAt *int64,
	isRunning, isPaused, isFinished bool,
	currentRound int64,
	inRest bool,
) error {
	s.updated = append(s.updated, updateCall{
		sessionID: sessionID, elapsedSeconds: elapsedSeconds,
		remaining: remainingSeconds, pausedTotal: pausedTotalSeconds,
		pauseStartedAt: pauseStartedAt, lastSyncedAt: lastSyncedAt,
		isRunning: isRunning, isPaused: isPaused, isFinished: isFinished,
		currentRound: currentRound, inRest: inRest,
	})
	if s.updateErr != nil {
		return s.updateErr
	}
	if row, ok := s.rows[sessionID]; ok {
		row.ElapsedSeconds = elapsedSeconds
		row.RemainingSeconds = remainingSeconds
		row.PausedTotalSeconds = pausedTotalSeconds
		row.PauseStartedAt = pauseStartedAt
		row.LastSyncedAt = lastSyncedAt
		row.IsRunning = isRunning
		row.IsPaused = isPaused
		row.IsFinished = isFinished
		row.CurrentRound = currentRound
		row.InRest = inRest
		s.rows[sessionID] = row
	}
	return nil
}

func (s *fakeStore) FinishTimerSession(sessionID int64) error {
	if s.finishErr != nil {
		return s.finishErr
	}
	s.finished = append(s.finished, sessionID)
	if row, ok := s.rows[sessionID]; ok {
		row.IsRunning = false
		row.IsPaused = false
		row.IsFinished = true
		s.rows[sessionID] = row
	}
	return nil
}

func (s *fakeStore) DeleteTimerSession(sessionID int64) error {
	s.deleted = append(s.deleted, sessionID)
	delete(s.rows, sessionID)
	return nil
}

func (s *fakeStore) GetActiveTimerSession() (storage.TimerSessionRow, error) {
	if s.active == nil {
		return storage.TimerSessionRow{}, storage.ErrHabitNotFound
	}
	return *s.active, nil
}

func (s *fakeStore) CreateSession(habitID, durationSeconds, count int64, date string) (int64, error) {
	s.dailySessions = append(s.dailySessions, dailyCall{
		habitID: habitID, duration: durationSeconds, count: count, date: date,
	})
	return int64(len(s.dailySessions)), nil
}

// fakeRuntime 是 Runtime 的内存实现。
type fakeRuntime struct {
	clock     *domain.ClockManager
	store     *fakeStore
	timezone  int8
	sessionID *int64
	habitID   *int64
}

func newFakeRuntime(mode domain.ModeEnum) *fakeRuntime {
	cfg := domain.NewDefaultClockTaskConfig()
	cfg.DefaultMode = mode
	return &fakeRuntime{
		clock:    domain.NewClockManager(cfg),
		store:    newFakeStore(),
		timezone: 8,
	}
}

func (r *fakeRuntime) ClockManager() *domain.ClockManager { return r.clock }

func (r *fakeRuntime) TimerStore() Store { return r.store }

func (r *fakeRuntime) TimezoneOffset() int8 { return r.timezone }

func (r *fakeRuntime) SessionID() *int64 { return r.sessionID }

func (r *fakeRuntime) SetSessionID(id *int64) { r.sessionID = id }

func (r *fakeRuntime) HabitID() *int64 { return r.habitID }

func (r *fakeRuntime) SetHabitID(id *int64) { r.habitID = id }

func ptrInt64(v int64) *int64 { return &v }
