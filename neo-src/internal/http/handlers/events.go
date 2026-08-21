// Package handlers —— Server-Sent Events 流。
//
// Wire format：`event: <type>\ndata: <json>\n\n`。显式的 `event:` 行
// 让前端 EventSource 按事件名路由。
package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// handleEvents 以 1Hz 向客户端流式推送 clock 状态：连接时立即发一个
// `state_changed` 事件，随后每秒一个 `tick` 事件。
func Events(c *gin.Context) {
	a := appFromCtx(c)

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"err": "streaming unsupported"})
		return
	}

	// 先推一次初始状态，别让客户端干等第一个 tick。
	a.RLock()
	habitID := a.CurrentHabitID
	a.RUnlock()
	state := a.Clock.Update()
	writeSSE(c.Writer, "state_changed", buildStateResponse(state, modeKey(state.GetMode()), a.Settings.Config().Basic.Timezone, habitID))
	flusher.Flush()

	const (
		maxSession = time.Hour
		heartbeat  = 10 * time.Second
		maxGap     = 30 * time.Second
	)
	start := time.Now()
	lastHeartbeat := time.Now()

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	clientGone := c.Request.Context().Done()
	for {
		select {
		case <-clientGone:
			return
		case <-ticker.C:
			if time.Since(start) > maxSession {
				return
			}
			if time.Since(lastHeartbeat) > maxGap {
				return
			}
			a.RLock()
			habitID := a.CurrentHabitID
			a.RUnlock()
			state := a.Clock.Update()
			writeSSE(c.Writer, "tick", buildStateResponse(state, modeKey(state.GetMode()), a.Settings.Config().Basic.Timezone, habitID))
			if time.Since(lastHeartbeat) >= heartbeat {
				lastHeartbeat = time.Now()
				_, _ = fmt.Fprint(c.Writer, ": heartbeat\n\n")
			}
			flusher.Flush()
		}
	}
}

// writeSSE 向给定 writer 写入一个形如下面的 SSE 帧：
//
//	event: <name>\ndata: <json>\n\n
//
// data 行是单行 JSON；内嵌换行不做转义（state JSON 从不包含字面换行符）。
func writeSSE(w http.ResponseWriter, event string, payload any) {
	body, err := json.Marshal(payload)
	if err != nil {
		// 回退为空事件，保持流存活。
		_, _ = fmt.Fprintf(w, "event: %s\ndata: {}\n\n", event)
		return
	}
	_, _ = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, body)
}

// FrontendLog 接收浏览器发来的 JSON 日志条目，并用 server logger 重新
// 输出。暴露在 `POST /api/log`（公开，无需鉴权）。
func FrontendLog(c *gin.Context) {
	var entry struct {
		Category string `json:"category"`
		Level    string `json:"level"`
		Message  string `json:"message"`
		Runtime  string `json:"runtime"`
	}
	raw, err := c.GetRawData()
	if err != nil || len(raw) == 0 {
		c.JSON(http.StatusOK, gin.H{"success": false, "err": "empty body"})
		return
	}
	if err := json.Unmarshal(raw, &entry); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "err": "invalid json"})
		return
	}
	if entry.Level == "" {
		entry.Level = "info"
	}
	log.Printf("[frontend:%s][%s] %s: %s", entry.Category, entry.Runtime, strings.ToUpper(entry.Level), entry.Message)
	c.JSON(http.StatusOK, gin.H{"success": true})
}
