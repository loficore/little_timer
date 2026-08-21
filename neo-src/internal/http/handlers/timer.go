// Package handlers — 计时器 endpoint。
//
// 每个计时器处理器从 Gin 上下文取 *App（由 auth 中间件设置）。
package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"little-timer/internal/domain"
	"little-timer/internal/http/app"
)

// 硬编码计时器时长的包级常量。
const (
	DefaultWorkDuration = 25 * 60 // 秒
	DefaultRestDuration = 5 * 60  // 秒
)

// appFromCtx 取出 auth 中间件放入的 *App。若中间件未安装则
// panic —— 那是程序员错误，不是需要处理的运行时状况。
func appFromCtx(c *gin.Context) *app.App {
	return c.MustGet("app").(*app.App)
}

// buildStateResponse 以 map 形式返回状态 JSON 对象，使字段顺序
// 在各 Go 版本间保持稳定（Gin 的 JSON 编码器会对键排序）。
func buildStateResponse(state *domain.ClockState, modeKey string, timezone int8, habitID *int64) gin.H {
	out := gin.H{
		"time":           state.GetTimeInfo(),
		"elapsed":        state.GetElapsedSeconds(),
		"mode":           modeKey,
		"is_running":     !state.IsPaused(),
		"is_finished":    state.IsFinished(),
		"in_rest":        state.InRest(),
		"loop_remaining": state.GetLoopRemaining(),
		"loop_total":     state.GetLoopTotal(),
		"rest_remaining": state.GetRestRemainingTime(),
		"timezone":       timezone,
	}
	if habitID != nil {
		out["habit_id"] = *habitID
	}
	return out
}

func modeKey(m domain.ModeEnum) string {
	if m == domain.CountdownMode {
		return "countdown"
	}
	return "stopwatch"
}

// GET /api/state

func TimerState(c *gin.Context) {
	a := appFromCtx(c)
	state := a.Clock.Update()
	tz := a.Settings.Config().Basic.Timezone

	a.RLock()
	habitID := a.CurrentHabitID
	a.RUnlock()

	c.JSON(http.StatusOK, buildStateResponse(state, modeKey(state.GetMode()), tz, habitID))
}

// GET /api/timer/progress

// handleGetProgress 返回实时进度 + 模式 + paused/finished 标志。
// 若无活动会话则惰性加载进度。
func TimerProgress(c *gin.Context) {
	a := appFromCtx(c)

	a.RLock()
	hasSession := a.CurrentTimerSessionID != nil
	a.RUnlock()
	if !hasSession {
		a.LoadTimerProgress()
	}

	a.RLock()
	sessionID := a.CurrentTimerSessionID
	habitID := a.CurrentHabitID
	a.RUnlock()

	state := a.Clock.Update()
	c.JSON(http.StatusOK, gin.H{
		"session_id":        sessionID,
		"habit_id":          habitID,
		"mode":              modeKey(state.GetMode()),
		"is_running":        !state.IsPaused(),
		"is_paused":         state.IsPaused(),
		"is_finished":       state.IsFinished(),
		"elapsed_seconds":   state.GetElapsedSeconds(),
		"remaining_seconds": state.GetRemainingSeconds(),
		"in_rest":           state.InRest(),
	})
}

// POST /api/start

// startRequest 是 `POST /api/start` 的 JSON 请求体。所有字段均可选。
type startRequest struct {
	HabitID      *int64 `json:"habit_id,omitempty"`
	Mode         string `json:"mode,omitempty"`
	WorkDuration int64  `json:"work_duration,omitempty"`
	RestDuration int64  `json:"rest_duration,omitempty"`
	LoopCount    int64  `json:"loop_count,omitempty"`
}

// handleStart。Body: {habit_id?, mode?, work_duration?, rest_duration?, loop_count?}。
func TimerStart(c *gin.Context) {
	a := appFromCtx(c)

	var req startRequest
	_ = c.ShouldBindJSON(&req) // body 可选——下方有默认值。

	mode := "stopwatch"
	if req.Mode == "countdown" {
		mode = "countdown"
	}
	work := req.WorkDuration
	if work == 0 {
		work = DefaultWorkDuration
	}
	rest := req.RestDuration
	loop := req.LoopCount

	a.Lock()
	defer a.Unlock()

	// 已在运行的分支——保留同一会话并上报当前 habit id。
	if a.CurrentTimerSessionID != nil {
		// 查询实时数据行。
		row, err := a.SQLite.Timers().GetTimerSessionByID(*a.CurrentTimerSessionID)
		if err == nil {
			if row.IsRunning && !row.IsFinished && !row.IsPaused {
				a.CurrentHabitID = req.HabitID
				if a.CurrentHabitID == nil && row.HabitID != nil {
					h := *row.HabitID
					a.CurrentHabitID = &h
				}
				c.JSON(http.StatusOK, gin.H{
					"status":     "already_running",
					"habit_id":   a.CurrentHabitID,
					"session_id": *a.CurrentTimerSessionID,
				})
				return
			}
			// 暂停分支——恢复。
			state := a.Clock.Update()
			if state.IsPaused() && (!state.IsFinished() || row.IsPaused) {
				pausedTotal := row.PausedTotalSeconds
				now := time.Now().Unix()
				if row.PauseStartedAt != nil && now > *row.PauseStartedAt {
					pausedTotal += now - *row.PauseStartedAt
				}
				a.Clock.HandleEvent(domain.UserStartTimerEvent{})
				_ = a.SQLite.Timers().UpdateTimerSession(
					row.ID, row.ElapsedSeconds, row.RemainingSeconds,
					pausedTotal, nil, &now,
					true, false, false,
					row.CurrentRound, row.InRest,
				)
				a.CurrentHabitID = req.HabitID
				if a.CurrentHabitID == nil && row.HabitID != nil {
					h := *row.HabitID
					a.CurrentHabitID = &h
				}
				c.JSON(http.StatusOK, gin.H{
					"status":     "started",
					"habit_id":   a.CurrentHabitID,
					"session_id": row.ID,
				})
				return
			}
		}
		// 过期会话——清理。
		a.ResetTimerSession()
	}

	sessionID, err := a.CreateTimerSession(req.HabitID, mode, work, rest, loop)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to create timer session"})
		return
	}
	a.CurrentHabitID = req.HabitID
	a.Clock.HandleEvent(domain.UserStartTimerEvent{})
	c.JSON(http.StatusOK, gin.H{
		"status":     "started",
		"habit_id":   req.HabitID,
		"session_id": sessionID,
	})
}

// POST /api/pause

func TimerPause(c *gin.Context) {
	a := appFromCtx(c)
	a.Lock()
	defer a.Unlock()
	a.Clock.HandleEvent(domain.UserPauseTimerEvent{})
	a.SaveProgressLocked()
	c.JSON(http.StatusOK, gin.H{"status": "paused"})
}

// POST /api/reset

func TimerReset(c *gin.Context) {
	a := appFromCtx(c)
	a.Lock()
	defer a.Unlock()
	a.ResetTimerSession()
	a.CurrentHabitID = nil
	a.Clock.HandleEvent(domain.UserResetTimerEvent{})
	c.JSON(http.StatusOK, gin.H{"status": "reset"})
}

// POST /api/finish

// handleFinish。成功时创建一条与当前 habit 关联的每日会话记录，
// 使习惯统计保持同步。
func TimerFinish(c *gin.Context) {
	a := appFromCtx(c)
	a.Lock()
	defer a.Unlock()

	habitID := a.CurrentHabitID
	sessionID := a.CurrentTimerSessionID

	elapsed, err := a.FinishTimerSession()
	if err != nil {
		// 兜底路径:发出 user_finish_timer，从时钟状态计算 elapsed，
		// 并在存在活动 habit 时持久化一条每日会话记录。
		a.Clock.HandleEvent(domain.UserFinishTimerEvent{})
		state := a.Clock.Update()
		elapsedSeconds := state.GetElapsedSeconds()
		if habitID != nil && elapsedSeconds > 0 {
			if _, err := a.SQLite.Timers().CreateSession(*habitID, elapsedSeconds, 1, domain.TodayString(a.Settings.Config().Basic.Timezone)); err != nil {
				log.Printf("failed to create fallback session: %v", err)
			}
		}
		a.ResetTimerSession()
		c.JSON(http.StatusOK, gin.H{
			"status":          "finished",
			"elapsed_seconds": elapsedSeconds,
		})
		return
	}

	if habitID != nil && elapsed > 0 {
		if _, err := a.SQLite.Timers().CreateSession(*habitID, elapsed, 1, domain.TodayString(a.Settings.Config().Basic.Timezone)); err != nil {
			log.Printf("failed to create session at end: %v", err)
		}
	}
	a.ResetTimerSession()

	c.JSON(http.StatusOK, gin.H{
		"status":          "finished",
		"elapsed_seconds": elapsed,
		"session_id":      sessionID,
	})
}

// POST /api/mode

// handleModeSwitch。Body 为 JSON 对象 `{"mode":"countdown"|"stopwatch"}`。
func TimerMode(c *gin.Context) {
	a := appFromCtx(c)

	var body struct {
		Mode string `json:"mode"`
	}
	// 两种形式都接受:带 "mode" 的 JSON 对象，或裸字符串。浏览器客户端
	// 始终发送 JSON，因此这是常见路径。
	raw, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{})
		return
	}
	trimmed := strings.TrimSpace(string(raw))
	if strings.HasPrefix(trimmed, "{") {
		_ = json.Unmarshal(raw, &body)
		trimmed = strings.TrimSpace(body.Mode)
	}

	var newMode domain.ModeEnum
	switch trimmed {
	case "countdown":
		newMode = domain.CountdownMode
	case "stopwatch":
		newMode = domain.StopwatchMode
	default:
		c.JSON(http.StatusOK, gin.H{})
		return
	}

	a.Clock.HandleEvent(domain.UserChangeModeEvent{Mode: newMode})
	c.JSON(http.StatusOK, gin.H{
		"status":   "mode_changed",
		"new_mode": trimmed,
	})
}

// POST /api/timer/rest

// handleStartRest 把时钟切换到 5 分钟倒计时并启动。
func TimerStartRest(c *gin.Context) {
	a := appFromCtx(c)
	const restSeconds uint64 = DefaultRestDuration

	a.Clock.HandleEvent(domain.UserChangeConfigEvent{
		Config: domain.ClockTaskConfig{
			DefaultMode: domain.CountdownMode,
			Countdown: domain.CountdownConfig{
				DurationSeconds:     restSeconds,
				Loop:                false,
				LoopCount:           0,
				LoopIntervalSeconds: 0,
			},
			Stopwatch: domain.StopwatchConfig{
				MaxSeconds: 24 * 3600,
			},
		},
	})
	a.Clock.HandleEvent(domain.UserStartTimerEvent{})
	c.JSON(http.StatusOK, gin.H{
		"status":       "rest_started",
		"rest_seconds": restSeconds,
	})
}

// GET /api/timer/config  /  POST /api/timer/config

func TimerConfig(c *gin.Context) {
	a := appFromCtx(c)
	cfg := a.Settings.BuildClockConfig()
	c.JSON(http.StatusOK, gin.H{
		"default_mode": cfg.DefaultMode.String(),
		"countdown": gin.H{
			"duration_seconds":      cfg.Countdown.DurationSeconds,
			"loop":                  cfg.Countdown.Loop,
			"loop_count":            cfg.Countdown.LoopCount,
			"loop_interval_seconds": cfg.Countdown.LoopIntervalSeconds,
		},
		"stopwatch": gin.H{
			"max_seconds": cfg.Stopwatch.MaxSeconds,
		},
	})
}

// TimerUpdateConfig 应用部分配置更新。Body 为 ClockTaskConfig 形状，
// JSON 字段名与 `GET` 相同。
func TimerUpdateConfig(c *gin.Context) {
	a := appFromCtx(c)
	var req domain.ClockTaskConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid json"})
		return
	}
	a.Clock.HandleEvent(domain.UserChangeConfigEvent{Config: req})
	c.JSON(http.StatusOK, gin.H{"status": "config_updated"})
}

// 内部实现。
