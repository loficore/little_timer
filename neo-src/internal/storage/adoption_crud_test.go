package storage

import (
	"errors"
	"testing"

	"little-timer/internal/domain"
)

// TestRecordAdoption_AndFind 验证 RecordAdoption / FindAdoption:
//   - 写入返回非零 id;
//   - 同 (ai_job_id, change_id) 命中返回完整字段;
//   - 不同 changeID 命中 (nil, nil)。
func TestRecordAdoption_AndFind(t *testing.T) {
	m := openTempSqlite(t)
	ai := m.AITasks()
	ac := m.Adoptions()

	taskID, err := ai.CreateTask("t", "manual")
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	jobID, err := ai.EnqueueJob(taskID, "openai_compat", "m", `{}`, domain.AIJobModeReview)
	if err != nil {
		t.Fatalf("EnqueueJob: %v", err)
	}

	id, err := ac.RecordAdoption(domain.Adoption{
		TaskID:        taskID,
		AIJobID:       jobID,
		ChangeID:      "chg-1",
		VersionBefore: 0,
		VersionAfter:  1,
	})
	if err != nil || id == 0 {
		t.Fatalf("RecordAdoption id=%d err=%v", id, err)
	}

	got, err := ac.FindAdoption(jobID, "chg-1")
	if err != nil {
		t.Fatalf("FindAdoption hit: %v", err)
	}
	if got == nil {
		t.Fatalf("FindAdoption hit → nil")
	}
	if got.TaskID != taskID || got.AIJobID != jobID || got.ChangeID != "chg-1" ||
		got.VersionBefore != 0 || got.VersionAfter != 1 {
		t.Errorf("got=%+v, want task=%d job=%d change=chg-1 ver=0→1", got, taskID, jobID)
	}

	miss, err := ac.FindAdoption(jobID, "nope")
	if err != nil {
		t.Fatalf("FindAdoption miss err: %v", err)
	}
	if miss != nil {
		t.Errorf("未命中 miss=%+v, want nil", miss)
	}
}

// TestRecordAdoption_Duplicate_ReturnsErrAdoptionDuplicate 验证
// idx_adoptions_idempotent UNIQUE 兜底:同 (ai_job_id, change_id) 重复采纳
// → ErrAdoptionDuplicate(handler 据此按幂等返 200,而非 hard 500)。
func TestRecordAdoption_Duplicate_ReturnsErrAdoptionDuplicate(t *testing.T) {
	m := openTempSqlite(t)
	ai := m.AITasks()
	ac := m.Adoptions()

	taskID, _ := ai.CreateTask("t", "manual")
	jobID, _ := ai.EnqueueJob(taskID, "openai_compat", "m", `{}`, domain.AIJobModeReview)
	a := domain.Adoption{TaskID: taskID, AIJobID: jobID, ChangeID: "dup", VersionBefore: 0, VersionAfter: 1}

	if _, err := ac.RecordAdoption(a); err != nil {
		t.Fatalf("首次 RecordAdoption: %v", err)
	}
	_, err := ac.RecordAdoption(a)
	if !errors.Is(err, ErrAdoptionDuplicate) {
		t.Fatalf("重复 RecordAdoption err=%v, want ErrAdoptionDuplicate", err)
	}
}
