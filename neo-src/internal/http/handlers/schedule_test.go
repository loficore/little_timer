// Package handlers —— /api/schedule/run + /api/schedule/apply 端点测试。
//
// 关注点:
//  1. preview 模式不落库(只返 Plan);
//  2. apply 模式乐观锁冲突 → 整个事务回滚(任一 409 即全滚);
//
// 这两条是 Task 6 的 binding requirement。setupRouter 复用 doJSON + testRouterApp
// 的乐观锁 plumbing(aigtd_test.go),让 PATCH /api/tasks/:id 也能自动拿到
// 当前 server version。
package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"testing"

	"github.com/gin-gonic/gin"

	"little-timer/internal/http/app"
	"little-timer/internal/http/middleware"
)

// setupRouter 注册 task CRUD + /api/tasks/:id/scores + /api/schedule/* 路由,
// 挂 ConflictGuard 与 doJSON 所需的 testRouterApp。
//
// 复用 createTask / doJSON / testRouterApp(from aigtd_test.go + scores_test.go)。
func setupRouter(t *testing.T, a *app.App) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("app", a)
		c.Next()
	})
	r.Use(middleware.RequireVersionForMutation())

	// task 路由 —— 复用 TaskCreate / TaskUpdate 让 createTaskWithScore 可用。
	r.GET("/api/tasks", TaskList)
	r.POST("/api/tasks", TaskCreate)
	r.GET("/api/tasks/:id", TaskGet)
	r.PATCH("/api/tasks/:id", TaskUpdate)
	r.PATCH("/api/tasks/:id/scores", TaskScoresUpdate)

	// scheduler 路由 —— Task 6 新增。
	r.POST("/api/schedule/run", ScheduleRun)
	r.POST("/api/schedule/apply", ScheduleApply)

	testRouterApp = a
	t.Cleanup(func() { testRouterApp = nil })
	return r
}

// createTaskWithScore 创建一条带 estimated_minutes + 5 维评分的顶层 task。
//
// 实现顺序:POST /api/tasks(createTask) → PATCH estimated_minutes →
// PATCH /api/tasks/:id/scores。乐观锁由 doJSON 经 If-Match 注入(DB 端当前
// version),无需手动追踪。
func createTaskWithScore(t *testing.T, r *gin.Engine, title string, estMin, priority, urgency, energy int, context string) int64 {
	t.Helper()
	taskID := createTask(t, r, title)

	// PATCH estimated_minutes(doJSON 自动注入 If-Match = 当前 server version)。
	if w := doJSON(t, r, http.MethodPatch, "/api/tasks/"+strconv.FormatInt(taskID, 10),
		map[string]any{"estimated_minutes": estMin}); w.Code != http.StatusOK {
		t.Fatalf("set estimated_minutes: code=%d body=%s", w.Code, w.Body.String())
	}

	// PATCH /scores —— 带版本号 body(doJSON 也同时注 If-Match;两者一致)。
	ver, err := testRouterApp.SQLite.AITasks().GetTaskVersion(taskID)
	if err != nil {
		t.Fatalf("get version: %v", err)
	}
	if w := doJSON(t, r, http.MethodPatch,
		"/api/tasks/"+strconv.FormatInt(taskID, 10)+"/scores",
		map[string]any{
			"version":         ver,
			"priority_score":  priority,
			"urgency_score":   urgency,
			"energy_required": energy,
			"context_tag":     context,
		}); w.Code != http.StatusOK {
		t.Fatalf("set scores: code=%d body=%s", w.Code, w.Body.String())
	}
	return taskID
}

// scheduleResp 与 scheduler.Plan 字段对齐(case-insensitive 匹配)。
// 仅截取测试关心的三个字段。不加 inner json tag —— Go 默认对字段名做
// 大小写不敏感匹配,正好对应 scheduler.Placement 的导出名。
type scheduleResp struct {
	Placements []struct {
		TaskID         int64
		ScheduledStart int64
		ScheduledEnd   int64
	} `json:"placements"`
}

// TestScheduleRun_Preview_NoMutation 验证 preview 模式不写库:
// 调度返回的 plan 里包含刚建的 task,但 DB 行 scheduled_start 仍为 NULL。
func TestScheduleRun_Preview_NoMutation(t *testing.T) {
	a := newTestApp(t)
	r := setupRouter(t, a)

	taskID := createTaskWithScore(t, r, "x", 30, 5, 3, 2, "general")

	w := doJSON(t, r, http.MethodPost, "/api/schedule/run", map[string]any{
		"date": "2026-10-05",
		"mode": "preview",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}

	var resp scheduleResp
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v body=%s", err, w.Body.String())
	}
	if len(resp.Placements) != 1 || resp.Placements[0].TaskID != taskID {
		t.Errorf("placements = %+v, want exactly taskID=%d", resp.Placements, taskID)
	}

	// DB 行 scheduled_start 应仍为 NULL(preview 不应写入)。
	var scheduledStart sql.NullInt64
	if err := a.SQLite.DB().QueryRow(
		`SELECT scheduled_start FROM tasks WHERE id = ?;`, taskID,
	).Scan(&scheduledStart); err != nil {
		t.Fatalf("scan: %v", err)
	}
	if scheduledStart.Valid {
		t.Errorf("preview 写入 scheduled_start=%d, want NULL", scheduledStart.Int64)
	}
}

// TestScheduleApply_RollsBackOn409 验证 apply 模式乐观锁冲突触发全事务回滚:
//
//  1. 建 taskA(取其当前 version)、taskB;
//  2. 先 PATCH taskB 升一次 version → B 当前 version 必 > 客户端发送的旧值;
//  3. apply 用 A 的正确 version + B 的 stale version → A 在事务内成功写入
//     scheduled_start,轮到 B 时 0 行受影响 → 整个事务回滚;
//  4. 验证 A 的 scheduled_start 在 DB 里仍为 NULL(回滚生效),B 自然也为 NULL。
func TestScheduleApply_RollsBackOn409(t *testing.T) {
	a := newTestApp(t)
	r := setupRouter(t, a)

	taskA := createTaskWithScore(t, r, "A", 30, 5, 3, 2, "general")
	taskB := createTaskWithScore(t, r, "B", 30, 5, 3, 2, "general")

	// 先改 taskB 把它的 version 升一档(doJSON 自动注入 If-Match = B 当前 version)。
	if w := doJSON(t, r, http.MethodPatch, "/api/tasks/"+strconv.FormatInt(taskB, 10),
		map[string]any{"title": "changed"}); w.Code != http.StatusOK {
		t.Fatalf("bump B: code=%d body=%s", w.Code, w.Body.String())
	}

	// 取 A 的当前 version(正确值);B 用 stale version=0(已升一档,必冲突)。
	aVer, err := a.SQLite.AITasks().GetTaskVersion(taskA)
	if err != nil {
		t.Fatalf("get A version: %v", err)
	}

	// apply:A(version=aVer, 正确) + B(version=0, stale)。
	w := doJSON(t, r, http.MethodPost, "/api/schedule/apply", map[string]any{
		"placements": []map[string]any{
			{
				"task_id":         taskA,
				"scheduled_start": int64(1735689600),
				"scheduled_end":   int64(1735691400),
				"version":         aVer,
			},
			{
				"task_id":         taskB,
				"scheduled_start": int64(1735691400),
				"scheduled_end":   int64(1735693200),
				"version":         0, // stale:B 已升到 aVer+1
			},
		},
	})
	if w.Code != http.StatusConflict {
		t.Fatalf("code=%d body=%s, want 409", w.Code, w.Body.String())
	}

	// 验证 taskA 未被修改(事务回滚)。
	var startA sql.NullInt64
	if err := a.SQLite.DB().QueryRow(
		`SELECT scheduled_start FROM tasks WHERE id = ?;`, taskA,
	).Scan(&startA); err != nil {
		t.Fatalf("scan A: %v", err)
	}
	if startA.Valid {
		t.Errorf("taskA 应回滚,scheduled_start=%d, want NULL", startA.Int64)
	}

	// B 也未被修改(从未被尝试写入)。
	var startB sql.NullInt64
	if err := a.SQLite.DB().QueryRow(
		`SELECT scheduled_start FROM tasks WHERE id = ?;`, taskB,
	).Scan(&startB); err != nil {
		t.Fatalf("scan B: %v", err)
	}
	if startB.Valid {
		t.Errorf("taskB 应未写入,scheduled_start=%d, want NULL", startB.Int64)
	}
}
