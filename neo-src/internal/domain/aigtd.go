package domain

import "time"

// TaskStatus 是任务的生命周期状态。
type TaskStatus string

const (
	TaskStatusInbox    TaskStatus = "inbox"
	TaskStatusActive   TaskStatus = "active"
	TaskStatusDone     TaskStatus = "done"
	TaskStatusArchived TaskStatus = "archived"
	TaskStatusRejected TaskStatus = "rejected"
)

// AIStatus 是 AI 拆解流水线的状态。
type AIStatus string

const (
	AIStatusPending    AIStatus = "pending"
	AIStatusProcessing AIStatus = "processing"
	AIStatusDone       AIStatus = "done"
	AIStatusError      AIStatus = "error"
)

// SubtaskStatus 是子任务执行状态。
// 字面值与 tasks.status CHECK 约束对齐(同列复用),子任务仅用 active/done/archived 三态
// —— 待办/完成/跳过。CHECK 约束见 internal/storage/migration.go。
type SubtaskStatus string

const (
	SubtaskStatusActive   SubtaskStatus = "active"
	SubtaskStatusDone     SubtaskStatus = "done"
	SubtaskStatusArchived SubtaskStatus = "archived"
)

// IsValidSubtaskStatus 检查 s 是否是 SubtaskStatus 合法值。CHECK 约束只接受
// inbox/active/done/archived/rejected 五个值;子任务语义限定到 active/done/archived。
func IsValidSubtaskStatus(s SubtaskStatus) bool {
	switch s {
	case SubtaskStatusActive, SubtaskStatusDone, SubtaskStatusArchived:
		return true
	}
	return false
}

// TaskRow 是一行 `tasks`。自引用:parent_id 为 NULL 表示顶层任务。
type TaskRow struct {
	ID               int64      `json:"id"`
	ParentID         *int64     `json:"parent_id,omitempty"`
	RawText          string     `json:"raw_text,omitempty"`
	Title            string     `json:"title"`
	Notes            string     `json:"notes,omitempty"`
	Status           TaskStatus `json:"status"`
	DueDate          *string    `json:"due_date,omitempty"`
	ScheduledStart   *int64     `json:"scheduled_start,omitempty"`
	ScheduledEnd     *int64     `json:"scheduled_end,omitempty"`
	Pinned           bool       `json:"pinned"`
	EstimatedMinutes int64      `json:"estimated_minutes"`
	OrderIndex       int64      `json:"order_index"`
	Source           string     `json:"source"`
	AIStatus         AIStatus   `json:"ai_status"`
	AIError          string     `json:"ai_error,omitempty"`
	UserEditedTitle  bool       `json:"user_edited_title"`
	// v11:乐观锁版本号,server 在每次 UpdateTask 时 ++;int64 与 DB INTEGER 对齐。
	Version int64 `json:"version"`
	// v11:5 维调度评分(见 scheduler 设计 §3.1)。blocked_by 由用户显式标,
	// 不经 LLM;其它 4 维 LLM 预填 + 用户可改。omitempty 让空切片不出现在 JSON 里。
	PriorityScore  int        `json:"priority_score"`
	UrgencyScore   int        `json:"urgency_score"`
	EnergyRequired int        `json:"energy_required"`
	ContextTag     string     `json:"context_tag"`
	BlockedBy      []int64    `json:"blocked_by,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	ParsedAt       *time.Time `json:"parsed_at,omitempty"`
}

// SubtaskRow 是一行 `task_subtasks` 语义的 tasks 子任务(通过 parent_id 关联)。
type SubtaskRow struct {
	ID               int64         `json:"id"`
	TaskID           int64         `json:"task_id"`
	Title            string        `json:"title"`
	Status           SubtaskStatus `json:"status"`
	EstimatedMinutes int64         `json:"estimated_minutes"`
	OrderIndex       int64         `json:"order_index"`
	CreatedAt        time.Time     `json:"created_at"`
}

// AIJobStatus 是 ai_jobs 的状态机。
type AIJobStatus string

const (
	AIJobStatusQueued  AIJobStatus = "queued"
	AIJobStatusRunning AIJobStatus = "running"
	AIJobStatusSuccess AIJobStatus = "success"
	AIJobStatusFailed  AIJobStatus = "failed"
)

// AIJobMode 是 ai_jobs 的处理模式。append=只增不改;replace=重建(保留 done);
// review=后续 GTD 回顾(见 issue #7,本期不实现)。
type AIJobMode string

const (
	AIJobModeAppend  AIJobMode = "append"
	AIJobModeReplace AIJobMode = "replace"
	AIJobModeReview  AIJobMode = "review"
)

// AIJobRow 是一行 `ai_jobs`。
type AIJobRow struct {
	ID              int64       `json:"id"`
	TaskID          int64       `json:"task_id"`
	Provider        string      `json:"provider"`
	Model           string      `json:"model"`
	Status          AIJobStatus `json:"status"`
	RequestPayload  string      `json:"request_payload"`
	ResponsePayload string      `json:"response_payload,omitempty"`
	ErrorMessage    string      `json:"error_message,omitempty"`
	InputTokens     *int        `json:"input_tokens,omitempty"`
	OutputTokens    *int        `json:"output_tokens,omitempty"`
	Attempts        int         `json:"attempts"`
	Mode            AIJobMode   `json:"mode"`
	StartedAt       *time.Time  `json:"started_at,omitempty"`
	FinishedAt      *time.Time  `json:"finished_at,omitempty"`
	CreatedAt       time.Time   `json:"created_at"`
}

// AIDecompositionResult 是 AI 解析随手记后返回的结构化结果。
//
// v11(capture/reparse 扩展):额外携带 4 维调度评分(spec §5.3)。LLM 输出
// 形态为嵌套 `_scores` 对象,validator 也兼容旧的顶层键。缺省 / 越界时由
// ValidateDecomposition 回填默认值 3/5/2/general。blocked_by 不经 LLM。
type AIDecompositionResult struct {
	Title            string         `json:"title"`
	Notes            string         `json:"notes,omitempty"`
	IsComplex        bool           `json:"is_complex"`
	DueDate          *string        `json:"due_date,omitempty"`
	EstimatedMinutes int            `json:"estimated_minutes"`
	Subtasks         []SubtaskDraft `json:"subtasks"`
	// 4 维评分。JSON tag 同时用于顶层兼容形态;spec §5.3 的嵌套 `_scores`
	// 由 schema.go 显式解析(不会随本结构体直接反序列化)。
	PriorityScore  int    `json:"priority_score"`
	UrgencyScore   int    `json:"urgency_score"`
	EnergyRequired int    `json:"energy_required"`
	ContextTag     string `json:"context_tag"`
}

// SubtaskDraft 是 AI 拆解结果中的一个子任务草案。
type SubtaskDraft struct {
	Title            string  `json:"title"`
	EstimatedMinutes int     `json:"estimated_minutes"`
	DueDate          *string `json:"due_date,omitempty"`
}

// IsValidTaskStatus 报告字符串是否为合法任务状态。
func IsValidTaskStatus(s string) bool {
	switch TaskStatus(s) {
	case TaskStatusInbox, TaskStatusActive, TaskStatusDone,
		TaskStatusArchived, TaskStatusRejected:
		return true
	}
	return false
}

// Adoption 是 GTD Review 用户采纳审计的一行(ai_jobs.response_payload 是 AI
// 提议的不可变快照,adoptions 是用户决策记录——v12 新增)。task_id /
// ai_job_id / change_id / version_before / version_after 构成采纳的
// 完整可重放链;version_before 与 version_after 之差恒为 1(adopt handler
// 每次恰好 bump 一次 task version,见 daily_sweep adopt handler)。
type Adoption struct {
	ID            int64     `json:"id"`
	TaskID        int64     `json:"task_id"`
	AIJobID       int64     `json:"ai_job_id"`
	ChangeID      string    `json:"change_id"`
	VersionBefore int64     `json:"version_before"`
	VersionAfter  int64     `json:"version_after"`
	AppliedAt     time.Time `json:"applied_at"`
}

// DailySweepLog 是同日去重日志的一行(对应 daily_sweep_log 表)。cron 触发
// 前查 date;手动触发不查(用户主动想再做一次)。source 标识 cron | manual。
type DailySweepLog struct {
	Date    string    `json:"date"`
	SweptAt time.Time `json:"swept_at"`
	Source  string    `json:"source"`
}

// ReviewProposal 是 GTD Review 模式 LLM 产出的一条提议。Action 决定 dispatch
// (reschedule/drop/modify_subtasks/no_op);SuggestedSubtaskChanges 仅在
// Action=modify_subtasks 时被 apply。Reason 由 LLM 提供,供前端展示与人工核对。
type ReviewProposal struct {
	ChangeID                string              `json:"change_id"`
	TaskID                  int64               `json:"task_id"`
	Action                  string              `json:"action"` // reschedule|drop|modify_subtasks|no_op
	SuggestedStart          *int64              `json:"suggested_scheduled_start,omitempty"`
	SuggestedEnd            *int64              `json:"suggested_scheduled_end,omitempty"`
	SuggestedSubtaskChanges []SuggestedSubChange `json:"suggested_subtask_changes,omitempty"`
	Reason                  string              `json:"reason"`
}

// SuggestedSubChange 是 ReviewProposal 内的一条子任务变更。Op 集合:
// delete | modify_title | add | mark_done。mark_done 仅对 active 子任务生效
// (delta Δ3);非 active 目标在 apply 阶段被跳过而非报错。
type SuggestedSubChange struct {
	SubtaskID *int64  `json:"subtask_id"`
	Op        string  `json:"op"` // delete|modify_title|add|mark_done
	NewTitle  *string `json:"new_title,omitempty"`
	Reason    string  `json:"reason,omitempty"`
}
