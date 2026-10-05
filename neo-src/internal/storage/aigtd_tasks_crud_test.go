package storage

import (
	"errors"
	"fmt"
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
		Title:   stringPtr("新标题"),
		Notes:   stringPtr("备注"),
		DueDate: stringPtr("2026-12-01"),
		Pinned:  boolPtr(true),
	}, 0); err != nil {
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

	// 局部更新:只改 Title → 其他字段不变。(version 已由上一次 update 增到 1)
	if err := c.UpdateTask(id, TaskUpdateFields{
		Title: stringPtr("再次更新"),
	}, 1); err != nil {
		t.Fatal(err)
	}
	task, _, _ = c.GetTask(id)
	if task.Title != "再次更新" {
		t.Errorf("title after partial = %q", task.Title)
	}
	if task.Notes != "备注" {
		t.Errorf("notes changed unexpectedly: %q", task.Notes)
	}

	// ClearDueDate:true → due_date 变 NULL。(此时 version=2)
	if err := c.UpdateTask(id, TaskUpdateFields{ClearDueDate: true}, 2); err != nil {
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
	id1, _ := c.EnqueueJob(taskID, "openai_compat", "m", `{"v":1}`, domain.AIJobModeAppend)
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
	id2, err := c.EnqueueJob(taskID, "openai_compat", "m", `{"v":2}`, domain.AIJobModeAppend)
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
	id3, err := c.EnqueueJob(taskID, "openai_compat", "m", `{"v":3}`, domain.AIJobModeAppend)
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
	_, _ = c.EnqueueJob(taskID, "openai_compat", "m", `{}`, domain.AIJobModeAppend)
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
	if _, err := c.EnqueueJob(taskID, "openai_compat", "m", `{}`, domain.AIJobModeAppend); err != nil {
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

func TestAppendSubtasks_OnlyAddsNoDelete(t *testing.T) {
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
	old1, _ := c.CreateSubtask(taskID, "旧1", 10, nil, 0)
	old2, _ := c.CreateSubtask(taskID, "旧2", 20, nil, 1)
	done := domain.SubtaskStatusDone
	if err := c.UpdateSubtask(old1, SubtaskUpdateFields{Status: &done}); err != nil {
		t.Fatal(err)
	}

	// append 纯追加,不做 title 去重(spec §3.3 / Q2):"旧2" 重名也插入
	err := c.AppendSubtasks(taskID, []SubtaskInput{
		{Title: "新增1", EstimatedMinutes: 30},
		{Title: "旧2", EstimatedMinutes: 99},
	})
	if err != nil {
		t.Fatal(err)
	}

	_, subs, err := c.GetTask(taskID)
	if err != nil {
		t.Fatal(err)
	}
	var titles []string
	for _, s := range subs {
		titles = append(titles, s.Title)
	}
	if len(subs) != 4 {
		t.Fatalf("append 后子任务数 = %d, want 4: %v", len(subs), titles)
	}
	// 旧1(带 done)仍在,状态未被改
	for _, s := range subs {
		if s.ID == old1 && s.Status != domain.SubtaskStatusDone {
			t.Error("append 修改了已 done 子任务的状态")
		}
	}
	_ = old2
}

func TestReplaceSubtasksKeepDone(t *testing.T) {
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
	keep, _ := c.CreateSubtask(taskID, "已完成", 10, nil, 0)
	drop, _ := c.CreateSubtask(taskID, "未完成", 20, nil, 1)
	done := domain.SubtaskStatusDone
	active := domain.SubtaskStatusActive
	if err := c.UpdateSubtask(keep, SubtaskUpdateFields{Status: &done}); err != nil {
		t.Fatal(err)
	}
	if err := c.UpdateSubtask(drop, SubtaskUpdateFields{Status: &active}); err != nil {
		t.Fatal(err)
	}

	err := c.ReplaceSubtasksKeepDone(taskID, []SubtaskInput{{Title: "新的", EstimatedMinutes: 30}})
	if err != nil {
		t.Fatal(err)
	}

	_, subs, err := c.GetTask(taskID)
	if err != nil {
		t.Fatal(err)
	}
	var titles []string
	for _, s := range subs {
		titles = append(titles, s.Title)
	}
	if !containsStr(titles, "已完成") {
		t.Error("replace 删了 done 子任务")
	}
	if containsStr(titles, "未完成") {
		t.Error("replace 未删除非 done 子任务")
	}
	if !containsStr(titles, "新的") {
		t.Error("replace 未插入新子任务")
	}
}

func containsStr(xs []string, want string) bool {
	for _, x := range xs {
		if x == want {
			return true
		}
	}
	return false
}

func stringPtr(s string) *string { return &s }
func boolPtr(b bool) *bool       { return &b }
func intPtr(i int) *int          { return &i }

// TestUpdateTask_VersionConflict_ReturnsErrVersionConflict 验证乐观锁:
// 第一次 UpdateTask(version=0) 成功并把 version 自增到 1;
// 用旧 version=0 再次 UpdateTask → 必须返回 ErrVersionConflict,且错误
// 消息携带当前 server 版本(便于 UI 重新拉取后回写)。
func TestUpdateTask_VersionConflict_ReturnsErrVersionConflict(t *testing.T) {
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
	// 新建 task 的 version=0 -> 第一次 update 用 version=0 应成功。
	title := "new"
	if err := c.UpdateTask(taskID, TaskUpdateFields{Title: &title}, 0); err != nil {
		t.Fatalf("first update with version=0: %v", err)
	}
	// version 现在是 1;再用 version=0 旧版本应被拒。
	err := c.UpdateTask(taskID, TaskUpdateFields{Title: &title}, 0)
	if err == nil {
		t.Fatal("expected ErrVersionConflict, got nil")
	}
	if !errors.Is(err, ErrVersionConflict) {
		t.Fatalf("want errors.Is(err, ErrVersionConflict), got %v", err)
	}
	// 错误应携带 server 端当前 version(=1)用于客户端刷新。
	var ver int64
	_, scanErr := fmt.Sscanf(err.Error(), "task version conflict (current=%d)", &ver)
	if scanErr != nil || ver != 1 {
		t.Errorf("ErrVersionConflict 应携带 current=%d, got %q", 1, err.Error())
	}
}

// TestUpdateTask_GetTaskVersion 验证 GetTaskVersion 返回当前 server 版本,
// 后续 UpdateTask 用这个版本应能成功。
func TestUpdateTask_GetTaskVersion(t *testing.T) {
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
	v, err := c.GetTaskVersion(taskID)
	if err != nil {
		t.Fatalf("GetTaskVersion: %v", err)
	}
	if v != 0 {
		t.Errorf("fresh task version = %d, want 0", v)
	}
	title := "x"
	if err := c.UpdateTask(taskID, TaskUpdateFields{Title: &title}, v); err != nil {
		t.Fatalf("update with fresh version: %v", err)
	}
	v2, _ := c.GetTaskVersion(taskID)
	if v2 != 1 {
		t.Errorf("after update version = %d, want 1", v2)
	}
}

// TestUpdateTaskTx_VersionConflict_ReturnsErrVersionConflict 验证事务变体:
// 同样走 updateWithVersion 集中函数,语义与非-tx 一致。该测试刻意 rollback
// 事务,只验证乐观锁分支不影响外部状态。
func TestUpdateTaskTx_VersionConflict_ReturnsErrVersionConflict(t *testing.T) {
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
	// 直接把 version 推到 1,然后用 version=0 在事务中更新应被拒。
	if _, err := m.DB().Exec(`UPDATE tasks SET version=1 WHERE id=?`, taskID); err != nil {
		t.Fatal(err)
	}

	tx, err := m.DB().Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	title := "tx"
	err = c.UpdateTaskTx(tx, taskID, TaskUpdateFields{Title: &title}, 0)
	if !errors.Is(err, ErrVersionConflict) {
		t.Fatalf("tx variant want ErrVersionConflict, got %v", err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatalf("rollback: %v", err)
	}
	// 回滚了 → DB 上 title 未变,version 仍为 1(我们刚推的)。
	var gotTitle string
	var ver int64
	if err := m.DB().QueryRow(`SELECT title, version FROM tasks WHERE id=?`, taskID).
		Scan(&gotTitle, &ver); err != nil {
		t.Fatal(err)
	}
	if gotTitle == "tx" {
		t.Error("tx 回滚后 title 不应被更新")
	}
	if ver != 1 {
		t.Errorf("tx 回滚后 version = %d, want 1", ver)
	}
}

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
	if _, err := c.EnqueueJob(taskID, "openai_compat", "m", `{}`, domain.AIJobModeAppend); err != nil {
		t.Fatalf("first enqueue: %v", err)
	}
	// 第二条同 task 的 queued → 被部分 UNIQUE 索引拒
	_, err := c.EnqueueJob(taskID, "openai_compat", "m", `{}`, domain.AIJobModeAppend)
	if !errors.Is(err, ErrJobInFlight) {
		t.Fatalf("second enqueue err = %v, want ErrJobInFlight", err)
	}

	// 第一条变 success 后,同 task 又可以入队
	job, _ := c.ClaimNextQueuedJob()
	if err := c.MarkJobSuccess(job.ID, "", 0, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := c.EnqueueJob(taskID, "openai_compat", "m", `{}`, domain.AIJobModeAppend); err != nil {
		t.Fatalf("enqueue after terminal: %v", err)
	}
}
