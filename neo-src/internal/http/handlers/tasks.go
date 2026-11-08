// Package handlers —— /api/tasks/* 与 /api/tasks/:id/subtasks/* 端点。
//
// 本文件只做 transport 绑定:JSON 三态探测、URL 参数解析、tasks.Error →
// HTTP 状态码映射。乐观锁、user_edited_title 规则、字段语义全在
// internal/tasks(issue #66)。
package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"little-timer/internal/domain"
	"little-timer/internal/http/app"
	"little-timer/internal/http/middleware"
	"little-timer/internal/tasks"
)

// TaskDTO 是出参形状,与前端 bindings 对齐。
type TaskDTO struct {
	ID               int64   `json:"id"`
	ParentID         *int64  `json:"parent_id,omitempty"`
	RawText          string  `json:"raw_text,omitempty"`
	Title            string  `json:"title"`
	Notes            string  `json:"notes,omitempty"`
	Status           string  `json:"status"`
	DueDate          *string `json:"due_date,omitempty"`
	ScheduledStart   *int64  `json:"scheduled_start,omitempty"`
	ScheduledEnd     *int64  `json:"scheduled_end,omitempty"`
	Pinned           bool    `json:"pinned"`
	EstimatedMinutes int64   `json:"estimated_minutes"`
	OrderIndex       int64   `json:"order_index"`
	Source           string  `json:"source"`
	AIStatus         string  `json:"ai_status"`
	AIError          string  `json:"ai_error,omitempty"`
	UserEditedTitle  bool    `json:"user_edited_title"`
	// v11:5 维调度评分 + 乐观锁版本号(见 scheduler 设计 §3.1)。BlockedBy
	// 为空切片时 omitempty 不出现在 JSON 里,与 domain.TaskRow 对齐。
	Version        int64        `json:"version"`
	PriorityScore  int          `json:"priority_score"`
	UrgencyScore   int          `json:"urgency_score"`
	EnergyRequired int          `json:"energy_required"`
	ContextTag     string       `json:"context_tag"`
	BlockedBy      []int64      `json:"blocked_by,omitempty"`
	Subtasks       []SubtaskDTO `json:"subtasks,omitempty"`
	CreatedAt      string       `json:"created_at"`
	UpdatedAt      string       `json:"updated_at"`
	ParsedAt       *string      `json:"parsed_at,omitempty"`
}

// SubtaskDTO 是子任务的出参形状。
type SubtaskDTO struct {
	ID               int64  `json:"id"`
	TaskID           int64  `json:"task_id"`
	Title            string `json:"title"`
	Status           string `json:"status"`
	EstimatedMinutes int64  `json:"estimated_minutes"`
	OrderIndex       int64  `json:"order_index"`
	// Version 是子任务自己的乐观锁版本（不是父任务的）。
	Version int64 `json:"version"`
}

func toTaskDTO(t *domain.TaskRow, subs []domain.SubtaskRow) TaskDTO {
	dto := TaskDTO{
		ID:               t.ID,
		ParentID:         t.ParentID,
		RawText:          t.RawText,
		Title:            t.Title,
		Notes:            t.Notes,
		Status:           string(t.Status),
		DueDate:          t.DueDate,
		ScheduledStart:   t.ScheduledStart,
		ScheduledEnd:     t.ScheduledEnd,
		Pinned:           t.Pinned,
		EstimatedMinutes: t.EstimatedMinutes,
		OrderIndex:       t.OrderIndex,
		Source:           t.Source,
		AIStatus:         string(t.AIStatus),
		AIError:          t.AIError,
		UserEditedTitle:  t.UserEditedTitle,
		Version:          t.Version,
		PriorityScore:    t.PriorityScore,
		UrgencyScore:     t.UrgencyScore,
		EnergyRequired:   t.EnergyRequired,
		ContextTag:       t.ContextTag,
		BlockedBy:        t.BlockedBy,
		CreatedAt:        t.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:        t.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
	if t.ParsedAt != nil {
		s := t.ParsedAt.Format("2006-01-02T15:04:05Z07:00")
		dto.ParsedAt = &s
	}
	for _, s := range subs {
		// 复用唯一转换函数，避免这里漏字段（曾经漏了 version，导致前端拿到 0
		// 去 PATCH 子任务必然 409）。
		dto.Subtasks = append(dto.Subtasks, toSubtaskDTO(s))
	}
	return dto
}

// taskDTOFromRow 把单行 task 转成 DTO(不带子任务)。供 /scores 等只需要
// task 本身的分支复用,避免每次都拉取子任务。
func taskDTOFromRow(t *domain.TaskRow) TaskDTO {
	return toTaskDTO(t, nil)
}

// taskOps 返回任务用例层。Store 由 storage.AITasksCrud 直接满足,构造
// 成本只有一个 struct,无需在 App 上缓存(也不让 handler import storage)。
func taskOps(a *app.App) *tasks.Operations {
	return tasks.New(a.SQLite.AITasks())
}

// respondTaskError 是 tasks.Error → HTTP 状态码的唯一映射点
// (issue #66:「409 只在一处映射」)。fallback 用于非类型化错误(500)的
// 文案,保持各端点的既有措辞。
func respondTaskError(c *gin.Context, err error, fallback string) {
	var de *tasks.Error
	if errors.As(err, &de) {
		switch de.Code {
		case tasks.ErrVersionConflict.Code:
			respondVersionConflict(c, "")
			return
		case tasks.ErrNotFound.Code:
			c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "task not found"})
			return
		case tasks.ErrInvalidSubtaskStatus.Code:
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": de.Message})
			return
		case tasks.ErrTitleRequired.Code:
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": de.Message})
			return
		}
	}
	c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": fallback})
}

// TaskList GET /api/tasks?status=&parent_id=
func TaskList(c *gin.Context) {
	a := appFromCtx(c)
	limit, offset, valid := parsePagination(c)
	if !valid {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid pagination"})
		return
	}
	status := c.Query("status")
	var parentID *int64
	if pidStr := c.Query("parent_id"); pidStr != "" {
		pid, perr := strconv.ParseInt(pidStr, 10, 64)
		if perr != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid parent_id"})
			return
		}
		parentID = &pid
	}
	rows, err := taskOps(a).List(status, parentID, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "list failed"})
		return
	}
	out := make([]TaskDTO, 0, len(rows))
	for i := range rows {
		row := rows[i].Task
		out = append(out, toTaskDTO(&row, rows[i].Subtasks))
	}
	c.JSON(http.StatusOK, out)
}

// TaskGet GET /api/tasks/:id
func TaskGet(c *gin.Context) {
	a := appFromCtx(c)
	id, err := parseInt64Param(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid id"})
		return
	}
	task, subs, err := taskOps(a).Get(id)
	if err != nil {
		respondTaskError(c, err, "get failed")
		return
	}
	c.JSON(http.StatusOK, toTaskDTO(task, subs))
}

// TaskCreate POST /api/tasks —— 直接建一条(绕过 AI)
func TaskCreate(c *gin.Context) {
	a := appFromCtx(c)
	var req struct {
		Title string `json:"title"`
		Notes string `json:"notes"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid json"})
		return
	}
	if req.Title == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "title required"})
		return
	}
	taskID, err := taskOps(a).Create(tasks.CreateInput{
		Title:  req.Title,
		Notes:  req.Notes,
		Source: "manual",
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "create failed"})
		return
	}
	task, subs, err := taskOps(a).Get(taskID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "create failed"})
		return
	}
	c.JSON(http.StatusOK, toTaskDTO(task, subs))
}

// TaskUpdate PATCH /api/tasks/:id —— 指针语义更新
//
// 用 json.RawMessage 探测键是否存在以及值是否为 JSON null,以明确区分
// "缺字段(不动) / 显式 null(置 NULL) / 有值(更新)" 三种语义。
func TaskUpdate(c *gin.Context) {
	a := appFromCtx(c)
	id, err := parseInt64Param(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid id"})
		return
	}
	rawBody, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "read body"})
		return
	}
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(rawBody, &probe); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid json"})
		return
	}
	var body map[string]any
	_ = json.Unmarshal(rawBody, &body)

	var patch tasks.UpdatePatch
	if v, ok := body["title"].(string); ok {
		patch.Title = &v
	}
	if v, ok := body["notes"].(string); ok {
		patch.Notes = &v
	}
	// due_date: 三态(缺/显式 null/值)
	if rm, ok := probe["due_date"]; ok {
		if isJSONNull(rm) {
			patch.ClearDueDate = true
		} else if v, ok := body["due_date"].(string); ok {
			patch.DueDate = &v
		}
	}
	if v, ok := body["status"].(string); ok && domain.IsValidTaskStatus(v) {
		s := domain.TaskStatus(v)
		patch.Status = &s
	}
	if rm, ok := probe["parent_id"]; ok {
		if isJSONNull(rm) {
			patch.ClearParentID = true
		} else if v, ok := body["parent_id"].(float64); ok {
			pid := int64(v)
			patch.ParentID = &pid
		}
	}
	if rm, ok := probe["scheduled_start"]; ok {
		if isJSONNull(rm) {
			patch.ClearScheduledStart = true
		} else if v, ok := body["scheduled_start"].(float64); ok {
			s := int64(v)
			patch.ScheduledStart = &s
		}
	}
	if rm, ok := probe["scheduled_end"]; ok {
		if isJSONNull(rm) {
			patch.ClearScheduledEnd = true
		} else if v, ok := body["scheduled_end"].(float64); ok {
			s := int64(v)
			patch.ScheduledEnd = &s
		}
	}
	if v, ok := body["pinned"].(bool); ok {
		patch.Pinned = &v
	}
	if v, ok := body["order_index"].(float64); ok {
		o := int64(v)
		patch.OrderIndex = &o
	}
	if v, ok := body["estimated_minutes"].(float64); ok {
		e := int64(v)
		patch.EstimatedMinutes = &e
	}

	task, err := taskOps(a).Update(id, patch, middleware.GetVersion(c))
	if err != nil {
		respondTaskError(c, err, "update failed")
		return
	}
	_, subs, err := taskOps(a).Get(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "update failed"})
		return
	}
	c.JSON(http.StatusOK, toTaskDTO(&task, subs))
}

// TaskDelete DELETE /api/tasks/:id
func TaskDelete(c *gin.Context) {
	a := appFromCtx(c)
	id, err := parseInt64Param(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid id"})
		return
	}
	// C1 fix:删除也走乐观锁 —— 用 ConflictGuard 解析出的期望版本,
	// 0 行受影响(版本不符/task 不存在)→ 409。
	if err := taskOps(a).Delete(id, middleware.GetVersion(c)); err != nil {
		respondTaskError(c, err, "delete failed")
		return
	}
	c.Status(http.StatusNoContent)
}

// SubtaskCreate POST /api/tasks/:id/subtasks
func SubtaskCreate(c *gin.Context) {
	a := appFromCtx(c)
	id, err := parseInt64Param(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid task id"})
		return
	}
	var body struct {
		Title            string  `json:"title"`
		EstimatedMinutes int     `json:"estimated_minutes"`
		DueDate          *string `json:"due_date"`
		OrderIndex       int     `json:"order_index"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid json"})
		return
	}
	if body.Title == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "title required"})
		return
	}
	sub, err := taskOps(a).CreateSubtask(id, tasks.SubtaskInput{
		Title:            body.Title,
		EstimatedMinutes: body.EstimatedMinutes,
		DueDate:          body.DueDate,
		OrderIndex:       body.OrderIndex,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "create failed"})
		return
	}
	c.JSON(http.StatusOK, toSubtaskDTO(sub))
}

// SubtaskUpdate PATCH /api/tasks/:id/subtasks/:sub_id
func SubtaskUpdate(c *gin.Context) {
	a := appFromCtx(c)
	subID, err := parseInt64Param(c, "sub_id")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid sub_id"})
		return
	}
	var body map[string]any
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid json"})
		return
	}
	var patch tasks.SubtaskPatch
	if v, ok := body["title"].(string); ok {
		patch.Title = &v
	}
	if v, ok := body["status"].(string); ok {
		s := domain.SubtaskStatus(v)
		patch.Status = &s
	}
	if v, ok := body["estimated_minutes"].(float64); ok {
		e := int64(v)
		patch.EstimatedMinutes = &e
	}
	if v, ok := body["order_index"].(float64); ok {
		o := int64(v)
		patch.OrderIndex = &o
	}
	// C2 fix:子任务也走乐观锁。子任务本身是 tasks 行(带 parent_id 与
	// version),与 UpdateTask 同源同语义。
	if err := taskOps(a).UpdateSubtask(subID, patch, middleware.GetVersion(c)); err != nil {
		respondTaskError(c, err, "update failed")
		return
	}
	c.Status(http.StatusNoContent)
}

// SubtaskDelete DELETE /api/tasks/:id/subtasks/:sub_id
func SubtaskDelete(c *gin.Context) {
	a := appFromCtx(c)
	subID, err := parseInt64Param(c, "sub_id")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid sub_id"})
		return
	}
	if err := taskOps(a).DeleteSubtask(subID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "delete failed"})
		return
	}
	c.Status(http.StatusNoContent)
}

func toSubtaskDTO(s domain.SubtaskRow) SubtaskDTO {
	return SubtaskDTO{
		ID:               s.ID,
		TaskID:           s.TaskID,
		Title:            s.Title,
		Status:           string(s.Status),
		EstimatedMinutes: s.EstimatedMinutes,
		OrderIndex:       s.OrderIndex,
		Version:          s.Version,
	}
}

// isJSONNull 判断 RawMessage 是否就是 JSON 字面量 null。
func isJSONNull(rm json.RawMessage) bool {
	return strings.TrimSpace(string(rm)) == "null"
}

// parseInt64Param 把 URL 路径参数解析为 int64。
func parseInt64Param(c *gin.Context, name string) (int64, error) {
	return strconv.ParseInt(c.Param(name), 10, 64)
}
