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
type SubtaskStatus string

const (
	SubtaskStatusPending SubtaskStatus = "pending"
	SubtaskStatusDone    SubtaskStatus = "done"
	SubtaskStatusSkipped SubtaskStatus = "skipped"
)

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
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
	ParsedAt         *time.Time `json:"parsed_at,omitempty"`
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
	StartedAt       *time.Time  `json:"started_at,omitempty"`
	FinishedAt      *time.Time  `json:"finished_at,omitempty"`
	CreatedAt       time.Time   `json:"created_at"`
}

// AIDecompositionResult 是 AI 解析随手记后返回的结构化结果。
type AIDecompositionResult struct {
	Title            string         `json:"title"`
	Notes            string         `json:"notes,omitempty"`
	IsComplex        bool           `json:"is_complex"`
	DueDate          *string        `json:"due_date,omitempty"`
	EstimatedMinutes int            `json:"estimated_minutes"`
	Subtasks         []SubtaskDraft `json:"subtasks"`
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
