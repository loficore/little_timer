// Package handlers —— AI GTD / tasks / llm settings 端点测试。
package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"

	"little-timer/internal/domain"
	"little-timer/internal/http/app"
	"little-timer/internal/http/middleware"
	"little-timer/internal/storage"
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
	// 与生产 router 对齐:所有 mutation 路由强制带 version(乐观锁 plumbing)。
	r.Use(middleware.RequireVersionForMutation())

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
	testRouterApp = a
	t.Cleanup(func() { testRouterApp = nil })
	return r
}

// testRouterApp 由 setupAIGtdRouter 记录,供 doJSON 根据路径查 task 当前
// version 用。测试串行执行,单包内不并发访问这个变量即可。
var testRouterApp *app.App

// doJSON 发请求并返回 recorder。
//
// 与生产对齐:所有 mutation 必带 version(ConflictGuard)。乐观锁 plumbing
// (Task 4) 让 handler 用 middleware.GetVersion(c);为避免每条用例手动跟踪
// 每次 mutation 后的 version,这里从 /api/tasks/{id}* 路径解析 task ID,
// 查 DB 拿当前 server 端 version 作为 If-Match;其余路径回退 "0"。
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
	req.Header.Set("If-Match", currentVersionForPath(method, path))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// currentVersionForPath 读 path 中 task ID 对应的当前 DB version。
// 读方法(GET/HEAD/OPTIONS)中间件放行,不需要版本,返 "0"。
//
// 子任务路径 `/api/tasks/:id/subtasks/:sub_id` 的乐观锁 version 是
// **子任务**自身的 version(子任务也是 tasks 行,与顶层 task 同源)。
// 顶层 task 的 version 不动 → 不能用它作 If-Match,否则第一次 PATCH 后
// 后续 PATCH 永远 409(GetTaskVersion(taskID) 一直=0)。
func currentVersionForPath(method, path string) string {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return "0"
	}
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	if len(parts) >= 3 && parts[0] == "api" && parts[1] == "tasks" {
		if testRouterApp == nil || testRouterApp.SQLite == nil {
			return "0"
		}
		// 子任务路径:/api/tasks/{task_id}/subtasks/{sub_id}
		if len(parts) >= 5 && parts[3] == "subtasks" {
			subID, err := strconv.ParseInt(parts[4], 10, 64)
			if err != nil {
				return "0"
			}
			v, err := testRouterApp.SQLite.AITasks().GetTaskVersion(subID)
			if err != nil {
				return "0"
			}
			return strconv.FormatInt(v, 10)
		}
		// 顶层 task 路径:/api/tasks/{id}[/scores]
		id, err := strconv.ParseInt(parts[2], 10, 64)
		if err != nil {
			return "0"
		}
		v, err := testRouterApp.SQLite.AITasks().GetTaskVersion(id)
		if err != nil {
			return "0"
		}
		return strconv.FormatInt(v, 10)
	}
	return "0"
}

func TestSubtaskUpdate_StatusValidation(t *testing.T) {
	// B1 fix regression: handler 必须拒绝 status 不在 {active,done,archived} 的请求,
	// 返回 400 而不是 500(SQLite CHECK constraint 拒绝 'pending'/'skipped')。
	a := newTestApp(t)
	r := setupAIGtdRouter(t, a)

	w := doJSON(t, r, http.MethodPost, "/api/tasks", map[string]any{"title": "t"})
	if w.Code != http.StatusOK {
		t.Fatalf("create task: code=%d body=%s", w.Code, w.Body.String())
	}
	var task TaskDTO
	_ = json.Unmarshal(w.Body.Bytes(), &task)

	w = doJSON(t, r, http.MethodPost, "/api/tasks/"+strconv.FormatInt(task.ID, 10)+"/subtasks",
		map[string]any{"title": "s"})
	if w.Code != http.StatusOK {
		t.Fatalf("create subtask: code=%d body=%s", w.Code, w.Body.String())
	}
	var sub SubtaskDTO
	_ = json.Unmarshal(w.Body.Bytes(), &sub)
	subPath := "/api/tasks/" + strconv.FormatInt(task.ID, 10) + "/subtasks/" + strconv.FormatInt(sub.ID, 10)

	// 合法值:全部 200(用 PATCH 子任务 status)
	for _, status := range []string{"active", "done", "archived"} {
		w = doJSON(t, r, http.MethodPatch, subPath, map[string]any{"status": status})
		if w.Code != http.StatusOK && w.Code != http.StatusNoContent {
			t.Errorf("status=%q: code=%d body=%s", status, w.Code, w.Body.String())
		}
	}

	// 非法值:必须 400,且不返回 500(否则 SQLite CHECK 触发)
	for _, bad := range []string{"pending", "skipped", "inbox", "rejected", "", "xyz"} {
		w = doJSON(t, r, http.MethodPatch, subPath, map[string]any{"status": bad})
		if w.Code == http.StatusInternalServerError {
			t.Errorf("status=%q should not 500 (CHECK failure path): body=%s", bad, w.Body.String())
		}
		if w.Code != http.StatusBadRequest {
			t.Errorf("status=%q: want 400, got %d", bad, w.Code)
		}
	}
}

func TestAIGtdReparse_SecondInFlight409(t *testing.T) {
	a := newTestApp(t)
	r := setupAIGtdRouter(t, a)

	w := doJSON(t, r, http.MethodPost, "/api/tasks", map[string]any{"title": "t", "raw_text": "r"})
	var task TaskDTO
	_ = json.Unmarshal(w.Body.Bytes(), &task)
	path := "/api/aigtd/reparse/" + strconv.FormatInt(task.ID, 10)

	w = doJSON(t, r, http.MethodPost, path, map[string]any{})
	if w.Code != http.StatusAccepted {
		t.Fatalf("first reparse: code=%d body=%s", w.Code, w.Body.String())
	}
	// 第二个:HasInFlightJob 或部分 UNIQUE 索引 → 409
	w = doJSON(t, r, http.MethodPost, path, map[string]any{})
	if w.Code != http.StatusConflict {
		t.Fatalf("second reparse: code=%d body=%s, want 409", w.Code, w.Body.String())
	}
}

// TestAIGtdReparse_DBUniqueFallback409 确定性测 reviewer M4 修复路径:
// 绕过 precheck(HasInFlightJob + AIStatus==processing 都注入返回 false),
// 直接在 DB 种一个 queued job(同 task)→ 调 reparse → EnqueueJob 必被
// idx_ai_jobs_in_flight 部分 UNIQUE 索引拒 → handler 必须把 ErrJobInFlight
// 映射为 409(而非 500)。这条分支是 race 兜底,正常路径被 precheck 抢先时
// 不会被命中,需要靠注入绕过才能可靠测试。
func TestAIGtdReparse_DBUniqueFallback409(t *testing.T) {
	a := newTestApp(t)
	r := setupAIGtdRouter(t, a)

	w := doJSON(t, r, http.MethodPost, "/api/tasks", map[string]any{"title": "t", "raw_text": "r"})
	var task TaskDTO
	_ = json.Unmarshal(w.Body.Bytes(), &task)
	taskID := task.ID

	// 注入:跳过 precheck,让 handler 走到 EnqueueJob
	origInFlight, origProcessing := hasInFlightJobFn, taskIsProcessingFn
	hasInFlightJobFn = func(c *storage.AITasksCrud, id int64) (bool, error) { return false, nil }
	taskIsProcessingFn = func(t *domain.TaskRow) bool { return false }
	t.Cleanup(func() {
		hasInFlightJobFn = origInFlight
		taskIsProcessingFn = origProcessing
	})

	// DB 中预先种一个 queued job(同 task_id),使 idx_ai_jobs_in_flight
	// 在第二次 EnqueueJob 时必触发拒绝。
	_, err := a.SQLite.AITasks().EnqueueJob(taskID, "openai_compat", "m", `pre-existing`, domain.AIJobModeAppend)
	if err != nil {
		t.Fatalf("seed queued job: %v", err)
	}

	// 现在调 reparse → precheck 已被注入返 false → 走到 EnqueueJob → UNIQUE 拒绝 → 409
	w = doJSON(t, r, http.MethodPost, "/api/aigtd/reparse/"+strconv.FormatInt(taskID, 10), map[string]any{})
	if w.Code != http.StatusConflict {
		t.Fatalf("DB UNIQUE 冲突应转 409, got %d body=%s", w.Code, w.Body.String())
	}
}

// TestAIGtdReparse_ConcurrentRaceOnlyOne202 用并发请求逼出 TOCTOU 窗口:
// 两个 goroutine 同时 reparse 同一 task。无论命中 HasInFlightJob 预检还是
// DB 层部分 UNIQUE 索引兜底,最终必须恰好一个 202、一个 409。
func TestAIGtdReparse_ConcurrentRaceOnlyOne202(t *testing.T) {
	a := newTestApp(t)
	r := setupAIGtdRouter(t, a)

	w := doJSON(t, r, http.MethodPost, "/api/tasks", map[string]any{"title": "t", "raw_text": "r"})
	var task TaskDTO
	_ = json.Unmarshal(w.Body.Bytes(), &task)
	path := "/api/aigtd/reparse/" + strconv.FormatInt(task.ID, 10)

	const n = 2
	codes := make([]int, n)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			<-start
			rr := doJSON(t, r, http.MethodPost, path, map[string]any{})
			codes[idx] = rr.Code
		}(i)
	}
	close(start)
	wg.Wait()

	accepted, conflict := 0, 0
	for _, c := range codes {
		switch c {
		case http.StatusAccepted:
			accepted++
		case http.StatusConflict:
			conflict++
		default:
			t.Fatalf("unexpected status %d (codes=%v)", c, codes)
		}
	}
	if accepted != 1 || conflict != 1 {
		t.Fatalf("want exactly one 202 + one 409, got %v", codes)
	}
}

func TestTaskUpdate_SameTitleDoesNotFlipFlag(t *testing.T) {
	a := newTestApp(t)
	r := setupAIGtdRouter(t, a)
	w := doJSON(t, r, http.MethodPost, "/api/tasks", map[string]any{"title": "原值"})
	var task TaskDTO
	_ = json.Unmarshal(w.Body.Bytes(), &task)
	if task.UserEditedTitle {
		t.Fatal("新建 task 不应 user_edited_title=true")
	}
	path := "/api/tasks/" + strconv.FormatInt(task.ID, 10)
	// 同值 PATCH → flag 不应翻
	w = doJSON(t, r, http.MethodPatch, path, map[string]any{"title": "原值"})
	if w.Code != http.StatusOK && w.Code != http.StatusNoContent {
		t.Fatalf("patch: code=%d", w.Code)
	}
	w = doJSON(t, r, http.MethodGet, path, nil)
	var after TaskDTO
	_ = json.Unmarshal(w.Body.Bytes(), &after)
	if after.UserEditedTitle {
		t.Error("同值 PATCH title 不应翻 user_edited_title")
	}
}

func TestAIGtdReparse_ModeValidation(t *testing.T) {
	a := newTestApp(t)
	r := setupAIGtdRouter(t, a)

	w := doJSON(t, r, http.MethodPost, "/api/tasks", map[string]any{"title": "t1", "raw_text": "r"})
	var task TaskDTO
	_ = json.Unmarshal(w.Body.Bytes(), &task)
	path := "/api/aigtd/reparse/" + strconv.FormatInt(task.ID, 10)

	// 非法 mode → 400
	w = doJSON(t, r, http.MethodPost, path, map[string]any{"mode": "bogus"})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("bogus mode: code=%d, want 400; body=%s", w.Code, w.Body.String())
	}

	// 合法 replace → 202
	w = doJSON(t, r, http.MethodPost, path, map[string]any{"mode": "replace"})
	if w.Code != http.StatusAccepted {
		t.Fatalf("replace mode: code=%d body=%s", w.Code, w.Body.String())
	}

	// 默认 append(不传 mode)+ 新 task(上面 task 已 in-flight)
	w = doJSON(t, r, http.MethodPost, "/api/tasks", map[string]any{"title": "t2", "raw_text": "r2"})
	var t2 TaskDTO
	_ = json.Unmarshal(w.Body.Bytes(), &t2)
	w = doJSON(t, r, http.MethodPost, "/api/aigtd/reparse/"+strconv.FormatInt(t2.ID, 10), map[string]any{})
	if w.Code != http.StatusAccepted {
		t.Fatalf("default mode: code=%d", w.Code)
	}
}

func TestTaskUpdate_UserEditedTitleFlips(t *testing.T) {
	a := newTestApp(t)
	r := setupAIGtdRouter(t, a)

	w := doJSON(t, r, http.MethodPost, "/api/tasks", map[string]any{"title": "AI 起的标题"})
	var task TaskDTO
	_ = json.Unmarshal(w.Body.Bytes(), &task)
	if task.UserEditedTitle {
		t.Fatal("新建 task 不应 user_edited_title=true")
	}
	path := "/api/tasks/" + strconv.FormatInt(task.ID, 10)

	// PATCH title → 翻转 true
	w = doJSON(t, r, http.MethodPatch, path, map[string]any{"title": "我改的"})
	if w.Code != http.StatusOK && w.Code != http.StatusNoContent {
		t.Fatalf("patch title: code=%d body=%s", w.Code, w.Body.String())
	}
	w = doJSON(t, r, http.MethodGet, path, nil)
	var after TaskDTO
	_ = json.Unmarshal(w.Body.Bytes(), &after)
	if !after.UserEditedTitle {
		t.Error("PATCH title 后 user_edited_title 应为 true")
	}

	// PATCH 其他字段不应翻转(新建另一个 task 测)
	w = doJSON(t, r, http.MethodPost, "/api/tasks", map[string]any{"title": "另一个"})
	var t2 TaskDTO
	_ = json.Unmarshal(w.Body.Bytes(), &t2)
	w = doJSON(t, r, http.MethodPatch, "/api/tasks/"+strconv.FormatInt(t2.ID, 10), map[string]any{"notes": "x"})
	if w.Code != http.StatusOK && w.Code != http.StatusNoContent {
		t.Fatalf("patch notes: code=%d", w.Code)
	}
	w = doJSON(t, r, http.MethodGet, "/api/tasks/"+strconv.FormatInt(t2.ID, 10), nil)
	var a2 TaskDTO
	_ = json.Unmarshal(w.Body.Bytes(), &a2)
	if a2.UserEditedTitle {
		t.Error("仅改 notes 不应翻转 user_edited_title")
	}
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

// TestTaskUpdate_409OnVersionMismatch 验证 C3:TaskUpdate 的乐观锁冲突
// 必须映射为 409(带标准 {error,code} body),而不是 500。
func TestTaskUpdate_409OnVersionMismatch(t *testing.T) {
	a := newTestApp(t)
	r := setupAIGtdRouter(t, a)
	taskID := createTask(t, r, "t")
	path := "/api/tasks/" + strconv.FormatInt(taskID, 10)

	// 第一次更新:doJSON 自动带 If-Match = 当前版本(0)→ 成功,version→1。
	w := doJSON(t, r, http.MethodPatch, path, map[string]any{"title": "新标题"})
	if w.Code != http.StatusOK {
		t.Fatalf("first update: code=%d body=%s", w.Code, w.Body.String())
	}

	// 第二次用旧 version=0(不走 doJSON,避免注入最新 If-Match)→ 409。
	w = doJSONNoIfMatch(t, r, http.MethodPatch, path, map[string]any{
		"version": 0,
		"title":   "冲突标题",
	})
	if w.Code != http.StatusConflict {
		t.Fatalf("want 409, got code=%d body=%s", w.Code, w.Body.String())
	}
	var resp struct {
		Error string `json:"error"`
		Code  string `json:"code"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Code != "version_conflict" {
		t.Errorf("code=%q, want version_conflict", resp.Code)
	}
	// 冲突不应落库。
	row, _, _ := a.SQLite.AITasks().GetTask(taskID)
	if row.Title != "新标题" {
		t.Errorf("冲突更新不应落库, title=%q", row.Title)
	}
}

// TestTaskDelete_409OnVersionMismatch 验证 C1:DeleteTask 也强制乐观锁,
// 旧版本删除 → 409 且行仍在;正确版本删除 → 204。
func TestTaskDelete_409OnVersionMismatch(t *testing.T) {
	a := newTestApp(t)
	r := setupAIGtdRouter(t, a)
	taskID := createTask(t, r, "t")
	path := "/api/tasks/" + strconv.FormatInt(taskID, 10)

	// 先 bump 到 version=1。
	w := doJSON(t, r, http.MethodPatch, path, map[string]any{"title": "新标题"})
	if w.Code != http.StatusOK {
		t.Fatalf("bump: code=%d body=%s", w.Code, w.Body.String())
	}
	// 旧 version=0 删除 → 409。
	w = doJSONNoIfMatch(t, r, http.MethodDelete, path, map[string]any{"version": 0})
	if w.Code != http.StatusConflict {
		t.Fatalf("stale delete want 409, got code=%d body=%s", w.Code, w.Body.String())
	}
	if _, _, err := a.SQLite.AITasks().GetTask(taskID); err != nil {
		t.Fatalf("冲突删除后 task 应仍在: %v", err)
	}
	// 正确版本(doJSON 注入当前 If-Match=1)→ 204。
	w = doJSON(t, r, http.MethodDelete, path, nil)
	if w.Code != http.StatusNoContent {
		t.Fatalf("correct delete want 204, got code=%d body=%s", w.Code, w.Body.String())
	}
}

// TestSubtaskUpdate_409OnVersionMismatch 验证 C2:子任务更新强制乐观锁,
// 旧版本 → 409;正确版本 → 成功。
func TestSubtaskUpdate_409OnVersionMismatch(t *testing.T) {
	a := newTestApp(t)
	r := setupAIGtdRouter(t, a)
	taskID := createTask(t, r, "t")

	w := doJSON(t, r, http.MethodPost,
		"/api/tasks/"+strconv.FormatInt(taskID, 10)+"/subtasks",
		map[string]any{"title": "子任务"})
	if w.Code != http.StatusOK {
		t.Fatalf("create subtask: code=%d body=%s", w.Code, w.Body.String())
	}
	var sub SubtaskDTO
	_ = json.Unmarshal(w.Body.Bytes(), &sub)
	subPath := "/api/tasks/" + strconv.FormatInt(taskID, 10) +
		"/subtasks/" + strconv.FormatInt(sub.ID, 10)

	// 第一次:doJSON 自动注入子任务当前版本(0)→ 成功,version→1。
	w = doJSON(t, r, http.MethodPatch, subPath, map[string]any{"status": "done"})
	if w.Code != http.StatusNoContent && w.Code != http.StatusOK {
		t.Fatalf("first subtask update: code=%d body=%s", w.Code, w.Body.String())
	}
	// 旧 version=0 → 409。
	w = doJSONNoIfMatch(t, r, http.MethodPatch, subPath, map[string]any{
		"version": 0,
		"status":  "active",
	})
	if w.Code != http.StatusConflict {
		t.Fatalf("stale subtask update want 409, got code=%d body=%s", w.Code, w.Body.String())
	}
	// 状态未被冲突写覆盖。
	_, subs, _ := a.SQLite.AITasks().GetTask(taskID)
	if len(subs) != 1 || subs[0].Status != domain.SubtaskStatusDone {
		t.Errorf("冲突更新不应改状态, got %+v", subs)
	}
}

// TestTaskCreate_SingleWrite 验证 I7:POST /api/tasks 用单次 UpdateTask
// 写 title+notes+status,version 只 bump 一次(==1),且 notes/status/ai_status
// 正确落库(不再吞 notes 写的错误)。
// (随 commit 2 落地)
