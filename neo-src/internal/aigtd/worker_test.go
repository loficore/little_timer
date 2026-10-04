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
