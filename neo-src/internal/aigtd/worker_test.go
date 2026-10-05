package aigtd_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"little-timer/internal/aigtd"
	"little-timer/internal/domain"
	"little-timer/internal/storage"
)

// fakeSettings 实现 aigtd.SettingsGetter。
type fakeSettings struct {
	date      string
	provider  string
	baseURL   string
	model     string
	apiKey    string
	maxTokens int
	timeout   int
}

func (s *fakeSettings) CurrentDate() string { return s.date }
func (s *fakeSettings) LLMSettings() aigtd.LLMConfig {
	return aigtd.LLMConfig{
		Provider:       s.provider,
		BaseURL:        s.baseURL,
		Model:          s.model,
		APIKey:         s.apiKey,
		MaxTokens:      s.maxTokens,
		TimeoutSeconds: s.timeout,
	}
}

func newTestDB(t *testing.T) *storage.SqliteManager {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "lt.db")
	m := storage.NewSqliteManager().Init(dbPath)
	if err := m.Open(); err != nil {
		t.Fatal(err)
	}
	if err := m.Migrate(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = m.Close() })
	return m
}

func waitTaskStatus(t *testing.T, c *storage.AITasksCrud, id int64, want domain.AIStatus, timeout time.Duration) *domain.TaskRow {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		task, _, err := c.GetTask(id)
		if err != nil {
			t.Fatal(err)
		}
		if task.AIStatus == want {
			return task
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for ai_status=%s", want)
	return nil
}

func waitJobSuccess(t *testing.T, c *storage.AITasksCrud, taskID, jobID int64, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		j, err := c.LatestJobForTask(taskID)
		if err == nil && j != nil && j.ID == jobID && j.Status == domain.AIJobStatusSuccess {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for job %d success", jobID)
}

func TestWorkerPool_AppendKeepsDoneAndUserTitle(t *testing.T) {
	m := newTestDB(t)
	c := m.AITasks()
	taskID, _ := c.CreateTask("原始", "manual")
	keep, _ := c.CreateSubtask(taskID, "已完成A", 10, nil, 0)
	// 加一个未完成的子任务:append 必须保留它(与 replace 区分的关键)。
	_, _ = c.CreateSubtask(taskID, "未完成B", 15, nil, 1)
	done := domain.SubtaskStatusDone
	if err := c.UpdateSubtask(keep, storage.SubtaskUpdateFields{Status: &done}); err != nil {
		t.Fatal(err)
	}
	// 用户改标题
	edited := "用户标题"
	editedFlag := true
	ver, _ := c.GetTaskVersion(taskID)
	if err := c.UpdateTask(taskID, storage.TaskUpdateFields{Title: &edited, UserEditedTitle: &editedFlag}, ver); err != nil {
		t.Fatal(err)
	}

	aigtd.SetTestOverride(`{"title":"LLM新标题","is_complex":true,"estimated_minutes":30,"subtasks":[{"title":"新1","estimated_minutes":15},{"title":"新2","estimated_minutes":15}]}`, 0)
	defer aigtd.ClearTestOverride()

	jobID, err := c.EnqueueJob(taskID, "openai_compat", "m", `{}`, domain.AIJobModeAppend)
	if err != nil {
		t.Fatal(err)
	}
	s := &fakeSettings{date: "2026-10-05", provider: "openai_compat", model: "m"}
	wp := aigtd.NewWorkerPool(c, s, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	wp.Start(ctx)
	defer func() { wp.Stop(); wp.Wait() }()

	waitJobSuccess(t, c, taskID, jobID, 5*time.Second)

	task, subs, err := c.GetTask(taskID)
	if err != nil {
		t.Fatal(err)
	}
	if task.Title != "用户标题" {
		t.Errorf("append 覆盖了用户 title: got %q", task.Title)
	}
	var titles []string
	for _, x := range subs {
		titles = append(titles, x.Title)
	}
	// append 应保留全部已有(含 done 与未完成)+ 追加新;共 4 条。
	want := []string{"已完成A", "未完成B", "新1", "新2"}
	if !equalTitles(titles, want) {
		t.Errorf("append 结果 = %v, want %v", titles, want)
	}
}

func TestWorkerPool_ReplaceKeepsDoneAndOverwritesTitle(t *testing.T) {
	m := newTestDB(t)
	c := m.AITasks()
	taskID, _ := c.CreateTask("原始", "manual")
	keep, _ := c.CreateSubtask(taskID, "已完成A", 10, nil, 0)
	drop, _ := c.CreateSubtask(taskID, "未完成B", 20, nil, 1)
	done := domain.SubtaskStatusDone
	if err := c.UpdateSubtask(keep, storage.SubtaskUpdateFields{Status: &done}); err != nil {
		t.Fatal(err)
	}
	edited := "用户标题"
	flag := true
	ver, _ := c.GetTaskVersion(taskID)
	if err := c.UpdateTask(taskID, storage.TaskUpdateFields{Title: &edited, UserEditedTitle: &flag}, ver); err != nil {
		t.Fatal(err)
	}
	_ = drop

	aigtd.SetTestOverride(`{"title":"LLM新标题","is_complex":true,"estimated_minutes":30,"subtasks":[{"title":"新1","estimated_minutes":15},{"title":"新2","estimated_minutes":15}]}`, 0)
	defer aigtd.ClearTestOverride()

	jobID, err := c.EnqueueJob(taskID, "openai_compat", "m", `{}`, domain.AIJobModeAppend)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.DB().Exec(`UPDATE ai_jobs SET mode='replace' WHERE id=?`, jobID); err != nil {
		t.Fatal(err)
	}

	s := &fakeSettings{date: "2026-10-05", provider: "openai_compat", model: "m"}
	wp := aigtd.NewWorkerPool(c, s, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	wp.Start(ctx)
	defer func() { wp.Stop(); wp.Wait() }()
	waitJobSuccess(t, c, taskID, jobID, 5*time.Second)

	task, subs, err := c.GetTask(taskID)
	if err != nil {
		t.Fatal(err)
	}
	if task.Title != "LLM新标题" {
		t.Errorf("replace 应覆盖 title: got %q", task.Title)
	}
	var titles []string
	for _, x := range subs {
		titles = append(titles, x.Title)
	}
	if !containsStr(titles, "已完成A") {
		t.Errorf("replace 删了 done 子任务: %v", titles)
	}
	if containsStr(titles, "未完成B") {
		t.Errorf("replace 未删除非 done 子任务: %v", titles)
	}
	if !containsStr(titles, "新1") {
		t.Errorf("replace 未插入新子任务: %v", titles)
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

func TestWorkerPool_ProcessesQueuedJob(t *testing.T) {
	m := newTestDB(t)
	taskID, _ := m.AITasks().CreateTask("准备下周汇报 PPT", "manual")
	_, _ = m.AITasks().EnqueueJob(taskID, "openai_compat", "m", `{"raw":"x"}`, domain.AIJobModeAppend)

	mock := &mockProvider{responses: []string{
		`{"title":"准备 PPT","is_complex":true,"estimated_minutes":60,
		  "subtasks":[{"title":"列大纲","estimated_minutes":25},
		              {"title":"填数据","estimated_minutes":30}]}`,
	}}
	s := &fakeSettings{date: "2026-10-04", provider: "openai_compat", model: "m", apiKey: "k", maxTokens: 2048, timeout: 5}

	wp := aigtd.NewWorkerPool(m.AITasks(), s, 1)
	wp.SetProviderFactory(func(cfg aigtd.LLMConfig) (aigtd.Provider, error) { return mock, nil })
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	wp.Start(ctx)
	defer func() { wp.Stop(); wp.Wait() }()

	task := waitTaskStatus(t, m.AITasks(), taskID, domain.AIStatusDone, 5*time.Second)
	_, subs, _ := m.AITasks().GetTask(taskID)
	if len(subs) != 2 {
		t.Fatalf("want 2 subtasks, got %d", len(subs))
	}
	if task.Title != "准备 PPT" {
		t.Errorf("title = %q", task.Title)
	}
	if task.Status != domain.TaskStatusActive {
		t.Errorf("status = %q, want active", task.Status)
	}
}

func TestWorkerPool_ParseFailureMarksError(t *testing.T) {
	m := newTestDB(t)
	taskID, _ := m.AITasks().CreateTask("x", "manual")
	_, _ = m.AITasks().EnqueueJob(taskID, "openai_compat", "m", `{}`, domain.AIJobModeAppend)

	mock := &mockProvider{responses: []string{`bad`, `also bad`}}
	s := &fakeSettings{date: "2026-10-04", provider: "openai_compat", model: "m", apiKey: "k"}
	wp := aigtd.NewWorkerPool(m.AITasks(), s, 1)
	wp.SetProviderFactory(func(cfg aigtd.LLMConfig) (aigtd.Provider, error) { return mock, nil })
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	wp.Start(ctx)
	defer func() { wp.Stop(); wp.Wait() }()

	task := waitTaskStatus(t, m.AITasks(), taskID, domain.AIStatusError, 5*time.Second)
	if task.AIError == "" {
		t.Error("ai_error should be non-empty")
	}
}

func TestWorkerPool_ReclaimStuckJob(t *testing.T) {
	m := newTestDB(t)
	taskID, _ := m.AITasks().CreateTask("x", "manual")
	_, _ = m.AITasks().EnqueueJob(taskID, "openai_compat", "m", `{}`, domain.AIJobModeAppend)

	// 先 claim 一次(变 running),再倒回 started_at,模拟 worker 崩溃后残留。
	stuck, err := m.AITasks().ClaimNextQueuedJob()
	if err != nil || stuck == nil {
		t.Fatalf("claim: %v %v", stuck, err)
	}
	if _, err := m.DB().Exec(
		`UPDATE ai_jobs SET started_at = datetime('now','-10 minutes') WHERE id=?`, stuck.ID,
	); err != nil {
		t.Fatal(err)
	}

	mock := &mockProvider{responses: []string{
		`{"title":"ok","is_complex":false,"estimated_minutes":15,"subtasks":[]}`,
	}}
	s := &fakeSettings{date: "2026-10-04", provider: "openai_compat", model: "m", apiKey: "k"}
	wp := aigtd.NewWorkerPool(m.AITasks(), s, 1)
	wp.SetProviderFactory(func(cfg aigtd.LLMConfig) (aigtd.Provider, error) { return mock, nil })
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	wp.Start(ctx)
	defer func() { wp.Stop(); wp.Wait() }()

	waitTaskStatus(t, m.AITasks(), taskID, domain.AIStatusDone, 5*time.Second)
}

func TestWorkerPool_UnconfiguredProviderMarksError(t *testing.T) {
	m := newTestDB(t)
	taskID, _ := m.AITasks().CreateTask("x", "manual")
	_, _ = m.AITasks().EnqueueJob(taskID, "openai_compat", "m", `{}`, domain.AIJobModeAppend)

	// apiKey 为空 → 默认工厂应返 ErrProviderUnconfigured。
	s := &fakeSettings{date: "2026-10-04", provider: "openai_compat", model: "m", apiKey: ""}
	wp := aigtd.NewWorkerPool(m.AITasks(), s, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	wp.Start(ctx)
	defer func() { wp.Stop(); wp.Wait() }()

	task := waitTaskStatus(t, m.AITasks(), taskID, domain.AIStatusError, 5*time.Second)
	if task.AIError == "" {
		t.Error("unconfigured provider should set ai_error")
	}
}

func TestWorkerPool_TestOverride_BypassesFactory(t *testing.T) {
	m := newTestDB(t)
	taskID, _ := m.AITasks().CreateTask("x", "manual")
	_, _ = m.AITasks().EnqueueJob(taskID, "openai_compat", "m", `{}`, domain.AIJobModeAppend)

	aigtd.SetTestOverride(
		`{"title":"overridden","is_complex":false,"estimated_minutes":12,"subtasks":[]}`,
		0,
	)
	defer aigtd.ClearTestOverride()

	s := &fakeSettings{date: "2026-10-04", provider: "openai_compat", model: "m", apiKey: ""}
	// 注意:apiKey 为空,真实工厂会返 ErrProviderUnconfigured;但 override 优先,
	// 所以任务仍然应成功。
	wp := aigtd.NewWorkerPool(m.AITasks(), s, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	wp.Start(ctx)
	defer func() { wp.Stop(); wp.Wait() }()

	task := waitTaskStatus(t, m.AITasks(), taskID, domain.AIStatusDone, 5*time.Second)
	if task.Title != "overridden" {
		t.Errorf("title = %q, want overridden", task.Title)
	}
}

func TestWorkerPool_TestOverride_FailWithMarksError(t *testing.T) {
	m := newTestDB(t)
	taskID, _ := m.AITasks().CreateTask("x", "manual")
	_, _ = m.AITasks().EnqueueJob(taskID, "openai_compat", "m", `{}`, domain.AIJobModeAppend)

	aigtd.SetTestOverride("", 500) // 不返回有效响应 → ErrRetryable
	defer aigtd.ClearTestOverride()

	s := &fakeSettings{date: "2026-10-04", provider: "openai_compat", model: "m", apiKey: ""}
	wp := aigtd.NewWorkerPool(m.AITasks(), s, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	wp.Start(ctx)
	defer func() { wp.Stop(); wp.Wait() }()

	// fail_with=500 触发 ErrRetryable + 重试 3 次后失败
	waitTaskStatus(t, m.AITasks(), taskID, domain.AIStatusError, 5*time.Second)
}

// TestWorkerPool_ReparsePreservesUserEditedTitle 验证 reparse 路径不覆盖
// 用户已编辑过的 task.title —— 仅在首次解析(task.Title 为空)时写入 LLM
// 出的 title。这样 reparse 替换子任务不会"偷偷"重置用户重命名。
func TestWorkerPool_ReparsePreservesUserEditedTitle(t *testing.T) {
	m := newTestDB(t)
	aigtd.SetTestOverride(
		`{"title":"LLM 给的标题","is_complex":true,"estimated_minutes":30,"subtasks":[{"title":"a","estimated_minutes":15},{"title":"a2","estimated_minutes":15}]}`,
		0,
	)
	defer aigtd.ClearTestOverride()

	taskID, _ := m.AITasks().CreateTask("原始随手记", "manual")
	_, _ = m.AITasks().EnqueueJob(taskID, "openai_compat", "m", `{}`, domain.AIJobModeAppend)

	s := &fakeSettings{date: "2026-10-04", provider: "openai_compat", model: "m", apiKey: ""}
	wp := aigtd.NewWorkerPool(m.AITasks(), s, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	wp.Start(ctx)
	defer func() { wp.Stop(); wp.Wait() }()

	// 等首次解析完成
	task := waitTaskStatus(t, m.AITasks(), taskID, domain.AIStatusDone, 5*time.Second)
	if task.Title != "LLM 给的标题" {
		t.Fatalf("首次解析应写入 LLM title, got %q", task.Title)
	}

	// 用户编辑 title(handler 实际行为:同时翻 user_edited_title=true)
	edited := "用户改过的标题"
	flag := true
	ver, _ := m.AITasks().GetTaskVersion(taskID)
	if err := m.AITasks().UpdateTask(taskID, storage.TaskUpdateFields{Title: &edited, UserEditedTitle: &flag}, ver); err != nil {
		t.Fatal(err)
	}

	// reparse:再入队一个 job,worker 用不同 LLM 响应(模拟用户改了 raw_text 后再点重新解析)
	aigtd.SetTestOverride(
		`{"title":"REPARSE 改写的标题","is_complex":true,"estimated_minutes":45,"subtasks":[{"title":"b","estimated_minutes":20},{"title":"b2","estimated_minutes":20}]}`,
		0,
	)
	reparseJobID, err := m.AITasks().EnqueueJob(taskID, "openai_compat", "m", `{}`, domain.AIJobModeAppend)
	if err != nil {
		t.Fatal(err)
	}

	// 等"第二条 job 本身"变 success —— 不能只等 task.ai_status==done,因为 reparse
	// 前它已经是 done,waitTaskStatus 会立即返回导致断言失真(tautology)。
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		j, jerr := m.AITasks().LatestJobForTask(taskID)
		if jerr == nil && j != nil && j.ID == reparseJobID && j.Status == domain.AIJobStatusSuccess {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	after, subs, err := m.AITasks().GetTask(taskID)
	if err != nil {
		t.Fatal(err)
	}
	// append 模式:旧子任务 a/a2 应保留 + 新 b/b2 追加(共 4 条)
	var titles []string
	for _, s := range subs {
		titles = append(titles, s.Title)
	}
	want := []string{"a", "a2", "b", "b2"}
	if !equalTitles(titles, want) {
		t.Fatalf("append 应追加新子任务,实际 subtasks = %v, want %v", titles, want)
	}
	// 验证用户 title 未被覆盖 + user_edited_title 已重置
	if after.Title != "用户改过的标题" {
		t.Errorf("reparse 覆盖了用户 title: got %q, 期望 用户改过的标题", after.Title)
	}
	if after.UserEditedTitle {
		t.Error("reparse 后 user_edited_title 应被重置为 false")
	}
}

func equalTitles(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
