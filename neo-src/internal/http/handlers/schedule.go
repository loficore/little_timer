// Package handlers —— /api/schedule/* 端点。
//
// 设计边界(scheduler 设计 §3 + Task 6 决议):
//
//   - scheduler.Schedule 是纯函数规则引擎(Task 2),本文件只负责:
//     1. 从 tasks 表加载候选 task(以及将来从 habit_schedules 加载 habit slots);
//     2. preview → 直接回 Plan,不落库;
//     3. apply → 在单个事务里逐条 UpdateTask(version 乐观锁),
//     任一冲突 → 整个事务回滚(handler 返 409)。
//
//   - scheduled_start/scheduled_end 的写入走 storage.AITasks.UpdateTaskTx
//     (Task 4 的 updateWithVersion 中心函数),事务/非事务路径共享 SQL,
//     0 行受影响 → storage.ErrVersionConflict,handler 据此返 409。
//
//   - habit_schedules 表属于 Plan 3("习惯融合"),spec §7.2 明确 v1 不在范围;
//     loadHabitSlotsForDate 直接返回空切片,留 TODO(plan-3) 锚点。
package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"little-timer/internal/http/app"
	"little-timer/internal/scheduler"
	"little-timer/internal/storage"
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
	Placements []schedulePlacement `json:"placements"`
}

// schedulePlacement 是单个 placement 的传输形状(与 scheduler.Placement
// 解耦,避免把内部类型暴露给 HTTP 层)。
type schedulePlacement struct {
	TaskID         int64 `json:"task_id"`
	ScheduledStart int64 `json:"scheduled_start"`
	ScheduledEnd   int64 `json:"scheduled_end"`
	Version        int64 `json:"version"`
}

// ScheduleRun POST /api/schedule/run
//
// body: {"date": "YYYY-MM-DD", "mode": "preview"|"apply"}
//
//   - preview → 返回 scheduler.Plan,DB 不动;
//   - apply   → 在事务里逐条写 scheduled_start/end;任一乐观锁冲突 → 409。
//
// ConflictGuard 在 router 层强制 mutation 带 version(用于 audit 一致性,
// 本 handler 不直接用 GetVersion —— 实际的乐观锁 version 由 preview 调用
// 的 GetTaskVersion 或客户端传入的 placement.version 提供)。
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

	tasks, err := loadCandidateTasks(a, body.Date)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	habits := loadHabitSlotsForDate(a, body.Date)
	plan := scheduler.Schedule(tasks, habits, body.Date, scheduler.Options{})

	if body.Mode == "apply" {
		if err := applyPlanInTx(a, plan); err != nil {
			if errors.Is(err, storage.ErrVersionConflict) {
				c.JSON(http.StatusConflict, gin.H{
					"error": err.Error(),
					"code":  "version_conflict",
				})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
	db := a.SQLite.DB()
	tx, err := db.Begin()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	for _, p := range body.Placements {
		s, e := p.ScheduledStart, p.ScheduledEnd
		fields := storage.TaskUpdateFields{
			ScheduledStart: &s,
			ScheduledEnd:   &e,
		}
		if err := a.SQLite.AITasks().UpdateTaskTx(tx, p.TaskID, fields, p.Version); err != nil {
			if errors.Is(err, storage.ErrVersionConflict) {
				c.JSON(http.StatusConflict, gin.H{
					"error": err.Error(),
					"code":  "version_conflict",
				})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}
	if err := tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	committed = true
	c.JSON(http.StatusOK, gin.H{"applied": len(body.Placements)})
}

// loadCandidateTasks 把 tasks 表中所有非 archived/rejected 的顶层 task 转成
// scheduler.Task。date 参数当前 v1 暂未使用(全局候选),留作将来"按 due_date
// 过滤当天要排的 task"扩展点。
func loadCandidateTasks(a *app.App, date string) ([]scheduler.Task, error) {
	if a == nil || a.SQLite == nil {
		return nil, nil
	}
	rows, err := a.SQLite.AITasks().ListTasks("", nil, 1000, 0)
	if err != nil {
		return nil, err
	}
	out := make([]scheduler.Task, 0, len(rows))
	for i := range rows {
		r := rows[i]
		out = append(out, scheduler.Task{
			ID:             r.ID,
			Title:          r.Title,
			EstimatedMin:   int(r.EstimatedMinutes),
			Pinned:         r.Pinned,
			DueDate:        r.DueDate,
			ScheduledStart: r.ScheduledStart,
			ScheduledEnd:   r.ScheduledEnd,
			Scores: scheduler.Score{
				Priority: r.PriorityScore,
				Urgency:  r.UrgencyScore,
				Energy:   r.EnergyRequired,
				Context:  r.ContextTag,
			},
			BlockedBy: r.BlockedBy,
		})
	}
	return out, nil
}

// loadHabitSlotsForDate 返回某天的习惯时段。
//
// TODO(plan-3): habit_schedules 表尚未在 v11 落地(spec §7.2 明确"习惯融合"
// 属于 Plan 3,v1 不在范围)。这里返回空切片,调度引擎会跳过 habit 占位;
// Plan 3 实现时按 DayOfWeek 过滤 habits 后填进 HabitSlot。
func loadHabitSlotsForDate(a *app.App, date string) []scheduler.HabitSlot {
	return []scheduler.HabitSlot{}
}

// applyPlanInTx 把 plan 的 placements 写到 tasks 表的 scheduled_start/end,
// 单个事务内全部成功才提交;任一冲突 → 整事务回滚,返回
// storage.ErrVersionConflict(handler 转 409)。
//
// version 在 Begin 之前预取(GetTaskVersion via pool),避免在写事务持锁期间
// 再发起 pool 读——既符合 Task 4 "pool 读可接受" 的最低边界,也把锁路径
// 缩到最短。预取与 Begin 之间若有并发写,UpdateTaskTx 仍以 stale version
// 触发冲突,语义不变。
func applyPlanInTx(a *app.App, plan scheduler.Plan) error {
	if len(plan.Placements) == 0 {
		return nil
	}
	versions := make(map[int64]int64, len(plan.Placements))
	for _, p := range plan.Placements {
		v, err := a.SQLite.AITasks().GetTaskVersion(p.TaskID)
		if err != nil {
			return err
		}
		versions[p.TaskID] = v
	}

	db := a.SQLite.DB()
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	for _, p := range plan.Placements {
		s, e := p.ScheduledStart, p.ScheduledEnd
		fields := storage.TaskUpdateFields{
			ScheduledStart: &s,
			ScheduledEnd:   &e,
		}
		if err := a.SQLite.AITasks().UpdateTaskTx(tx, p.TaskID, fields, versions[p.TaskID]); err != nil {
			return err // 已 wrap ErrVersionConflict 或 ErrAITaskUpdateFailed
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	committed = true
	return nil
}
