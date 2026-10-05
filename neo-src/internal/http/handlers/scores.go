// Package handlers —— PATCH /api/tasks/:id/scores 端点。
//
// 指针语义:priority_score / urgency_score / energy_required / context_tag
// 全部 *T,nil 表示"不动"——与 TaskUpdate 的 JSON 探测写法不同,这里走
// ShouldBindJSON 的 omitempty 即可(distinguish absent vs null 在分数场景
// 下不必要,客户端不会主动 null 化评分)。
//
// context_tag 兜底:非枚举值(写作 / 编码 / 沟通 / 学习 / 杂事 / general)
// → 写入 "general" 并通过 X-LLM-Warning 响应头提示前端 / LLM 客户端。
//
// 乐观锁:由 middleware.GetVersion(c) 提供期望版本(若 body.version
// 缺/为 0 优先 body,否则 fallback 到 header)。UpdateTask 不带版本
// 命中 0 行 → storage.ErrVersionConflict → 409 Conflict。
package handlers

import (
	"errors"
	"strconv"

	"github.com/gin-gonic/gin"

	"little-timer/internal/http/middleware"
	"little-timer/internal/storage"
)

// validContextTags 是 5 维调度评分 context_tag 的白名单;其他值视为
// LLM 自由发挥的字符串,落到 general 以保证后续规则引擎判定可预测。
var validContextTags = map[string]bool{
	"写作": true, "编码": true, "沟通": true, "学习": true, "杂事": true, "general": true,
}

// TaskScoresUpdate PATCH /api/tasks/:id/scores —— 部分更新 5 维评分。
func TaskScoresUpdate(c *gin.Context) {
	a := appFromCtx(c)
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid id"})
		return
	}

	var body struct {
		Version        int64   `json:"version"`
		PriorityScore  *int    `json:"priority_score,omitempty"`
		UrgencyScore   *int    `json:"urgency_score,omitempty"`
		EnergyRequired *int    `json:"energy_required,omitempty"`
		ContextTag     *string `json:"context_tag,omitempty"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	if body.Version <= 0 {
		if v := middleware.GetVersion(c); v > 0 {
			body.Version = v
		}
	}
	fields := storage.TaskUpdateFields{
		PriorityScore:  body.PriorityScore,
		UrgencyScore:   body.UrgencyScore,
		EnergyRequired: body.EnergyRequired,
	}
	if body.ContextTag != nil {
		if !validContextTags[*body.ContextTag] {
			// LLM 输出非枚举值 → 兜底 general,同时通过响应头提示。
			general := "general"
			fields.ContextTag = &general
			c.Header("X-LLM-Warning", "context_tag fallback to general")
		} else {
			fields.ContextTag = body.ContextTag
		}
	}
	if err := a.SQLite.AITasks().UpdateTask(id, fields, body.Version); err != nil {
		if errors.Is(err, storage.ErrVersionConflict) {
			c.JSON(409, gin.H{"error": "version conflict", "code": "version_conflict"})
			return
		}
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	task, _, _ := a.SQLite.AITasks().GetTask(id)
	c.JSON(200, taskDTOFromRow(task))
}
