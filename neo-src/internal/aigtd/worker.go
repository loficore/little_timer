package aigtd

import (
	"context"
	"errors"
	"sync"
	"time"

	"little-timer/internal/domain"
	"little-timer/internal/log"
	"little-timer/internal/storage"
)

// ErrProviderUnconfigured 标记用户尚未配置 LLM API Key。
var ErrProviderUnconfigured = errors.New("aigtd: provider API key not configured")

// APIKeySecretName 是 API Key 在 crypto.SecretStorage 里的键名。
// handler 写入、worker 读取都必须用同一个常量。
const APIKeySecretName = "llm_api_key"

// LLMConfig 是一次 provider 构造所需的全部参数。
type LLMConfig struct {
	Provider       string // "openai_compat" | "anthropic"
	BaseURL        string // 仅 openai_compat 使用
	Model          string
	APIKey         string
	MaxTokens      int
	TimeoutSeconds int
}

// SettingsGetter 让 WorkerPool 不直接依赖 settings 包。
type SettingsGetter interface {
	CurrentDate() string
	LLMSettings() LLMConfig
}

// ProviderFactory 构造一个 Provider。默认实现构造真实 HTTP 客户端,测试可替换。
type ProviderFactory func(cfg LLMConfig) (Provider, error)

// maxJobAttempts 是单个 job 重试上限(可重试错误下)。
const maxJobAttempts = 3

// reclaimThresholdSeconds 是 running 卡死判定的超时阈值。
const reclaimThresholdSeconds = 300

// idlePollInterval 是 worker 空轮询间隔。
const idlePollInterval = 200 * time.Millisecond

// WorkerPool 从 ai_jobs 队列里取任务,调用 Provider 拆解,把结果写回 tasks。
// 线程安全,可启停。
type WorkerPool struct {
	aiTasks  *storage.AITasksCrud
	settings SettingsGetter
	maxConc  int
	factory  ProviderFactory

	stopCh   chan struct{}
	stopOnce sync.Once
	wg       sync.WaitGroup
	started  bool
}

// NewWorkerPool 构造 WorkerPool。Provider 工厂默认构造真实 HTTP 客户端;
// 测试可在 Start 前调用 SetProviderFactory 替换。
func NewWorkerPool(aiTasks *storage.AITasksCrud, s SettingsGetter, maxConc int) *WorkerPool {
	if maxConc <= 0 {
		maxConc = 2
	}
	return &WorkerPool{
		aiTasks:  aiTasks,
		settings: s,
		maxConc:  maxConc,
		factory:  defaultProviderFactory,
		stopCh:   make(chan struct{}),
	}
}

// SetProviderFactory 注入自定义 Provider 构造器(测试 / `_test/prompt` 钩子使用)。
func (w *WorkerPool) SetProviderFactory(f ProviderFactory) { w.factory = f }

// Start 启动 worker goroutines。先 reclaim 卡死的 running job。
func (w *WorkerPool) Start(ctx context.Context) {
	if w.started {
		return
	}
	w.started = true

	if n, err := w.aiTasks.ReclaimStuckRunningJobs(reclaimThresholdSeconds); err != nil {
		log.Warn("worker.reclaim failed", "error", err.Error())
	} else if n > 0 {
		log.Info("worker.reclaimed", "count", n)
	}

	w.wg.Add(w.maxConc)
	for i := 0; i < w.maxConc; i++ {
		go w.loop(ctx)
	}
}

// Stop 通知所有 worker 退出(不等它们完成当前 job)。可重复调用。
func (w *WorkerPool) Stop() { w.stopOnce.Do(func() { close(w.stopCh) }) }

// Wait 阻塞直到所有 worker goroutine 退出。
func (w *WorkerPool) Wait() { w.wg.Wait() }

func (w *WorkerPool) loop(ctx context.Context) {
	defer w.wg.Done()
	ticker := time.NewTicker(idlePollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-w.stopCh:
			return
		default:
		}

		job, err := w.aiTasks.ClaimNextQueuedJob()
		if err != nil {
			log.Warn("worker.claim failed", "error", err.Error())
		}
		if job == nil {
			select {
			case <-ctx.Done():
				return
			case <-w.stopCh:
				return
			case <-ticker.C:
			}
			continue
		}

		w.process(ctx, *job)
	}
}

func (w *WorkerPool) process(ctx context.Context, job domain.AIJobRow) {
	task, _, err := w.aiTasks.GetTask(job.TaskID)
	if err != nil || task == nil {
		_ = w.aiTasks.MarkJobFailed(job.ID, "task not found", "", 0, 0)
		_ = w.aiTasks.SetAIStatus(job.TaskID, domain.AIStatusError, "task not found")
		return
	}

	if err := w.aiTasks.SetAIStatus(task.ID, domain.AIStatusProcessing, ""); err != nil {
		log.Warn("worker.mark processing failed", "task_id", task.ID, "error", err.Error())
	}

	cfg := w.settings.LLMSettings()
	var provider Provider
	if ov := currentTestOverride(); ov != nil {
		provider = &staticProvider{text: ov.text, failWith: ov.failWith}
	} else {
		var err error
		provider, err = w.factory(cfg)
		if err != nil {
			w.fail(job, task.ID, "provider 不可用:"+err.Error())
			return
		}
	}

	// 决定合并模式 + 构造 prompt 上下文。append 模式把已有子任务注入 prompt;
	// replace 模式不传 hints(避免 LLM 因"不要重复"规则主动省略未完成子任务
	// 而被 worker 当作"可删除"——见 reviewer M1)。
	mode := job.Mode
	if mode == "" {
		mode = domain.AIJobModeAppend
	}
	var hints []SubtaskHint
	if mode != domain.AIJobModeReplace {
		_, existingSubs, _ := w.aiTasks.GetTask(task.ID)
		hints = make([]SubtaskHint, 0, len(existingSubs))
		for _, x := range existingSubs {
			hints = append(hints, SubtaskHint{Title: x.Title, Status: string(x.Status)})
		}
	}

	req := CompletionRequest{
		SystemPrompt: SystemPrompt(w.settings.CurrentDate()),
		UserPrompt:   BuildUserPrompt(task.RawText, hints, mode),
		Model:        cfg.Model,
		MaxTokens:    cfg.MaxTokens,
	}
	outcome, err := ParseWithRetryDetailed(ctx, provider, req, 2)
	if err != nil {
		// 即便解析失败,也要把最后一次响应 + token 记到 ai_jobs,方便排查。
		// Provider 错误时 outcome 为 nil(响应无意义),退化为空。
		var raw string
		var inTok, outTok int
		if outcome != nil {
			raw = outcome.RawText
			inTok = outcome.InputTokens
			outTok = outcome.OutputTokens
		}
		// 重要:不要先 MarkJobFailed 再 RequeueJob —— 终态写完后 RequeueJob
		// 的 WHERE status='running' 失效,attempts 不递增,任务永远 processing。
		if errors.Is(err, ErrRetryable) && job.Attempts < maxJobAttempts {
			if rerr := w.aiTasks.RequeueJob(job.ID); rerr != nil {
				log.Warn("worker.requeue failed", "job_id", job.ID, "error", rerr.Error())
			}
			_ = w.aiTasks.SetAIStatus(task.ID, domain.AIStatusPending, "")
			// audit(响应 + token)随下一次 claim 时通过 MarkJobSuccess/Failed 写入终态。
			return
		}
		_ = w.aiTasks.MarkJobFailed(job.ID, err.Error(), raw, inTok, outTok)
		w.fail(job, task.ID, err.Error())
		return
	}
	res := outcome.Result

	// 成功:把解析结果写回 task + 子任务。逐项 best-effort;出错就 fail 整个 job。
	status := domain.TaskStatusActive
	estMin := int64(res.EstimatedMinutes)
	updateFields := storage.TaskUpdateFields{
		Notes:            &res.Notes,
		DueDate:          res.DueDate,
		EstimatedMinutes: &estMin,
		Status:           &status,
	}
	// title 软化:append 模式下,用户改过的 title(user_edited_title=true)不被
	// 覆盖;replace 模式强制覆盖(replace 是用户显式选的全量重建)。
	// reparse 完成后 UserEditedTitle 一律 reset=false,用户需再次编辑才会重保护。
	clearFlag := false
	if mode == domain.AIJobModeReplace || !task.UserEditedTitle {
		updateFields.Title = &res.Title
	}
	updateFields.UserEditedTitle = &clearFlag
	// 乐观锁:取当前 server 端 version。TaskRow.Version 当前未被 scanTask
	// 填入(taskCols 不含 version),所以显式取一次。冲突(版本已被别处
	// 修改)在 work 上下文里少见,但语义上仍要正确传递。
	ver, verr := w.aiTasks.GetTaskVersion(task.ID)
	if verr != nil {
		w.fail(job, task.ID, "read version:"+verr.Error())
		return
	}
	if err := w.aiTasks.UpdateTask(task.ID, updateFields, ver); err != nil {
		w.fail(job, task.ID, "update task:"+err.Error())
		return
	}
	subs := make([]storage.SubtaskInput, 0, len(res.Subtasks))
	for _, st := range res.Subtasks {
		subs = append(subs, storage.SubtaskInput{
			Title:            st.Title,
			EstimatedMinutes: st.EstimatedMinutes,
			DueDate:          st.DueDate,
		})
	}
	if mode == domain.AIJobModeReplace {
		if err := w.aiTasks.ReplaceSubtasksKeepDone(task.ID, subs); err != nil {
			w.fail(job, task.ID, "replace subtasks:"+err.Error())
			return
		}
	} else {
		if err := w.aiTasks.AppendSubtasks(task.ID, subs); err != nil {
			w.fail(job, task.ID, "append subtasks:"+err.Error())
			return
		}
	}
	if err := w.aiTasks.SetAIStatus(task.ID, domain.AIStatusDone, ""); err != nil {
		log.Warn("worker.mark done failed", "task_id", task.ID, "error", err.Error())
	}
	if err := w.aiTasks.MarkParsedAt(task.ID); err != nil {
		log.Warn("worker.mark parsed_at failed", "task_id", task.ID, "error", err.Error())
	}
	if err := w.aiTasks.MarkJobSuccess(job.ID, outcome.RawText, outcome.InputTokens, outcome.OutputTokens); err != nil {
		log.Warn("worker.mark job success failed", "job_id", job.ID, "error", err.Error())
	}
}

func (w *WorkerPool) fail(job domain.AIJobRow, taskID int64, msg string) {
	if err := w.aiTasks.MarkJobFailed(job.ID, msg, "", 0, 0); err != nil {
		log.Warn("worker.mark job failed", "job_id", job.ID, "error", err.Error())
	}
	if err := w.aiTasks.SetAIStatus(taskID, domain.AIStatusError, msg); err != nil {
		log.Warn("worker.mark ai error", "task_id", taskID, "error", err.Error())
	}
}

// defaultProviderFactory 根据 LLMConfig 构造真实 Provider。
// API Key 为空时返 ErrProviderUnconfigured,worker 直接 fail job 并设 ai_error。
func defaultProviderFactory(cfg LLMConfig) (Provider, error) {
	if cfg.APIKey == "" {
		return nil, ErrProviderUnconfigured
	}
	timeout := time.Duration(cfg.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	switch cfg.Provider {
	case "anthropic":
		return NewAnthropicProvider(cfg.APIKey, cfg.Model, timeout), nil
	default:
		// openai_compat 是默认(含 DeepSeek / Ollama / 本地 server 等)
		baseURL := cfg.BaseURL
		if baseURL == "" {
			baseURL = "https://api.deepseek.com/v1"
		}
		return NewOpenAICompatProvider(baseURL, cfg.APIKey, cfg.Model, timeout), nil
	}
}
