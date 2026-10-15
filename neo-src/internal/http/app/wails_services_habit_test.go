package app

import (
	"path/filepath"
	"testing"

	"little-timer/internal/domain"
	"little-timer/internal/settings"
	"little-timer/internal/storage"
)

// wailsTestApp 构造一个带临时 SQLite + 真 SettingsManager 的 App，
// 供 Wails service 层测试用。
func wailsTestApp(t *testing.T) (*App, int64) {
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
	a := NewApp(domain.NewClockManager(domain.NewDefaultClockTaskConfig()), sm, sqlite, nil, dbPath)
	setID, err := sqlite.HabitSets().Create("Test", "", "#000000")
	if err != nil {
		t.Fatalf("create set: %v", err)
	}
	return a, setID
}

// TestWailsCreateHabit_RejectsDuplicate 覆盖 candidate 2 修复的 drift：
// Wails HabitService.CreateHabit 之前缺 NameExistsInSet 校验，Android
// 端能建同名 habit。Phase 2 后，Wails 与 HTTP 共用 a.Habits.CreateHabit，
// 校验自动同步。
func TestWailsCreateHabit_RejectsDuplicate(t *testing.T) {
	a, setID := wailsTestApp(t)
	svc := NewHabitService(a)

	first, err := svc.CreateHabit(setID, "Read", 600, "#00ff00")
	if err != nil {
		t.Fatalf("first create: %v", err)
	}
	if first == nil {
		t.Fatal("first create returned nil")
	}

	second, err := svc.CreateHabit(setID, "Read", 600, "#00ff00")
	if err != nil {
		t.Fatalf("second create should not return Go error, got: %v", err)
	}
	m, ok := second.(map[string]any)
	if !ok {
		t.Fatalf("second create returned %T, want map[string]any", second)
	}
	if success, _ := m["success"].(bool); success {
		t.Fatalf("second create: success = true, want false; map = %+v", m)
	}
	if code, _ := m["code"].(string); code != "habit.name_exists" {
		t.Errorf("code = %q, want habit.name_exists", code)
	}
}

// TestWailsUpdateHabit_RejectsDuplicate 覆盖 Wails Update 的同类 drift。
func TestWailsUpdateHabit_RejectsDuplicate(t *testing.T) {
	a, setID := wailsTestApp(t)
	svc := NewHabitService(a)

	first, _ := svc.CreateHabit(setID, "Book", 600, "#000000")
	_, _ = svc.CreateHabit(setID, "Movie", 600, "#000000")
	firstID := first.(map[string]any)["id"].(int64)

	res, err := svc.UpdateHabit(firstID, "Movie", 600, "#000000", "")
	if err != nil {
		t.Fatalf("update should not return Go error, got: %v", err)
	}
	m, ok := res.(map[string]any)
	if !ok {
		t.Fatalf("update returned %T, want map[string]any", res)
	}
	if success, _ := m["success"].(bool); success {
		t.Fatalf("update: success = true, want false; map = %+v", m)
	}
	if code, _ := m["code"].(string); code != "habit.name_exists" {
		t.Errorf("code = %q, want habit.name_exists", code)
	}
}
