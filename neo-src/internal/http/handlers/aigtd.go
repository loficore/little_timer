// Package handlers —— /api/aigtd/* 端点:捕获、重新解析、job 状态查询。
//
// 捕获是异步的:handler 只负责建 task + 入队 ai_jobs,worker(在 cmd/server
// 启动)负责实际调用 LLM。
package handlers

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"little-timer/internal/aigtd"
	"little-timer/internal/domain"
	"little-timer/internal/http/middleware"
	"little-timer/internal/storage"
)

const maxRawTextLen = 2000

// hasInFlightJobFn / taskIsProcessingFn 是 reparse precheck 的可注入点。
// 生产默认实现直连 storage;测试替换后可绕过 precheck,确定性触达 DB 层
// 部分 UNIQUE 索引的 ErrJobInFlight→409 分支(reviewer M4)。
var (
	hasInFlightJobFn   = func(c *storage.AITasksCrud, taskID int64) (bool, error) { return c.HasInFlightJob(taskID) }
	taskIsProcessingFn = func(t *domain.TaskRow) bool { return t.AIStatus == domain.AIStatusProcessing }
)

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
	if _, err := a.SQLite.AITasks().EnqueueJob(taskID, cfg.Provider, cfg.Model, raw, domain.AIJobModeAppend); err != nil {
		// 刚 CreateTask 的行 version=0,rollback 时显式带 0(内部调用不走
		// ConflictGuard 的 context,这里必须自带版本)。
		_ = a.SQLite.AITasks().DeleteTask(taskID, 0)
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
	// 走可注入的 precheckFn:生产等价于直接调 storage;测试可绕过以确定性地
	// 触达 DB 层 idx_ai_jobs_in_flight 的 ErrJobInFlight→409 分支(reviewer M4)。
	inFlight, err := hasInFlightJobFn(a.SQLite.AITasks(), taskID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "job lookup failed"})
		return
	}
	if inFlight || taskIsProcessingFn(task) {
		c.JSON(http.StatusConflict, gin.H{"success": false, "code": "aigtd.processing", "error": "task is being processed"})
		return
	}

	// 可选改写原文 + 处理 mode。
	var req struct {
		RawText *string `json:"raw_text"`
		Mode    string  `json:"mode"`
	}
	_ = c.ShouldBindJSON(&req)
	mode := domain.AIJobMode(req.Mode)
	if req.Mode == "" {
		mode = domain.AIJobModeAppend
	}
	switch mode {
	case domain.AIJobModeAppend, domain.AIJobModeReplace:
		// ok
	default:
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid mode (allowed: append|replace)"})
		return
	}
	rawText := task.RawText
	if req.RawText != nil {
		raw := strings.TrimSpace(*req.RawText)
		if raw == "" || len([]rune(raw)) > maxRawTextLen {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid raw_text"})
			return
		}
		if err := a.SQLite.AITasks().UpdateTask(taskID, taskRawUpdate(raw), middleware.GetVersion(c)); err != nil {
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
	jobID, err := a.SQLite.AITasks().EnqueueJob(taskID, cfg.Provider, cfg.Model, rawText, mode)
	if err != nil {
		if errors.Is(err, storage.ErrJobInFlight) {
			c.JSON(http.StatusConflict, gin.H{"success": false, "code": "aigtd.processing", "error": "task is being processed"})
			return
		}
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
