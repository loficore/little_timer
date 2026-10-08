// Package handlers —— /api/settings/daily-sweep:GTD Daily Sweep 触发配置读写。
//
// 两列 daily_sweep_enabled / daily_sweep_time 直接写在 settings 表(v12),
// 与 llm_settings.go 同风格:不敏感、无主密码要求。时间用应用层正则校验
// (HH:MM 24 小时制),非法 → 400 daily_sweep.invalid_time,不落库。
package handlers

import (
	"net/http"
	"regexp"

	"github.com/gin-gonic/gin"
)

// dailySweepTimeRE 校验 HH:MM 24 小时制(HH ∈ [00,23]、MM ∈ [00,59])。
// 定义见 docs/superpowers/specs/2026-10-08-gtd-review-gaps-design.md §3.2。
var dailySweepTimeRE = regexp.MustCompile(`^(?:[01]\d|2[0-3]):[0-5]\d$`)

// DailySweepSettingsDTO 是 GET/PUT /api/settings/daily-sweep 的出入参。
type DailySweepSettingsDTO struct {
	Enabled bool   `json:"enabled"`
	Time    string `json:"time"`
}

// DailySweepSettingsGet GET /api/settings/daily-sweep
func DailySweepSettingsGet(c *gin.Context) {
	a := appFromCtx(c)
	enabled, at, err := a.SQLite.DailySweep().GetSettings()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "read failed"})
		return
	}
	c.JSON(http.StatusOK, DailySweepSettingsDTO{Enabled: enabled, Time: at})
}

// DailySweepSettingsUpdate PUT /api/settings/daily-sweep
//
// body 字段(全部可选):enabled(bool)、time(HH:MM)。只更新出现的字段。
// time 非法 → 400 {"code":"daily_sweep.invalid_time"}。
func DailySweepSettingsUpdate(c *gin.Context) {
	a := appFromCtx(c)
	var body map[string]any
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid json"})
		return
	}

	var enabledPtr *bool
	if v, ok := body["enabled"].(bool); ok {
		enabledPtr = &v
	}
	var timePtr *string
	if v, ok := body["time"].(string); ok {
		if !dailySweepTimeRE.MatchString(v) {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"code":    "daily_sweep.invalid_time",
				"error":   "time must be HH:MM (24h)",
			})
			return
		}
		timePtr = &v
	}

	if err := a.SQLite.DailySweep().SetSettings(enabledPtr, timePtr); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "update failed"})
		return
	}
	enabled, at, err := a.SQLite.DailySweep().GetSettings()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "read failed"})
		return
	}
	c.JSON(http.StatusOK, DailySweepSettingsDTO{Enabled: enabled, Time: at})
}
