// Package handlers —— AI GTD / tasks / llm settings 端点测试。
package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/gin-gonic/gin"

	"little-timer/internal/http/app"
)

// setupAIGtdRouter 注册 Task 7 的全部新路由。
func setupAIGtdRouter(t *testing.T, a *app.App) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("app", a)
		c.Next()
	})

	r.GET("/api/tasks", TaskList)
	r.POST("/api/tasks", TaskCreate)
	r.GET("/api/tasks/:id", TaskGet)
	r.PATCH("/api/tasks/:id", TaskUpdate)
	r.DELETE("/api/tasks/:id", TaskDelete)
	r.POST("/api/tasks/:id/subtasks", SubtaskCreate)
	r.PATCH("/api/tasks/:id/subtasks/:sub_id", SubtaskUpdate)
	r.DELETE("/api/tasks/:id/subtasks/:sub_id", SubtaskDelete)

	r.POST("/api/aigtd/capture", AIGtdCapture)
	r.POST("/api/aigtd/reparse/:task_id", AIGtdReparse)
	r.GET("/api/aigtd/jobs", AIGtdJobs)

	r.GET("/api/settings/llm", LLMSettingsGet)
	r.PUT("/api/settings/llm", LLMSettingsUpdate)
	return r
}

func doJSON(t *testing.T, r *gin.Engine, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatal(err)
		}
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestTaskCRUD(t *testing.T) {
	a := newTestApp(t)
	r := setupAIGtdRouter(t, a)

	// 创建
	w := doJSON(t, r, http.MethodPost, "/api/tasks", map[string]any{"title": "写报告", "notes": "周报"})
	if w.Code != http.StatusOK {
		t.Fatalf("create: code=%d body=%s", w.Code, w.Body.String())
	}
	var created TaskDTO
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.ID == 0 || created.Title != "写报告" {
		t.Fatalf("created = %+v", created)
	}

	// 列表可见
	w = doJSON(t, r, http.MethodGet, "/api/tasks", nil)
	var list []TaskDTO
	_ = json.Unmarshal(w.Body.Bytes(), &list)
	if len(list) != 1 || list[0].ID != created.ID {
		t.Fatalf("list = %+v", list)
	}

	// 局部更新:先设 due_date,再清空
	w = doJSON(t, r, http.MethodPatch, "/api/tasks/"+strconv.FormatInt(created.ID, 10), map[string]any{"due_date": "2026-12-01"})
	if w.Code != http.StatusOK {
		t.Fatalf("patch set due: code=%d", w.Code)
	}
	var patched TaskDTO
	_ = json.Unmarshal(w.Body.Bytes(), &patched)
	if patched.DueDate == nil || *patched.DueDate != "2026-12-01" {
		t.Fatalf("due_date = %v", patched.DueDate)
	}
	if patched.Notes != "周报" {
		t.Fatalf("notes changed unexpectedly: %q", patched.Notes)
	}

	w = doJSON(t, r, http.MethodPatch, "/api/tasks/"+strconv.FormatInt(created.ID, 10), map[string]any{"due_date": nil})
	var cleared TaskDTO
	_ = json.Unmarshal(w.Body.Bytes(), &cleared)
	if cleared.DueDate != nil {
		t.Fatalf("due_date should be cleared, got %v", *cleared.DueDate)
	}

	// 加子任务
	w = doJSON(t, r, http.MethodPost, "/api/tasks/"+strconv.FormatInt(created.ID, 10)+"/subtasks",
		map[string]any{"title": "列大纲", "estimated_minutes": 25})
	if w.Code != http.StatusOK {
		t.Fatalf("subtask create: code=%d body=%s", w.Code, w.Body.String())
	}

	// 详情含子任务
	w = doJSON(t, r, http.MethodGet, "/api/tasks/"+strconv.FormatInt(created.ID, 10), nil)
	var detail TaskDTO
	_ = json.Unmarshal(w.Body.Bytes(), &detail)
	if len(detail.Subtasks) != 1 || detail.Subtasks[0].Title != "列大纲" {
		t.Fatalf("subtasks = %+v", detail.Subtasks)
	}

	// 删除
	w = doJSON(t, r, http.MethodDelete, "/api/tasks/"+strconv.FormatInt(created.ID, 10), nil)
	if w.Code != http.StatusNoContent {
		t.Fatalf("delete: code=%d", w.Code)
	}
	w = doJSON(t, r, http.MethodGet, "/api/tasks/"+strconv.FormatInt(created.ID, 10), nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("get after delete: code=%d", w.Code)
	}
}

func TestCaptureEnqueuesJob(t *testing.T) {
	a := newTestApp(t)
	r := setupAIGtdRouter(t, a)

	w := doJSON(t, r, http.MethodPost, "/api/aigtd/capture", map[string]any{"raw_text": "准备下周汇报 PPT"})
	if w.Code != http.StatusAccepted {
		t.Fatalf("capture: code=%d body=%s", w.Code, w.Body.String())
	}
	var resp struct {
		TaskID int64 `json:"task_id"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.TaskID == 0 {
		t.Fatal("task_id missing")
	}

	// jobs 端点返回 pending
	w = doJSON(t, r, http.MethodGet, "/api/aigtd/jobs?task_id="+strconv.FormatInt(resp.TaskID, 10), nil)
	if w.Code != http.StatusOK {
		t.Fatalf("jobs: code=%d", w.Code)
	}
	var job struct {
		AIStatus    string `json:"ai_status"`
		LatestJobID int64  `json:"latest_job_id"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &job)
	if job.AIStatus != "pending" {
		t.Errorf("ai_status = %q, want pending", job.AIStatus)
	}
	if job.LatestJobID == 0 {
		t.Error("latest_job_id should be set (job enqueued)")
	}

	// 空文本 → 400
	w = doJSON(t, r, http.MethodPost, "/api/aigtd/capture", map[string]any{"raw_text": "  "})
	if w.Code != http.StatusBadRequest {
		t.Errorf("empty capture: code=%d, want 400", w.Code)
	}
}

func TestLLMSettings_RoundTrip(t *testing.T) {
	a := newTestApp(t)
	r := setupAIGtdRouter(t, a)

	// 设置主密码(解锁 SecretStorage)。
	if err := a.SetMasterPassword("testpass123"); err != nil {
		t.Fatalf("set master password: %v", err)
	}

	w := doJSON(t, r, http.MethodPut, "/api/settings/llm", map[string]any{
		"provider": "openai_compat",
		"base_url": "https://api.deepseek.com/v1",
		"model":    "deepseek-chat",
		"api_key":  "sk-test-1234",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("put: code=%d body=%s", w.Code, w.Body.String())
	}
	var got LLMSettingsDTO
	_ = json.Unmarshal(w.Body.Bytes(), &got)
	if !got.APIKeySet || got.APIKeyMasked != "***1234" {
		t.Fatalf("after put: %+v", got)
	}
	if got.Provider != "openai_compat" || got.Model != "deepseek-chat" {
		t.Fatalf("after put config: %+v", got)
	}

	// 清除 Key
	w = doJSON(t, r, http.MethodPut, "/api/settings/llm", map[string]any{"clear_key": true})
	_ = json.Unmarshal(w.Body.Bytes(), &got)
	if got.APIKeySet {
		t.Fatalf("key should be cleared: %+v", got)
	}
	// 非 Key 字段保留
	if got.Provider != "openai_compat" || got.Model != "deepseek-chat" {
		t.Errorf("config should survive clear_key: %+v", got)
	}
}

func TestLLMSettings_RejectsBadProvider(t *testing.T) {
	a := newTestApp(t)
	r := setupAIGtdRouter(t, a)

	w := doJSON(t, r, http.MethodPut, "/api/settings/llm", map[string]any{"provider": "bogus"})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("bad provider: code=%d, want 400", w.Code)
	}
}

