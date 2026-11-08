// Package handlers — 计时器 endpoint。
//
// 每个计时器处理器从 Gin 上下文取 *App（由 auth 中间件设置）。
package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"little-timer/internal/domain"
	"little-timer/internal/http/app"
	"little-timer/internal/timer"
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

	a.Lock()
	res := a.Timer.Progress()
	a.Unlock()

	c.JSON(http.StatusOK, gin.H{
		"session_id":        res.SessionID,
		"habit_id":          res.HabitID,
		"mode":              modeKey(res.Mode),
		"is_running":        res.IsRunning,
		"is_paused":         res.IsPaused,
		"is_finished":       res.IsFinished,
		"elapsed_seconds":   res.ElapsedSeconds,
		"remaining_seconds": res.RemainingSeconds,
		"in_rest":           res.InRest,
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
	_ = c.ShouldBindJSON(&req) // body 可选——默认值由 timer.Operations 负责。

	a.Lock()
	defer a.Unlock()

	res, err := a.Timer.Start(timer.StartInput{
		HabitID:      req.HabitID,
		Mode:         req.Mode,
		WorkDuration: req.WorkDuration,
		RestDuration: req.RestDuration,
		LoopCount:    req.LoopCount,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to create timer session"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"status":     res.Status,
		"habit_id":   res.HabitID,
		"session_id": res.SessionID,
	})
}

// POST /api/pause

func TimerPause(c *gin.Context) {
	a := appFromCtx(c)
	a.Lock()
	defer a.Unlock()
	a.Timer.Pause()
	c.JSON(http.StatusOK, gin.H{"status": "paused"})
}

// POST /api/reset

func TimerReset(c *gin.Context) {
	a := appFromCtx(c)
	a.Lock()
	defer a.Unlock()
	a.Timer.Reset()
	c.JSON(http.StatusOK, gin.H{"status": "reset"})
}

// POST /api/finish

// handleFinish。成功时创建一条与当前 habit 关联的每日会话记录，
// 使习惯统计保持同步。
func TimerFinish(c *gin.Context) {
	a := appFromCtx(c)
	a.Lock()
	defer a.Unlock()

	res, err := a.Timer.Finish()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Failed to finish timer session"})
		return
	}

	out := gin.H{
		"status":          res.Status,
		"elapsed_seconds": res.ElapsedSeconds,
	}
	// 兜底路径没有可报告的 session id；旧响应在该路径也不带这个字段。
	if res.SessionID != nil {
		out["session_id"] = *res.SessionID
	}
	c.JSON(http.StatusOK, out)
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
