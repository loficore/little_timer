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

func TestWorkerPool_ProcessesQueuedJob(t *testing.T) {
	m := newTestDB(t)
	taskID, _ := m.AITasks().CreateTask("准备下周汇报 PPT", "manual")
	_, _ = m.AITasks().EnqueueJob(taskID, "openai_compat", "m", `{"raw":"x"}`)

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
	_, _ = m.AITasks().EnqueueJob(taskID, "openai_compat", "m", `{}`)

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
	_, _ = m.AITasks().EnqueueJob(taskID, "openai_compat", "m", `{}`)

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
	_, _ = m.AITasks().EnqueueJob(taskID, "openai_compat", "m", `{}`)

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
	_, _ = m.AITasks().EnqueueJob(taskID, "openai_compat", "m", `{}`)

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
	_, _ = m.AITasks().EnqueueJob(taskID, "openai_compat", "m", `{}`)

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
	_, _ = m.AITasks().EnqueueJob(taskID, "openai_compat", "m", `{}`)

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

	// 用户编辑 title
	edited := "用户改过的标题"
	if err := m.AITasks().UpdateTask(taskID, storage.TaskUpdateFields{Title: &edited}); err != nil {
		t.Fatal(err)
	}

	// reparse:再入队一个 job,worker 用不同 LLM 响应(模拟用户改了 raw_text 后再点重新解析)
	aigtd.SetTestOverride(
		`{"title":"REPARSE 改写的标题","is_complex":true,"estimated_minutes":45,"subtasks":[{"title":"b","estimated_minutes":20},{"title":"b2","estimated_minutes":20}]}`,
		0,
	)
	reparseJobID, err := m.AITasks().EnqueueJob(taskID, "openai_compat", "m", `{}`)
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
	// 先证明 reparse 确实跑了(子任务被替换成新一组)
	if len(subs) != 2 || subs[0].Title != "b" || subs[1].Title != "b2" {
		t.Fatalf("reparse 未生效,subtasks = %+v", subs)
	}
	// 再证明用户 title 未被覆盖
	if after.Title != "用户改过的标题" {
		t.Errorf("reparse 覆盖了用户 title: got %q, 期望 用户改过的标题", after.Title)
	}
}
