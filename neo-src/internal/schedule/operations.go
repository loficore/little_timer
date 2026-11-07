package schedule

import (
	"database/sql"
	"errors"

	"little-timer/internal/domain"
	"little-timer/internal/scheduler"
	"little-timer/internal/storage"
)

// Error 是调度用例层的类型化错误。
type Error struct {
	Code    string
	Message string
}

func (e *Error) Error() string { return e.Message }

// ErrVersionConflict 表示某个 placement 的乐观锁命中 0 行;整个事务必须回滚。
var ErrVersionConflict = &Error{Code: "version_conflict", Message: "version conflict"}

// Store 是本包依赖的持久化能力集合。storage 满足它。
type Store interface {
	DB() *sql.DB
	// ListTasks 列出顶层任务(parentID = nil 时)。
	ListTasks(status string, parentID *int64, limit, offset int) ([]domain.TaskRow, error)
	GetTaskVersion(id int64) (int64, error)
	UpdateTaskTx(tx *sql.Tx, id int64, fields storage.TaskUpdateFields, version int64) error
}

// HabitSlotSource 提供某天的习惯时段。habit_schedules 表未落地前,默认
// 实现恒返回空切片(缺口跟踪见 GitHub issue #40)。
type HabitSlotSource interface {
	SlotsForDate(date string) ([]scheduler.HabitSlot, error)
}

// EmptyHabitSlotSource 是 HabitSlotSource 的默认空实现。
type EmptyHabitSlotSource struct{}

// SlotsForDate 恒返回空切片。
func (EmptyHabitSlotSource) SlotsForDate(string) ([]scheduler.HabitSlot, error) {
	return []scheduler.HabitSlot{}, nil
}

// Placement 是客户端确认要落库的一个摆位。
type Placement struct {
	TaskID         int64
	ScheduledStart int64
	ScheduledEnd   int64
	Version        int64
}

// sqliteStore 把 storage.SqliteManager 适配成 Store。
type sqliteStore struct {
	sqlite *storage.SqliteManager
}

func (s sqliteStore) DB() *sql.DB { return s.sqlite.DB() }

func (s sqliteStore) ListTasks(status string, parentID *int64, limit, offset int) ([]domain.TaskRow, error) {
	return s.sqlite.AITasks().ListTasks(status, parentID, limit, offset)
}

func (s sqliteStore) GetTaskVersion(id int64) (int64, error) {
	return s.sqlite.AITasks().GetTaskVersion(id)
}

func (s sqliteStore) UpdateTaskTx(tx *sql.Tx, id int64, fields storage.TaskUpdateFields, version int64) error {
	return s.sqlite.AITasks().UpdateTaskTx(tx, id, fields, version)
}

// NewForSqlite 用真实 SqliteManager 构造 Operations(生产接线入口)。
// 放在本包内,避免 adapter 层 import internal/storage。
func NewForSqlite(sqlite *storage.SqliteManager, slots HabitSlotSource) *Operations {
	return New(sqliteStore{sqlite: sqlite}, slots)
}

// Operations 是 schedule preview / apply 的用例层。
type Operations struct {
	store Store
	slots HabitSlotSource
}

// New 构造 Operations。slots 为 nil 时用空实现。
func New(store Store, slots HabitSlotSource) *Operations {
	if slots == nil {
		slots = EmptyHabitSlotSource{}
	}
	return &Operations{store: store, slots: slots}
}

// Preview 加载候选任务与当天习惯时段,调纯函数 scheduler.Schedule。
//
// 不落库。nil 切片在此归一化为空切片,避免 JSON 序列化成 null
// (前端 plan.Warnings.length 会抛错)。
func (o *Operations) Preview(date string) (scheduler.Plan, error) {
	candidates, err := o.loadCandidates(date)
	if err != nil {
		return scheduler.Plan{}, err
	}
	habits, err := o.slots.SlotsForDate(date)
	if err != nil {
		return scheduler.Plan{}, err
	}
	plan := scheduler.Schedule(candidates, habits, date, scheduler.Options{})
	if plan.Placements == nil {
		plan.Placements = []scheduler.Placement{}
	}
	if plan.Unscheduled == nil {
		plan.Unscheduled = []int64{}
	}
	if plan.Warnings == nil {
		plan.Warnings = []string{}
	}
	return plan, nil
}

// ApplyPlan 把 preview 得到的 plan 落库:事务内逐条写
// scheduled_start/scheduled_end,version 在 Begin 之前预取。
//
// 任一冲突 → 整事务回滚 → ErrVersionConflict。
func (o *Operations) ApplyPlan(plan scheduler.Plan) error {
	if len(plan.Placements) == 0 {
		return nil
	}
	versions := make(map[int64]int64, len(plan.Placements))
	for _, p := range plan.Placements {
		v, err := o.store.GetTaskVersion(p.TaskID)
		if err != nil {
			return err
		}
		versions[p.TaskID] = v
	}
	placements := make([]Placement, 0, len(plan.Placements))
	for _, p := range plan.Placements {
		placements = append(placements, Placement{
			TaskID:         p.TaskID,
			ScheduledStart: p.ScheduledStart,
			ScheduledEnd:   p.ScheduledEnd,
			Version:        versions[p.TaskID],
		})
	}
	return o.Apply(placements)
}

// Apply 在单个事务里逐条写 scheduled_start/end;任一冲突 → 全回滚。
//
// version 由调用方提供(客户端所见版本),不在 server 端重读。
func (o *Operations) Apply(placements []Placement) error {
	if len(placements) == 0 {
		return nil
	}
	db := o.store.DB()
	if db == nil {
		return errors.New("schedule: database not connected")
	}
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

	for _, p := range placements {
		s, e := p.ScheduledStart, p.ScheduledEnd
		fields := storage.TaskUpdateFields{
			ScheduledStart: &s,
			ScheduledEnd:   &e,
		}
		if err := o.store.UpdateTaskTx(tx, p.TaskID, fields, p.Version); err != nil {
			if errors.Is(err, storage.ErrVersionConflict) {
				return ErrVersionConflict
			}
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	committed = true
	return nil
}

// loadCandidates 把 tasks 表中所有非 archived/rejected 的顶层 task 转成
// scheduler.Task。date 参数当前 v1 暂未使用(全局候选),留作将来"按
// due_date 过滤当天要排的 task"扩展点。
func (o *Operations) loadCandidates(date string) ([]scheduler.Task, error) {
	_ = date
	rows, err := o.store.ListTasks("", nil, 1000, 0)
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
