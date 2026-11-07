package sweep

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"little-timer/internal/domain"
	"little-timer/internal/storage"
)

func newOps(t *testing.T) (*Operations, *storage.SqliteManager) {
	t.Helper()
	sqlite := storage.NewSqliteManager().Init(filepath.Join(t.TempDir(), "s.db"))
	if err := sqlite.Open(); err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := sqlite.Migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	t.Cleanup(func() { _ = sqlite.Close() })
	return NewForSqlite(sqlite, func() (string, string) { return "openai_compat", "m" }), sqlite
}

func createTask(t *testing.T, sqlite *storage.SqliteManager, title string) int64 {
	t.Helper()
	id, err := sqlite.AITasks().CreateTask(title, "manual")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	return id
}

func enqueueReview(t *testing.T, sqlite *storage.SqliteManager, taskID int64) int64 {
	t.Helper()
	id, err := sqlite.AITasks().EnqueueJob(taskID, "openai_compat", "m", `{}`, domain.AIJobModeReview)
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	return id
}

func version(t *testing.T, sqlite *storage.SqliteManager, taskID int64) int64 {
	t.Helper()
	v, err := sqlite.AITasks().GetTaskVersion(taskID)
	if err != nil {
		t.Fatalf("version: %v", err)
	}
	return v
}

// TestAdopt_IdempotentHit：已存在的 (ai_job_id, change_id) → Idempotent=true,
// 不新写审计、不 bump。
func TestAdopt_IdempotentHit(t *testing.T) {
	ops, sqlite := newOps(t)
	taskID := createTask(t, sqlite, "x")
	jobID := enqueueReview(t, sqlite, taskID)
	start := int64(100)
	end := int64(200)
	in := AdoptInput{
		AIJobID: jobID, ChangeID: "chg-1", Version: 0,
		Proposal: domain.ReviewProposal{ChangeID: "chg-1", TaskID: taskID, Action: "reschedule",
			SuggestedStart: &start, SuggestedEnd: &end},
	}
	first, err := ops.Adopt(in)
	if err != nil {
		t.Fatalf("first adopt: %v", err)
	}
	if first.Idempotent {
		t.Error("首次采纳不该是幂等命中")
	}

	// 第二次(version 故意 stale)。
	second, err := ops.Adopt(in)
	if err != nil {
		t.Fatalf("second adopt: %v", err)
	}
	if !second.Idempotent || second.VersionAfter != first.VersionAfter {
		t.Errorf("第二次=%+v, want Idempotent + 同 version", second)
	}
	var n int
	_ = sqlite.DB().QueryRow(`SELECT COUNT(*) FROM adoptions WHERE ai_job_id=? AND change_id=?`, jobID, "chg-1").Scan(&n)
	if n != 1 {
		t.Errorf("adoptions=%d, want 1", n)
	}
	if v := version(t, sqlite, taskID); v != 1 {
		t.Errorf("version=%d, want 1", v)
	}
}

// TestAdopt_TaskIDMismatch：body 的 proposal.task_id 与 ai_jobs.task_id 不符
// → ErrInvalidProposal,且不动任何 task。
func TestAdopt_TaskIDMismatch(t *testing.T) {
	ops, sqlite := newOps(t)
	realTask := createTask(t, sqlite, "real")
	otherTask := createTask(t, sqlite, "other")
	jobID := enqueueReview(t, sqlite, realTask)

	_, err := ops.Adopt(AdoptInput{
		AIJobID: jobID, ChangeID: "c", Version: 0,
		Proposal: domain.ReviewProposal{ChangeID: "c", TaskID: otherTask, Action: "no_op"},
	})
	if !errors.Is(err, ErrInvalidProposal) {
		t.Fatalf("err=%v, want ErrInvalidProposal", err)
	}
	if v := version(t, sqlite, otherTask); v != 0 {
		t.Errorf("other task version=%d, want 0", v)
	}
}

// TestAdopt_RescheduleRequiresTime：两个时间都 nil → ErrInvalidProposal。
func TestAdopt_RescheduleRequiresTime(t *testing.T) {
	ops, sqlite := newOps(t)
	taskID := createTask(t, sqlite, "x")
	jobID := enqueueReview(t, sqlite, taskID)

	_, err := ops.Adopt(AdoptInput{
		AIJobID: jobID, ChangeID: "c", Version: 0,
		Proposal: domain.ReviewProposal{ChangeID: "c", TaskID: taskID, Action: "reschedule"},
	})
	if !errors.Is(err, ErrInvalidProposal) {
		t.Fatalf("err=%v, want ErrInvalidProposal", err)
	}
	if v := version(t, sqlite, taskID); v != 0 {
		t.Errorf("version=%d, want 0(没被静默放过)", v)
	}
}

// TestAdopt_VersionConflictRollsBack：stale version → ErrVersionConflict,
// 且 adoptions 无行、task 未被改。
func TestAdopt_VersionConflictRollsBack(t *testing.T) {
	ops, sqlite := newOps(t)
	taskID := createTask(t, sqlite, "x")
	jobID := enqueueReview(t, sqlite, taskID)
	start := int64(100)

	_, err := ops.Adopt(AdoptInput{
		AIJobID: jobID, ChangeID: "c", Version: 999,
		Proposal: domain.ReviewProposal{ChangeID: "c", TaskID: taskID, Action: "reschedule",
			SuggestedStart: &start},
	})
	if !errors.Is(err, ErrVersionConflict) {
		t.Fatalf("err=%v, want ErrVersionConflict", err)
	}
	var n int
	_ = sqlite.DB().QueryRow(`SELECT COUNT(*) FROM adoptions WHERE ai_job_id=?`, jobID).Scan(&n)
	if n != 0 {
		t.Errorf("adoptions=%d, want 0(整事务回滚)", n)
	}
	var sched *int64
	var v int64
	_ = sqlite.DB().QueryRow(`SELECT scheduled_start, version FROM tasks WHERE id=?`, taskID).Scan(&sched, &v)
	if sched != nil || v != 0 {
		t.Errorf("task 被改动: scheduled_start=%v version=%d", sched, v)
	}
}

// TestAdopt_ModifySubtasksBumpsVersion：纯子任务改动也 bump task version。
func TestAdopt_ModifySubtasksBumpsVersion(t *testing.T) {
	ops, sqlite := newOps(t)
	taskID := createTask(t, sqlite, "x")
	subID, err := sqlite.AITasks().CreateSubtask(taskID, "子1", 15, nil, 0)
	if err != nil {
		t.Fatalf("create subtask: %v", err)
	}
	jobID := enqueueReview(t, sqlite, taskID)

	res, err := ops.Adopt(AdoptInput{
		AIJobID: jobID, ChangeID: "c", Version: 0,
		Proposal: domain.ReviewProposal{ChangeID: "c", TaskID: taskID, Action: "modify_subtasks",
			SuggestedSubtaskChanges: []domain.SuggestedSubChange{{SubtaskID: &subID, Op: "mark_done"}}},
	})
	if err != nil {
		t.Fatalf("adopt: %v", err)
	}
	if res.VersionAfter != 1 {
		t.Errorf("version_after=%d, want 1", res.VersionAfter)
	}
	if v := version(t, sqlite, taskID); v != 1 {
		t.Errorf("task version=%d, want 1", v)
	}
	var status string
	_ = sqlite.DB().QueryRow(`SELECT status FROM tasks WHERE id=?`, subID).Scan(&status)
	if status != "done" {
		t.Errorf("subtask status=%q, want done", status)
	}
}

// TestAdopt_NonActiveSubtaskSkipped：目标非 active → 跳过而非报错。
func TestAdopt_NonActiveSubtaskSkipped(t *testing.T) {
	ops, sqlite := newOps(t)
	taskID := createTask(t, sqlite, "x")
	subID, err := sqlite.AITasks().CreateSubtask(taskID, "已归档", 15, nil, 0)
	if err != nil {
		t.Fatalf("create subtask: %v", err)
	}
	archived := domain.SubtaskStatusArchived
	if err := sqlite.AITasks().UpdateSubtask(subID, storage.SubtaskUpdateFields{Status: &archived},
		version(t, sqlite, subID)); err != nil {
		t.Fatalf("archive: %v", err)
	}
	jobID := enqueueReview(t, sqlite, taskID)

	if _, err := ops.Adopt(AdoptInput{
		AIJobID: jobID, ChangeID: "c", Version: 0,
		Proposal: domain.ReviewProposal{ChangeID: "c", TaskID: taskID, Action: "modify_subtasks",
			SuggestedSubtaskChanges: []domain.SuggestedSubChange{{SubtaskID: &subID, Op: "mark_done"}}},
	}); err != nil {
		t.Fatalf("非 active 应跳过而非报错: %v", err)
	}
	var status string
	_ = sqlite.DB().QueryRow(`SELECT status FROM tasks WHERE id=?`, subID).Scan(&status)
	if status != "archived" {
		t.Errorf("archived 被改动为 %q", status)
	}
}

// TestAdopt_CrossTaskSubtaskSkipped：指向别的 task 的子任务 → 不改,当前
// task 仍正常 bump。
func TestAdopt_CrossTaskSubtaskSkipped(t *testing.T) {
	ops, sqlite := newOps(t)
	taskID := createTask(t, sqlite, "mine")
	otherTask := createTask(t, sqlite, "other")
	otherSub, err := sqlite.AITasks().CreateSubtask(otherTask, "别人的", 15, nil, 0)
	if err != nil {
		t.Fatalf("create subtask: %v", err)
	}
	jobID := enqueueReview(t, sqlite, taskID)

	if _, err := ops.Adopt(AdoptInput{
		AIJobID: jobID, ChangeID: "c", Version: 0,
		Proposal: domain.ReviewProposal{ChangeID: "c", TaskID: taskID, Action: "modify_subtasks",
			SuggestedSubtaskChanges: []domain.SuggestedSubChange{{SubtaskID: &otherSub, Op: "mark_done"}}},
	}); err != nil {
		t.Fatalf("adopt: %v", err)
	}
	var status string
	_ = sqlite.DB().QueryRow(`SELECT status FROM tasks WHERE id=?`, otherSub).Scan(&status)
	if status != "active" {
		t.Errorf("跨 task 子任务被改成 %q", status)
	}
	if v := version(t, sqlite, taskID); v != 1 {
		t.Errorf("当前 task version=%d, want 1", v)
	}
}

// TestReject_EmptyChangeID：空 change_id → ErrInvalidProposal;非空是 no-op。
func TestReject_EmptyChangeID(t *testing.T) {
	ops, _ := newOps(t)
	if err := ops.Reject("  "); !errors.Is(err, ErrInvalidProposal) {
		t.Errorf("err=%v, want ErrInvalidProposal", err)
	}
	if err := ops.Reject("chg"); err != nil {
		t.Errorf("err=%v, want nil", err)
	}
}

// TestStart_EnqueuesPerActiveTask：每个顶层 task 入队一个 review job。
func TestStart_EnqueuesPerActiveTask(t *testing.T) {
	ops, sqlite := newOps(t)
	a := createTask(t, sqlite, "a")
	b := createTask(t, sqlite, "b")

	ids, err := ops.Start(context.Background())
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	if len(ids) != 2 {
		t.Fatalf("job_ids=%v, want 2", ids)
	}
	for _, taskID := range []int64{a, b} {
		job, _ := sqlite.AITasks().LatestJobForTask(taskID)
		if job == nil || job.Mode != domain.AIJobModeReview {
			t.Errorf("task %d job=%+v, want review", taskID, job)
		}
	}
}

// TestStart_SkipsInFlight：已有 in-flight job 的 task 被跳过且不报错。
func TestStart_SkipsInFlight(t *testing.T) {
	ops, sqlite := newOps(t)
	taskID := createTask(t, sqlite, "a")
	enqueueReview(t, sqlite, taskID)

	ids, err := ops.Start(context.Background())
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	if len(ids) != 0 {
		t.Errorf("job_ids=%v, want 空(in-flight 跳过)", ids)
	}
}

// TestToday_ReturnsProposalsWithJobID：拉成功 review job 的 proposals,
// 带 ai_job_id / version / is_adopted。
func TestToday_ReturnsProposalsWithJobID(t *testing.T) {
	ops, sqlite := newOps(t)
	taskID := createTask(t, sqlite, "x")
	jobID := enqueueReview(t, sqlite, taskID)
	if err := sqlite.AITasks().MarkJobSuccess(jobID,
		`{"proposals":[{"change_id":"chg-1","task_id":`+itoa(taskID)+`,"action":"drop","reason":"r"}]}`,
		0, 0); err != nil {
		t.Fatalf("mark success: %v", err)
	}

	res, err := ops.Today("2026-11-08")
	if err != nil {
		t.Fatalf("today: %v", err)
	}
	if len(res.Proposals) != 1 {
		t.Fatalf("proposals=%d, want 1", len(res.Proposals))
	}
	p := res.Proposals[0]
	if p.AIJobID != jobID || p.Proposal.Action != "drop" || p.IsAdopted {
		t.Errorf("proposal=%+v, want jobID=%d action=drop 未采纳", p, jobID)
	}
}

func itoa(v int64) string {
	if v == 0 {
		return "0"
	}
	digits := []byte{}
	for v > 0 {
		digits = append([]byte{byte('0' + v%10)}, digits...)
		v /= 10
	}
	return string(digits)
}
