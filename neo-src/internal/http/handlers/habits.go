// Package handlers — Habit / habit-set / session / timer-session CRUD.
//
// File `habits.go` ports the habit-related handlers from std_server.zig.
// Routes (paths match Zig exactly):
//
//   GET    /api/habit-sets                   → handleHabitSetList
//   POST   /api/habit-sets                   → handleHabitSetCreate
//   PUT    /api/habit-sets/:id               → handleHabitSetUpdate
//   DELETE /api/habit-sets/:id               → handleHabitSetDelete
//
//   GET    /api/habits                       → handleHabitList
//   POST   /api/habits                       → handleHabitCreate
//   PUT    /api/habits/:id                   → handleHabitUpdate
//   DELETE /api/habits/:id                   → handleHabitDelete
//   GET    /api/habits/:id/detail            → handleHabitDetail
//
//   POST   /api/sessions                     → handleSessionCreate
//   GET    /api/sessions                     → handleSessionList
//
//   POST   /api/timer-sessions               → handleTimerSessionCreate
//   GET    /api/timer-sessions               → handleTimerSessionList
//   PUT    /api/timer-sessions/:id           → handleTimerSessionUpdate
//   DELETE /api/timer-sessions/:id           → handleTimerSessionDelete
package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"little-timer/internal/storage"
)

// -----------------------------------------------------------------------------
// /api/habit-sets
// -----------------------------------------------------------------------------

// handleHabitSetCreate mirrors `handleCreateHabitSet`.
// Body: {name, description?, color?}.
func handleHabitSetCreate(c *gin.Context) {
	a := appFromCtx(c)

	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Color       string `json:"color"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"err": "Invalid JSON"})
		return
	}
	if req.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"err": "Missing name"})
		return
	}
	if req.Color == "" {
		req.Color = "#6366f1"
	}

	id, err := a.SQLite.HabitSets().Create(req.Name, req.Description, req.Color)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"err": "Failed to create habit set"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"id":          id,
		"name":        req.Name,
		"description": req.Description,
		"color":       req.Color,
	})
}

// handleHabitSetList mirrors `handleGetHabitSets`.
func handleHabitSetList(c *gin.Context) {
	a := appFromCtx(c)
	rows, err := a.SQLite.HabitSets().List()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"err": "Failed to get habit sets"})
		return
	}
	c.JSON(http.StatusOK, rows)
}

// handleHabitSetUpdate mirrors `handleUpdateHabitSet`.
func handleHabitSetUpdate(c *gin.Context) {
	a := appFromCtx(c)
	id, err := pathID(c, "/api/habit-sets/")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"err": "Invalid id"})
		return
	}

	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Color       string `json:"color"`
		Wallpaper   string `json:"wallpaper"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"err": "Invalid JSON"})
		return
	}
	if req.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"err": "Missing name"})
		return
	}
	if req.Color == "" {
		req.Color = "#6366f1"
	}
	if err := a.SQLite.HabitSets().Update(id, req.Name, req.Description, req.Color, req.Wallpaper); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"err": "Failed to update habit set"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"id":          id,
		"name":        req.Name,
		"description": req.Description,
		"color":       req.Color,
		"wallpaper":   req.Wallpaper,
	})
}

// handleHabitSetDelete mirrors `handleDeleteHabitSet`.
func handleHabitSetDelete(c *gin.Context) {
	a := appFromCtx(c)
	id, err := pathID(c, "/api/habit-sets/")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"err": "Invalid id"})
		return
	}
	if err := a.SQLite.HabitSets().Delete(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"err": "Failed to delete habit set"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// -----------------------------------------------------------------------------
// /api/habits
// -----------------------------------------------------------------------------

// handleHabitCreate mirrors `handleCreateHabit`.
// Body: {set_id, name, goal_seconds?, color?}.
func handleHabitCreate(c *gin.Context) {
	a := appFromCtx(c)

	var req struct {
		SetID       int64  `json:"set_id"`
		Name        string `json:"name"`
		GoalSeconds int64  `json:"goal_seconds"`
		Color       string `json:"color"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"err": "Invalid JSON"})
		return
	}
	if req.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"err": "Missing name"})
		return
	}
	if req.GoalSeconds == 0 {
		req.GoalSeconds = 1500
	}
	if req.Color == "" {
		req.Color = "#6366f1"
	}

	id, err := a.SQLite.Habits().Create(req.SetID, req.Name, req.GoalSeconds, req.Color)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"err": "Failed to create habit"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"id":           id,
		"set_id":       req.SetID,
		"name":         req.Name,
		"goal_seconds": req.GoalSeconds,
		"color":        req.Color,
	})
}

// handleHabitList mirrors `handleGetHabits`.  Optional `?set_id=N` query
// narrows the list to a single habit set.
func handleHabitList(c *gin.Context) {
	a := appFromCtx(c)
	if setIDStr := c.Query("set_id"); setIDStr != "" {
		setID, err := strconv.ParseInt(setIDStr, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"err": "Invalid set_id"})
			return
		}
		rows, err := a.SQLite.Habits().ListBySet(setID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"err": "Failed to get habits"})
			return
		}
		c.JSON(http.StatusOK, rows)
		return
	}
	rows, err := a.SQLite.Habits().List()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"err": "Failed to get habits"})
		return
	}
	c.JSON(http.StatusOK, rows)
}

// handleHabitUpdate mirrors `handleUpdateHabit`.
func handleHabitUpdate(c *gin.Context) {
	a := appFromCtx(c)
	id, err := pathID(c, "/api/habits/")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"err": "Invalid id"})
		return
	}

	var req struct {
		Name        string `json:"name"`
		GoalSeconds int64  `json:"goal_seconds"`
		Color       string `json:"color"`
		Wallpaper   string `json:"wallpaper"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"err": "Invalid JSON"})
		return
	}
	if req.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"err": "Missing name"})
		return
	}
	if req.GoalSeconds == 0 {
		req.GoalSeconds = 1500
	}
	if req.Color == "" {
		req.Color = "#6366f1"
	}
	if err := a.SQLite.Habits().Update(id, req.Name, req.GoalSeconds, req.Color, req.Wallpaper); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"err": "Failed to update habit"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"id":           id,
		"name":         req.Name,
		"goal_seconds": req.GoalSeconds,
		"color":        req.Color,
		"wallpaper":    req.Wallpaper,
	})
}

// handleHabitDelete mirrors `handleDeleteHabit`.
func handleHabitDelete(c *gin.Context) {
	a := appFromCtx(c)
	id, err := pathID(c, "/api/habits/")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"err": "Invalid id"})
		return
	}
	if err := a.SQLite.Habits().Delete(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"err": "Failed to delete habit"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// handleHabitDetail mirrors `handleGetHabitDetail` — single habit with
// today's accumulated seconds + progress percent.
func handleHabitDetail(c *gin.Context) {
	a := appFromCtx(c)
	id, err := pathIDWithSuffix(c, "/api/habits/", "/detail")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"err": "Invalid id"})
		return
	}

	date := c.Query("date")
	if date == "" {
		date = todayString()
	}

	row, err := a.SQLite.Habits().GetByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"err": "Habit not found"})
		return
	}

	todaySeconds, _ := a.SQLite.Timers().TodaySecondsForHabit(id, date)
	var progressPercent int64
	if row.GoalSeconds > 0 {
		progressPercent = (todaySeconds * 100) / row.GoalSeconds
	}
	c.JSON(http.StatusOK, gin.H{
		"id":              row.ID,
		"name":            row.Name,
		"goal_seconds":    row.GoalSeconds,
		"color":           row.Color,
		"today_seconds":   todaySeconds,
		"progress_percent": progressPercent,
	})
}

// -----------------------------------------------------------------------------
// /api/sessions
// -----------------------------------------------------------------------------

// handleSessionCreate mirrors `handleCreateSession`.
// Body: {habit_id, duration_seconds, count?}.
func handleSessionCreate(c *gin.Context) {
	a := appFromCtx(c)

	var req struct {
		HabitID         int64 `json:"habit_id"`
		DurationSeconds int64 `json:"duration_seconds"`
		Count           int64 `json:"count"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"err": "Invalid JSON"})
		return
	}
	if req.Count == 0 {
		req.Count = 1
	}

	id, err := a.SQLite.Timers().CreateSession(req.HabitID, req.DurationSeconds, req.Count, todayString())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"err": "Failed to create session"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"id":              id,
		"habit_id":        req.HabitID,
		"duration_seconds": req.DurationSeconds,
		"date":            todayString(),
	})
}

// handleSessionList mirrors `handleGetSessions`.  Supports three query
// shapes (matching the Zig source): `?date=YYYY-MM-DD`,
// `?start_date=…&end_date=…`, or no date → today.
func handleSessionList(c *gin.Context) {
	a := appFromCtx(c)
	date := c.Query("date")
	startDate := c.Query("start_date")
	endDate := c.Query("end_date")

	var (
		rows []storage.SessionRow
		err  error
	)
	switch {
	case startDate != "" && endDate != "":
		rows, err = a.SQLite.Timers().ListSessionsByDateRange(startDate, endDate)
	case date != "":
		rows, err = a.SQLite.Timers().ListSessionsByDate(date)
	default:
		rows, err = a.SQLite.Timers().ListSessionsByDate(todayString())
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"err": "Failed to get sessions"})
		return
	}
	c.JSON(http.StatusOK, rows)
}

// -----------------------------------------------------------------------------
// /api/timer-sessions
// -----------------------------------------------------------------------------

// handleTimerSessionCreate mirrors `handleCreateTimerSession` (the
// Zig source uses the same body shape as `POST /api/start` minus the
// paused/finished fields).
func handleTimerSessionCreate(c *gin.Context) {
	a := appFromCtx(c)

	var req struct {
		HabitID      *int64 `json:"habit_id,omitempty"`
		Mode         string `json:"mode,omitempty"`
		WorkDuration int64  `json:"work_duration,omitempty"`
		RestDuration int64  `json:"rest_duration,omitempty"`
		LoopCount    int64  `json:"loop_count,omitempty"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"err": "Invalid JSON"})
		return
	}
	if req.WorkDuration == 0 {
		req.WorkDuration = 25 * 60
	}
	if req.Mode == "" {
		req.Mode = "stopwatch"
	}

	id, err := a.SQLite.Timers().CreateTimerSession(req.HabitID, req.Mode, req.WorkDuration, req.RestDuration, req.LoopCount)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"err": "Failed to create timer session"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"id": id})
}

// handleTimerSessionList mirrors the implicit `getTimerSessionById` /
// list-all pattern.  Without a query, returns the active (unfinished)
// session, mirroring the Zig behavior of returning one row.
func handleTimerSessionList(c *gin.Context) {
	a := appFromCtx(c)
	if idStr := c.Query("id"); idStr != "" {
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"err": "Invalid id"})
			return
		}
		row, err := a.SQLite.Timers().GetTimerSessionByID(id)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"err": "Timer session not found"})
			return
		}
		c.JSON(http.StatusOK, row)
		return
	}
	row, err := a.SQLite.Timers().GetActiveTimerSession()
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"err": "No active timer session"})
		return
	}
	c.JSON(http.StatusOK, row)
}

// handleTimerSessionUpdate mirrors `updateTimerSession`.
func handleTimerSessionUpdate(c *gin.Context) {
	a := appFromCtx(c)
	id, err := pathID(c, "/api/timer-sessions/")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"err": "Invalid id"})
		return
	}

	var req struct {
		ElapsedSeconds     int64  `json:"elapsed_seconds"`
		RemainingSeconds   *int64 `json:"remaining_seconds"`
		PausedTotalSeconds int64  `json:"paused_total_seconds"`
		PauseStartedAt     *int64 `json:"pause_started_at"`
		IsRunning          bool   `json:"is_running"`
		IsPaused           bool   `json:"is_paused"`
		IsFinished         bool   `json:"is_finished"`
		CurrentRound       int64  `json:"current_round"`
		InRest             bool   `json:"in_rest"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"err": "Invalid JSON"})
		return
	}
	now := nowUnix()
	if err := a.SQLite.Timers().UpdateTimerSession(
		id, req.ElapsedSeconds, req.RemainingSeconds,
		req.PausedTotalSeconds, req.PauseStartedAt, &now,
		req.IsRunning, req.IsPaused, req.IsFinished,
		req.CurrentRound, req.InRest,
	); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"err": "Failed to update timer session"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// handleTimerSessionDelete mirrors `deleteTimerSession`.
func handleTimerSessionDelete(c *gin.Context) {
	a := appFromCtx(c)
	id, err := pathID(c, "/api/timer-sessions/")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"err": "Invalid id"})
		return
	}
	if err := a.SQLite.Timers().DeleteTimerSession(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"err": "Failed to delete timer session"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}