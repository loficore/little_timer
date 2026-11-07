// Package handlers —— /api/aigtd/* 端点:捕获、重新解析、job 状态查询。
//
// 本文件只做 transport 绑定与 aigtd.Error → HTTP 状态码映射;原文校验、
// 入队、失败回滚、processing 前置检查全在 internal/aigtd(issue #66)。
//
// 捕获是异步的:operation 只负责建 task + 入队 ai_jobs,worker(在
// cmd/server 启动)负责实际调用 LLM。
package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"little-timer/internal/aigtd"
	"little-timer/internal/domain"
	"little-timer/internal/http/app"
	"little-timer/internal/http/middleware"
)

// hasInFlightJobFn / taskIsProcessingFn 是 reparse precheck 的可注入点。
// 生产默认实现直连 storage;测试替换后可绕过 precheck,确定性触达 DB 层
// 部分 UNIQUE 索引的 ErrJobInFlight→409 分支(reviewer M4)。
var (
	hasInFlightJobFn   = func(s aigtd.TaskStore, taskID int64) (bool, error) { return s.HasInFlightJob(taskID) }
	taskIsProcessingFn = func(t *domain.TaskRow) bool { return t.AIStatus == domain.AIStatusProcessing }
)

// aigtdOpsFor 构造 capture / reparse 的用例层。
//
// precheck 的两个可注入点保留在包级(测试替换后这条路径必须生效,见
// TestAIGtdReparse_DBUniqueFallback409):每次构造时把当前值焊进
// aigtd.InFlightProbe / ProcessingProbe,NewOperations 再快照一次。
func aigtdOpsFor(a *app.App) *aigtd.Operations {
	aigtd.InFlightProbe = func(s aigtd.TaskStore, taskID int64) (bool, error) {
		return hasInFlightJobFn(s, taskID)
	}
	aigtd.ProcessingProbe = func(t *domain.TaskRow) bool {
		return taskIsProcessingFn(t)
	}
	cfg := aiConfigFromApp(a)
	return aigtd.NewOperations(a.SQLite.AITasks(), func() aigtd.LLMConfig { return cfg })
}

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
	taskID, err := aigtdOpsFor(a).Capture(req.RawText, req.Source)
	if err != nil {
		respondAIGtdError(c, err, "aigtd.create_failed")
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"task_id": taskID})
}

// AIGtdReparse POST /api/aigtd/reparse/:task_id
//
// body { raw_text?, mode? } → 202 { job_id }
// 若任务正在 processing → 409。
func AIGtdReparse(c *gin.Context) {
	a := appFromCtx(c)
	taskID, err := parseInt64Param(c, "task_id")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid task_id"})
		return
	}
	var req struct {
		RawText *string `json:"raw_text"`
		Mode    string  `json:"mode"`
	}
	_ = c.ShouldBindJSON(&req)

	jobID, err := aigtdOpsFor(a).Reparse(taskID, req.RawText, req.Mode, middleware.GetVersion(c))
	if err != nil {
		respondAIGtdError(c, err, "aigtd.reparse_failed")
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"job_id": jobID})
}

// respondAIGtdError 是 aigtd.Error → HTTP 状态码的唯一映射点(issue #66)。
// fallbackCode 用于非类型化错误(500)。
func respondAIGtdError(c *gin.Context, err error, fallbackCode string) {
	var de *aigtd.Error
	if errors.As(err, &de) {
		switch {
		case errors.Is(err, aigtd.ErrProcessing):
			c.JSON(http.StatusConflict, gin.H{"success": false, "code": "aigtd.processing", "error": "task is being processed"})
			return
		case errors.Is(err, aigtd.ErrNotFound):
			c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "task not found"})
			return
		case errors.Is(err, aigtd.ErrEmpty):
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "code": "aigtd.empty", "error": "raw_text is empty"})
			return
		case errors.Is(err, aigtd.ErrTooLong):
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "code": "aigtd.too_long", "error": "raw_text too long"})
			return
		case errors.Is(err, aigtd.ErrBadRequest):
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid raw_text"})
			return
		}
	}
	c.JSON(http.StatusInternalServerError, gin.H{"success": false, "code": fallbackCode, "error": "request failed"})
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
