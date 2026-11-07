package tasks

import (
	"errors"
	"strings"

	"little-timer/internal/domain"
	"little-timer/internal/storage"
)

// Operations 是任务树的用例层。
type Operations struct {
	store Store
}

// New 构造 Operations。
func New(store Store) *Operations {
	return &Operations{store: store}
}

// List 返回指定筛选条件下的任务,逐条带上子任务。
//
// 列表也带拆解步骤:AI 拆好的步骤要能在卡片上看到、进编辑弹窗修改,
// 刷新页面后不能凭空消失。(任务量是本地单用户量级,逐条查询可接受。)
func (o *Operations) List(status string, parentID *int64, limit, offset int) ([]TaskWithSubtasks, error) {
	rows, err := o.store.ListTasks(status, parentID, limit, offset)
	if err != nil {
		return nil, err
	}
	out := make([]TaskWithSubtasks, 0, len(rows))
	for i := range rows {
		subs, err := o.store.ListSubtasks(rows[i].ID)
		if err != nil {
			return nil, err
		}
		out = append(out, TaskWithSubtasks{Task: rows[i], Subtasks: subs})
	}
	return out, nil
}

// Get 返回 task 本体与子任务;不存在 → ErrNotFound。
//
// storage 的 GetTask 在无行时返回包装过的 ErrAITaskQueryFailed(不是
// sql.ErrNoRows),且原有 HTTP 契约是"任何取不到都算 404",故这里统一
// 归为 ErrNotFound。
func (o *Operations) Get(id int64) (*domain.TaskRow, []domain.SubtaskRow, error) {
	task, subs, err := o.store.GetTask(id)
	if err != nil || task == nil {
		return nil, nil, ErrNotFound
	}
	return task, subs, nil
}

// Create 直接建一条任务(绕过 AI),并一次性初始化 title / notes /
// status / ai_status。
//
// issue #10:手动创建时用户自带 title,视为"用户编辑过" → 翻
// user_edited_title=true,后续 append reparse 不覆盖用户起的标题。
func (o *Operations) Create(in CreateInput) (int64, error) {
	title := in.Title
	if strings.TrimSpace(title) == "" {
		return 0, ErrTitleRequired
	}
	source := in.Source
	if source == "" {
		source = "manual"
	}
	taskID, err := o.store.CreateTask(title, source)
	if err != nil {
		return 0, err
	}
	notes := in.Notes
	active := domain.TaskStatusActive
	userEdited := true
	if err := o.store.UpdateTask(taskID, storage.TaskUpdateFields{
		Title:           &title,
		Notes:           &notes,
		Status:          &active,
		UserEditedTitle: &userEdited,
	}, 0); err != nil {
		return 0, err
	}
	// ai_status 不在 TaskUpdateFields 里,单独写。
	if err := o.store.SetAIStatus(taskID, domain.AIStatusDone, ""); err != nil {
		return 0, err
	}
	return taskID, nil
}

// Update 按 patch 的指针语义更新任务,带乐观锁。
//
// user_edited_title 只在标题真正变化时置位 —— round-trip / 自动保存的
// 同值 PATCH 不能把 flag 粘死。读旧值失败时保守不翻。
func (o *Operations) Update(id int64, patch UpdatePatch, version int64) (domain.TaskRow, error) {
	fields := storage.TaskUpdateFields{
		Notes:               patch.Notes,
		DueDate:             patch.DueDate,
		ClearDueDate:        patch.ClearDueDate,
		Status:              patch.Status,
		ParentID:            patch.ParentID,
		ClearParentID:       patch.ClearParentID,
		ScheduledStart:      patch.ScheduledStart,
		ClearScheduledStart: patch.ClearScheduledStart,
		ScheduledEnd:        patch.ScheduledEnd,
		ClearScheduledEnd:   patch.ClearScheduledEnd,
		Pinned:              patch.Pinned,
		OrderIndex:          patch.OrderIndex,
		EstimatedMinutes:    patch.EstimatedMinutes,
		UserEditedTitle:     patch.UserEditedTitle,
	}
	if patch.Title != nil && strings.TrimSpace(*patch.Title) != "" {
		title := *patch.Title
		fields.Title = &title
		if cur, _, gerr := o.store.GetTask(id); gerr == nil && cur != nil {
			if strings.TrimSpace(title) != strings.TrimSpace(cur.Title) {
				edited := true
				fields.UserEditedTitle = &edited
			}
		}
	}
	if err := o.store.UpdateTask(id, fields, version); err != nil {
		return domain.TaskRow{}, mapStoreError(err)
	}
	task, _, err := o.store.GetTask(id)
	if err != nil {
		return domain.TaskRow{}, err
	}
	if task == nil {
		return domain.TaskRow{}, ErrNotFound
	}
	return *task, nil
}

// Delete 删除任务(级联子任务与 ai_jobs),带乐观锁。
func (o *Operations) Delete(id, version int64) error {
	if err := o.store.DeleteTask(id, version); err != nil {
		return mapStoreError(err)
	}
	return nil
}

// CreateSubtask 建一条子任务并回读完整行(含它自己的 version)。
func (o *Operations) CreateSubtask(taskID int64, in SubtaskInput) (domain.SubtaskRow, error) {
	if strings.TrimSpace(in.Title) == "" {
		return domain.SubtaskRow{}, ErrTitleRequired
	}
	subID, err := o.store.CreateSubtask(taskID, in.Title, in.EstimatedMinutes, in.DueDate, in.OrderIndex)
	if err != nil {
		return domain.SubtaskRow{}, err
	}
	_, subs, err := o.store.GetTask(taskID)
	if err != nil {
		return domain.SubtaskRow{}, err
	}
	for _, s := range subs {
		if s.ID == subID {
			return s, nil
		}
	}
	// 回读不到时退化返回最小行,保持 HTTP 契约(响应带 id 即可)。
	return domain.SubtaskRow{ID: subID, TaskID: taskID, Title: in.Title,
		Status: domain.SubtaskStatusActive, EstimatedMinutes: int64(in.EstimatedMinutes),
		OrderIndex: int64(in.OrderIndex)}, nil
}

// UpdateSubtask 更新子任务,带乐观锁。非法 status → ErrInvalidSubtaskStatus。
//
// 子任务本身是 tasks 行(带 parent_id 与 version),与 Update 同源同语义。
func (o *Operations) UpdateSubtask(id int64, patch SubtaskPatch, version int64) error {
	if patch.Status != nil && !domain.IsValidSubtaskStatus(*patch.Status) {
		return ErrInvalidSubtaskStatus
	}
	fields := storage.SubtaskUpdateFields{
		Title:            patch.Title,
		Status:           patch.Status,
		EstimatedMinutes: patch.EstimatedMinutes,
		OrderIndex:       patch.OrderIndex,
	}
	if err := o.store.UpdateSubtask(id, fields, version); err != nil {
		return mapStoreError(err)
	}
	return nil
}

// DeleteSubtask 删除一条子任务。
func (o *Operations) DeleteSubtask(id int64) error {
	return o.store.DeleteSubtask(id)
}

// mapStoreError 把 storage 的哨兵错误翻译成域错误,避免 storage 类型
// 泄漏到 adapter。
func mapStoreError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, storage.ErrVersionConflict) {
		return ErrVersionConflict
	}
	return err
}
