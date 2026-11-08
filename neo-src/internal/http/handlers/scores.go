// Package handlers —— PATCH /api/tasks/:id/scores 端点。
//
// 本文件只做 transport 绑定与错误映射:范围校验、context_tag 兜底、
// 乐观锁全在 internal/tasks(issue #66)。
//
// 乐观锁:优先 middleware.GetVersion(c)(来自 `If-Match` header,或
// ConflictGuard 从 body 解析的 version);仅当它为 0 时回退 body.version。
//
// context_tag 兜底命中时,operation 返回 warning,这里转成 X-LLM-Warning
// 响应头提示前端 / LLM 客户端。
package handlers

import (
	"errors"
	"strconv"

	"github.com/gin-gonic/gin"

	"little-timer/internal/http/middleware"
	"little-timer/internal/tasks"
)

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
	// I4 fix:header 优先(ConflictGuard 已把它存进 ctx);只有 header 为 0
	// 时才回退到 body.version。这与 TaskUpdate / ScheduleApply 的契约一致。
	version := middleware.GetVersion(c)
	if version == 0 {
		version = body.Version
	}

	task, warning, err := taskOps(a).PatchScores(id, tasks.ScoresPatch{
		PriorityScore:  body.PriorityScore,
		UrgencyScore:   body.UrgencyScore,
		EnergyRequired: body.EnergyRequired,
		ContextTag:     body.ContextTag,
	}, version)
	if err != nil {
		var re *tasks.ScoreRangeError
		if errors.As(err, &re) {
			c.JSON(400, gin.H{
				"error": re.Error(),
				"code":  re.Code(),
				"field": re.Field,
				"value": re.Value,
				"range": [2]int{re.Min, re.Max},
			})
			return
		}
		if errors.Is(err, tasks.ErrVersionConflict) {
			respondVersionConflict(c, "", true, nil)
			return
		}
		// 非类型化错误:与重构前一致,只回 {"error": ...}。
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	if warning != "" {
		c.Header("X-LLM-Warning", warning)
	}
	c.JSON(200, taskDTOFromRow(&task))
}
