// Package handlers —— PATCH /api/tasks/:id/scores 端点测试。
//
// 关注点:部分字段更新 / context_tag 枚举兜底 / 乐观锁 409。
// 不复用 aigtd_test.go 的 setupAIGtdRouter 是因为它不挂 scores 路由;
// 这里就地 setupTasksRouter 注册完整的 task 路由(含 scores)+ ConflictGuard
// + doJSON 版本解析,3 个测试各自构造场景。
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
	"little-timer/internal/http/middleware"
)

// setupTasksRouter 注册 /api/tasks/* 与 /api/tasks/:id/scores 路由,
// 挂 ConflictGuard 中间件 + doJSON 所需的 testRouterApp。
func setupTasksRouter(t *testing.T, a *app.App) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("app", a)
		c.Next()
	})
	r.Use(middleware.RequireVersionForMutation())

	r.GET("/api/tasks", TaskList)
	r.POST("/api/tasks", TaskCreate)
	r.GET("/api/tasks/:id", TaskGet)
	r.PATCH("/api/tasks/:id", TaskUpdate)
	r.DELETE("/api/tasks/:id", TaskDelete)
	r.POST("/api/tasks/:id/subtasks", SubtaskCreate)
	r.PATCH("/api/tasks/:id/subtasks/:sub_id", SubtaskUpdate)
	r.DELETE("/api/tasks/:id/subtasks/:sub_id", SubtaskDelete)
	r.PATCH("/api/tasks/:id/scores", TaskScoresUpdate)

	testRouterApp = a
	t.Cleanup(func() { testRouterApp = nil })
	return r
}

// createTask 走 POST /api/tasks 建一条 task,返回 ID。
func createTask(t *testing.T, r *gin.Engine, title string) int64 {
	t.Helper()
	w := doJSON(t, r, http.MethodPost, "/api/tasks", map[string]any{"title": title})
	if w.Code != http.StatusOK {
		t.Fatalf("create task: code=%d body=%s", w.Code, w.Body.String())
	}
	var task TaskDTO
	if err := json.Unmarshal(w.Body.Bytes(), &task); err != nil {
		t.Fatalf("unmarshal task: %v", err)
	}
	if task.ID == 0 {
		t.Fatalf("task id missing: %s", w.Body.String())
	}
	return task.ID
}

// doJSONNoIfMatch 与 doJSON 行为一致但不发 If-Match header。
// 让 body 的 version 字段生效,刻意制造版本冲突用。
func doJSONNoIfMatch(t *testing.T, r *gin.Engine, method, path string, body any) *httptest.ResponseRecorder {
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

// TestTaskScoresUpdate_PartialFields 验证 PATCH /scores 只动传入字段,
// urgency_score 未传则保持 DB 默认 5。
func TestTaskScoresUpdate_PartialFields(t *testing.T) {
	a := newTestApp(t)
	r := setupTasksRouter(t, a)
	taskID := createTask(t, r, "t")
	path := "/api/tasks/" + strconv.FormatInt(taskID, 10) + "/scores"

	w := doJSON(t, r, http.MethodPatch, path, map[string]any{
		"version":        0,
		"priority_score": 5,
		"context_tag":    "编码",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
	var resp TaskDTO
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.PriorityScore != 5 {
		t.Errorf("priority_score=%d, want 5", resp.PriorityScore)
	}
	if resp.ContextTag != "编码" {
		t.Errorf("context_tag=%q, want %q", resp.ContextTag, "编码")
	}
	// 未改字段 urgency_score 保持 DB 默认 5(spec §3.1)。
	if resp.UrgencyScore != 5 {
		t.Errorf("urgency_score 被动 = %d, want 5", resp.UrgencyScore)
	}
}

// TestTaskScoresUpdate_InvalidContextTag_FallsBack 验证非枚举
// context_tag 落库为 "general",且响应带 X-LLM-Warning header。
func TestTaskScoresUpdate_InvalidContextTag_FallsBack(t *testing.T) {
	a := newTestApp(t)
	r := setupTasksRouter(t, a)
	taskID := createTask(t, r, "t")
	path := "/api/tasks/" + strconv.FormatInt(taskID, 10) + "/scores"

	w := doJSON(t, r, http.MethodPatch, path, map[string]any{
		"version":     0,
		"context_tag": "写代码", // 非枚举
	})
	if w.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
	if got := w.Header().Get("X-LLM-Warning"); got == "" {
		t.Errorf("X-LLM-Warning header 未设置, want 非空")
	}
	// 响应 DTO 应为 "general"
	var resp TaskDTO
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.ContextTag != "general" {
		t.Errorf("response context_tag=%q, want %q", resp.ContextTag, "general")
	}
	// DB 真实值
	row, _, err := a.SQLite.AITasks().GetTask(taskID)
	if err != nil || row == nil {
		t.Fatalf("get task: %v", err)
	}
	if row.ContextTag != "general" {
		t.Errorf("DB context_tag=%q, want %q", row.ContextTag, "general")
	}
}

// TestTaskScoresUpdate_409OnVersionMismatch 验证乐观锁:
// 第一次更新用当前版本成功,第二次用 version=0(且不发 If-Match
// 以免被 doJSON 自动注入最新版本)必触发 409。
func TestTaskScoresUpdate_409OnVersionMismatch(t *testing.T) {
	a := newTestApp(t)
	r := setupTasksRouter(t, a)
	taskID := createTask(t, r, "t")
	path := "/api/tasks/" + strconv.FormatInt(taskID, 10) + "/scores"

	// 第一次更新(doJSON 自动带 If-Match = 0 = 新建版本)→ 成功
	w := doJSON(t, r, http.MethodPatch, path, map[string]any{
		"version":        0,
		"priority_score": 4,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("first: code=%d body=%s", w.Code, w.Body.String())
	}

	// 第二次仍用 version=0,且不走 doJSON(避免注入最新 If-Match)→ 409
	w = doJSONNoIfMatch(t, r, http.MethodPatch, path, map[string]any{
		"version":        0,
		"priority_score": 5,
	})
	if w.Code != http.StatusConflict {
		t.Errorf("code=%d, want 409; body=%s", w.Code, w.Body.String())
	}
}
