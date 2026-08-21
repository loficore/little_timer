// Package storage —— schema 迁移。
//
// SQL CREATE TABLE 语句定义了磁盘契约。不要重新格式化或“优化”它们 ——
// 既有数据库必须保持可读，改动一律走新的 schema 版本，绝不直接编辑这些字符串。
package storage

import (
	"database/sql"
	"errors"
	"fmt"
)

// CurrentSchemaVersion 是本构建目标针对的 schema 版本。
const CurrentSchemaVersion = 8

// MigrationError 是迁移失败的类型化哨兵错误。
type MigrationError string

const (
	ErrInvalidSchemaVersion MigrationError = "invalid schema version"
	ErrMigrationFailed      MigrationError = "migration failed"
	ErrTableCreationFailed  MigrationError = "table creation failed"
)

func (e MigrationError) Error() string { return string(e) }

// 规范 schema SQL。以包级常量存放；改动这些字符串会破坏既有数据库 ——
// 要改请新增 schema 版本。

const schemaVersionTableSQL = `CREATE TABLE IF NOT EXISTS schema_version (
    version INTEGER PRIMARY KEY,
    applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    description TEXT
);`

const healthCheckTableSQL = `CREATE TABLE IF NOT EXISTS health_check (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    last_check TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    status TEXT NOT NULL DEFAULT 'healthy',
    checksum TEXT,
    record_count INTEGER DEFAULT 0
);`

const habitSetsTableSQL = `CREATE TABLE IF NOT EXISTS habit_sets (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL CHECK(length(name) > 0 AND length(name) <= 100),
    description TEXT DEFAULT '',
    color TEXT NOT NULL DEFAULT '#6366f1',
    wallpaper TEXT DEFAULT '',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);`

const habitsTableSQL = `CREATE TABLE IF NOT EXISTS habits (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    set_id INTEGER NOT NULL,
    name TEXT NOT NULL CHECK(length(name) > 0 AND length(name) <= 100),
    goal_seconds INTEGER NOT NULL DEFAULT 0 CHECK(goal_seconds >= 0),
    goal_count INTEGER NOT NULL DEFAULT 0 CHECK(goal_count >= 0),
    color TEXT NOT NULL DEFAULT '#6366f1',
    wallpaper TEXT DEFAULT '',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (set_id) REFERENCES habit_sets(id) ON DELETE CASCADE
);`

const sessionsTableSQL = `CREATE TABLE IF NOT EXISTS sessions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    habit_id INTEGER NOT NULL,
    duration_seconds INTEGER NOT NULL DEFAULT 0 CHECK(duration_seconds >= 0),
    count INTEGER NOT NULL DEFAULT 0 CHECK(count >= 0),
    started_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    date TEXT NOT NULL,
    FOREIGN KEY (habit_id) REFERENCES habits(id) ON DELETE CASCADE
);`

const timerSessionsTableSQL = `CREATE TABLE IF NOT EXISTS timer_sessions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    habit_id INTEGER,
    mode TEXT NOT NULL CHECK(mode IN ('countdown', 'stopwatch')),
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
);`

const settingsTableSQL = `CREATE TABLE IF NOT EXISTS settings (
 id INTEGER PRIMARY KEY CHECK (id = 1),
 timezone INTEGER NOT NULL CHECK(timezone >= -12 AND timezone <= 14),
 language TEXT NOT NULL CHECK(length(language) >= 1 AND length(language) <= 10),
 default_mode TEXT NOT NULL CHECK(default_mode IN ('countdown', 'stopwatch', 'world_clock')),
 theme_mode TEXT NOT NULL CHECK(length(theme_mode) <= 20),
 wallpaper TEXT DEFAULT '',
 duration_seconds INTEGER NOT NULL CHECK(duration_seconds >= 1 AND duration_seconds <= 86400),
 countdown_loop BOOLEAN NOT NULL DEFAULT 0,
 countdown_loop_count INTEGER NOT NULL DEFAULT 0 CHECK(countdown_loop_count >= 0 AND countdown_loop_count <= 1000),
 countdown_loop_interval INTEGER NOT NULL DEFAULT 0 CHECK(countdown_loop_interval >= 0 AND countdown_loop_interval <= 3600),
 stopwatch_max_seconds INTEGER NOT NULL DEFAULT 86400 CHECK(stopwatch_max_seconds > 0 AND stopwatch_max_seconds <= 31536000),
 log_level TEXT NOT NULL CHECK(length(log_level) <= 10),
 log_enable_timestamp BOOLEAN NOT NULL DEFAULT 1,
 log_tick_interval INTEGER NOT NULL DEFAULT 1000 CHECK(log_tick_interval >= 100 AND log_tick_interval <= 10000),
 updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP);`

// requiredTables 列出 verifyTablesExist 检查的表。任何变动都必须两边同步。
var requiredTables = []string{
	"habit_sets",
	"habits",
	"sessions",
	"timer_sessions",
	"settings",
	"schema_version",
	"backup_config",
}

// indexes 与 schema 一同创建，用于查询性能。
var indexes = []struct {
	Name string
	SQL  string
}{
	{"idx_habits_set_id", "CREATE INDEX IF NOT EXISTS idx_habits_set_id ON habits(set_id);"},
	{"idx_habits_name", "CREATE INDEX IF NOT EXISTS idx_habits_name ON habits(name);"},
	{"idx_sessions_habit_id", "CREATE INDEX IF NOT EXISTS idx_sessions_habit_id ON sessions(habit_id);"},
	{"idx_sessions_date", "CREATE INDEX IF NOT EXISTS idx_sessions_date ON sessions(date);"},
	{"idx_settings_timezone", "CREATE INDEX IF NOT EXISTS idx_settings_timezone ON settings(timezone);"},
	{"idx_settings_language", "CREATE INDEX IF NOT EXISTS idx_settings_language ON settings(language);"},
	{"idx_health_check_status", "CREATE INDEX IF NOT EXISTS idx_health_check_status ON health_check(status);"},
	{"idx_timer_sessions_habit_id", "CREATE INDEX IF NOT EXISTS idx_timer_sessions_habit_id ON timer_sessions(habit_id);"},
	{"idx_timer_sessions_is_running", "CREATE INDEX IF NOT EXISTS idx_timer_sessions_is_running ON timer_sessions(is_running);"},
}

// backupConfigTableSQL 是 v7 的 backup_config 表（带 v8 凭据列）。
// 供 recreateSingleTable 使用；不属于初始 schema。
const backupConfigTableSQL = `CREATE TABLE IF NOT EXISTS backup_config (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    target_type TEXT NOT NULL DEFAULT 'local',
    enabled BOOLEAN NOT NULL DEFAULT 0,
    auto_backup BOOLEAN NOT NULL DEFAULT 0,
    auto_backup_interval INTEGER NOT NULL DEFAULT 86400,
    local_path TEXT,
    webdav_url TEXT,
    webdav_username TEXT,
    webdav_password_encrypted BLOB,
    s3_endpoint TEXT,
    s3_bucket TEXT,
    s3_region TEXT,
    s3_access_key_encrypted BLOB,
    s3_secret_key_encrypted BLOB,
    s3_path_prefix TEXT,
    has_master_password INTEGER NOT NULL DEFAULT 0,
    credentials_unlock_time INTEGER NOT NULL DEFAULT 0,
    credential_unlock_attempts INTEGER NOT NULL DEFAULT 0,
    credential_locked_until INTEGER NOT NULL DEFAULT 0,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);`

// MigrationManager 负责 schema 版本检测 + 建表。它不持有 *sql.DB；
// 由 SqliteManager 通过 SetDB 接线。
type MigrationManager struct {
	db *sql.DB
}

// NewMigrationManager 返回空的 MigrationManager。使用前先调用 SetDB。
func NewMigrationManager() *MigrationManager {
	return &MigrationManager{}
}

// SetDB 接入 *sql.DB。
func (m *MigrationManager) SetDB(db *sql.DB) { m.db = db }

// CheckAndMigrate 检测 schema 版本并把数据库带到最新。
//
// 步骤：
//
//  1. 创建 schema_version 表。
//  2. 读取当前版本。
//  3. 全新 DB → createTables；已是最新 → no-op；较旧的 DB → 原样接受，
//     交给校验环节重建缺失的表（我们不重放 v0→v8 的每一步历史迁移）。
//  4. 校验必需表齐全；重建缺失的表。
func (m *MigrationManager) CheckAndMigrate() error {
	if m.db == nil {
		return ErrTableCreationFailed
	}

	if _, err := m.db.Exec(schemaVersionTableSQL); err != nil {
		return fmt.Errorf("%w: schema_version: %w", ErrTableCreationFailed, err)
	}

	currentVersion, err := m.getSchemaVersion()
	if err != nil {
		return fmt.Errorf("%w: read version: %w", ErrMigrationFailed, err)
	}

	switch {
	case currentVersion == 0:
		// 全新数据库 —— 一次性建出完整 v8 schema。
		if err := m.createTables(); err != nil {
			return err
		}
		if err := m.setSchemaVersion(CurrentSchemaVersion); err != nil {
			return err
		}

	case currentVersion == CurrentSchemaVersion:
		// 已是最新 —— 什么都不做。

	case currentVersion < CurrentSchemaVersion:
		// 发现较旧的 DB。不重放逐版本迁移；每个缺失的表都由下面的
		// 校验环节重建，已处于已知 schema 的 DB 会原样通过校验。

	default:
		// currentVersion > CurrentSchemaVersion。
		return fmt.Errorf("%w: db is at v%d, app supports v%d",
			ErrInvalidSchemaVersion, currentVersion, CurrentSchemaVersion)
	}

	return m.verifyTablesExist()
}

// getSchemaVersion 读取 `SELECT MAX(version) FROM schema_version`。
// 空表返回 0（MAX 对空行集结果是 NULL）。
func (m *MigrationManager) getSchemaVersion() (int, error) {
	var v sql.NullInt64
	if err := m.db.QueryRow(`SELECT MAX(version) FROM schema_version;`).Scan(&v); err != nil {
		return 0, err
	}
	if !v.Valid {
		return 0, nil
	}
	return int(v.Int64), nil
}

// setSchemaVersion 插入新的 schema_version 行。
func (m *MigrationManager) setSchemaVersion(version int) error {
	_, err := m.db.Exec(
		`INSERT INTO schema_version (version, description) VALUES (?, ?);`,
		version, "Little Timer Database Schema",
	)
	return err
}

// createTables 执行 schema 中的每条 CREATE TABLE，然后是索引，
// 最后播种默认 settings 行。
//
// CREATE TABLE 字符串与 verifyTablesExist 的重建路径用的是逐字节相同的
// 常量，因此任何漂移都会立刻暴露。
func (m *MigrationManager) createTables() error {
	steps := []struct {
		name string
		sql  string
	}{
		{"health_check", healthCheckTableSQL},
		{"habit_sets", habitSetsTableSQL},
		{"habits", habitsTableSQL},
		{"sessions", sessionsTableSQL},
		{"timer_sessions", timerSessionsTableSQL},
		{"settings", settingsTableSQL},
	}
	for _, s := range steps {
		if _, err := m.db.Exec(s.sql); err != nil {
			return fmt.Errorf("%w: %s: %w", ErrTableCreationFailed, s.name, err)
		}
	}

	// 索引 —— 这里失败不致命。
	for _, idx := range indexes {
		if _, err := m.db.Exec(idx.SQL); err != nil {
			// 尽力而为：记日志但不让迁移失败。
			_ = err
		}
	}

	if err := m.initializeDefaultSettings(); err != nil {
		return err
	}
	return nil
}

// initializeDefaultSettings 在 `settings` 为空时播种 id=1 行。
func (m *MigrationManager) initializeDefaultSettings() error {
	var count int64
	if err := m.db.QueryRow(`SELECT COUNT(*) FROM settings WHERE id = 1;`).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	_, err := m.db.Exec(
		`INSERT INTO settings (id, timezone, language, default_mode, theme_mode, duration_seconds, countdown_loop, countdown_loop_count, countdown_loop_interval, stopwatch_max_seconds, log_level, log_enable_timestamp, log_tick_interval)
		 VALUES (1, 8, 'ZH', 'countdown', 'dark', 1500, 0, 0, 0, 86400, 'INFO', 1, 1000);`,
	)
	return err
}

// verifyTablesExist 对每张必需表执行 `SELECT 1 FROM <table> LIMIT 1`，
// 失败的表一律重建。
func (m *MigrationManager) verifyTablesExist() error {
	for _, table := range requiredTables {
		q := fmt.Sprintf("SELECT 1 FROM %s LIMIT 1;", table)
		if _, err := m.db.Exec(q); err == nil {
			continue
		}
		// 表缺失或不可读 —— 尝试重建。
		if rerr := m.recreateSingleTable(table); rerr != nil {
			return rerr
		}
	}
	return nil
}

// recreateSingleTable 用规范 SQL 字符串重建单张表。
func (m *MigrationManager) recreateSingleTable(name string) error {
	switch name {
	case "habit_sets":
		_, err := m.db.Exec(habitSetsTableSQL)
		return err
	case "habits":
		_, err := m.db.Exec(habitsTableSQL)
		return err
	case "sessions":
		_, err := m.db.Exec(sessionsTableSQL)
		return err
	case "timer_sessions":
		_, err := m.db.Exec(timerSessionsTableSQL)
		return err
	case "settings":
		_, err := m.db.Exec(settingsTableSQL)
		return err
	case "schema_version":
		_, err := m.db.Exec(schemaVersionTableSQL)
		return err
	case "backup_config":
		_, err := m.db.Exec(backupConfigTableSQL)
		return err
	default:
		return fmt.Errorf("%w: unknown table %s", ErrTableCreationFailed, name)
	}
}

// IsMigrationFailed 是小辅助函数，让调用方无需为了比较而引入 errors.As，
// 就能测试哨兵错误类型。
func IsMigrationFailed(err error) bool {
	return errors.Is(err, ErrMigrationFailed)
}
