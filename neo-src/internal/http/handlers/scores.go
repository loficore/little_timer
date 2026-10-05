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
// 乐观锁:优先 middleware.GetVersion(c)(来自 `If-Match` header,或
// ConflictGuard 从 body 解析的 version);仅当它为 0 时回退 body.version。
// UpdateTask 带该期望版本命中 0 行 → storage.ErrVersionConflict → 409 Conflict。
//
// 范围校验(Review Focus #3):priority_score ∈ [1,5],urgency_score ∈ [1,10],
// energy_required ∈ [1,3];越界不再落到 SQLite CHECK 变 500,而是 handler
// 直接返 400 scores_out_of_range。
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

// scoreRange 描述一个整数评分的合法闭区间 [min,max]。
type scoreRange struct {
	field string
	min   int
	max   int
}

// 与 tasks 表 CHECK 约束(migration v11)保持一致;handler 先校验,
// 避免越界值触发 SQLite constraint → 500。
var (
	priorityScoreRange  = scoreRange{"priority_score", 1, 5}
	urgencyScoreRange   = scoreRange{"urgency_score", 1, 10}
	energyRequiredRange = scoreRange{"energy_required", 1, 3}
)

// checkScoreRange 校验非 nil 的指针值是否落在 [min,max];越界时返回
// 可直接 JSON 的错误载荷(供 handler 返 400)。
func checkScoreRange(r scoreRange, v *int) (payload gin.H, bad bool) {
	if v == nil {
		return nil, false
	}
	if *v < r.min || *v > r.max {
		return gin.H{
			"error": "score out of range",
			"code":  "scores_out_of_range",
			"field": r.field,
			"value": *v,
			"range": [2]int{r.min, r.max},
		}, true
	}
	return nil, false
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
	// I4 fix:header 优先(ConflictGuard 已把它存进 ctx);只有 header 为 0
	// 时才回退到 body.version。这与 TaskUpdate / ScheduleApply 的契约一致。
	version := middleware.GetVersion(c)
	if version == 0 {
		version = body.Version
	}
	// I5 fix:范围校验,越界直接 400,不让 SQLite CHECK 把它转成 500。
	for _, r := range []scoreRange{priorityScoreRange, urgencyScoreRange, energyRequiredRange} {
		var p *int
		switch r.field {
		case "priority_score":
			p = body.PriorityScore
		case "urgency_score":
			p = body.UrgencyScore
		case "energy_required":
			p = body.EnergyRequired
		}
		if payload, bad := checkScoreRange(r, p); bad {
			c.JSON(400, payload)
			return
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
	if err := a.SQLite.AITasks().UpdateTask(id, fields, version); err != nil {
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
