// Package handlers — 跨项目残留 session 复现测试。
//
// 路由确认(参见 router.go:86-99):
//   POST /api/start          body: {habit_id?, mode?, work_duration?, rest_duration?, loop_count?}
//   POST /api/finish         无 body(后端取 App.CurrentTimerSessionID / CurrentHabitID)
//   POST /api/reset          无 body
//   GET  /api/timer/progress 返回 {session_id, habit_id, mode, is_running, ...}
//
// frontend 客户端(assets/src/utils/apiClient.ts:150-211):
//   startTimer()      → POST /api/start
//   finishTimer()     → POST /api/timer/finish
//   resetTimer()      → POST /api/reset
//   getTimerProgress()→ GET  /api/timer/progress
//
// 假设 H1(spec §6):Playwright 跨项目共享同一 Go 进程,mobile-390 跑完后
// App.CurrentTimerSessionID 残留,resetStaleTimer 仅 UI 清理不保证清后端,
// 下一个 start() 撞上残留态触发「已运行 → 直接返回旧 session」路径,
// SSE/state 回推让前端 isRunning 翻 false。
package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"little-timer/internal/domain"
)

// TestTimerStart_ConvergesStaleSession:在没有任何显式 finish/reset 的
// 情况下,后端必须保证「一次 start 后再次 start」收敛到只剩一个新 session。
//
// 当前实现的失败模式:
//   - 第二次 start 走 already_running 分支,返回 session_id = A(而不是新 session B),
//     且不会清理 A;此时 /api/timer/progress 仍指向 A,App.CurrentTimerSessionID 仍是 A。
//
// 期望修复后的行为(在 Task 2 实现):
//   - 第二次 start 前应先把 A 收敛(若 elapsed==0 则直接删除,
//     否则标记为 finished 但不写入今日 sessions 表),
//     然后再创建 B;CurrentTimerSessionID 切到 B,GET /api/timer/progress 返回 B。
func TestTimerStart_ConvergesStaleSession(t *testing.T) {
	a := newTestApp(t)
	r := setupTestRouter(t, a)

	// 1. 首次 start 建 session A。
	w1 := httptest.NewRecorder()
	req1 := httptest.NewRequest(http.MethodPost, "/api/start",
		bytes.NewBufferString(`{"mode":"stopwatch"}`))
	req1.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w1, req1)
	if w1.Code != http.StatusOK {
		t.Fatalf("first POST /api/start: code=%d body=%s", w1.Code, w1.Body.String())
	}
	var resp1 map[string]any
	if err := json.Unmarshal(w1.Body.Bytes(), &resp1); err != nil {
		t.Fatalf("first response not JSON: %v", err)
	}
	if resp1["status"] != "started" {
		t.Fatalf("first start status = %v, want started", resp1["status"])
	}
	sessionA := int64(resp1["session_id"].(float64))
	if sessionA == 0 {
		t.Fatal("first start returned no session_id")
	}

	// 2. 不 finish/reset,直接再 start。
	//    当前实现的失败点:handler 返回 status="already_running" 且 session_id = sessionA。
	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodPost, "/api/start",
		bytes.NewBufferString(`{"mode":"stopwatch"}`))
	req2.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatalf("second POST /api/start: code=%d body=%s", w2.Code, w2.Body.String())
	}
	var resp2 map[string]any
	if err := json.Unmarshal(w2.Body.Bytes(), &resp2); err != nil {
		t.Fatalf("second response not JSON: %v", err)
	}

	// 期望:第二次 start 创建全新 sessionB(不等于 A)。
	// 当前实现在 already_running 分支返回 sessionA,断言此处失败。
	if resp2["status"] == "already_running" {
		t.Fatalf("second start returned status=%q with session_id=%v; expected a NEW session (handler must retire the prior session before creating a new one)",
			resp2["status"], resp2["session_id"])
	}
	if resp2["status"] != "started" {
		t.Fatalf("second start status = %v, want started", resp2["status"])
	}
	sessionBFlt, ok := resp2["session_id"].(float64)
	if !ok {
		t.Fatalf("second start response missing session_id: %v", resp2)
	}
	sessionB := int64(sessionBFlt)
	if sessionB == 0 {
		t.Fatal("second start returned zero session_id")
	}
	if sessionB == sessionA {
		t.Fatalf("second start reused stale session_id=%d (same as A); convergence required", sessionB)
	}

	// 3. GET /api/timer/progress:活动 session 唯一且是 B。
	wProg := httptest.NewRecorder()
	reqProg := httptest.NewRequest(http.MethodGet, "/api/timer/progress", nil)
	r.ServeHTTP(wProg, reqProg)
	if wProg.Code != http.StatusOK {
		t.Fatalf("GET /api/timer/progress: code=%d", wProg.Code)
	}
	var prog map[string]any
	if err := json.Unmarshal(wProg.Body.Bytes(), &prog); err != nil {
		t.Fatalf("progress not JSON: %v", err)
	}
	if prog["session_id"] == nil {
		t.Fatalf("progress.session_id is nil; want B=%d", sessionB)
	}
	gotSessionID := int64(prog["session_id"].(float64))
	if gotSessionID != sessionB {
		t.Errorf("progress.session_id = %d, want %d (B after convergence)", gotSessionID, sessionB)
	}

	// 4. POST /api/timer/finish 应结束当前 session = B,elapsed 应与 B 一致。
	//    若 finish 错误地把 A 标记为 finished(且 A 残留 session_id),
	//    这个断言会失败。
	wFin := httptest.NewRecorder()
	reqFin := httptest.NewRequest(http.MethodPost, "/api/timer/finish", nil)
	r.ServeHTTP(wFin, reqFin)
	if wFin.Code != http.StatusOK {
		t.Fatalf("POST /api/timer/finish: code=%d body=%s", wFin.Code, wFin.Body.String())
	}
	var finResp map[string]any
	if err := json.Unmarshal(wFin.Body.Bytes(), &finResp); err != nil {
		t.Fatalf("finish response not JSON: %v", err)
	}
	if finResp["status"] != "finished" {
		t.Errorf("finish status = %v, want finished", finResp["status"])
	}
	if sid, ok := finResp["session_id"].(float64); !ok || int64(sid) != sessionB {
		t.Errorf("finish session_id = %v, want %v (B)", finResp["session_id"], sessionB)
	}

	// 5. 完成后 timer_sessions 不应再有活动行。
	if _, err := a.SQLite.Timers().GetActiveTimerSession(); err == nil {
		t.Error("expected no active timer_session after finish, but GetActiveTimerSession returned a row")
	}
}

// TestTimerStart_DoesNotDoubleCountStaleSession:收敛路径不能把残留 session
// 写入今日 sessions 表(污染用户统计)。
//
// 场景:首次 start(stopwatch)未结束 → 再次 start → 收敛 A。
// 期望:A 不会以「完成」形式出现在 sessions 表;若 elapsed==0 应直接删除,
// 否则只标记 timer_sessions.is_finished,但不写 sessions 行。
//
// 当前实现的失败模式:
//   - already_running 分支直接返回 A(未写 sessions),看起来不污染;
//     但若 Task 2 修复方式不慎调 TimerFinish handler 路径(连带 CreateSession),
//     会写一条「用户根本没真正完成」的会话到今日统计。
//   - 此测试作为修复护栏:无论 Task 2 怎么收敛,今日 sessions 不得新增行。
func TestTimerStart_DoesNotDoubleCountStaleSession(t *testing.T) {
	a := newTestApp(t)
	r := setupTestRouter(t, a)

	// 1. 首次 start。
	w1 := httptest.NewRecorder()
	req1 := httptest.NewRequest(http.MethodPost, "/api/start",
		bytes.NewBufferString(`{"mode":"stopwatch"}`))
	req1.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w1, req1)
	if w1.Code != http.StatusOK {
		t.Fatalf("first POST /api/start: code=%d body=%s", w1.Code, w1.Body.String())
	}
	var resp1 map[string]any
	if err := json.Unmarshal(w1.Body.Bytes(), &resp1); err != nil {
		t.Fatalf("first response not JSON: %v", err)
	}
	sessionA := int64(resp1["session_id"].(float64))

	// 2. 第二次 start(触发收敛)。
	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodPost, "/api/start",
		bytes.NewBufferString(`{"mode":"stopwatch"}`))
	req2.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatalf("second POST /api/start: code=%d body=%s", w2.Code, w2.Body.String())
	}

	// 3. 今日 sessions 表应为 0 行 —— A 没真正完成,不应被计入。
	todayStr := domain.TodayString(0)
	rows, err := a.SQLite.Timers().ListSessionsByDate(todayStr, 100, 0)
	if err != nil {
		t.Fatalf("ListSessionsByDate(%s): %v", todayStr, err)
	}
	for _, row := range rows {
		if row.DurationSeconds > 0 {
			t.Errorf("session row counted as finished on %s: habit_id=%d duration=%ds (must not leak from converged session A=%d)",
				row.Date, row.HabitID, row.DurationSeconds, sessionA)
		}
	}
}