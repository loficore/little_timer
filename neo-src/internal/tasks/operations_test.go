package tasks

import (
	"errors"
	"path/filepath"
	"testing"

	"little-timer/internal/domain"
	"little-timer/internal/storage"
)

// newStore 建一个真实 SQLite 的 AITasksCrud,作为 Operations 的 Store。
func newStore(t *testing.T) *storage.AITasksCrud {
	t.Helper()
	sqlite := storage.NewSqliteManager().Init(filepath.Join(t.TempDir(), "t.db"))
	if err := sqlite.Open(); err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := sqlite.Migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	t.Cleanup(func() { _ = sqlite.Close() })
	return sqlite.AITasks()
}

// mustCreate 建一条 task 并返回 id。
func mustCreate(t *testing.T, ops *Operations, title string) int64 {
	t.Helper()
	id, err := ops.Create(CreateInput{Title: title, Source: "manual"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	return id
}

func getRow(t *testing.T, ops *Operations, id int64) *domain.TaskRow {
	t.Helper()
	row, _, err := ops.Get(id)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	return row
}

// TestUpdate_TitleChangeSetsUserEdited：title 真正变化时翻 user_edited_title。
func TestUpdate_TitleChangeSetsUserEdited(t *testing.T) {
	ops := New(newStore(t))
	id := mustCreate(t, ops, "old")

	title := "new"
	if _, err := ops.Update(id, UpdatePatch{Title: &title}, getRow(t, ops, id).Version); err != nil {
		t.Fatalf("update: %v", err)
	}
	if !getRow(t, ops, id).UserEditedTitle {
		t.Error("title 变化后 user_edited_title 应为 true")
	}
}

// TestUpdate_SameTitleKeepsFlag：同值 PATCH(自动保存 round-trip)不能粘死 flag。
func TestUpdate_SameTitleKeepsFlag(t *testing.T) {
	ops := New(newStore(t))
	id := mustCreate(t, ops, "same")

	// 手动任务创建即 user_edited_title=true;先把它还原成 false 以验证
	// "同值不置位" 而不是被初始值掩盖。
	row := getRow(t, ops, id)
	if !row.UserEditedTitle {
		t.Fatalf("手动创建应为 user_edited_title=true(前置)")
	}
	edited := false
	if _, err := ops.Update(id, UpdatePatch{Title: &row.Title, UserEditedTitle: &edited}, row.Version); err != nil {
		t.Fatalf("update: %v", err)
	}
	title := "same"
	if _, err := ops.Update(id, UpdatePatch{Title: &title}, getRow(t, ops, id).Version); err != nil {
		t.Fatalf("same-title update: %v", err)
	}
	if getRow(t, ops, id).UserEditedTitle {
		t.Error("同值 PATCH 不应把 user_edited_title 置 true")
	}
}

// TestUpdate_DueDateTriState：缺字段不动 / 显式 null 清空 / 有值更新。
func TestUpdate_DueDateTriState(t *testing.T) {
	ops := New(newStore(t))
	id := mustCreate(t, ops, "t")

	// 有值
	due := "2026-01-02"
	if _, err := ops.Update(id, UpdatePatch{DueDate: &due}, getRow(t, ops, id).Version); err != nil {
		t.Fatalf("set due: %v", err)
	}
	if got := getRow(t, ops, id).DueDate; got == nil || *got != due {
		t.Fatalf("due_date=%v, want %q", got, due)
	}

	// 缺字段:不动
	notes := "n"
	if _, err := ops.Update(id, UpdatePatch{Notes: &notes}, getRow(t, ops, id).Version); err != nil {
		t.Fatalf("notes update: %v", err)
	}
	if got := getRow(t, ops, id).DueDate; got == nil || *got != due {
		t.Fatalf("缺字段不应改 due_date, got %v", got)
	}

	// 显式 null:清空
	if _, err := ops.Update(id, UpdatePatch{ClearDueDate: true}, getRow(t, ops, id).Version); err != nil {
		t.Fatalf("clear due: %v", err)
	}
	if got := getRow(t, ops, id).DueDate; got != nil {
		t.Fatalf("显式 null 应清空 due_date, got %v", *got)
	}
}

// TestUpdate_VersionConflict：stale version → ErrVersionConflict。
func TestUpdate_VersionConflict(t *testing.T) {
	ops := New(newStore(t))
	id := mustCreate(t, ops, "t")

	title := "x"
	_, err := ops.Update(id, UpdatePatch{Title: &title}, 99)
	if !errors.Is(err, ErrVersionConflict) {
		t.Fatalf("err=%v, want ErrVersionConflict", err)
	}
}

// TestDelete_VersionConflict：stale version 删除 → ErrVersionConflict。
func TestDelete_VersionConflict(t *testing.T) {
	ops := New(newStore(t))
	id := mustCreate(t, ops, "t")

	if err := ops.Delete(id, 99); !errors.Is(err, ErrVersionConflict) {
		t.Fatalf("err=%v, want ErrVersionConflict", err)
	}
	if err := ops.Delete(id, getRow(t, ops, id).Version); err != nil {
		t.Fatalf("delete: %v", err)
	}
}

// TestCreateSubtask_ReturnsRow：建子任务后直接返回完整行(含 version)。
func TestCreateSubtask_ReturnsRow(t *testing.T) {
	ops := New(newStore(t))
	taskID := mustCreate(t, ops, "t")

	sub, err := ops.CreateSubtask(taskID, SubtaskInput{Title: "s", EstimatedMinutes: 15})
	if err != nil {
		t.Fatalf("create subtask: %v", err)
	}
	if sub.ID == 0 || sub.TaskID != taskID || sub.Title != "s" || sub.EstimatedMinutes != 15 {
		t.Fatalf("subtask=%+v, want full row", sub)
	}
	if sub.Status != domain.SubtaskStatusActive {
		t.Errorf("status=%q, want active", sub.Status)
	}
}

// TestUpdateSubtask_InvalidStatus：非法 status → typed 400 语义错误。
func TestUpdateSubtask_InvalidStatus(t *testing.T) {
	ops := New(newStore(t))
	taskID := mustCreate(t, ops, "t")
	sub, err := ops.CreateSubtask(taskID, SubtaskInput{Title: "s"})
	if err != nil {
		t.Fatalf("create subtask: %v", err)
	}
	bad := domain.SubtaskStatus("pending")
	err = ops.UpdateSubtask(sub.ID, SubtaskPatch{Status: &bad}, sub.Version)
	if !errors.Is(err, ErrInvalidSubtaskStatus) {
		t.Fatalf("err=%v, want ErrInvalidSubtaskStatus", err)
	}
}

// TestGet_NotFound：不存在的 id → ErrNotFound。
func TestGet_NotFound(t *testing.T) {
	ops := New(newStore(t))
	if _, _, err := ops.Get(4242); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err=%v, want ErrNotFound", err)
	}
}
