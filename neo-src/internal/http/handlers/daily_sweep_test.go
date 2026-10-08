// Package handlers —— GTD Daily Sweep / Review 端点测试。
package handlers

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"little-timer/internal/domain"
	"little-timer/internal/http/app"
	"little-timer/internal/storage"
)

func nowLocalDate() string { return time.Now().In(time.Local).Format("2006-01-02") }

// setupDailySweepRouter 在 setupRouter(含 ConflictGuard + doJSON plumbing)之上
// 注册 daily-sweep 与 daily-sweep settings 路由。
func setupDailySweepRouter(t *testing.T, a *app.App) *gin.Engine {
	t.Helper()
	r := setupRouter(t, a)
	r.GET("/api/settings/daily-sweep", DailySweepSettingsGet)
	r.PUT("/api/settings/daily-sweep", DailySweepSettingsUpdate)
	r.POST("/api/daily-sweep/start", DailySweepStart)
	r.GET("/api/daily-sweep/today", DailySweepToday)
	r.POST("/api/daily-sweep/adopt", DailySweepAdopt)
	r.POST("/api/daily-sweep/reject", DailySweepReject)
	return r
}

// doJSONVersion 发一个显式指定 If-Match 的请求(测试 stale version 幂等用)。
func doJSONVersion(t *testing.T, r *gin.Engine, method, path string, body any, version string) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatal(err)
		}
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("If-Match", version)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func ptrInt64(v int64) *int64 { return &v }

func ptrSubStatus(v domain.SubtaskStatus) *domain.SubtaskStatus { return &v }

func mustVersion(t *testing.T, a *app.App, taskID int64) int64 {
	t.Helper()
	v, err := a.SQLite.AITasks().GetTaskVersion(taskID)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

// TestDailySweepSettings_InvalidTime 锁定非法 HH:MM 全部 400 + code。
func TestDailySweepSettings_InvalidTime(t *testing.T) {
	a := newTestApp(t)
	r := setupDailySweepRouter(t, a)

	for _, bad := range []string{"8:00", "24:00", "08:60", "0800", "08:00 ", "aa:bb", "08-00"} {
		w := doJSON(t, r, "PUT", "/api/settings/daily-sweep", map[string]any{"time": bad})
		if w.Code != 400 {
			t.Errorf("time=%q: code=%d body=%s, want 400", bad, w.Code, w.Body.String())
			continue
		}
		if !strings.Contains(w.Body.String(), "daily_sweep.invalid_time") {
			t.Errorf("time=%q: body 缺 daily_sweep.invalid_time: %s", bad, w.Body.String())
		}
	}
	// 合法值:边界 00:00 与 23:59 应接受。
	for _, ok := range []string{"07:15", "00:00", "23:59"} {
		w := doJSON(t, r, "PUT", "/api/settings/daily-sweep", map[string]any{"time": ok})
		if w.Code != 200 {
			t.Errorf("time=%q: code=%d body=%s, want 200", ok, w.Code, w.Body.String())
		}
	}
}

// TestDailySweepSettings_RoundTrip 验证 GET 默认 + PUT 只更新出现的字段。
func TestDailySweepSettings_RoundTrip(t *testing.T) {
	a := newTestApp(t)
	r := setupDailySweepRouter(t, a)

	w := doJSON(t, r, "GET", "/api/settings/daily-sweep", nil)
	if w.Code != 200 {
		t.Fatalf("GET code=%d", w.Code)
	}
	var dto DailySweepSettingsDTO
	if err := json.Unmarshal(w.Body.Bytes(), &dto); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !dto.Enabled || dto.Time != "08:00" {
		t.Fatalf("默认=(%v,%s), want (true,08:00)", dto.Enabled, dto.Time)
	}

	w = doJSON(t, r, "PUT", "/api/settings/daily-sweep", map[string]any{"enabled": false, "time": "10:30"})
	if w.Code != 200 {
		t.Fatalf("PUT code=%d body=%s", w.Code, w.Body.String())
	}
	en, at, _ := a.SQLite.DailySweep().GetSettings()
	if en || at != "10:30" {
		t.Fatalf("更新后=(%v,%s), want (false,10:30)", en, at)
	}

	// 仅改 time → enabled 保持 false(指针语义)
	w = doJSON(t, r, "PUT", "/api/settings/daily-sweep", map[string]any{"time": "06:45"})
	if w.Code != 200 {
		t.Fatalf("PUT time-only code=%d", w.Code)
	}
	en, at, _ = a.SQLite.DailySweep().GetSettings()
	if en || at != "06:45" {
		t.Fatalf("仅改 time 后=(%v,%s), want (false,06:45)", en, at)
	}
}

// TestDailySweepStart_EnqueuesReviewJobs 验证 start 为顶层 task 入队 review job,
// 并写 manual sweep 日志;返回 202 + job_ids。
func TestDailySweepStart_EnqueuesReviewJobs(t *testing.T) {
	a := newTestApp(t)
	r := setupDailySweepRouter(t, a)
	taskID, _ := a.SQLite.AITasks().CreateTask("x", "manual")
	_ = a.SQLite.AITasks().SetTaskStatus(taskID, domain.TaskStatusActive)

	w := doJSON(t, r, "POST", "/api/daily-sweep/start", map[string]any{})
	if w.Code != 202 {
		t.Fatalf("code=%d body=%s, want 202", w.Code, w.Body.String())
	}
	var resp struct {
		JobIDs []int64 `json:"job_ids"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp.JobIDs) != 1 {
		t.Fatalf("job_ids=%v, want 1", resp.JobIDs)
	}
	job, _ := a.SQLite.AITasks().LatestJobForTask(taskID)
	if job == nil || job.Mode != domain.AIJobModeReview {
		t.Fatalf("job=%+v, want mode=review", job)
	}
	today := nowLocalDate()
	if ok, _ := a.SQLite.DailySweep().HasSweepToday(today); !ok {
		t.Errorf("manual sweep 后应有 %s 日志", today)
	}
}

// TestDailySweepToday_ReturnsProposalsWithJobID 验证 today 拉取成功的 review job
// 的 proposals,并带 ai_job_id + version。
func TestDailySweepToday_ReturnsProposalsWithJobID(t *testing.T) {
	a := newTestApp(t)
	r := setupDailySweepRouter(t, a)
	taskID, _ := a.SQLite.AITasks().CreateTask("x", "manual")
	jobID, _ := a.SQLite.AITasks().EnqueueJob(taskID, "openai_compat", "m", `{}`, domain.AIJobModeReview)
	_ = a.SQLite.AITasks().MarkJobSuccess(jobID,
		`{"proposals":[{"change_id":"11111111-1111-4111-8111-111111111111","task_id":`+
			strconv.FormatInt(taskID, 10)+`,"action":"drop","reason":"r"}]}`, 0, 0)

	w := doJSON(t, r, "GET", "/api/daily-sweep/today", nil)
	if w.Code != 200 {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
	var resp struct {
		Proposals []struct {
			AIJobID  int64                 `json:"ai_job_id"`
			Proposal domain.ReviewProposal `json:"proposal"`
			Version  int64                 `json:"version"`
		} `json:"proposals"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp.Proposals) != 1 {
		t.Fatalf("proposals=%d, want 1", len(resp.Proposals))
	}
	if resp.Proposals[0].AIJobID != jobID {
		t.Errorf("ai_job_id=%d, want %d", resp.Proposals[0].AIJobID, jobID)
	}
	if resp.Proposals[0].Proposal.Action != "drop" {
		t.Errorf("action=%s, want drop", resp.Proposals[0].Proposal.Action)
	}
}

// TestAdopt_Idempotent_SameChangeTwice 验证同一 change 第二次 adopt:
//   - 仍 200 + idempotent:true(即使 body.version=0 stale);
//   - tasks.version 不再 +1(总 +1);
//   - adoptions 表只 1 行。
//
// 铁证:临时在 FindAdoption 早返 false 处改为返 true,此测试 fail。
func TestAdopt_Idempotent_SameChangeTwice(t *testing.T) {
	a := newTestApp(t)
	r := setupDailySweepRouter(t, a)
	taskID, _ := a.SQLite.AITasks().CreateTask("x", "manual")
	jobID, _ := a.SQLite.AITasks().EnqueueJob(taskID, "openai_compat", "m", `{}`, domain.AIJobModeReview)

	prop := domain.ReviewProposal{ChangeID: "chg-1", TaskID: taskID, Action: "reschedule",
		SuggestedStart: ptrInt64(100), SuggestedEnd: ptrInt64(200)}
	body := map[string]any{"change_id": "chg-1", "ai_job_id": jobID, "version": 0, "proposal": prop}

	w := doJSONVersion(t, r, "POST", "/api/daily-sweep/adopt", body, "0")
	if w.Code != 200 {
		t.Fatalf("第一次 code=%d body=%s", w.Code, w.Body.String())
	}
	// 第二次:body.version 故意用 0(stale)。幂等路径应直接返,不重做修改。
	w = doJSONVersion(t, r, "POST", "/api/daily-sweep/adopt", body, "0")
	if w.Code != 200 {
		t.Fatalf("第二次 code=%d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"idempotent":true`) {
		t.Errorf("body 缺 idempotent:true: %s", w.Body.String())
	}
	var n int
	var ver int64
	a.SQLite.DB().QueryRow(`SELECT COUNT(*) FROM adoptions WHERE ai_job_id=? AND change_id=?`, jobID, "chg-1").Scan(&n)
	a.SQLite.DB().QueryRow(`SELECT version FROM tasks WHERE id=?`, taskID).Scan(&ver)
	if n != 1 || ver != 1 {
		t.Errorf("adoptions=%d version=%d, want 1 / 1", n, ver)
	}
}

// TestAdopt_ModifySubtasksOnly_BumpsTaskVersion 验证纯子任务改动(action=modify_subtasks)
// 也必须 bump task version(delta §5.4 + Review Focus #2):
// 不 bump 会让同一 task 上的连续 adopt 用同一 version 通过闸门。
func TestAdopt_ModifySubtasksOnly_BumpsTaskVersion(t *testing.T) {
	a := newTestApp(t)
	r := setupDailySweepRouter(t, a)
	taskID, _ := a.SQLite.AITasks().CreateTask("x", "manual")
	subID, _ := a.SQLite.AITasks().CreateSubtask(taskID, "子1", 15, nil, 0)
	jobID, _ := a.SQLite.AITasks().EnqueueJob(taskID, "openai_compat", "m", `{}`, domain.AIJobModeReview)

	prop := domain.ReviewProposal{ChangeID: "chg-m", TaskID: taskID, Action: "modify_subtasks",
		SuggestedSubtaskChanges: []domain.SuggestedSubChange{{SubtaskID: &subID, Op: "mark_done"}}}
	w := doJSONVersion(t, r, "POST", "/api/daily-sweep/adopt",
		map[string]any{"change_id": "chg-m", "ai_job_id": jobID, "version": 0, "proposal": prop}, "0")
	if w.Code != 200 {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
	var ver int64
	a.SQLite.DB().QueryRow(`SELECT version FROM tasks WHERE id=?`, taskID).Scan(&ver)
	if ver != 1 {
		t.Errorf("task version=%d, want 1(纯子任务改动也要 bump)", ver)
	}
	var status string
	a.SQLite.DB().QueryRow(`SELECT status FROM tasks WHERE id=?`, subID).Scan(&status)
	if status != "done" {
		t.Errorf("subtask status=%s, want done", status)
	}
}

// TestAdopt_MarkDone_NonActiveSkipped 验证 mark_done 目标非 active(done/archived)
// → 跳过该 change,handler 不报错;done/archived 数据永不被 AI 改。
func TestAdopt_MarkDone_NonActiveSkipped(t *testing.T) {
	a := newTestApp(t)
	r := setupDailySweepRouter(t, a)
	taskID, _ := a.SQLite.AITasks().CreateTask("x", "manual")
	subID, _ := a.SQLite.AITasks().CreateSubtask(taskID, "已归档", 15, nil, 0)
	ver := mustVersion(t, a, subID)
	archived := domain.SubtaskStatusArchived
	if err := a.SQLite.AITasks().UpdateSubtask(subID,
		storage.SubtaskUpdateFields{Status: &archived}, ver); err != nil {
		t.Fatal(err)
	}
	jobID, _ := a.SQLite.AITasks().EnqueueJob(taskID, "openai_compat", "m", `{}`, domain.AIJobModeReview)

	prop := domain.ReviewProposal{ChangeID: "chg-s", TaskID: taskID, Action: "modify_subtasks",
		SuggestedSubtaskChanges: []domain.SuggestedSubChange{{SubtaskID: &subID, Op: "mark_done"}}}
	w := doJSONVersion(t, r, "POST", "/api/daily-sweep/adopt",
		map[string]any{"change_id": "chg-s", "ai_job_id": jobID, "version": 0, "proposal": prop}, "0")
	if w.Code != 200 {
		t.Fatalf("非 active 应跳过而非报错: code=%d body=%s", w.Code, w.Body.String())
	}
	var status string
	a.SQLite.DB().QueryRow(`SELECT status FROM tasks WHERE id=?`, subID).Scan(&status)
	if status != "archived" {
		t.Errorf("archived 被改动为 %s", status)
	}
}

// TestAdopt_VersionConflict_409 验证 stale version 走 409,而非幂等(change_id 全新时)。
func TestAdopt_VersionConflict_409(t *testing.T) {
	a := newTestApp(t)
	r := setupDailySweepRouter(t, a)
	taskID, _ := a.SQLite.AITasks().CreateTask("x", "manual")
	jobID, _ := a.SQLite.AITasks().EnqueueJob(taskID, "openai_compat", "m", `{}`, domain.AIJobModeReview)
	prop := domain.ReviewProposal{ChangeID: "chg-c", TaskID: taskID, Action: "no_op"}
	w := doJSONVersion(t, r, "POST", "/api/daily-sweep/adopt",
		map[string]any{"change_id": "chg-c", "ai_job_id": jobID, "version": 999, "proposal": prop}, "0")
	if w.Code != 409 {
		t.Fatalf("code=%d body=%s, want 409", w.Code, w.Body.String())
	}
}

// TestDailySweep_FullFlow 端到端:settings 改 → 手动 start → today 拉提议 →
// adopt 成功 → 重复 adopt 幂等 → 换一个 change 走 mark_done → 子任务变 done。
//
// 覆盖:Tasks 5 + 6 + 2 (幂等 + 写 adoptions + bump + mark_done) 的链式契约。
func TestDailySweep_FullFlow(t *testing.T) {
	a := newTestApp(t)
	r := setupDailySweepRouter(t, a)

	// 1) settings 改 time → 读回(指针语义:只改 time → enabled 保持默认)
	w := doJSON(t, r, "PUT", "/api/settings/daily-sweep", map[string]any{"time": "07:15"})
	if w.Code != 200 {
		t.Fatalf("PUT settings code=%d body=%s", w.Code, w.Body.String())
	}
	en, at, _ := a.SQLite.DailySweep().GetSettings()
	if !en || at != "07:15" {
		t.Fatalf("settings=(%v,%s), want (true,07:15)", en, at)
	}

	// 2) 准备 task + 活跃子任务
	taskID, _ := a.SQLite.AITasks().CreateTask("报告", "manual")
	_ = a.SQLite.AITasks().SetTaskStatus(taskID, domain.TaskStatusActive)
	subID, _ := a.SQLite.AITasks().CreateSubtask(taskID, "已写完", 15, nil, 0)

	// 3) 手动 start → 应入队一个 review job
	w = doJSON(t, r, "POST", "/api/daily-sweep/start", map[string]any{})
	if w.Code != 202 {
		t.Fatalf("start code=%d body=%s", w.Code, w.Body.String())
	}
	var startResp struct {
		JobIDs []int64 `json:"job_ids"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &startResp)
	if len(startResp.JobIDs) != 1 {
		t.Fatalf("job_ids=%v, want 1", startResp.JobIDs)
	}
	jobID := startResp.JobIDs[0]

	// 4) 直接把 proposals 写进 response_payload(绕过真实 LLM;worker 路径另测)
	propsJSON := `{"proposals":[
      {"change_id":"full-1","task_id":` + strconv.FormatInt(taskID, 10) +
		`,"action":"reschedule","suggested_scheduled_start":100,"suggested_scheduled_end":200,"reason":"d1"},
      {"change_id":"full-2","task_id":` + strconv.FormatInt(taskID, 10) +
		`,"action":"modify_subtasks","suggested_subtask_changes":[
         {"subtask_id":` + strconv.FormatInt(subID, 10) + `,"op":"mark_done","reason":"done"}],"reason":"d2"}
    ]}`
	if err := a.SQLite.AITasks().MarkJobSuccess(jobID, propsJSON, 0, 0); err != nil {
		t.Fatal(err)
	}

	// 5) today 拉提议 → 应有 2 条
	w = doJSON(t, r, "GET", "/api/daily-sweep/today", nil)
	if w.Code != 200 {
		t.Fatalf("today code=%d", w.Code)
	}
	var todayResp struct {
		Proposals []struct {
			AIJobID  int64                 `json:"ai_job_id"`
			Proposal domain.ReviewProposal `json:"proposal"`
			Version  int64                 `json:"version"`
		} `json:"proposals"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &todayResp)
	if len(todayResp.Proposals) != 2 {
		t.Fatalf("proposals=%d, want 2", len(todayResp.Proposals))
	}

	// 6) adopt full-1(reschedule)→ task version +1,scheduled_start=100
	rescheduleProp := todayResp.Proposals[0].Proposal
	w = doJSON(t, r, "POST", "/api/daily-sweep/adopt",
		map[string]any{"change_id": "full-1", "ai_job_id": jobID, "version": 0, "proposal": rescheduleProp})
	if w.Code != 200 {
		t.Fatalf("adopt full-1 code=%d body=%s", w.Code, w.Body.String())
	}
	var ver int64
	a.SQLite.DB().QueryRow(`SELECT version FROM tasks WHERE id=?`, taskID).Scan(&ver)
	if ver != 1 {
		t.Errorf("adopt 后 version=%d, want 1", ver)
	}
	var sched int64
	a.SQLite.DB().QueryRow(`SELECT scheduled_start FROM tasks WHERE id=?`, taskID).Scan(&sched)
	if sched != 100 {
		t.Errorf("scheduled_start=%d, want 100", sched)
	}

	// 7) 重复 adopt full-1 → 200 idempotent,version 不再 +1
	w = doJSON(t, r, "POST", "/api/daily-sweep/adopt",
		map[string]any{"change_id": "full-1", "ai_job_id": jobID, "version": 0, "proposal": rescheduleProp})
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"idempotent":true`) {
		t.Fatalf("重复 adopt code=%d body=%s, want 200 + idempotent:true", w.Code, w.Body.String())
	}
	a.SQLite.DB().QueryRow(`SELECT version FROM tasks WHERE id=?`, taskID).Scan(&ver)
	if ver != 1 {
		t.Errorf("幂等后 version=%d, want 仍 1", ver)
	}

	// 8) adopt full-2(mark_done)→ version +1 到 2,子任务变 done
	markProp := todayResp.Proposals[1].Proposal
	w = doJSON(t, r, "POST", "/api/daily-sweep/adopt",
		map[string]any{"change_id": "full-2", "ai_job_id": jobID, "version": 1, "proposal": markProp})
	if w.Code != 200 {
		t.Fatalf("adopt mark_done code=%d body=%s", w.Code, w.Body.String())
	}
	a.SQLite.DB().QueryRow(`SELECT version FROM tasks WHERE id=?`, taskID).Scan(&ver)
	if ver != 2 {
		t.Errorf("mark_done 后 version=%d, want 2", ver)
	}
	var subStatus string
	a.SQLite.DB().QueryRow(`SELECT status FROM tasks WHERE id=?`, subID).Scan(&subStatus)
	if subStatus != "done" {
		t.Errorf("子任务 status=%s, want done", subStatus)
	}

	// 9) reject full-... (用新 change) → 204
	w = doJSON(t, r, "POST", "/api/daily-sweep/reject", map[string]any{"change_id": "some-other"})
	if w.Code != 204 {
		t.Errorf("reject code=%d, want 204", w.Code)
	}

	// 10) adoptions 表应只有 2 行(full-1, full-2)
	var adoptionN int
	a.SQLite.DB().QueryRow(`SELECT COUNT(*) FROM adoptions WHERE ai_job_id=?`, jobID).Scan(&adoptionN)
	if adoptionN != 2 {
		t.Errorf("adoptions=%d, want 2", adoptionN)
	}
}
