package schedule

import (
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	"little-timer/internal/domain"
	"little-timer/internal/scheduler"
	"little-timer/internal/storage"
)

// storeAdapter 把 SqliteManager 适配成 Store(避免测试用 mock 伪造事务语义)。
type storeAdapter struct {
	sqlite *storage.SqliteManager
}

func (s storeAdapter) DB() *sql.DB { return s.sqlite.DB() }

func (s storeAdapter) ListTasks(status string, parentID *int64, limit, offset int) ([]domain.TaskRow, error) {
	return s.sqlite.AITasks().ListTasks(status, parentID, limit, offset)
}

func (s storeAdapter) GetTaskVersion(id int64) (int64, error) {
	return s.sqlite.AITasks().GetTaskVersion(id)
}

func (s storeAdapter) UpdateTaskTx(tx *sql.Tx, id int64, fields storage.TaskUpdateFields, version int64) error {
	return s.sqlite.AITasks().UpdateTaskTx(tx, id, fields, version)
}

func newOps(t *testing.T) (*Operations, storeAdapter) {
	t.Helper()
	sqlite := storage.NewSqliteManager().Init(filepath.Join(t.TempDir(), "s.db"))
	if err := sqlite.Open(); err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := sqlite.Migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	t.Cleanup(func() { _ = sqlite.Close() })
	store := storeAdapter{sqlite: sqlite}
	return New(store, nil), store
}

// createTask 建一条带评分的顶层 task(经 storage 直接写,模拟前端数据)。
func createTask(t *testing.T, store storeAdapter, title string, estMin int) int64 {
	t.Helper()
	id, err := store.sqlite.AITasks().CreateTask(title, "manual")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	title2, notes, status := title, "", domain.TaskStatusActive
	est := int64(estMin)
	p, u, e := 5, 3, 2
	tag := "general"
	if err := store.sqlite.AITasks().UpdateTask(id, storage.TaskUpdateFields{
		Title: &title2, Notes: &notes, Status: &status, EstimatedMinutes: &est,
		PriorityScore: &p, UrgencyScore: &u, EnergyRequired: &e, ContextTag: &tag,
	}, 0); err != nil {
		t.Fatalf("update: %v", err)
	}
	return id
}

// TestPreview_LoadsCandidatesAndSlots：preview 返回包含候选 task 的 plan,
// 且不写库。
func TestPreview_LoadsCandidatesAndSlots(t *testing.T) {
	ops, store := newOps(t)
	id := createTask(t, store, "写周报", 30)

	plan, err := ops.Preview("2026-11-08")
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if len(plan.Placements) != 1 || plan.Placements[0].TaskID != id {
		t.Fatalf("placements=%+v, want exactly task %d", plan.Placements, id)
	}
	// 空切片而非 nil(JSON 契约)。
	if plan.Unscheduled == nil || plan.Warnings == nil {
		t.Errorf("nil 切片应归一化为空切片, got %+v", plan)
	}
	// 不写库。
	var start sql.NullInt64
	if err := store.DB().QueryRow(`SELECT scheduled_start FROM tasks WHERE id=?;`, id).Scan(&start); err != nil {
		t.Fatalf("scan: %v", err)
	}
	if start.Valid {
		t.Errorf("preview 写入了 scheduled_start=%d", start.Int64)
	}
}

// TestApply_AllOrNothingOnConflict：第二个 placement 冲突 → 第一个回滚。
func TestApply_AllOrNothingOnConflict(t *testing.T) {
	ops, store := newOps(t)
	taskA := createTask(t, store, "A", 30)
	taskB := createTask(t, store, "B", 30)

	verA, err := store.GetTaskVersion(taskA)
	if err != nil {
		t.Fatalf("version A: %v", err)
	}
	// 把 B 的 version 顶一档,使 stale version=0 必冲突。
	bt := "B2"
	if err := store.sqlite.AITasks().UpdateTask(taskB, storage.TaskUpdateFields{Title: &bt}, 1); err != nil {
		t.Fatalf("bump B: %v", err)
	}

	err = ops.Apply([]Placement{
		{TaskID: taskA, ScheduledStart: 1000, ScheduledEnd: 2000, Version: verA},
		{TaskID: taskB, ScheduledStart: 2000, ScheduledEnd: 3000, Version: 0},
	})
	if !errors.Is(err, ErrVersionConflict) {
		t.Fatalf("err=%v, want ErrVersionConflict", err)
	}

	for _, id := range []int64{taskA, taskB} {
		var start sql.NullInt64
		if err := store.DB().QueryRow(`SELECT scheduled_start FROM tasks WHERE id=?;`, id).Scan(&start); err != nil {
			t.Fatalf("scan: %v", err)
		}
		if start.Valid {
			t.Errorf("task %d 未回滚, scheduled_start=%d", id, start.Int64)
		}
	}
}

// TestApply_EmptyPlacements：空集合是 no-op。
func TestApply_EmptyPlacements(t *testing.T) {
	ops, _ := newOps(t)
	if err := ops.Apply(nil); err != nil {
		t.Fatalf("err=%v, want nil", err)
	}
	if err := ops.Apply([]Placement{}); err != nil {
		t.Fatalf("err=%v, want nil", err)
	}
}

// TestApply_Success：全成功时写入并 bump 版本。
func TestApply_Success(t *testing.T) {
	ops, store := newOps(t)
	id := createTask(t, store, "A", 30)
	ver, _ := store.GetTaskVersion(id)

	if err := ops.Apply([]Placement{{TaskID: id, ScheduledStart: 1000, ScheduledEnd: 2000, Version: ver}}); err != nil {
		t.Fatalf("apply: %v", err)
	}
	var start int64
	if err := store.DB().QueryRow(`SELECT scheduled_start FROM tasks WHERE id=?;`, id).Scan(&start); err != nil {
		t.Fatalf("scan: %v", err)
	}
	if start != 1000 {
		t.Errorf("scheduled_start=%d, want 1000", start)
	}
}

// TestApplyPlan_PrefetchesVersions：ApplyPlan 自己预取当前版本,不依赖客户端。
func TestApplyPlan_PrefetchesVersions(t *testing.T) {
	ops, store := newOps(t)
	id := createTask(t, store, "A", 30)

	plan := scheduler.Plan{
		Date:       "2026-11-08",
		Placements: []scheduler.Placement{{TaskID: id, ScheduledStart: 500, ScheduledEnd: 900}},
	}
	if err := ops.ApplyPlan(plan); err != nil {
		t.Fatalf("apply plan: %v", err)
	}
	var start int64
	_ = store.DB().QueryRow(`SELECT scheduled_start FROM tasks WHERE id=?;`, id).Scan(&start)
	if start != 500 {
		t.Errorf("scheduled_start=%d, want 500", start)
	}
}
