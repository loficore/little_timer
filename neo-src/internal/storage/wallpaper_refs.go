// Package storage —— 壁纸引用计数与解绑。
//
// CountWallpaperRefs 与 UnbindWallpaper 横跨三张带 `wallpaper` 列的表：
// habits、habit_sets、settings。所有查询都用精确匹配 `wallpaper = ?`，
// 因此只影响画廊管理的引用（`local:` 前缀）；Zig 时代遗留值如
// `/wallpapers/book.jpg` 不会被触碰。
package storage

import (
	"fmt"
)

// CountWallpaperRefs 统计 habits、habit_sets、settings 三表中引用给定
// localRef 的行数。只使用精确匹配 `wallpaper = ?` —— 像
// `/wallpapers/book.jpg` 这样的遗留值（没有 `local:` 前缀）不会被计入，
// 因为调用方传入的一定是 `local:` 前缀字符串。
func (m *SqliteManager) CountWallpaperRefs(localRef string) (int64, error) {
	if m.db == nil {
		return 0, ErrDatabaseNotConnected
	}

	const query = `
		SELECT
			(SELECT COUNT(*) FROM habits WHERE wallpaper = ?) +
			(SELECT COUNT(*) FROM habit_sets WHERE wallpaper = ?) +
			(SELECT COUNT(*) FROM settings WHERE wallpaper = ?) AS refs;
	`

	var refs int64
	err := m.db.QueryRow(query, localRef, localRef, localRef).Scan(&refs)
	if err != nil {
		return 0, fmt.Errorf("CountWallpaperRefs: %w", err)
	}
	return refs, nil
}

// UnbindWallpaper 在三张表（habits、habit_sets、settings）中把所有匹配
// localRef 行的 wallpaper 列清空。操作是事务性的：任何错误都触发回滚。
// 返回受影响行数总和（三次 UPDATE 的 RowsAffected 相加）。重复调用幂等
// （返回 0）。
func (m *SqliteManager) UnbindWallpaper(localRef string) (int64, error) {
	if m.db == nil {
		return 0, ErrDatabaseNotConnected
	}

	tx, err := m.db.Begin()
	if err != nil {
		return 0, fmt.Errorf("UnbindWallpaper: begin tx: %w", err)
	}
	defer tx.Rollback() // 已提交时是 no-op

	var total int64

	// UPDATE habits 表
	res, err := tx.Exec(`UPDATE habits SET wallpaper = '' WHERE wallpaper = ?`, localRef)
	if err != nil {
		return 0, fmt.Errorf("UnbindWallpaper: habits: %w", err)
	}
	n, _ := res.RowsAffected()
	total += n

	// UPDATE habit_sets 表
	res, err = tx.Exec(`UPDATE habit_sets SET wallpaper = '' WHERE wallpaper = ?`, localRef)
	if err != nil {
		return 0, fmt.Errorf("UnbindWallpaper: habit_sets: %w", err)
	}
	n, _ = res.RowsAffected()
	total += n

	// UPDATE settings 表
	res, err = tx.Exec(`UPDATE settings SET wallpaper = '' WHERE wallpaper = ?`, localRef)
	if err != nil {
		return 0, fmt.Errorf("UnbindWallpaper: settings: %w", err)
	}
	n, _ = res.RowsAffected()
	total += n

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("UnbindWallpaper: commit: %w", err)
	}

	return total, nil
}
