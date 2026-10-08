package storage

import (
	"errors"
	"sync"
	"testing"

	"little-timer/internal/domain"
)

// TestBumpTaskVersionTx_HappyPath 验证 CAS 自增 version;错误版本 → ErrVersionConflict。
func TestBumpTaskVersionTx_HappyPath(t *testing.T) {
	m := openTempSqlite(t)
	c := m.AITasks()
	taskID, _ := c.CreateTask("t", "manual")

	tx, err := c.db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()

	if err := c.BumpTaskVersionTx(tx, taskID, 0); err != nil {
		t.Fatalf("BumpTaskVersionTx(0): %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}

	if v, _ := c.GetTaskVersion(taskID); v != 1 {
		t.Errorf("version=%d, want 1", v)
	}

	tx2, _ := c.db.Begin()
	defer tx2.Rollback()
	if err := c.BumpTaskVersionTx(tx2, taskID, 999); err == nil {
		t.Error("错版本应 ErrVersionConflict")
	} else if !errors.Is(err, ErrVersionConflict) {
		t.Errorf("err=%v, want ErrVersionConflict", err)
	}
}

// TestDeleteSubtaskTx_OnlyChild 验证只能删子任务(parent_id 非空)。
func TestDeleteSubtaskTx_OnlyChild(t *testing.T) {
	m := openTempSqlite(t)
	c := m.AITasks()
	parentID, _ := c.CreateTask("p", "manual")
	subID, _ := c.CreateSubtask(parentID, "child", 15, nil, 0)

	tx, _ := c.db.Begin()
	if err := c.DeleteSubtaskTx(tx, subID); err != nil {
		t.Fatal(err)
	}
	_ = tx.Commit()

	if _, subs, _ := c.GetTask(parentID); len(subs) != 0 {
		t.Errorf("删后应无子任务, got %d", len(subs))
	}
	// 顶层 task id 试图 DeleteSubtaskTx:不应被删(无 parent_id)。
	tx2, _ := c.db.Begin()
	if err := c.DeleteSubtaskTx(tx2, parentID); err != nil {
		t.Fatal(err)
	}
	_ = tx2.Commit()
	if task, _, err := c.GetTask(parentID); err != nil || task == nil {
		t.Errorf("顶层 task 不应被 DeleteSubtaskTx 删: %v", err)
	}
}

// TestAppendSubtasksTx_OrderIndex 接在最大值之后。
func TestAppendSubtasksTx_OrderIndex(t *testing.T) {
	m := openTempSqlite(t)
	c := m.AITasks()
	parentID, _ := c.CreateTask("p", "manual")
	_, _ = c.CreateSubtask(parentID, "a", 10, nil, 0)
	_, _ = c.CreateSubtask(parentID, "b", 10, nil, 1)

	tx, _ := c.db.Begin()
	err := c.AppendSubtasksTx(tx, parentID, []SubtaskInput{
		{Title: "c", EstimatedMinutes: 10},
		{Title: "d", EstimatedMinutes: 10},
	})
	if err != nil {
		t.Fatal(err)
	}
	_ = tx.Commit()

	_, subs, _ := c.GetTask(parentID)
	if len(subs) != 4 {
		t.Fatalf("subs=%d, want 4", len(subs))
	}
	if subs[2].Title != "c" || subs[3].Title != "d" {
		t.Errorf("顺序=%s,%s; want c,d", subs[2].Title, subs[3].Title)
	}
}

// TestGetSubtaskStatusTx 验证返回 status + version;顶层 task 返错误。
func TestGetSubtaskStatusTx(t *testing.T) {
	m := openTempSqlite(t)
	c := m.AITasks()
	parentID, _ := c.CreateTask("p", "manual")
	subID, _ := c.CreateSubtask(parentID, "s", 10, nil, 0)

	tx, _ := c.db.Begin()
	defer tx.Rollback()
	status, ver, err := c.GetSubtaskStatusTx(tx, subID)
	if err != nil {
		t.Fatal(err)
	}
	if status != domain.SubtaskStatusActive || ver != 0 {
		t.Errorf("got=(%s,%d), want (active,0)", status, ver)
	}
	if _, _, err := c.GetSubtaskStatusTx(tx, parentID); err == nil {
		t.Error("顶层 task 应返错误")
	}
}

// TestAdoption_Idempotency_Concurrent 铁证:100 并发同 (ai_job_id, change_id)
// → adoptions 只 1 行(UNIQUE 索引兜底)。
//
// 临时删 idx_adoptions_idempotent 此测试会 fail。
func TestAdoption_Idempotency_Concurrent(t *testing.T) {
	m := openTempSqlite(t)
	ai := m.AITasks()
	ac := m.Adoptions()

	taskID, _ := ai.CreateTask("t", "manual")
	jobID, _ := ai.EnqueueJob(taskID, "openai_compat", "m", `{}`, domain.AIJobModeReview)

	const N = 100
	var wg sync.WaitGroup
	wg.Add(N)
	for i := 0; i < N; i++ {
		go func() {
			defer wg.Done()
			_, _ = ac.RecordAdoption(domain.Adoption{
				TaskID: taskID, AIJobID: jobID, ChangeID: "concur",
				VersionBefore: 0, VersionAfter: 1,
			})
		}()
	}
	wg.Wait()

	var n int
	m.DB().QueryRow(`SELECT COUNT(*) FROM adoptions WHERE ai_job_id=? AND change_id=?`, jobID, "concur").Scan(&n)
	if n != 1 {
		t.Errorf("adoptions=%d, want 1(UNIQUE 索引兜底)", n)
	}
}
