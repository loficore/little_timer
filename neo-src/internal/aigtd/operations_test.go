package aigtd

import (
	"errors"
	"testing"

	"little-timer/internal/domain"
	"little-timer/internal/storage"
)

// fakeTaskStore 是 TaskStore 的可注入实现,记录调用以便断言回滚/入队参数。
type fakeTaskStore struct {
	createdRaw  string
	taskID      int64
	createErr   error
	deleteCalls []deleteCall
	enqueueErr  error
	enqueued    []enqueueCall
	reparseRaw  *string
	reparseVer  int64
	reparseMode domain.AIJobMode
	inFlight    bool
	task        *domain.TaskRow
}

type deleteCall struct {
	id      int64
	version int64
}

type enqueueCall struct {
	taskID int64
	raw    string
	mode   domain.AIJobMode
}

func (f *fakeTaskStore) CreateTask(rawText, source string) (int64, error) {
	f.createdRaw = rawText
	if f.createErr != nil {
		return 0, f.createErr
	}
	return f.taskID, nil
}

func (f *fakeTaskStore) DeleteTask(id int64, version int64) error {
	f.deleteCalls = append(f.deleteCalls, deleteCall{id, version})
	return nil
}

func (f *fakeTaskStore) GetTask(id int64) (*domain.TaskRow, []domain.SubtaskRow, error) {
	if f.task == nil {
		return nil, nil, errors.New("not found")
	}
	return f.task, nil, nil
}

func (f *fakeTaskStore) HasInFlightJob(taskID int64) (bool, error) {
	return f.inFlight, nil
}

func (f *fakeTaskStore) EnqueueJob(taskID int64, provider, model, reqPayload string, mode domain.AIJobMode) (int64, error) {
	if f.enqueueErr != nil {
		return 0, f.enqueueErr
	}
	f.enqueued = append(f.enqueued, enqueueCall{taskID, reqPayload, mode})
	return 77, nil
}

func (f *fakeTaskStore) EnqueueReparse(taskID int64, rawUpdate *storage.TaskUpdateFields, version int64, provider, model string, mode domain.AIJobMode) (int64, error) {
	f.reparseVer = version
	f.reparseMode = mode
	if rawUpdate != nil && rawUpdate.RawText != nil {
		v := *rawUpdate.RawText
		f.reparseRaw = &v
	}
	if f.enqueueErr != nil {
		return 0, f.enqueueErr
	}
	return 88, nil
}

func testConfig() LLMConfig {
	return LLMConfig{Provider: "openai_compat", Model: "deepseek-chat"}
}

// TestCapture_EmptyRawText：空 raw_text → ErrEmpty(HTTP 侧 code aigtd.empty)。
func TestCapture_EmptyRawText(t *testing.T) {
	ops := NewOperations(&fakeTaskStore{taskID: 1}, testConfig)
	for _, in := range []string{"", "   ", "\n\t"} {
		if _, err := ops.Capture(in, "manual"); !errors.Is(err, ErrEmpty) {
			t.Errorf("raw=%q: err=%v, want ErrEmpty", in, err)
		}
	}
}

// TestCapture_TooLong：超过 2000 rune → ErrTooLong。
func TestCapture_TooLong(t *testing.T) {
	ops := NewOperations(&fakeTaskStore{taskID: 1}, testConfig)
	long := ""
	for i := 0; i < 2001; i++ {
		long += "字"
	}
	if _, err := ops.Capture(long, "manual"); !errors.Is(err, ErrTooLong) {
		t.Fatalf("err=%v, want ErrTooLong", err)
	}
}

// TestCapture_EnqueueFailureRollsBack：入队失败必须回滚刚建的 task(version=0)。
func TestCapture_EnqueueFailureRollsBack(t *testing.T) {
	f := &fakeTaskStore{taskID: 42, enqueueErr: errors.New("boom")}
	ops := NewOperations(f, testConfig)

	if _, err := ops.Capture("写周报", "manual"); err == nil {
		t.Fatal("入队失败应返回错误")
	}
	if len(f.deleteCalls) != 1 || f.deleteCalls[0].id != 42 || f.deleteCalls[0].version != 0 {
		t.Fatalf("deleteCalls=%+v, want one DeleteTask(42, 0)", f.deleteCalls)
	}
}

// TestCapture_DefaultSource：source 缺省 manual,入队 append + raw 原文。
func TestCapture_DefaultSource(t *testing.T) {
	f := &fakeTaskStore{taskID: 7}
	ops := NewOperations(f, testConfig)

	id, err := ops.Capture("  写周报  ", "")
	if err != nil {
		t.Fatalf("capture: %v", err)
	}
	if id != 7 {
		t.Errorf("task id=%d, want 7", id)
	}
	if f.createdRaw != "写周报" {
		t.Errorf("created raw=%q, want trimmed", f.createdRaw)
	}
	if len(f.enqueued) != 1 || f.enqueued[0].mode != domain.AIJobModeAppend || f.enqueued[0].raw != "写周报" {
		t.Fatalf("enqueued=%+v, want one append with raw", f.enqueued)
	}
}

// TestReparse_InFlightRejected：已有 in-flight job → ErrProcessing。
func TestReparse_InFlightRejected(t *testing.T) {
	f := &fakeTaskStore{task: &domain.TaskRow{ID: 5}, inFlight: true}
	ops := NewOperations(f, testConfig)

	if _, err := ops.Reparse(5, nil, "", 0); !errors.Is(err, ErrProcessing) {
		t.Fatalf("err=%v, want ErrProcessing", err)
	}
	if len(f.enqueued) != 0 || f.reparseVer != 0 {
		t.Errorf("被拒时不该入队, got %+v", f.enqueued)
	}
}

// TestReparse_ProcessingStatusRejected：task 仍处于 processing → ErrProcessing。
func TestReparse_ProcessingStatusRejected(t *testing.T) {
	f := &fakeTaskStore{task: &domain.TaskRow{ID: 5, AIStatus: domain.AIStatusProcessing}}
	ops := NewOperations(f, testConfig)

	if _, err := ops.Reparse(5, nil, "", 0); !errors.Is(err, ErrProcessing) {
		t.Fatalf("err=%v, want ErrProcessing", err)
	}
}

// TestReparse_InvalidMode：mode 非 append/replace → ErrBadRequest。
func TestReparse_InvalidMode(t *testing.T) {
	f := &fakeTaskStore{task: &domain.TaskRow{ID: 5}}
	ops := NewOperations(f, testConfig)

	if _, err := ops.Reparse(5, nil, "review", 0); !errors.Is(err, ErrBadRequest) {
		t.Fatalf("err=%v, want ErrBadRequest", err)
	}
}

// TestReparse_RawTextRewrite：给了 raw_text 时 EnqueueReparse 收到改写 + 版本。
func TestReparse_RawTextRewrite(t *testing.T) {
	f := &fakeTaskStore{task: &domain.TaskRow{ID: 5}}
	ops := NewOperations(f, testConfig)

	raw := "  新原文  "
	id, err := ops.Reparse(5, &raw, "replace", 3)
	if err != nil {
		t.Fatalf("reparse: %v", err)
	}
	if id != 88 {
		t.Errorf("job id=%d, want 88", id)
	}
	if f.reparseRaw == nil || *f.reparseRaw != "新原文" {
		t.Fatalf("reparse raw=%v, want 新原文", f.reparseRaw)
	}
	if f.reparseVer != 3 || f.reparseMode != domain.AIJobModeReplace {
		t.Errorf("version=%d mode=%q, want 3 / replace", f.reparseVer, f.reparseMode)
	}
}

// TestReparse_EmptyRawTextRejected：显式空 raw_text → ErrBadRequest。
func TestReparse_EmptyRawTextRejected(t *testing.T) {
	f := &fakeTaskStore{task: &domain.TaskRow{ID: 5}}
	ops := NewOperations(f, testConfig)

	raw := "   "
	if _, err := ops.Reparse(5, &raw, "append", 0); !errors.Is(err, ErrBadRequest) {
		t.Fatalf("err=%v, want ErrBadRequest", err)
	}
}

// TestReparse_NotFound：task 不存在 → ErrNotFound。
func TestReparse_NotFound(t *testing.T) {
	ops := NewOperations(&fakeTaskStore{}, testConfig)
	if _, err := ops.Reparse(9, nil, "append", 0); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err=%v, want ErrNotFound", err)
	}
}

// TestReparse_JobInFlightMapsToProcessing：DB 层 UNIQUE 兜底也要归为
// ErrProcessing(409),不能泄漏成 500。
func TestReparse_JobInFlightMapsToProcessing(t *testing.T) {
	f := &fakeTaskStore{task: &domain.TaskRow{ID: 5}, enqueueErr: storage.ErrJobInFlight}
	ops := NewOperations(f, testConfig)

	if _, err := ops.Reparse(5, nil, "append", 0); !errors.Is(err, ErrProcessing) {
		t.Fatalf("err=%v, want ErrProcessing", err)
	}
}

// TestReparse_VersionConflictMapsToProcessing：reparse 的版本冲突按历史契约
// 也是 409 aigtd.processing。
func TestReparse_VersionConflictMapsToProcessing(t *testing.T) {
	f := &fakeTaskStore{task: &domain.TaskRow{ID: 5}, enqueueErr: storage.ErrVersionConflict}
	ops := NewOperations(f, testConfig)

	if _, err := ops.Reparse(5, nil, "append", 0); !errors.Is(err, ErrProcessing) {
		t.Fatalf("err=%v, want ErrProcessing", err)
	}
}
