// Package storage —— migration 路径测试。
//
// 覆盖 v8 → v9 迁移:habits.set_id 由 NOT NULL 变为可空,新增 tasks / ai_jobs
// 表,settings 增加 llm_* 列。
package storage

import (
	"database/sql"
	"path/filepath"
	"testing"
)

// v8SchemaSQL 是迁移前(版本 8)的 habit_sets / habits / schema_version 建表
// 语句。测试用它把数据库引导到"已是 v8"的状态,然后调用 Migrate() 走升级路径。
const v8SchemaSQL = `
CREATE TABLE habit_sets (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL CHECK(length(name) > 0 AND length(name) <= 100),
    description TEXT DEFAULT '',
    color TEXT NOT NULL DEFAULT '#6366f1',
    wallpaper TEXT DEFAULT '',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE habits (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    set_id INTEGER NOT NULL,
    name TEXT NOT NULL CHECK(length(name) > 0 AND length(name) <= 100),
    goal_seconds INTEGER NOT NULL DEFAULT 0 CHECK(goal_seconds >= 0),
    goal_count INTEGER NOT NULL DEFAULT 0 CHECK(goal_count >= 0),
    color TEXT NOT NULL DEFAULT '#6366f1',
    wallpaper TEXT DEFAULT '',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (set_id) REFERENCES habit_sets(id) ON DELETE CASCADE
);
CREATE TABLE sessions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    habit_id INTEGER NOT NULL,
    duration_seconds INTEGER NOT NULL DEFAULT 0,
    count INTEGER NOT NULL DEFAULT 0,
    started_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    date TEXT NOT NULL,
    FOREIGN KEY (habit_id) REFERENCES habits(id) ON DELETE CASCADE
);
CREATE TABLE timer_sessions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    habit_id INTEGER,
    mode TEXT NOT NULL,
    started_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    is_running INTEGER NOT NULL DEFAULT 0,
    is_finished INTEGER NOT NULL DEFAULT 0,
    is_paused INTEGER NOT NULL DEFAULT 0,
    elapsed_seconds INTEGER NOT NULL DEFAULT 0,
    paused_total_seconds INTEGER NOT NULL DEFAULT 0,
    pause_started_at INTEGER,
    last_synced_at INTEGER,
    remaining_seconds INTEGER,
    work_duration INTEGER NOT NULL DEFAULT 0,
    rest_duration INTEGER NOT NULL DEFAULT 0,
    loop_count INTEGER NOT NULL DEFAULT 0,
    current_round INTEGER NOT NULL DEFAULT 0,
    in_rest INTEGER NOT NULL DEFAULT 0,
    FOREIGN KEY (habit_id) REFERENCES habits(id) ON DELETE SET NULL
);
CREATE TABLE settings (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    timezone INTEGER NOT NULL,
    language TEXT NOT NULL,
    default_mode TEXT NOT NULL,
    theme_mode TEXT NOT NULL,
    wallpaper TEXT DEFAULT '',
    duration_seconds INTEGER NOT NULL,
    countdown_loop BOOLEAN NOT NULL DEFAULT 0,
    countdown_loop_count INTEGER NOT NULL DEFAULT 0,
    countdown_loop_interval INTEGER NOT NULL DEFAULT 0,
    stopwatch_max_seconds INTEGER NOT NULL DEFAULT 86400,
    log_level TEXT NOT NULL,
    log_enable_timestamp BOOLEAN NOT NULL DEFAULT 1,
    log_tick_interval INTEGER NOT NULL DEFAULT 1000,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE schema_version (
    version INTEGER PRIMARY KEY,
    applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    description TEXT
);
`

// TestMigrateV8ToV9_HabitsSetIdNullable 验证从已有 v8 数据库迁移到 v9 时:
//   - habits.set_id 变为可空(NULL 写入不再被拒)
//   - 新增 tasks / ai_jobs 表
//   - settings 表新增 llm_* 列
//   - 既有 habits 数据未被丢失
func TestMigrateV8ToV9_HabitsSetIdNullable(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "lt.db")
	m := NewSqliteManager().Init(dbPath)
	if err := m.Open(); err != nil {
		t.Fatal(err)
	}
	defer m.Close()

	// 1. 把数据库引导到"已是 v8"的状态。
	if _, err := m.DB().Exec(v8SchemaSQL); err != nil {
		t.Fatalf("seed v8 schema: %v", err)
	}
	if _, err := m.DB().Exec(
		`INSERT INTO habit_sets (name) VALUES ('晨间');`); err != nil {
		t.Fatal(err)
	}
	if _, err := m.DB().Exec(
		`INSERT INTO habits (set_id, name, goal_seconds) VALUES (1, '冥想', 600);`); err != nil {
		t.Fatal(err)
	}
	if _, err := m.DB().Exec(
		`INSERT INTO schema_version (version, description) VALUES (8, 'v8');`); err != nil {
		t.Fatal(err)
	}

	// 2. 跑迁移。
	if err := m.Migrate(); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	// 3. 迁移后:set_id 应允许 NULL。
	if _, err := m.DB().Exec(
		`INSERT INTO habits (set_id, name) VALUES (NULL, '无集');`); err != nil {
		t.Fatalf("v9 habits.set_id 仍未允许 NULL: %v", err)
	}

	// 4. 迁移后:新表存在。
	var n int
	if err := m.DB().QueryRow(`SELECT COUNT(*) FROM tasks`).Scan(&n); err != nil {
		t.Fatalf("tasks 表缺失: %v", err)
	}
	if err := m.DB().QueryRow(`SELECT COUNT(*) FROM ai_jobs`).Scan(&n); err != nil {
		t.Fatalf("ai_jobs 表缺失: %v", err)
	}

	// 5. 迁移后:settings 表存在 llm_* 列。
	for _, col := range []string{
		"llm_provider", "llm_model", "llm_api_key_encrypted",
		"llm_base_url", "llm_max_tokens", "llm_timeout_seconds",
	} {
		if !hasColumn(m.DB(), "settings", col) {
			t.Errorf("settings 表缺少列: %s", col)
		}
	}

	// 6. 既有 habits 数据未丢失。
	var name string
	if err := m.DB().QueryRow(
		`SELECT name FROM habits WHERE id=1`).Scan(&name); err != nil {
		t.Fatalf("读取既有 habit 失败: %v", err)
	}
	if name != "冥想" {
		t.Fatalf("habit 数据丢失: 期望 冥想, 实际 %q", name)
	}
}

// hasColumn 通过 PRAGMA table_info 检查列是否存在。
func hasColumn(db *sql.DB, table, column string) bool {
	rows, err := db.Query(`PRAGMA table_info(` + table + `);`)
	if err != nil {
		return false
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return false
		}
		if name == column {
			return true
		}
	}
	return false
}
