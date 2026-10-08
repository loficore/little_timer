// Package handlers —— GTD Daily Sweep / Review 端点测试。
package handlers

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"little-timer/internal/domain"
	"little-timer/internal/http/app"
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
	return r
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
