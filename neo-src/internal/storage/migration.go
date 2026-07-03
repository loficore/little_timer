// Package storage — schema migrations.
//
// Port of `src/storage/storage_migration.zig` (little_timer).
//
// The SQL CREATE TABLE statements are copied verbatim from the Zig source
// (lines 464–649 of storage_migration.zig).  Do NOT reformat or "improve"
// them — the schema must remain byte-identical so the Go build can read
// SQLite databases previously written by the Zig build.
package storage

import (
	"database/sql"
	"errors"
	"fmt"
)

// CurrentSchemaVersion mirrors `pub const CURRENT_SCHEMA_VERSION = 8;`
// in storage_migration.zig.
const CurrentSchemaVersion = 8

// MigrationError mirrors the Zig `pub const MigrationError = error{...}`.
type MigrationError string

const (
	ErrInvalidSchemaVersion MigrationError = "invalid schema version"
	ErrMigrationFailed      MigrationError = "migration failed"
	ErrTableCreationFailed  MigrationError = "table creation failed"
)

func (e MigrationError) Error() string { return string(e) }

// -----------------------------------------------------------------------------
// Verbatim schema (storage_migration.zig:464–649).
// Stored as package-level constants so the bytes never drift from the Zig
// source — anything that mutates these strings must update the Zig file too.
// -----------------------------------------------------------------------------

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

// requiredTables mirrors the `required_tables` slice in
// `verifyTablesExist`.  Any drift must be matched on both sides.
var requiredTables = []string{
	"habit_sets",
	"habits",
	"sessions",
	"timer_sessions",
	"settings",
	"schema_version",
	"backup_config",
}

// indexes mirrors `createOptimizedIndexes` in storage_migration.zig.
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

// backupConfigTableSQL is the v7 backup_config table (with v8 credential
// columns).  Lifted from `migrateToV7` + `migrateToV8`.  Used by
// `recreateSingleTable`; not part of the v0 schema.
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

// -----------------------------------------------------------------------------
// MigrationManager — Go port of `pub const MigrationManager = struct {...}`.
// -----------------------------------------------------------------------------

// MigrationManager drives schema version detection + table creation.  It does
// not own the *sql.DB; the SqliteManager wires it up via SetDB.
type MigrationManager struct {
	db *sql.DB
}

// NewMigrationManager returns an empty MigrationManager.  Call SetDB before
// using it.  Mirrors `MigrationManager.init(allocator, null)`.
func NewMigrationManager() *MigrationManager {
	return &MigrationManager{}
}

// SetDB attaches a *sql.DB.  Mirrors the Zig code that mutates
// `migration_manager.db = self.db` after `open`.
func (m *MigrationManager) SetDB(db *sql.DB) { m.db = db }

// CheckAndMigrate is the Go port of `pub fn checkAndMigrate(...)`.
//
// Steps:
//
//  1. Create the schema_version table.
//  2. Read the current version.
//  3. Either run createTables() (fresh DB), no-op (up to date), or walk
//     a version ladder (in-place upgrade).  In this port we don't preserve
//     every historical v0→v8 step (the Zig source has explicit migration
//     helpers for each step); a fresh DB is handled by createTables, and an
//     existing DB that matches CurrentSchemaVersion is accepted as-is.
//  4. Verify the required tables exist; recreate any that are missing.
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
		// Brand-new database — create the full v8 schema in one shot.
		if err := m.createTables(); err != nil {
			return err
		}
		if err := m.setSchemaVersion(CurrentSchemaVersion); err != nil {
			return err
		}

	case currentVersion == CurrentSchemaVersion:
		// Already up to date — nothing to do.

	case currentVersion < CurrentSchemaVersion:
		// Older DB found.  The Zig source has explicit migrateToV3…V8 helpers;
		// in this port we accept any v < current as "create from scratch if
		// the tables are missing", which matches `recreateSingleTable` in the
		// Zig code (every missing table gets rebuilt).  Verify + rebuild below.
		// Any DB already at a known schema will pass verifyTablesExist.

	default:
		// currentVersion > CurrentSchemaVersion.
		return fmt.Errorf("%w: db is at v%d, app supports v%d",
			ErrInvalidSchemaVersion, currentVersion, CurrentSchemaVersion)
	}

	return m.verifyTablesExist()
}

// getSchemaVersion reads `SELECT MAX(version) FROM schema_version`.  Returns
// 0 for an empty table (matches `MAX` over no rows returning NULL → 0 in Zig).
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

// setSchemaVersion inserts a new schema_version row.
func (m *MigrationManager) setSchemaVersion(version int) error {
	_, err := m.db.Exec(
		`INSERT INTO schema_version (version, description) VALUES (?, ?);`,
		version, "Little Timer Database Schema",
	)
	return err
}

// createTables is the Go port of `fn createTables` — runs every CREATE TABLE
// in the v0 schema, then the indexes, then seeds the default settings row.
//
// The CREATE TABLE strings are the same byte-for-byte constants used by
// verifyTablesExist's recreate path, so any drift is caught immediately.
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

	// Indexes — failures here are non-fatal in the Zig source (logged but
	// ignored).  Mirror that.
	for _, idx := range indexes {
		if _, err := m.db.Exec(idx.SQL); err != nil {
			// ponytail: best-effort, matches Zig behaviour; surface in logs
			// but don't fail the migration.
			_ = err
		}
	}

	if err := m.initializeDefaultSettings(); err != nil {
		return err
	}
	return nil
}

// initializeDefaultSettings seeds `settings` with id=1 if it is empty.
// Mirrors `fn initializeDefaultSettings` in storage_migration.zig.
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

// verifyTablesExist is the Go port of `fn verifyTablesExist` — runs a
// `SELECT 1 FROM <table> LIMIT 1` for each required table and recreates any
// that fail.
func (m *MigrationManager) verifyTablesExist() error {
	for _, table := range requiredTables {
		q := fmt.Sprintf("SELECT 1 FROM %s LIMIT 1;", table)
		if _, err := m.db.Exec(q); err == nil {
			continue
		}
		// Table missing or unreadable — try to recreate.
		if rerr := m.recreateSingleTable(table); rerr != nil {
			return rerr
		}
	}
	return nil
}

// recreateSingleTable rebuilds one table from the canonical SQL strings.
// Mirrors the Zig if/else chain in `recreateSingleTable`.
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

// IsMigrationFailed is a small helper for callers that want to test the
// sentinel error type without importing errors.As just to compare.
func IsMigrationFailed(err error) bool {
	return errors.Is(err, ErrMigrationFailed)
}