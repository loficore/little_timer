// Package handlers —— Settings endpoint（GET/POST /api/settings）。
//
// GET 以 JSON 返回完整的 SettingsConfig blob（basic、clock_defaults、
// logging、auth）；POST 接受部分字段的 SettingsConfig body。
package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"little-timer/internal/domain"
)

func SettingsGet(c *gin.Context) {
	a := appFromCtx(c)
	cfg := a.Settings.Config()
	c.JSON(http.StatusOK, cfg)
}

// SettingsUpdate 接受部分字段的 SettingsConfig body ——
// `parseSettingsFromJSON` 能容忍缺失字段。
func SettingsUpdate(c *gin.Context) {
	a := appFromCtx(c)
	raw, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "read body"})
		return
	}
	if err := a.Settings.HandleSettingsEvent(domain.SettingsChangeEvent{JSON: string(raw)}); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}
	// 合理性检查：响应必须是合法的 JSON 对象（Gin 的 c.JSON 拒绝编码
	// nil interface）。
	out := gin.H{"status": "settings_updated"}
	c.JSON(http.StatusOK, out)
}
