// Package handlers —— Habit / habit-set / session / timer-session CRUD。
package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"little-timer/internal/domain"
	"little-timer/internal/habits"
)

// parsePagination 从 query 参数提取 limit 和 offset。默认值：limit=100、
// offset=0；limit 上限 1000。输入不可解析或为负时 valid=false。
func parsePagination(c *gin.Context) (limit, offset int, valid bool) {
	limitStr := c.DefaultQuery("limit", "100")
	offsetStr := c.DefaultQuery("offset", "0")

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 0 {
		return 0, 0, false
	}
	offset, err = strconv.Atoi(offsetStr)
	if err != nil || offset < 0 {
		return 0, 0, false
	}
	if limit > 1000 {
		limit = 1000
	}
	return limit, offset, true
}

func HabitSetCreate(c *gin.Context) {
	a := appFromCtx(c)

	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Color       string `json:"color"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid JSON"})
		return
	}
	res, err := a.Habits.CreateHabitSet(habits.CreateHabitSetInput{
		Name: req.Name, Description: req.Description, Color: req.Color,
	})
	if err != nil {
		var he *habits.Error
		if errors.As(err, &he) {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "code": he.Code, "error": he.Message})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to create habit set"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"id":          res.ID,
		"name":        res.Name,
		"description": res.Description,
		"color":       res.Color,
	})
}

func HabitSetList(c *gin.Context) {
	a := appFromCtx(c)
	limit, offset, ok := parsePagination(c)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid pagination params"})
		return
	}
	rows, err := a.Habits.ListHabitSets(limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to get habit sets"})
		return
	}
	c.JSON(http.StatusOK, rows)
}

func HabitSetUpdate(c *gin.Context) {
	a := appFromCtx(c)
	id, err := pathID(c, "/api/habit-sets/")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid id"})
		return
	}

	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Color       string `json:"color"`
		Wallpaper   string `json:"wallpaper"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid JSON"})
		return
	}
	res, err := a.Habits.UpdateHabitSet(id, habits.UpdateHabitSetInput{
		Name: req.Name, Description: req.Description, Color: req.Color, Wallpaper: req.Wallpaper,
	})
	if err != nil {
		var he *habits.Error
		if errors.As(err, &he) {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "code": he.Code, "error": he.Message})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to update habit set"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"id":          res.ID,
		"name":        res.Name,
		"description": res.Description,
		"color":       res.Color,
		"wallpaper":   res.Wallpaper,
	})
}

func HabitSetDelete(c *gin.Context) {
	a := appFromCtx(c)
	id, err := pathID(c, "/api/habit-sets/")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid id"})
		return
	}
	if err := a.Habits.DeleteHabitSet(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to delete habit set"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func HabitCreate(c *gin.Context) {
	a := appFromCtx(c)

	var req struct {
		SetID       int64  `json:"set_id"`
		Name        string `json:"name"`
		GoalSeconds int64  `json:"goal_seconds"`
		Color       string `json:"color"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid JSON"})
		return
	}
	res, err := a.Habits.CreateHabit(habits.CreateHabitInput{
		SetID: req.SetID, Name: req.Name, GoalSeconds: req.GoalSeconds, Color: req.Color,
	})
	if err != nil {
		var he *habits.Error
		if errors.As(err, &he) {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "code": he.Code, "error": he.Message})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to create habit"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"id":           res.ID,
		"set_id":       res.SetID,
		"name":         res.Name,
		"goal_seconds": res.GoalSeconds,
		"color":        res.Color,
	})
}

// HabitList 可用 `?set_id=N` 限定到单个 habit set。
func HabitList(c *gin.Context) {
	a := appFromCtx(c)
	limit, offset, ok := parsePagination(c)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid pagination params"})
		return
	}
	var setID *int64
	if setIDStr := c.Query("set_id"); setIDStr != "" {
		v, err := strconv.ParseInt(setIDStr, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid set_id"})
			return
		}
		setID = &v
	}
	rows, err := a.Habits.ListHabits(setID, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to get habits"})
		return
	}
	c.JSON(http.StatusOK, rows)
}

func HabitUpdate(c *gin.Context) {
	a := appFromCtx(c)
	id, err := pathID(c, "/api/habits/")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid id"})
		return
	}

	var req struct {
		Name        string `json:"name"`
		GoalSeconds int64  `json:"goal_seconds"`
		Color       string `json:"color"`
		Wallpaper   string `json:"wallpaper"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid JSON"})
		return
	}
	res, err := a.Habits.UpdateHabit(id, habits.UpdateHabitInput{
		Name: req.Name, GoalSeconds: req.GoalSeconds, Color: req.Color, Wallpaper: req.Wallpaper,
	})
	if err != nil {
		var he *habits.Error
		if errors.As(err, &he) {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "code": he.Code, "error": he.Message})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to update habit"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"id":           res.ID,
		"name":         res.Name,
		"goal_seconds": res.GoalSeconds,
		"color":        res.Color,
		"wallpaper":    res.Wallpaper,
	})
}

func HabitDelete(c *gin.Context) {
	a := appFromCtx(c)
	id, err := pathID(c, "/api/habits/")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid id"})
		return
	}
	if err := a.Habits.DeleteHabit(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to delete habit"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// HabitDetail 返回单个 habit，附今天累计秒数与进度百分比。
func HabitDetail(c *gin.Context) {
	a := appFromCtx(c)
	id, err := pathIDWithSuffix(c, "/api/habits/", "/detail")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid id"})
		return
	}

	date := c.Query("date")
	detail, err := a.Habits.GetHabitDetail(id, date)
	if err != nil {
		var he *habits.Error
		if errors.As(err, &he) {
			c.JSON(http.StatusNotFound, gin.H{"success": false, "code": he.Code, "error": he.Message})
			return
		}
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "Habit not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"id":               detail.ID,
		"name":             detail.Name,
		"goal_seconds":     detail.GoalSeconds,
		"color":            detail.Color,
		"today_seconds":    detail.TodaySeconds,
		"progress_percent": detail.ProgressPercent,
	})
}

func SessionCreate(c *gin.Context) {
	a := appFromCtx(c)

	var req struct {
		HabitID         int64 `json:"habit_id"`
		DurationSeconds int64 `json:"duration_seconds"`
		Count           int64 `json:"count"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid JSON"})
		return
	}

	id, err := a.Habits.CreateSession(habits.CreateSessionInput{
		HabitID: req.HabitID, DurationSeconds: req.DurationSeconds, Count: req.Count,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to create session"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"id":               id,
		"habit_id":         req.HabitID,
		"duration_seconds": req.DurationSeconds,
		"date":             domain.TodayString(a.Settings.Config().Basic.Timezone),
	})
}

func SessionDelete(c *gin.Context) {
	a := appFromCtx(c)
	id, err := pathID(c, "/api/sessions/")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid id"})
		return
	}
	if err := a.Habits.DeleteSession(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "Session not found"})
		return
	}
	c.JSON(http.StatusNoContent, nil)
}

// HabitStats 返回 habit 的聚合统计。
func HabitStats(c *gin.Context) {
	a := appFromCtx(c)
	id, err := pathIDWithSuffix(c, "/api/habits/", "/stats")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid id"})
		return
	}

	// 先确认 habit 存在
	if _, err := a.Habits.GetHabit(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "Habit not found"})
		return
	}

	stats, err := a.Habits.GetHabitStats(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to get stats"})
		return
	}
	c.JSON(http.StatusOK, stats)
}

// SessionList 支持三种查询形态：`?date=YYYY-MM-DD`、
// `?start_date=…&end_date=…`，或不带日期 → 今天。
func SessionList(c *gin.Context) {
	a := appFromCtx(c)
	limit, offset, ok := parsePagination(c)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid pagination params"})
		return
	}
	date := c.Query("date")
	startDate := c.Query("start_date")
	endDate := c.Query("end_date")
	rows, err := a.Habits.ListSessions(date, startDate, endDate, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to get sessions"})
		return
	}
	c.JSON(http.StatusOK, rows)
}

func TimerSessionCreate(c *gin.Context) {
	a := appFromCtx(c)

	var req struct {
		HabitID      *int64 `json:"habit_id,omitempty"`
		Mode         string `json:"mode,omitempty"`
		WorkDuration int64  `json:"work_duration,omitempty"`
		RestDuration int64  `json:"rest_duration,omitempty"`
		LoopCount    int64  `json:"loop_count,omitempty"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid JSON"})
		return
	}
	if req.WorkDuration == 0 {
		req.WorkDuration = 25 * 60
	}
	if req.Mode == "" {
		req.Mode = "stopwatch"
	}

	id, err := a.Habits.CreateTimerSession(req.HabitID, req.Mode, req.WorkDuration, req.RestDuration, req.LoopCount)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to create timer session"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"id": id})
}

// TimerSessionList 返回 `?id=N` 对应的 session；未给 id 时返回活动
// （未结束）session。
func TimerSessionList(c *gin.Context) {
	a := appFromCtx(c)
	var id *int64
	if idStr := c.Query("id"); idStr != "" {
		v, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid id"})
			return
		}
		id = &v
	}
	row, err := a.Habits.GetTimerSession(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "Timer session not found"})
		return
	}
	c.JSON(http.StatusOK, row)
}

func TimerSessionUpdate(c *gin.Context) {
	a := appFromCtx(c)
	id, err := pathID(c, "/api/timer-sessions/")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid id"})
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
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid JSON"})
		return
	}
	now := nowUnix()
	if err := a.Habits.UpdateTimerSession(
		id, req.ElapsedSeconds, req.RemainingSeconds,
		req.PausedTotalSeconds, req.PauseStartedAt, &now,
		req.IsRunning, req.IsPaused, req.IsFinished,
		req.CurrentRound, req.InRest,
	); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to update timer session"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func TimerSessionDelete(c *gin.Context) {
	a := appFromCtx(c)
	id, err := pathID(c, "/api/timer-sessions/")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "Invalid id"})
		return
	}
	if err := a.Habits.DeleteTimerSession(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to delete timer session"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}
