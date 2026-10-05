// Package storage —— schema 迁移。
//
// SQL CREATE TABLE 语句定义了磁盘契约。不要重新格式化或“优化”它们 ——
// 既有数据库必须保持可读，改动一律走新的 schema 版本，绝不直接编辑这些字符串。
package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
)

// CurrentSchemaVersion 是本构建目标针对的 schema 版本。
const CurrentSchemaVersion = 10

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
    set_id INTEGER,
    name TEXT NOT NULL CHECK(length(name) > 0 AND length(name) <= 100),
    goal_seconds INTEGER NOT NULL DEFAULT 0 CHECK(goal_seconds >= 0),
    goal_count INTEGER NOT NULL DEFAULT 0 CHECK(goal_count >= 0),
    color TEXT NOT NULL DEFAULT '#6366f1',
    wallpaper TEXT DEFAULT '',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (set_id) REFERENCES habit_sets(id) ON DELETE SET NULL
);`

// habitsRebuildV9SQL 是 v8 → v9 迁移时用来替换旧 habits 表的临时表。
// 除了表名为 habits_new,列定义必须与 habitsTableSQL 保持一致:SQLite 无法
// 用 ALTER COLUMN 去除 NOT NULL,只能建新表 → 拷数据 → 删旧表 → 改名。
const habitsRebuildV9SQL = `CREATE TABLE habits_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    set_id INTEGER,
    name TEXT NOT NULL CHECK(length(name) > 0 AND length(name) <= 100),
    goal_seconds INTEGER NOT NULL DEFAULT 0 CHECK(goal_seconds >= 0),
    goal_count INTEGER NOT NULL DEFAULT 0 CHECK(goal_count >= 0),
    color TEXT NOT NULL DEFAULT '#6366f1',
    wallpaper TEXT DEFAULT '',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (set_id) REFERENCES habit_sets(id) ON DELETE SET NULL
);`

// tasksTableSQL 是 v9 引入的统一任务树(待办与日程合一),自引用 parent_id。
const tasksTableSQL = `CREATE TABLE IF NOT EXISTS tasks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    parent_id INTEGER,
    raw_text TEXT DEFAULT '',
    title TEXT NOT NULL DEFAULT '',
    notes TEXT DEFAULT '',
    status TEXT NOT NULL DEFAULT 'inbox'
        CHECK(status IN ('inbox','active','done','archived','rejected')),
    due_date TEXT,
    scheduled_start INTEGER,
    scheduled_end INTEGER,
    pinned INTEGER NOT NULL DEFAULT 0 CHECK(pinned IN (0,1)),
    estimated_minutes INTEGER NOT NULL DEFAULT 0,
    order_index INTEGER NOT NULL DEFAULT 0,
    source TEXT NOT NULL DEFAULT 'manual',
    ai_status TEXT NOT NULL DEFAULT 'pending'
        CHECK(ai_status IN ('pending','processing','done','error')),
    ai_error TEXT,
    user_edited_title INTEGER NOT NULL DEFAULT 0 CHECK(user_edited_title IN (0,1)),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    parsed_at TIMESTAMP,
    FOREIGN KEY (parent_id) REFERENCES tasks(id) ON DELETE CASCADE
);`

// aiJobsTableSQL 追踪 AI 拆解异步任务(调试/重放/状态机)。
const aiJobsTableSQL = `CREATE TABLE IF NOT EXISTS ai_jobs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    task_id INTEGER NOT NULL,
    provider TEXT NOT NULL,
    model TEXT NOT NULL,
    status TEXT NOT NULL
        CHECK(status IN ('queued','running','success','failed')),
    request_payload TEXT NOT NULL,
    response_payload TEXT,
    error_message TEXT,
    input_tokens INTEGER,
    output_tokens INTEGER,
    attempts INTEGER NOT NULL DEFAULT 1,
    mode TEXT NOT NULL DEFAULT 'append' CHECK(mode IN ('append','replace','review')),
    started_at TIMESTAMP,
    finished_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (task_id) REFERENCES tasks(id) ON DELETE CASCADE
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
	"tasks",
	"ai_jobs",
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
	{"idx_tasks_parent", "CREATE INDEX IF NOT EXISTS idx_tasks_parent ON tasks(parent_id, order_index);"},
	{"idx_tasks_status", "CREATE INDEX IF NOT EXISTS idx_tasks_status ON tasks(status);"},
	{"idx_tasks_ai_status", "CREATE INDEX IF NOT EXISTS idx_tasks_ai_status ON tasks(ai_status, created_at);"},
	{"idx_tasks_scheduled", "CREATE INDEX IF NOT EXISTS idx_tasks_scheduled ON tasks(scheduled_start);"},
	{"idx_ai_jobs_task_id", "CREATE INDEX IF NOT EXISTS idx_ai_jobs_task_id ON ai_jobs(task_id, created_at);"},
	// 部分 UNIQUE 索引:同一 task 至多一个 queued/running job,堵住
	// reparse 的 TOCTOU 并发(handler 转 409)。
	{"idx_ai_jobs_in_flight", "CREATE UNIQUE INDEX IF NOT EXISTS idx_ai_jobs_in_flight ON ai_jobs(task_id) WHERE status IN ('queued','running');"},
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
		// 升级完成后 bump schema_version(见下方 needsVersionBump)。

	default:
		// currentVersion > CurrentSchemaVersion。
		return fmt.Errorf("%w: db is at v%d, app supports v%d",
			ErrInvalidSchemaVersion, currentVersion, CurrentSchemaVersion)
	}

	if err := m.verifyTablesExist(); err != nil {
		return err
	}
	// 幂等的 v8 → v9 升级:重建 habits(set_id 可空)、补 settings llm 列、
	// 创建 tasks/ai_jobs 索引。对全新 DB 这些检查全部 no-op。
	if err := m.migrateV8ToV9(); err != nil {
		return err
	}
	// 幂等的 v9 → v10 升级:为 tasks/ai_jobs 补 v10 新列(fresh DB 已通过
	// tasksTableSQL/aiJobsTableSQL 包含这些列,addColumnIfMissing 会自动跳过)。
	if err := m.migrateV9ToV10(); err != nil {
		return err
	}

	// 旧 DB 升级后必须把 schema_version 推进到 CurrentSchemaVersion,
	// 否则它永远停留在旧版本号,下次启动重复走升级分支(当前幂等所以安全,
	// 但未来按版本分派的迁移会误分类)。fresh DB 已在上面写过,不重复写。
	if currentVersion != 0 && currentVersion < CurrentSchemaVersion {
		return m.setSchemaVersion(CurrentSchemaVersion)
	}
	return nil
}

// migrateV9ToV10 把 v9 DB 幂等地带到 v10:补 tasks.user_edited_title
// 与 ai_jobs.mode 两列。fresh DB 走 tasksTableSQL/aiJobsTableSQL 已含
// 这些列,addColumnIfMissing 会自动跳过。
//
// 注:SQLite 不允许 ADD COLUMN 含列级 CHECK 约束(只允许 NULL/NOT NULL/DEFAULT
// 等表约束);upgrade DB 仅加列 + DEFAULT,不补 CHECK。fresh DB 的建表语句含
// CHECK,值由应用层保证。
func (m *MigrationManager) migrateV9ToV10() error {
	if err := m.addColumnIfMissing("tasks", "user_edited_title",
		"INTEGER NOT NULL DEFAULT 0"); err != nil {
		return fmt.Errorf("%w: add tasks.user_edited_title: %w", ErrMigrationFailed, err)
	}
	if err := m.addColumnIfMissing("ai_jobs", "mode",
		"TEXT NOT NULL DEFAULT 'append'"); err != nil {
		return fmt.Errorf("%w: add ai_jobs.mode: %w", ErrMigrationFailed, err)
	}
	return nil
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

// reconcileStuckInFlightJobs 在创建 idx_ai_jobs_in_flight 部分 UNIQUE 索引
// 前调用:若 DB 中已存在多个 queued/running ai_jobs(跨 task 即可触发 UNIQUE
// 失败),直接 UPDATE 为 failed + finished_at,避免 UNIQUE 索引创建失败被吞掉
// (reviewer M2)。幂等;若没有 stuck 行,影响 0 行。
func (m *MigrationManager) reconcileStuckInFlightJobs() error {
	res, err := m.db.Exec(
		`UPDATE ai_jobs
		 SET status = 'failed',
		     finished_at = CURRENT_TIMESTAMP,
		     error_message = 'reconciled before idx_ai_jobs_in_flight: stale queued/running'
		 WHERE status IN ('queued', 'running');`,
	)
	if err != nil {
		return err
	}
	if affected, _ := res.RowsAffected(); affected > 0 {
		slog.Info("migration.reconcile in-flight jobs", "affected", affected)
	}
	return nil
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
		{"tasks", tasksTableSQL},
		{"ai_jobs", aiJobsTableSQL},
	}
	for _, s := range steps {
		if _, err := m.db.Exec(s.sql); err != nil {
			return fmt.Errorf("%w: %s: %w", ErrTableCreationFailed, s.name, err)
		}
	}

	// 索引 —— idx_ai_jobs_in_flight 是 TOCTOU 兜底,失败必须传播(其余尽力而为)。
	if err := m.createIndexes(); err != nil {
		return err
	}

	if err := m.initializeDefaultSettings(); err != nil {
		return err
	}
	return nil
}

// createIndexes 创建 indexes 中的全部索引。idx_ai_jobs_in_flight(部分
// UNIQUE)失败会先 reconcile 陈旧的 queued/running 行后重试,仍失败则传播
// 错误(reviewer M2:这条索引是 TOCTOU 兜底,静默失败会让整个防御失效)。
// 其余索引为尽力而为,失败不致命。
func (m *MigrationManager) createIndexes() error {
	for _, idx := range indexes {
		if _, err := m.db.Exec(idx.SQL); err != nil {
			if idx.Name == "idx_ai_jobs_in_flight" {
				if rerr := m.reconcileStuckInFlightJobs(); rerr != nil {
					return fmt.Errorf("%w: create %s: %w", ErrMigrationFailed, idx.Name, rerr)
				}
				if _, err2 := m.db.Exec(idx.SQL); err2 != nil {
					return fmt.Errorf("%w: create %s after reconcile: %w", ErrMigrationFailed, idx.Name, err2)
				}
				continue
			}
			// 其余索引尽力而为,记日志但不让迁移失败。
			slog.Warn("migration.index skipped", "index", idx.Name, "error", err.Error())
		}
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
	case "tasks":
		_, err := m.db.Exec(tasksTableSQL)
		return err
	case "ai_jobs":
		_, err := m.db.Exec(aiJobsTableSQL)
		return err
	default:
		return fmt.Errorf("%w: unknown table %s", ErrTableCreationFailed, name)
	}
}

// v8 → v9 升级中要在既有数据库上追加的 settings 列。fresh DB 在 createTables
// 里走 settingsTableSQL(本版本已包含这些列),所以这里只覆盖已有 settings 表。
var v9SettingsColumns = []struct {
	name string
	def  string
}{
	{"llm_provider", "TEXT NOT NULL DEFAULT 'openai_compat'"},
	{"llm_model", "TEXT NOT NULL DEFAULT 'deepseek-chat'"},
	{"llm_api_key_encrypted", "BLOB"},
	{"llm_base_url", "TEXT NOT NULL DEFAULT ''"},
	{"llm_max_tokens", "INTEGER NOT NULL DEFAULT 2048"},
	{"llm_timeout_seconds", "INTEGER NOT NULL DEFAULT 30"},
}

// migrateV8ToV9 把 v8 数据库带到 v9 状态。所有操作都是幂等的，fresh DB
//(已经按 v9 schema 建表) 上调用也是 no-op。
func (m *MigrationManager) migrateV8ToV9() error {
	// 1. habits.set_id 由 NOT NULL 变为可空。SQLite 无法 ALTER COLUMN，
	//    唯一办法是建新表 → 拷数据 → 改名。先检查是否需要。
	nullable, err := m.columnIsNullable("habits", "set_id")
	if err != nil {
		return fmt.Errorf("%w: inspect habits: %w", ErrMigrationFailed, err)
	}
	if !nullable {
		if err := m.rebuildHabitsNullable(); err != nil {
			return err
		}
	}

	// 2. settings 补 llm 列。fresh DB 上这些列已存在,addColumnIfMissing 会
	//    自动跳过。
	for _, c := range v9SettingsColumns {
		if err := m.addColumnIfMissing("settings", c.name, c.def); err != nil {
			return fmt.Errorf("%w: add settings.%s: %w", ErrMigrationFailed, c.name, err)
		}
	}

	// 3. 兜底建索引(新表可能由 verifyTablesExist 重建,索引不会跟着重建)。
	if err := m.createIndexes(); err != nil {
		return err
	}

	return nil
}

// columnIsNullable 报告表的指定列是否允许 NULL(notnull=0)。
func (m *MigrationManager) columnIsNullable(table, column string) (bool, error) {
	rows, err := m.db.Query(`PRAGMA table_info(` + table + `);`)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	for rows.Next() {
		var cid, notnull, pk int
		var name, ctype string
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return false, err
		}
		if name == column {
			return notnull == 0, nil
		}
	}
	if err := rows.Err(); err != nil {
		return false, err
	}
	return false, fmt.Errorf("column %s.%s not found", table, column)
}

// addColumnIfMissing 在指定表上添加一列;若列已存在则 no-op。
func (m *MigrationManager) addColumnIfMissing(table, column, def string) error {
	rows, err := m.db.Query(`PRAGMA table_info(` + table + `);`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var cid, notnull, pk int
		var name, ctype string
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return err
		}
		if name == column {
			return nil
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	_, err = m.db.Exec(fmt.Sprintf(`ALTER TABLE %s ADD COLUMN %s %s;`, table, column, def))
	return err
}

// rebuildHabitsNullable 用 SQLite 的标准"建新表 → 拷数据 → 删旧表 → 改名"
// 手法把 habits 表升级到 set_id 可空的 v9 形态。在单连接上执行,绕开外键
// 约束检查。
func (m *MigrationManager) rebuildHabitsNullable() error {
	conn, err := m.db.Conn(context.Background())
	if err != nil {
		return fmt.Errorf("%w: acquire conn: %w", ErrMigrationFailed, err)
	}
	defer conn.Close()

	stmts := []string{
		`PRAGMA foreign_keys = OFF;`,
		`BEGIN;`,
		habitsRebuildV9SQL,
		`INSERT INTO habits_new (id, set_id, name, goal_seconds, goal_count, color, wallpaper, created_at)
		 SELECT id, set_id, name, goal_seconds, goal_count, color, wallpaper, created_at FROM habits;`,
		`DROP TABLE habits;`,
		`ALTER TABLE habits_new RENAME TO habits;`,
		`CREATE INDEX IF NOT EXISTS idx_habits_set_id ON habits(set_id);`,
		`CREATE INDEX IF NOT EXISTS idx_habits_name ON habits(name);`,
		`COMMIT;`,
		`PRAGMA foreign_keys = ON;`,
	}
	for _, s := range stmts {
		if _, err := conn.ExecContext(context.Background(), s); err != nil {
			_, _ = conn.ExecContext(context.Background(), `ROLLBACK;`)
			return fmt.Errorf("%w: rebuild habits (%s): %w", ErrMigrationFailed, firstLine(s), err)
		}
	}
	return nil
}

// firstLine 取 SQL 语句第一行(去掉前导空白),用于错误信息更易读。
func firstLine(s string) string {
	for i, r := range s {
		if r == '\n' {
			return s[:i]
		}
	}
	return s
}

// IsMigrationFailed 是小辅助函数，让调用方无需为了比较而引入 errors.As，
// 就能测试哨兵错误类型。
func IsMigrationFailed(err error) bool {
	return errors.Is(err, ErrMigrationFailed)
}
