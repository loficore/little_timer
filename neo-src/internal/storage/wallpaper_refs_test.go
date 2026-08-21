package storage

import (
	"testing"
)

// 测试辅助函数 —— 小包装：打开临时 DB 并播种壁纸行。

// seedWallpaperRefs 插入一个 habit_set、一个 habit，并把 settings 行
// （id=1）都改为引用同一个壁纸 ref。返回 habit_set id 与 habit id。
func seedWallpaperRefs(t *testing.T, m *SqliteManager, ref string) (int64, int64) {
	t.Helper()
	db := m.DB()

	// 创建一个带壁纸 ref 的 habit_set。
	res, err := db.Exec(
		`INSERT INTO habit_sets (name, description, color, wallpaper) VALUES (?, ?, ?, ?)`,
		"test-set", "test desc", "#123456", ref,
	)
	if err != nil {
		t.Fatalf("seed habit_set: %v", err)
	}
	setID, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("seed habit_set LastInsertId: %v", err)
	}

	// 在该 set 下创建一个带壁纸 ref 的 habit。
	res, err = db.Exec(
		`INSERT INTO habits (set_id, name, goal_seconds, color, wallpaper) VALUES (?, ?, ?, ?, ?)`,
		setID, "test-habit", 600, "#654321", ref,
	)
	if err != nil {
		t.Fatalf("seed habit: %v", err)
	}
	habitID, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("seed habit LastInsertId: %v", err)
	}

	// 把默认 settings 行（id=1）也改为引用该壁纸。
	_, err = db.Exec(`UPDATE settings SET wallpaper = ? WHERE id = 1`, ref)
	if err != nil {
		t.Fatalf("seed settings wallpaper: %v", err)
	}

	return setID, habitID
}

// 测试。

func TestCountWallpaperRefsWithSeededData(t *testing.T) {
	m := openTempSqlite(t)
	ref := "local:a.png"
	seedWallpaperRefs(t, m, ref)

	count, err := m.CountWallpaperRefs(ref)
	if err != nil {
		t.Fatalf("CountWallpaperRefs: %v", err)
	}
	if count != 3 {
		t.Errorf("CountWallpaperRefs(%q) = %d, want 3", ref, count)
	}
}

func TestUnbindWallpaperClearsAllThree(t *testing.T) {
	m := openTempSqlite(t)
	ref := "local:a.png"
	seedWallpaperRefs(t, m, ref)

	affected, err := m.UnbindWallpaper(ref)
	if err != nil {
		t.Fatalf("UnbindWallpaper: %v", err)
	}
	if affected != 3 {
		t.Errorf("UnbindWallpaper affected = %d, want 3", affected)
	}

	// 验证三张表的 wallpaper 列现在都为空。
	var habitWP string
	if err := m.DB().QueryRow(
		`SELECT wallpaper FROM habits WHERE name = 'test-habit'`,
	).Scan(&habitWP); err != nil {
		t.Fatalf("read habits wallpaper: %v", err)
	}
	if habitWP != "" {
		t.Errorf("habits wallpaper after unbind: got %q, want \"\"", habitWP)
	}

	var setWP string
	if err := m.DB().QueryRow(
		`SELECT wallpaper FROM habit_sets WHERE name = 'test-set'`,
	).Scan(&setWP); err != nil {
		t.Fatalf("read habit_sets wallpaper: %v", err)
	}
	if setWP != "" {
		t.Errorf("habit_sets wallpaper after unbind: got %q, want \"\"", setWP)
	}

	var settingsWP string
	if err := m.DB().QueryRow(
		`SELECT wallpaper FROM settings WHERE id = 1`,
	).Scan(&settingsWP); err != nil {
		t.Fatalf("read settings wallpaper: %v", err)
	}
	if settingsWP != "" {
		t.Errorf("settings wallpaper after unbind: got %q, want \"\"", settingsWP)
	}
}

func TestUnbindWallpaperIdempotent(t *testing.T) {
	m := openTempSqlite(t)
	ref := "local:a.png"
	seedWallpaperRefs(t, m, ref)

	// 第一次 unbind —— 应返回 3。
	affected, err := m.UnbindWallpaper(ref)
	if err != nil {
		t.Fatalf("UnbindWallpaper #1: %v", err)
	}
	if affected != 3 {
		t.Errorf("UnbindWallpaper #1 = %d, want 3", affected)
	}

	// 第二次 unbind —— 应返回 0（幂等）。
	affected, err = m.UnbindWallpaper(ref)
	if err != nil {
		t.Fatalf("UnbindWallpaper #2: %v", err)
	}
	if affected != 0 {
		t.Errorf("UnbindWallpaper #2 = %d, want 0 (idempotent)", affected)
	}

	// 计数也应为 0。
	count, err := m.CountWallpaperRefs(ref)
	if err != nil {
		t.Fatalf("CountWallpaperRefs after unbind: %v", err)
	}
	if count != 0 {
		t.Errorf("CountWallpaperRefs after unbind = %d, want 0", count)
	}
}

func TestCountWallpaperRefsEmptyDB(t *testing.T) {
	m := openTempSqlite(t)

	// 全新 DB —— 不应有任何行引用壁纸。
	count, err := m.CountWallpaperRefs("local:nonexistent.png")
	if err != nil {
		t.Fatalf("CountWallpaperRefs on empty DB: %v", err)
	}
	if count != 0 {
		t.Errorf("CountWallpaperRefs on empty DB = %d, want 0", count)
	}
}

func TestUnbindWallpaperEmptyDB(t *testing.T) {
	m := openTempSqlite(t)

	affected, err := m.UnbindWallpaper("local:nonexistent.png")
	if err != nil {
		t.Fatalf("UnbindWallpaper on empty DB: %v", err)
	}
	if affected != 0 {
		t.Errorf("UnbindWallpaper on empty DB = %d, want 0", affected)
	}
}

// TestLegacyWallpaperNotCounted 验证像 `/wallpapers/book.jpg` 这样的
// 遗留壁纸值（缺少 `local:` 前缀）不会被 CountWallpaperRefs 或
// UnbindWallpaper 匹配。SQL 使用精确匹配 `wallpaper = ?`，因此只有完全
// 相同的字符串会受影响。
func TestLegacyWallpaperNotCounted(t *testing.T) {
	m := openTempSqlite(t)
	db := m.DB()

	// 播种一个 habit_set 和一个 habit，壁纸用遗留路径。
	_, err := db.Exec(
		`INSERT INTO habit_sets (name, color, wallpaper) VALUES (?, ?, ?)`,
		"legacy-set", "#000", "/wallpapers/book.jpg",
	)
	if err != nil {
		t.Fatalf("seed legacy habit_set: %v", err)
	}
	_, err = db.Exec(
		`INSERT INTO habits (set_id, name, goal_seconds, color, wallpaper)
		 SELECT id, ?, ?, ?, ? FROM habit_sets WHERE name = 'legacy-set'`,
		"legacy-habit", 600, "#111", "/wallpapers/book.jpg",
	)
	if err != nil {
		t.Fatalf("seed legacy habit: %v", err)
	}

	// 统计 local: ref 应返回 0 —— 遗留行不会被匹配。
	count, err := m.CountWallpaperRefs("local:a.png")
	if err != nil {
		t.Fatalf("CountWallpaperRefs for local: ref: %v", err)
	}
	if count != 0 {
		t.Errorf("CountWallpaperRefs(local:a.png) with legacy rows = %d, want 0", count)
	}

	// unbind 一个 local: ref 同样不应触碰遗留行。
	affected, err := m.UnbindWallpaper("local:a.png")
	if err != nil {
		t.Fatalf("UnbindWallpaper for local: ref: %v", err)
	}
	if affected != 0 {
		t.Errorf("UnbindWallpaper(local:a.png) with legacy rows = %d, want 0", affected)
	}

	// 验证遗留行的壁纸仍在。
	var wp string
	if err := db.QueryRow(
		`SELECT wallpaper FROM habit_sets WHERE name = 'legacy-set'`,
	).Scan(&wp); err != nil {
		t.Fatalf("read legacy habit_set: %v", err)
	}
	if wp != "/wallpapers/book.jpg" {
		t.Errorf("legacy habit_set wallpaper after unbind: got %q, want /wallpapers/book.jpg", wp)
	}

	if err := db.QueryRow(
		`SELECT wallpaper FROM habits WHERE name = 'legacy-habit'`,
	).Scan(&wp); err != nil {
		t.Fatalf("read legacy habit: %v", err)
	}
	if wp != "/wallpapers/book.jpg" {
		t.Errorf("legacy habit wallpaper after unbind: got %q, want /wallpapers/book.jpg", wp)
	}
}

// TestUnbindWallpaperOnlyAffectsMatchingRef 验证 unbind 一个壁纸 ref
// 不会清掉其他壁纸 ref。
func TestUnbindWallpaperOnlyAffectsMatchingRef(t *testing.T) {
	m := openTempSqlite(t)
	db := m.DB()

	// 一个 set 播种 ref A，另一个播种 ref B。
	_, err := db.Exec(
		`INSERT INTO habit_sets (name, color, wallpaper) VALUES (?, ?, ?)`,
		"set-a", "#aaa", "local:a.png",
	)
	if err != nil {
		t.Fatalf("seed set-a: %v", err)
	}
	_, err = db.Exec(
		`INSERT INTO habit_sets (name, color, wallpaper) VALUES (?, ?, ?)`,
		"set-b", "#bbb", "local:b.png",
	)
	if err != nil {
		t.Fatalf("seed set-b: %v", err)
	}

	// 只 unbind ref A。
	affected, err := m.UnbindWallpaper("local:a.png")
	if err != nil {
		t.Fatalf("UnbindWallpaper: %v", err)
	}
	if affected != 1 {
		t.Errorf("UnbindWallpaper affected = %d, want 1", affected)
	}

	// ref B 应还在。
	var wp string
	if err := db.QueryRow(
		`SELECT wallpaper FROM habit_sets WHERE name = 'set-b'`,
	).Scan(&wp); err != nil {
		t.Fatalf("read set-b: %v", err)
	}
	if wp != "local:b.png" {
		t.Errorf("set-b wallpaper after unbind of a: got %q, want local:b.png", wp)
	}
}
