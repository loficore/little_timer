package habits

import (
	"errors"
	"path/filepath"
	"testing"

	"little-timer/internal/domain"
	"little-timer/internal/settings"
	"little-timer/internal/storage"
)

// newTestOperations 构造一个带临时 SQLite + 真 SettingsManager 的 Operations。
// 跟 storage/*_test.go 的风格一致。
func newTestOperations(t *testing.T) (*Operations, int64) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	sqlite := storage.NewSqliteManager().Init(dbPath)
	if err := sqlite.Open(); err != nil {
		t.Fatalf("sqlite open: %v", err)
	}
	t.Cleanup(func() { _ = sqlite.Close() })
	if err := sqlite.Migrate(); err != nil {
		t.Fatalf("sqlite migrate: %v", err)
	}
	sm, err := settings.NewFromSqliteManager(sqlite, dbPath)
	if err != nil {
		t.Fatalf("settings: %v", err)
	}
	if err := sm.UpdateBasic(settings.BasicConfig{Timezone: 8, Language: "EN", DefaultMode: domain.DefaultModeStopwatch}); err != nil {
		t.Fatalf("update timezone: %v", err)
	}
	o := New(sqlite, sm)
	// 建一个 set 供 habit 测试用。
	setID, err := sqlite.HabitSets().Create("TestSet", "", DefaultColor)
	if err != nil {
		t.Fatalf("create set: %v", err)
	}
	return o, setID
}

func TestCreateHabit_NameRequired(t *testing.T) {
	o, setID := newTestOperations(t)
	_, err := o.CreateHabit(CreateHabitInput{SetID: setID, Name: "   "})
	if !errors.Is(err, ErrNameRequired) {
		t.Fatalf("err = %v, want ErrNameRequired", err)
	}
}

func TestCreateHabit_Defaults(t *testing.T) {
	o, setID := newTestOperations(t)
	res, err := o.CreateHabit(CreateHabitInput{SetID: setID, Name: "Read"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if res.GoalSeconds != DefaultGoalSeconds {
		t.Errorf("GoalSeconds = %d, want %d", res.GoalSeconds, DefaultGoalSeconds)
	}
	if res.Color != DefaultColor {
		t.Errorf("Color = %q, want %q", res.Color, DefaultColor)
	}
}

func TestCreateHabit_ResolvesZeroGoal(t *testing.T) {
	o, setID := newTestOperations(t)
	res, err := o.CreateHabit(CreateHabitInput{SetID: setID, Name: "Read", GoalSeconds: 0})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if res.GoalSeconds != DefaultGoalSeconds {
		t.Errorf("GoalSeconds = %d, want %d", res.GoalSeconds, DefaultGoalSeconds)
	}
}

func TestCreateHabit_DuplicateNameInSet(t *testing.T) {
	o, setID := newTestOperations(t)
	if _, err := o.CreateHabit(CreateHabitInput{SetID: setID, Name: "Read"}); err != nil {
		t.Fatalf("first: %v", err)
	}
	_, err := o.CreateHabit(CreateHabitInput{SetID: setID, Name: "Read"})
	if !errors.Is(err, ErrNameExists) {
		t.Fatalf("duplicate err = %v, want ErrNameExists", err)
	}
}

func TestCreateHabit_CrossSetOK(t *testing.T) {
	o, setA := newTestOperations(t)
	dbPath := filepath.Join(t.TempDir(), "test.db") // unused, just to access the same manager
	_ = dbPath
	setB, err := o.sqlite.HabitSets().Create("Other", "", DefaultColor)
	if err != nil {
		t.Fatalf("create setB: %v", err)
	}
	if _, err := o.CreateHabit(CreateHabitInput{SetID: setA, Name: "Read"}); err != nil {
		t.Fatalf("first: %v", err)
	}
	// 同名在另一 set 应允许。
	if _, err := o.CreateHabit(CreateHabitInput{SetID: setB, Name: "Read"}); err != nil {
		t.Fatalf("cross-set: %v", err)
	}
}

func TestUpdateHabit_NotFound(t *testing.T) {
	o, _ := newTestOperations(t)
	_, err := o.UpdateHabit(99999, UpdateHabitInput{Name: "X"})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestUpdateHabit_DuplicateNameInSet(t *testing.T) {
	o, setID := newTestOperations(t)
	// 建两个 habit
	hab1, err := o.CreateHabit(CreateHabitInput{SetID: setID, Name: "Book"})
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	if _, err := o.CreateHabit(CreateHabitInput{SetID: setID, Name: "Movie"}); err != nil {
		t.Fatalf("second: %v", err)
	}
	// 把 hab1 改名为 Movie —— 同 set 已有 Movie，应拒绝。
	_, err = o.UpdateHabit(hab1.ID, UpdateHabitInput{Name: "Movie"})
	if !errors.Is(err, ErrNameExists) {
		t.Fatalf("update err = %v, want ErrNameExists", err)
	}
}

func TestUpdateHabit_RenameToSelfOK(t *testing.T) {
	o, setID := newTestOperations(t)
	hab, err := o.CreateHabit(CreateHabitInput{SetID: setID, Name: "Book"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	// 改为自己同名（含大小写不同也算同一 set 已存在），excludeSelf 应放行。
	if _, err := o.UpdateHabit(hab.ID, UpdateHabitInput{Name: "Book", GoalSeconds: 600, Color: DefaultColor}); err != nil {
		t.Fatalf("rename-to-self: %v", err)
	}
}

func TestCreateHabitSet_NameRequired(t *testing.T) {
	o, _ := newTestOperations(t)
	_, err := o.CreateHabitSet(CreateHabitSetInput{Name: ""})
	if !errors.Is(err, ErrNameRequired) {
		t.Fatalf("err = %v, want ErrNameRequired", err)
	}
}

func TestCreateHabitSet_DefaultColor(t *testing.T) {
	o, _ := newTestOperations(t)
	res, err := o.CreateHabitSet(CreateHabitSetInput{Name: "Work"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if res.Color != DefaultColor {
		t.Errorf("Color = %q, want %q", res.Color, DefaultColor)
	}
}

func TestUpdateHabitSet_NameRequired(t *testing.T) {
	o, _ := newTestOperations(t)
	res, err := o.CreateHabitSet(CreateHabitSetInput{Name: "Work"})
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	_, err = o.UpdateHabitSet(res.ID, UpdateHabitSetInput{Name: ""})
	if !errors.Is(err, ErrNameRequired) {
		t.Fatalf("err = %v, want ErrNameRequired", err)
	}
}

func TestCreateSession_Defaults(t *testing.T) {
	o, setID := newTestOperations(t)
	hab, err := o.CreateHabit(CreateHabitInput{SetID: setID, Name: "Read"})
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	// count = 0, date = "" → 默认 count=1, date=today
	if _, err := o.CreateSession(CreateSessionInput{HabitID: hab.ID, DurationSeconds: 60}); err != nil {
		t.Fatalf("create: %v", err)
	}
	rows, err := o.ListSessions("", "", "", 100, 0)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(rows))
	}
	if rows[0].Count != 1 {
		t.Errorf("Count = %d, want 1", rows[0].Count)
	}
}

func TestGetHabitDetail_ProgressPercent(t *testing.T) {
	o, setID := newTestOperations(t)
	hab, err := o.CreateHabit(CreateHabitInput{SetID: setID, Name: "Read", GoalSeconds: 100})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	// 当天累计 30 秒
	if _, err := o.CreateSession(CreateSessionInput{HabitID: hab.ID, DurationSeconds: 30}); err != nil {
		t.Fatalf("session: %v", err)
	}
	detail, err := o.GetHabitDetail(hab.ID, "")
	if err != nil {
		t.Fatalf("detail: %v", err)
	}
	if detail.TodaySeconds != 30 {
		t.Errorf("TodaySeconds = %d, want 30", detail.TodaySeconds)
	}
	if detail.ProgressPercent != 30 {
		t.Errorf("ProgressPercent = %d, want 30 (30/100)", detail.ProgressPercent)
	}
}
