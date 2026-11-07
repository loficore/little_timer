// Package tasks 是任务树(顶层 task + 子任务)的用例层。
//
// 设计(issue #66):
//   - HTTP handler 只做 JSON 绑定与 tasks.Error → HTTP 状态码映射;
//   - 乐观锁(version 闸门)、user_edited_title 置位规则、三态字段语义
//     集中在本包;
//   - Store 由 storage.AITasksCrud 满足,handler 不再 import internal/storage。
package tasks

import (
	"little-timer/internal/domain"
	"little-timer/internal/storage"
)

// Error 是任务域的类型化错误。Adapter 用 errors.As 映射成 HTTP 响应;
// Code 是稳定的、前端可依赖的字符串。
type Error struct {
	Code    string
	Message string
}

func (e *Error) Error() string { return e.Message }

// 哨兵错误。UpdatePatch / UpdateSubtask 等规则方法返回这些。
var (
	// ErrVersionConflict 表示乐观锁 WHERE id=? AND version=? 命中 0 行
	// (402/409 语义):客户端持有的 version 已过期。
	ErrVersionConflict = &Error{Code: "version_conflict", Message: "version conflict"}
	// ErrNotFound 表示目标 task 不存在。
	ErrNotFound = &Error{Code: "tasks.not_found", Message: "task not found"}
	// ErrTitleRequired 表示任务 / 子任务标题为空。
	ErrTitleRequired = &Error{Code: "tasks.title_required", Message: "title required"}
	// ErrInvalidSubtaskStatus 表示子任务 status 不在 {active,done,archived}。
	ErrInvalidSubtaskStatus = &Error{
		Code:    "tasks.invalid_subtask_status",
		Message: "invalid status (allowed: active|done|archived)",
	}
)

// Store 是本包依赖的持久化能力集合。storage.AITasksCrud 直接满足它。
type Store interface {
	CreateTask(rawText, source string) (int64, error)
	GetTask(id int64) (*domain.TaskRow, []domain.SubtaskRow, error)
	ListTasks(status string, parentID *int64, limit, offset int) ([]domain.TaskRow, error)
	ListSubtasks(taskID int64) ([]domain.SubtaskRow, error)
	UpdateTask(id int64, fields storage.TaskUpdateFields, version int64) error
	DeleteTask(id int64, version int64) error
	SetAIStatus(taskID int64, status domain.AIStatus, errMsg string) error
	CreateSubtask(taskID int64, title string, estMin int, dueDate *string, orderIndex int) (int64, error)
	UpdateSubtask(id int64, fields storage.SubtaskUpdateFields, version int64) error
	DeleteSubtask(id int64) error
}

// UpdatePatch 是 PATCH /api/tasks/:id 的用例级入参。
//
// Clear* 标志区分"字段缺失(不动)"与"显式 null(置 NULL)":
// 指针为 nil 且 Clear 为 false 表示不改;Clear 为 true 表示置 NULL。
type UpdatePatch struct {
	Title               *string
	Notes               *string
	DueDate             *string
	ClearDueDate        bool
	Status              *domain.TaskStatus
	ParentID            *int64
	ClearParentID       bool
	ScheduledStart      *int64
	ClearScheduledStart bool
	ScheduledEnd        *int64
	ClearScheduledEnd   bool
	Pinned              *bool
	OrderIndex          *int64
	EstimatedMinutes    *int64
	// UserEditedTitle 是显式覆盖 user_edited_title 的入口(导入 / 测试用)。
	// 正常 PATCH 不设置它,由 Title 变化推导。
	UserEditedTitle *bool
}

// SubtaskInput 是 CreateSubtask 的入参。
type SubtaskInput struct {
	Title            string
	EstimatedMinutes int
	DueDate          *string
	OrderIndex       int
}

// SubtaskPatch 是 PATCH /api/tasks/:id/subtasks/:sub_id 的用例级入参。
type SubtaskPatch struct {
	Title            *string
	Status           *domain.SubtaskStatus
	EstimatedMinutes *int64
	OrderIndex       *int64
}

// CreateInput 是 Create 的入参:直接建一条任务(绕过 AI)。
type CreateInput struct {
	Title  string
	Notes  string
	Source string
}

// TaskWithSubtasks 是列表项:task 本体 + 它的拆解步骤。
type TaskWithSubtasks struct {
	Task     domain.TaskRow
	Subtasks []domain.SubtaskRow
}

// ScoresPatch 是 PATCH /api/tasks/:id/scores 的用例级入参。
//
// 全部 *T,nil 表示"不动"。context_tag 非枚举值由 operation 兜底为
// general 并返回 warning,由 adapter 转成 X-LLM-Warning 响应头。
type ScoresPatch struct {
	PriorityScore  *int
	UrgencyScore   *int
	EnergyRequired *int
	ContextTag     *string
}

// ScoreRangeError 表示某个评分字段越界。adapter 据此返回 400,并把
// Field / Value / Min / Max 原样回给客户端(前端可依赖的稳定 payload)。
type ScoreRangeError struct {
	Field string
	Value int
	Min   int
	Max   int
}

func (e *ScoreRangeError) Error() string { return "score out of range" }

// Code 返回稳定的错误码,与历史 HTTP 契约一致。
func (e *ScoreRangeError) Code() string { return "scores_out_of_range" }
