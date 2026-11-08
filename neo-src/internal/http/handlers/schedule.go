// Package handlers —— /api/schedule/* 端点。
//
// 本文件只做 transport 绑定与错误映射。候选任务加载、事务边界、版本闸门
// 全在 internal/schedule(issue #66);scheduler.Schedule 仍是纯函数。
//
// habit_schedules 表未落地,习惯时段恒为空切片;缺口跟踪见 GitHub issue #40。
package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"little-timer/internal/http/app"
	"little-timer/internal/schedule"
)

// scheduleRunRequest 是 POST /api/schedule/run 的入参。mode 缺省回落 preview。
type scheduleRunRequest struct {
	Date string `json:"date"`
	Mode string `json:"mode"`
}

// scheduleApplyRequest 是 POST /api/schedule/apply 的入参。placements 是
// 客户端确认要落库的 placement 集合,version 是客户端读到的当前 server 版本
// (与 /api/tasks GET 返回的 version 字段对齐,Task 1 v11)。
type scheduleApplyRequest struct {
	Placements []schedule.Placement `json:"placements"`
}

// scheduleOpsFor 构造调度用例层。
func scheduleOpsFor(a *app.App) *schedule.Operations {
	return schedule.NewForSqlite(a.SQLite, nil)
}

// ScheduleRun POST /api/schedule/run
//
// body: {"date": "YYYY-MM-DD", "mode": "preview"|"apply"}
//
//   - preview → 返回 scheduler.Plan,DB 不动;
//   - apply   → 在事务里逐条写 scheduled_start/end;任一乐观锁冲突 → 409。
func ScheduleRun(c *gin.Context) {
	a := appFromCtx(c)
	var body scheduleRunRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
		return
	}
	if body.Date == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "date required"})
		return
	}
	if body.Mode == "" {
		body.Mode = "preview"
	}
	if body.Mode != "preview" && body.Mode != "apply" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "mode must be preview or apply"})
		return
	}

	ops := scheduleOpsFor(a)
	plan, err := ops.Preview(body.Date)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if body.Mode == "apply" {
		if err := ops.ApplyPlan(plan); err != nil {
			respondScheduleError(c, err)
			return
		}
	}
	c.JSON(http.StatusOK, plan)
}

// ScheduleApply POST /api/schedule/apply
//
// body: {"placements": [{task_id, scheduled_start, scheduled_end, version}]}
//
// 事务逐条 UpdateTaskTx(version 乐观锁);任一冲突 → 整事务回滚 → 409。
// version 由客户端提供(handler 不在 server 端重读,以匹配客户端所见)。
func ScheduleApply(c *gin.Context) {
	a := appFromCtx(c)
	var body scheduleApplyRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
		return
	}
	if len(body.Placements) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "placements required"})
		return
	}
	if err := scheduleOpsFor(a).Apply(body.Placements); err != nil {
		respondScheduleError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"applied": len(body.Placements)})
}

// respondScheduleError 是 schedule.Error → HTTP 状态码的唯一映射点。
func respondScheduleError(c *gin.Context, err error) {
	var de *schedule.Error
	if errors.As(err, &de) && errors.Is(err, schedule.ErrVersionConflict) {
		respondVersionConflict(c)
		return
	}
	c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
}
