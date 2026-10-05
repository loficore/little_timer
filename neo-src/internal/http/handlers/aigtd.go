// Package handlers —— /api/aigtd/* 端点:捕获、重新解析、job 状态查询。
//
// 捕获是异步的:handler 只负责建 task + 入队 ai_jobs,worker(在 cmd/server
// 启动)负责实际调用 LLM。
package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"little-timer/internal/aigtd"
	"little-timer/internal/domain"
)

const maxRawTextLen = 2000

// AIGtdCapture POST /api/aigtd/capture
//
// body { raw_text, source? } → 202 { task_id }
func AIGtdCapture(c *gin.Context) {
	a := appFromCtx(c)
	var req struct {
		RawText string `json:"raw_text"`
		Source  string `json:"source"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "code": "aigtd.bad_request", "error": "invalid json"})
		return
	}
	raw := strings.TrimSpace(req.RawText)
	if raw == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "code": "aigtd.empty", "error": "raw_text is empty"})
		return
	}
	if len([]rune(raw)) > maxRawTextLen {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "code": "aigtd.too_long", "error": "raw_text too long"})
		return
	}
	source := req.Source
	if source == "" {
		source = "manual"
	}

	taskID, err := a.SQLite.AITasks().CreateTask(raw, source)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "code": "aigtd.create_failed", "error": "create failed"})
		return
	}

	// 立即入队(worker 在后台处理)。失败时回滚 CreateTask,避免出现
	// "卡片永远停在 Parsing 状态"的孤儿记录。
	cfg := aiConfigFromApp(a)
	if _, err := a.SQLite.AITasks().EnqueueJob(taskID, cfg.Provider, cfg.Model, raw); err != nil {
		_ = a.SQLite.AITasks().DeleteTask(taskID)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "code": "aigtd.enqueue_failed", "error": "enqueue failed"})
		return
	}

	c.JSON(http.StatusAccepted, gin.H{"task_id": taskID})
}

// AIGtdReparse POST /api/aigtd/reparse/:task_id
//
// body { raw_text? } → 202 { job_id }
// 若任务正在 processing → 409。
func AIGtdReparse(c *gin.Context) {
	a := appFromCtx(c)
	taskID, err := parseInt64Param(c, "task_id")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid task_id"})
		return
	}
	task, _, err := a.SQLite.AITasks().GetTask(taskID)
	if err != nil || task == nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "task not found"})
		return
	}
	// 已有 queued/running job 时拒绝重复入队(可能来自快速双击或重试风暴)。
	inFlight, err := a.SQLite.AITasks().HasInFlightJob(taskID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "job lookup failed"})
		return
	}
	if inFlight || task.AIStatus == domain.AIStatusProcessing {
		c.JSON(http.StatusConflict, gin.H{"success": false, "code": "aigtd.processing", "error": "task is being processed"})
		return
	}

	// 可选改写原文。
	var req struct {
		RawText *string `json:"raw_text"`
	}
	_ = c.ShouldBindJSON(&req)
	rawText := task.RawText
	if req.RawText != nil {
		raw := strings.TrimSpace(*req.RawText)
		if raw == "" || len([]rune(raw)) > maxRawTextLen {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid raw_text"})
			return
		}
		if err := a.SQLite.AITasks().UpdateTask(taskID, taskRawUpdate(raw)); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "update failed"})
			return
		}
		rawText = raw
	}

	if err := a.SQLite.AITasks().SetAIStatus(taskID, domain.AIStatusPending, ""); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "reset failed"})
		return
	}
	cfg := aiConfigFromApp(a)
	jobID, err := a.SQLite.AITasks().EnqueueJob(taskID, cfg.Provider, cfg.Model, rawText)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "enqueue failed"})
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"job_id": jobID})
}

// AIGtdJobs GET /api/aigtd/jobs?task_id=X
func AIGtdJobs(c *gin.Context) {
	a := appFromCtx(c)
	taskID, err := parseInt64Query(c, "task_id")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid task_id"})
		return
	}
	task, _, err := a.SQLite.AITasks().GetTask(taskID)
	if err != nil || task == nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "task not found"})
		return
	}
	resp := gin.H{
		"task_id":   taskID,
		"ai_status": string(task.AIStatus),
		"ai_error":  task.AIError,
	}
	if job, _ := a.SQLite.AITasks().LatestJobForTask(taskID); job != nil {
		resp["latest_job_id"] = job.ID
		resp["attempts"] = job.Attempts
	}
	c.JSON(http.StatusOK, resp)
}

// AIGtdTestPrompt POST /api/aigtd/_test/prompt —— 注入测试用 LLM 响应。
// body { text?: string, fail_with?: number }
// 设为空 text 或 fail_with>0 都清除覆盖。
func AIGtdTestPrompt(c *gin.Context) {
	var body struct {
		Text     string `json:"text"`
		FailWith int    `json:"fail_with"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid json"})
		return
	}
	if body.Text == "" && body.FailWith == 0 {
		aigtd.ClearTestOverride()
	} else {
		aigtd.SetTestOverride(body.Text, body.FailWith)
	}
	c.JSON(http.StatusOK, gin.H{"override": body.Text != "" || body.FailWith != 0})
}
