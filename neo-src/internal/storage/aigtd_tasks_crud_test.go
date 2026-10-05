package storage

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"little-timer/internal/domain"
)

func TestCreateAndGetTask_WithSubtasks(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "lt.db")
	m := NewSqliteManager().Init(dbPath)
	if err := m.Open(); err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	if err := m.Migrate(); err != nil {
		t.Fatal(err)
	}
	c := m.AITasks()

	id, err := c.CreateTask("准备下周汇报 PPT", "manual")
	if err != nil {
		t.Fatal(err)
	}
	if id == 0 {
		t.Fatal("task id should be non-zero")
	}

	subID, err := c.CreateSubtask(id, "收集业务数据", 20, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	if subID == 0 {
		t.Fatal("subtask id should be non-zero")
	}

	task, subs, err := c.GetTask(id)
	if err != nil {
		t.Fatal(err)
	}
	if task.RawText != "准备下周汇报 PPT" {
		t.Errorf("raw_text mismatch: %q", task.RawText)
	}
	if task.Status != domain.TaskStatusInbox {
		t.Errorf("default status should be inbox, got %q", task.Status)
	}
	if task.AIStatus != domain.AIStatusPending {
		t.Errorf("default ai_status should be pending, got %q", task.AIStatus)
	}
	if len(subs) != 1 || subs[0].Title != "收集业务数据" {
		t.Errorf("subtasks mismatch: %+v", subs)
	}
	if subs[0].EstimatedMinutes != 20 {
		t.Errorf("subtask estimated_minutes = %d", subs[0].EstimatedMinutes)
	}
}

func TestUpdateTask_PointerSemantics(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "lt.db")
	m := NewSqliteManager().Init(dbPath)
	if err := m.Open(); err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	if err := m.Migrate(); err != nil {
		t.Fatal(err)
	}
	c := m.AITasks()

	id, _ := c.CreateTask("原文本", "manual")
	if err := c.UpdateTask(id, TaskUpdateFields{
		Title:    stringPtr("新标题"),
		Notes:    stringPtr("备注"),
		DueDate:  stringPtr("2026-12-01"),
		Pinned:   boolPtr(true),
	}); err != nil {
		t.Fatalf("update: %v", err)
	}

	task, _, _ := c.GetTask(id)
	if task.Title != "新标题" {
		t.Errorf("title = %q", task.Title)
	}
	if task.Notes != "备注" {
		t.Errorf("notes = %q", task.Notes)
	}
	if task.DueDate == nil || *task.DueDate != "2026-12-01" {
		t.Errorf("due_date = %v", task.DueDate)
	}
	if !task.Pinned {
		t.Errorf("pinned should be true")
	}

	// 局部更新:只改 Title → 其他字段不变。
	if err := c.UpdateTask(id, TaskUpdateFields{
		Title: stringPtr("再次更新"),
	}); err != nil {
		t.Fatal(err)
	}
	task, _, _ = c.GetTask(id)
	if task.Title != "再次更新" {
		t.Errorf("title after partial = %q", task.Title)
	}
	if task.Notes != "备注" {
		t.Errorf("notes changed unexpectedly: %q", task.Notes)
	}

	// ClearDueDate:true → due_date 变 NULL。
	if err := c.UpdateTask(id, TaskUpdateFields{ClearDueDate: true}); err != nil {
		t.Fatal(err)
	}
	task, _, _ = c.GetTask(id)
	if task.DueDate != nil {
		t.Errorf("due_date not cleared: %v", task.DueDate)
	}
}

func TestClaimNextQueuedJob_FIFO(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "lt.db")
	m := NewSqliteManager().Init(dbPath)
	if err := m.Open(); err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	if err := m.Migrate(); err != nil {
		t.Fatal(err)
	}
	c := m.AITasks()

	taskID, _ := c.CreateTask("t", "manual")
	id1, _ := c.EnqueueJob(taskID, "openai_compat", "m", `{"v":1}`)
	got1, err := c.ClaimNextQueuedJob()
	if err != nil {
		t.Fatal(err)
	}
	if got1 == nil {
		t.Fatal("got1 = nil")
	}
	if got1.ID != id1 {
		t.Errorf("first claim should be id1(%d), got %d", id1, got1.ID)
	}
	// claim 后状态应为 running
	if got1.Status != domain.AIJobStatusRunning {
		t.Errorf("after claim status = %q, want running", got1.Status)
	}
	// 部分 UNIQUE 索引(同 task 至多一个 in-flight):先 MarkJobSuccess 释放约束
	// 才能继续入队 id2/id3。
	if err := c.MarkJobSuccess(id1, "", 0, 0); err != nil {
		t.Fatal(err)
	}
	time.Sleep(1100 * time.Millisecond) // 跨 1 秒,确保 created_at 不同
	id2, err := c.EnqueueJob(taskID, "openai_compat", "m", `{"v":2}`)
	if err != nil {
		t.Fatalf("enqueue id2: %v", err)
	}
	got2, _ := c.ClaimNextQueuedJob()
	if got2 == nil || got2.ID != id2 {
		t.Errorf("second claim should be id2(%d), got %v", id2, got2)
	}
	if err := c.MarkJobSuccess(id2, "", 0, 0); err != nil {
		t.Fatal(err)
	}
	id3, err := c.EnqueueJob(taskID, "openai_compat", "m", `{"v":3}`)
	if err != nil {
		t.Fatalf("enqueue id3: %v", err)
	}
	got3, _ := c.ClaimNextQueuedJob()
	if got3 == nil || got3.ID != id3 {
		t.Errorf("third claim should be id3(%d), got %v", id3, got3)
	}
}

func TestReclaimStuckRunningJobs(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "lt.db")
	m := NewSqliteManager().Init(dbPath)
	if err := m.Open(); err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	if err := m.Migrate(); err != nil {
		t.Fatal(err)
	}
	c := m.AITasks()

	taskID, _ := c.CreateTask("t", "manual")
	_, _ = c.EnqueueJob(taskID, "openai_compat", "m", `{}`)
	j, _ := c.ClaimNextQueuedJob()
	if j == nil {
		t.Fatal("expected a job to claim")
	}

	// 把它的人工 started_at 倒回 10 分钟前。
	if _, err := m.DB().Exec(
		`UPDATE ai_jobs SET started_at = datetime('now','-10 minutes') WHERE id=?`, j.ID,
	); err != nil {
		t.Fatal(err)
	}

	reclaimed, err := c.ReclaimStuckRunningJobs(300)
	if err != nil {
		t.Fatal(err)
	}
	if reclaimed != 1 {
		t.Errorf("reclaimed = %d, want 1", reclaimed)
	}

	// 再 Claim 应该拿到同一个 job(状态已被 reset 为 queued)。
	got, _ := c.ClaimNextQueuedJob()
	if got == nil || got.ID != j.ID {
		t.Errorf("expected to reclaim id=%d, got %v", j.ID, got)
	}

	// B4 fix regression: reclaim 必须累加 attempts(与 RequeueJob 共享上限),
	// 否则 worker panic/OOM 后会无限 reclaim 绕开 maxJobAttempts。
	// ai_jobs.attempts DEFAULT 1(详见 migration.go),reclaim 后 +1 = 2。
	if got.Attempts != 2 {
		t.Errorf("reclaim 后 attempts = %d, 期望 2", got.Attempts)
	}
}

func TestReplaceSubtasks_AtomicSwap(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "lt.db")
	m := NewSqliteManager().Init(dbPath)
	if err := m.Open(); err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	if err := m.Migrate(); err != nil {
		t.Fatal(err)
	}
	c := m.AITasks()

	taskID, _ := c.CreateTask("t", "manual")

	// 第一次 parse → 写一组 subtasks
	if _, err := c.CreateSubtask(taskID, "旧1", 10, nil, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := c.CreateSubtask(taskID, "旧2", 20, nil, 1); err != nil {
		t.Fatal(err)
	}

	// reparse → 替换为新一组
	newSubs := []SubtaskInput{
		{Title: "新1", EstimatedMinutes: 30},
		{Title: "新2", EstimatedMinutes: 40},
		{Title: "新3", EstimatedMinutes: 50},
	}
	if err := c.ReplaceSubtasks(taskID, newSubs); err != nil {
		t.Fatalf("ReplaceSubtasks: %v", err)
	}

	// 验证旧 subtasks 被清空、新 subtasks 到位
	_, subs, err := c.GetTask(taskID)
	if err != nil {
		t.Fatal(err)
	}
	if len(subs) != 3 {
		t.Fatalf("替换后数量 = %d, 期望 3", len(subs))
	}
	wantTitles := []string{"新1", "新2", "新3"}
	for i, st := range subs {
		if st.Title != wantTitles[i] {
			t.Errorf("subtask[%d].Title = %q, want %q", i, st.Title, wantTitles[i])
		}
	}
}

func TestHasInFlightJob(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "lt.db")
	m := NewSqliteManager().Init(dbPath)
	if err := m.Open(); err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	if err := m.Migrate(); err != nil {
		t.Fatal(err)
	}
	c := m.AITasks()

	taskID, _ := c.CreateTask("t", "manual")

	// 初始无 job
	got, _ := c.HasInFlightJob(taskID)
	if got {
		t.Error("空任务不应有 in-flight job")
	}

	// queued → true
	if _, err := c.EnqueueJob(taskID, "openai_compat", "m", `{}`); err != nil {
		t.Fatal(err)
	}
	got, _ = c.HasInFlightJob(taskID)
	if !got {
		t.Error("queued job 应当算 in-flight")
	}

	// 标 success → false
	var j *domain.AIJobRow
	j, _ = c.LatestJobForTask(taskID)
	if j == nil {
		t.Fatal("expected a job")
	}
	if err := c.MarkJobSuccess(j.ID, "", 0, 0); err != nil {
		t.Fatal(err)
	}
	got, _ = c.HasInFlightJob(taskID)
	if got {
		t.Error("success job 不应再算 in-flight")
	}
}

func stringPtr(s string) *string { return &s }
func boolPtr(b bool) *bool       { return &b }

func TestEnqueueJob_RejectsSecondInFlightJob(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "lt.db")
	m := NewSqliteManager().Init(dbPath)
	if err := m.Open(); err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	if err := m.Migrate(); err != nil {
		t.Fatal(err)
	}
	c := m.AITasks()

	taskID, _ := c.CreateTask("t", "manual")
	if _, err := c.EnqueueJob(taskID, "openai_compat", "m", `{}`); err != nil {
		t.Fatalf("first enqueue: %v", err)
	}
	// 第二条同 task 的 queued → 被部分 UNIQUE 索引拒
	_, err := c.EnqueueJob(taskID, "openai_compat", "m", `{}`)
	if !errors.Is(err, ErrJobInFlight) {
		t.Fatalf("second enqueue err = %v, want ErrJobInFlight", err)
	}

	// 第一条变 success 后,同 task 又可以入队
	job, _ := c.ClaimNextQueuedJob()
	if err := c.MarkJobSuccess(job.ID, "", 0, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := c.EnqueueJob(taskID, "openai_compat", "m", `{}`); err != nil {
		t.Fatalf("enqueue after terminal: %v", err)
	}
}
