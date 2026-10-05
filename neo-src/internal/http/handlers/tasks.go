// Package handlers —— /api/tasks/* 与 /api/tasks/:id/subtasks/* 端点。
//
// DTO 转换与领域类型分离;handler 只关心指针语义的更新与 status 流转。
package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"little-timer/internal/domain"
	"little-timer/internal/storage"
)

// TaskDTO 是出参形状,与前端 bindings 对齐。
type TaskDTO struct {
	ID               int64        `json:"id"`
	ParentID         *int64       `json:"parent_id,omitempty"`
	RawText          string       `json:"raw_text,omitempty"`
	Title            string       `json:"title"`
	Notes            string       `json:"notes,omitempty"`
	Status           string       `json:"status"`
	DueDate          *string      `json:"due_date,omitempty"`
	ScheduledStart   *int64       `json:"scheduled_start,omitempty"`
	ScheduledEnd     *int64       `json:"scheduled_end,omitempty"`
	Pinned           bool         `json:"pinned"`
	EstimatedMinutes int64        `json:"estimated_minutes"`
	OrderIndex       int64        `json:"order_index"`
	Source           string       `json:"source"`
	AIStatus         string       `json:"ai_status"`
	AIError          string       `json:"ai_error,omitempty"`
	UserEditedTitle  bool         `json:"user_edited_title"`
	Subtasks         []SubtaskDTO `json:"subtasks,omitempty"`
	CreatedAt        string       `json:"created_at"`
	UpdatedAt        string       `json:"updated_at"`
	ParsedAt         *string      `json:"parsed_at,omitempty"`
}

// SubtaskDTO 是子任务的出参形状。
type SubtaskDTO struct {
	ID               int64  `json:"id"`
	TaskID           int64  `json:"task_id"`
	Title            string `json:"title"`
	Status           string `json:"status"`
	EstimatedMinutes int64  `json:"estimated_minutes"`
	OrderIndex       int64  `json:"order_index"`
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
		CreatedAt:        t.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:        t.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
	if t.ParsedAt != nil {
		s := t.ParsedAt.Format("2006-01-02T15:04:05Z07:00")
		dto.ParsedAt = &s
	}
	for _, s := range subs {
		dto.Subtasks = append(dto.Subtasks, SubtaskDTO{
			ID:               s.ID,
			TaskID:           s.TaskID,
			Title:            s.Title,
			Status:           string(s.Status),
			EstimatedMinutes: s.EstimatedMinutes,
			OrderIndex:       s.OrderIndex,
		})
	}
	return dto
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
	rows, err := a.SQLite.AITasks().ListTasks(status, parentID, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "list failed"})
		return
	}
	out := make([]TaskDTO, 0, len(rows))
	for i := range rows {
		t := rows[i]
		out = append(out, toTaskDTO(&t, nil))
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
	task, subs, err := a.SQLite.AITasks().GetTask(id)
	if err != nil || task == nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "task not found"})
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
	taskID, err := a.SQLite.AITasks().CreateTask(req.Title, "manual")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "create failed"})
		return
	}
	// 手动创建时,把入参 title 写到 title 列;notes 也直接写入。
	title := req.Title
	if err := a.SQLite.AITasks().UpdateTask(taskID, storage.TaskUpdateFields{Title: &title}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "set title failed"})
		return
	}
	if req.Notes != "" {
		notes := req.Notes
		_ = a.SQLite.AITasks().UpdateTask(taskID, storage.TaskUpdateFields{Notes: &notes})
	}
	// 直接进入 active 状态(不需 AI 处理)。
	_ = a.SQLite.AITasks().SetTaskStatus(taskID, domain.TaskStatusActive)
	_ = a.SQLite.AITasks().SetAIStatus(taskID, domain.AIStatusDone, "")
	task, subs, _ := a.SQLite.AITasks().GetTask(taskID)
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

	fields := storage.TaskUpdateFields{}
	if v, ok := body["title"].(string); ok {
		if strings.TrimSpace(v) != "" {
			fields.Title = &v
			// 仅当新值与旧值不同时才标记 user_edited_title(避免客户端
			// round-trip / autosave 同值 PATCH 粘死 flag)。读旧值失败时保守
			// 不翻(下次真正改名的 PATCH 仍会翻)。
			if cur, _, gerr := a.SQLite.AITasks().GetTask(id); gerr == nil && cur != nil {
				if strings.TrimSpace(v) != strings.TrimSpace(cur.Title) {
					edited := true
					fields.UserEditedTitle = &edited
				}
			}
		}
	}
	if v, ok := body["notes"].(string); ok {
		fields.Notes = &v
	}
	// due_date: 三态(缺/显式 null/值)
	if rm, ok := probe["due_date"]; ok {
		if isJSONNull(rm) {
			fields.ClearDueDate = true
		} else if v, ok := body["due_date"].(string); ok {
			fields.DueDate = &v
		}
	}
	if v, ok := body["status"].(string); ok && domain.IsValidTaskStatus(v) {
		s := domain.TaskStatus(v)
		fields.Status = &s
	}
	if rm, ok := probe["parent_id"]; ok {
		if isJSONNull(rm) {
			fields.ClearParentID = true
		} else if v, ok := body["parent_id"].(float64); ok {
			pid := int64(v)
			fields.ParentID = &pid
		}
	}
	if rm, ok := probe["scheduled_start"]; ok {
		if isJSONNull(rm) {
			fields.ClearScheduledStart = true
		} else if v, ok := body["scheduled_start"].(float64); ok {
			s := int64(v)
			fields.ScheduledStart = &s
		}
	}
	if rm, ok := probe["scheduled_end"]; ok {
		if isJSONNull(rm) {
			fields.ClearScheduledEnd = true
		} else if v, ok := body["scheduled_end"].(float64); ok {
			s := int64(v)
			fields.ScheduledEnd = &s
		}
	}
	if v, ok := body["pinned"].(bool); ok {
		fields.Pinned = &v
	}
	if v, ok := body["order_index"].(float64); ok {
		o := int64(v)
		fields.OrderIndex = &o
	}
	if v, ok := body["estimated_minutes"].(float64); ok {
		e := int64(v)
		fields.EstimatedMinutes = &e
	}

	if err := a.SQLite.AITasks().UpdateTask(id, fields); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "update failed"})
		return
	}
	task, subs, _ := a.SQLite.AITasks().GetTask(id)
	c.JSON(http.StatusOK, toTaskDTO(task, subs))
}

// TaskDelete DELETE /api/tasks/:id
func TaskDelete(c *gin.Context) {
	a := appFromCtx(c)
	id, err := parseInt64Param(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid id"})
		return
	}
	if err := a.SQLite.AITasks().DeleteTask(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "delete failed"})
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
	subID, err := a.SQLite.AITasks().CreateSubtask(id, body.Title, body.EstimatedMinutes, body.DueDate, body.OrderIndex)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "create failed"})
		return
	}
	_, subs, _ := a.SQLite.AITasks().GetTask(id)
	for _, s := range subs {
		if s.ID == subID {
			c.JSON(http.StatusOK, toSubtaskDTO(s))
			return
		}
	}
	c.JSON(http.StatusOK, gin.H{"id": subID})
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
	fields := storage.SubtaskUpdateFields{}
	if v, ok := body["title"].(string); ok {
		fields.Title = &v
	}
	if v, ok := body["status"].(string); ok {
		s := domain.SubtaskStatus(v)
		if !domain.IsValidSubtaskStatus(s) {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false, "error": "invalid status (allowed: active|done|archived)",
			})
			return
		}
		fields.Status = &s
	}
	if v, ok := body["estimated_minutes"].(float64); ok {
		e := int64(v)
		fields.EstimatedMinutes = &e
	}
	if v, ok := body["order_index"].(float64); ok {
		o := int64(v)
		fields.OrderIndex = &o
	}
	if err := a.SQLite.AITasks().UpdateSubtask(subID, fields); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "update failed"})
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
	if err := a.SQLite.AITasks().DeleteSubtask(subID); err != nil {
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
