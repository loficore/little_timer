// capture / reparse 的用例层(issue #66)。
//
// HTTP handler 与(将来的)Wails service 都只做参数绑定与 Error →
// transport 状态码映射;原文校验、入队、失败回滚、processing 前置检查都
// 集中在这里。LLM client / worker / parser 仍留在本包其它文件。
package aigtd

import (
	"errors"
	"strings"

	"little-timer/internal/domain"
	"little-timer/internal/storage"
)

// MaxRawTextLen 是 capture / reparse 原文的 rune 上限。
const MaxRawTextLen = 2000

// Error 是 aigtd 用例层的类型化错误。Adapter 用 errors.As 映射成 HTTP
// 响应;Code 是稳定的、前端可依赖的字符串。
type Error struct {
	Code    string
	Message string
}

func (e *Error) Error() string { return e.Message }

// badRequest 构造带具体文案的入参错误。Code 与 ErrBadRequest 一致,所以
// errors.Is(err, ErrBadRequest) 仍然成立。
func badRequest(message string) *Error {
	return &Error{Code: ErrBadRequest.Code, Message: message}
}

// Is 让同 Code 的错误互相匹配(带 detail 的副本仍能被 errors.Is 命中哨兵)。
func (e *Error) Is(target error) bool {
	t, ok := target.(*Error)
	return ok && e.Code == t.Code
}

// 哨兵错误。
var (
	// ErrBadRequest 覆盖"请求体本身不合法":非法 mode、显式给出的原文为空
	// 或超长。具体文案由 badRequest 构造,adapter 原样回给客户端。
	ErrBadRequest = &Error{Code: "aigtd.bad_request", Message: "invalid request"}
	// ErrCreateFailed 是建 task 失败。
	ErrCreateFailed = &Error{Code: "aigtd.create_failed", Message: "create failed"}
	// ErrEnqueueFailed 是入队失败(已回滚刚建的 task)。
	ErrEnqueueFailed = &Error{Code: "aigtd.enqueue_failed", Message: "enqueue failed"}
	// ErrEmpty 是空原文。
	ErrEmpty = &Error{Code: "aigtd.empty", Message: "raw_text is empty"}
	// ErrTooLong 是原文超长。
	ErrTooLong = &Error{Code: "aigtd.too_long", Message: "raw_text too long"}
	// ErrProcessing 表示该 task 已有 queued/running job 或正在 processing。
	ErrProcessing = &Error{Code: "aigtd.processing", Message: "task is being processed"}
	// ErrNotFound 表示目标 task 不存在。
	ErrNotFound = &Error{Code: "aigtd.not_found", Message: "task not found"}
)

// TaskStore 是本包依赖的持久化能力集合。storage.AITasksCrud 直接满足它。
type TaskStore interface {
	CreateTask(rawText, source string) (int64, error)
	DeleteTask(id int64, version int64) error
	GetTask(id int64) (*domain.TaskRow, []domain.SubtaskRow, error)
	HasInFlightJob(taskID int64) (bool, error)
	EnqueueJob(taskID int64, provider, model, reqPayload string, mode domain.AIJobMode) (int64, error)
	EnqueueReparse(taskID int64, rawUpdate *storage.TaskUpdateFields, version int64, provider, model string, mode domain.AIJobMode) (int64, error)
}

// Operations 是 capture / reparse 的用例层。
//
// InFlightProbe / ProcessingProbe 是 reparse precheck 的可注入点:生产默认
// 实现直连 store;测试替换后可绕过 precheck,确定性触达 DB 层部分 UNIQUE
// 索引的 ErrJobInFlight→409 分支(reviewer M4)。它们是**实例字段**而非包级
// 变量,并发请求各持一份,不会互相覆盖。
type Operations struct {
	store  TaskStore
	config func() LLMConfig

	// InFlightProbe 报告该 task 是否已有 queued/running job。
	InFlightProbe func(store TaskStore, taskID int64) (bool, error)
	// ProcessingProbe 报告 task 是否仍处于 processing。
	ProcessingProbe func(task *domain.TaskRow) bool
}

// NewOperations 构造 Operations。config 在每次入队时求值,使设置页改动
// 无需重建 Operations。
func NewOperations(store TaskStore, config func() LLMConfig) *Operations {
	return &Operations{
		store:  store,
		config: config,
		InFlightProbe: func(s TaskStore, taskID int64) (bool, error) {
			return s.HasInFlightJob(taskID)
		},
		ProcessingProbe: func(t *domain.TaskRow) bool {
			return t.AIStatus == domain.AIStatusProcessing
		},
	}
}

// Capture 建一条 task 并立即入队(worker 在后台处理)。
//
// 入队失败时回滚刚建的 task,避免出现"卡片永远停在 Parsing 状态"的孤儿
// 记录。
func (o *Operations) Capture(rawText, source string) (int64, error) {
	raw := strings.TrimSpace(rawText)
	if raw == "" {
		return 0, ErrEmpty
	}
	if len([]rune(raw)) > MaxRawTextLen {
		return 0, ErrTooLong
	}
	if source == "" {
		source = "manual"
	}
	taskID, err := o.store.CreateTask(raw, source)
	if err != nil {
		return 0, ErrCreateFailed
	}
	cfg := o.config()
	if _, err := o.store.EnqueueJob(taskID, cfg.Provider, cfg.Model, raw, domain.AIJobModeAppend); err != nil {
		// 刚 CreateTask 的行 version=0,rollback 时显式带 0(内部调用不走
		// ConflictGuard 的 context,这里必须自带版本)。
		_ = o.store.DeleteTask(taskID, 0)
		return 0, ErrEnqueueFailed
	}
	return taskID, nil
}

// Reparse 为一个已存在的 task 入队重新解析。mode 缺省 append。
//
// 已有 queued/running job 或 task 仍在 processing → ErrProcessing(409)。
// 可选改写原文(rawText 非 nil 时走乐观锁)。DB 层的 ErrJobInFlight /
// ErrVersionConflict 同样归为 ErrProcessing,与历史 HTTP 契约一致。
func (o *Operations) Reparse(taskID int64, rawText *string, mode string, version int64) (int64, error) {
	task, _, err := o.store.GetTask(taskID)
	if err != nil || task == nil {
		return 0, ErrNotFound
	}
	inFlight, err := o.InFlightProbe(o.store, taskID)
	if err != nil {
		return 0, err
	}
	if inFlight || o.ProcessingProbe(task) {
		return 0, ErrProcessing
	}

	jobMode := domain.AIJobMode(mode)
	if mode == "" {
		jobMode = domain.AIJobModeAppend
	}
	switch jobMode {
	case domain.AIJobModeAppend, domain.AIJobModeReplace:
		// ok
	default:
		return 0, badRequest("invalid mode (allowed: append|replace)")
	}

	var rawUpdate *storage.TaskUpdateFields
	if rawText != nil {
		raw := strings.TrimSpace(*rawText)
		if raw == "" || len([]rune(raw)) > MaxRawTextLen {
			return 0, badRequest("invalid raw_text")
		}
		rawUpdate = &storage.TaskUpdateFields{RawText: &raw}
	}
	cfg := o.config()
	jobID, err := o.store.EnqueueReparse(taskID, rawUpdate, version, cfg.Provider, cfg.Model, jobMode)
	if err != nil {
		if errors.Is(err, storage.ErrJobInFlight) || errors.Is(err, storage.ErrVersionConflict) {
			return 0, ErrProcessing
		}
		return 0, err
	}
	return jobID, nil
}
