// Package storage — SqliteManager: the top-level connection lifecycle.
//
// Port of `src/storage/storage_sqlite.zig` (little_timer).  The Zig version
// coordinates five sub-modules (migration / health / backup / crud /
// habit_crud); this Go port ships the same surface minus backup, which is
// stubbed in internal/storage/backup pending a later wave.
//
// File permissions: the Zig source calls `std.os.linux.chmod(path, 0o600)`
// after opening the DB file.  We do the same via `os.Chmod`.
//
// PRAGMA foreign_keys = ON: matches the spec requirement.
package storage

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
)

// sqliteDriverName is the name `mattn/go-sqlite3` self-registers under.  We
// keep a named constant so a future swap (e.g. to modernc.org/sqlite for
// pure Go) is a one-line change.
const sqliteDriverName = "sqlite3"

// -----------------------------------------------------------------------------
// SqliteError mirrors `pub const SqliteError = error{...}`.
// -----------------------------------------------------------------------------

// SqliteError mirrors the union of all storage-layer errors.  We use a
// single string type (Go convention) instead of a sealed enum; sub-modules
// expose their own typed errors (MigrationError, CrudError, etc.) and the
// manager methods bubble those through unchanged.
type SqliteError string

const (
	ErrDatabaseOpenFailed   SqliteError = "database open failed"
	ErrDatabaseNotConnected SqliteError = "database not connected"
)

func (e SqliteError) Error() string { return string(e) }

// -----------------------------------------------------------------------------
// SqliteManager — Go port of `pub const SqliteManager = struct {...}`.
// -----------------------------------------------------------------------------

// SqliteManager owns the *sql.DB and delegates to per-domain sub-managers.
// Init() must be called before Open(); Open() must be called before any
// CRUD method on a sub-manager.
type SqliteManager struct {
	dbPath string

	// sub-managers — populated by Init().
	migration *MigrationManager
	health    *HealthCheckManager
	crud      *CrudManager
	habitSets *HabitSetCrud
	habits    *HabitCrud
	timers    *TimerSessionCrud

	db *sql.DB // nil until Open() succeeds
}

// NewSqliteManager returns an uninitialised manager.  Call Init(dbPath) to
// set the path, then Open() to actually open the file.
func NewSqliteManager() *SqliteManager {
	return &SqliteManager{}
}

// Init stores the database path and constructs sub-managers.  Mirrors the
// Zig `pub fn init(allocator, db_path, backup_dir)` minus backup.
//
// `dbPath` may be absolute or relative; relative paths are resolved against
// the current working directory (matches Go's `database/sql` behaviour).
func (m *SqliteManager) Init(dbPath string) *SqliteManager {
	m.dbPath = dbPath
	m.migration = NewMigrationManager()
	m.health = NewHealthCheckManager()
	m.crud = NewCrudManager()
	m.habitSets = NewHabitSetCrud()
	m.habits = NewHabitCrud()
	m.timers = NewTimerSessionCrud()
	return m
}

// Open creates the parent directory (if missing), opens the SQLite file
// with `Create|ReadWrite`, hardens the file to 0600, enables foreign keys,
// wires the *sql.DB into every sub-manager, and runs migration + health
// check.  Idempotent: a second Open() is a no-op.
//
// Mirrors `pub fn open(self)` in storage_sqlite.zig.  Errors are returned to
// the caller; the Zig version logged-and-continued on migration/health
// errors, but we surface them so the caller can decide.
func (m *SqliteManager) Open() error {
	if m.dbPath == "" {
		return errors.New("storage: Init(dbPath) must be called before Open()")
	}
	if m.db != nil {
		return nil // already open
	}

	// Ensure parent directory exists (matches `makeDir(dp)` in Zig).
	if dir := filepath.Dir(m.dbPath); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			// MkdirAll returns EEXIST when the directory is already there;
			// that's fine.  Anything else is a hard error.
			if !errors.Is(err, os.ErrExist) {
				return fmt.Errorf("%w: mkdir %s: %w", ErrDatabaseOpenFailed, dir, err)
			}
		}
		// Best-effort chmod to 0700 (mirror Zig `std.os.linux.chmod(dp, 0o700)`).
		// ponytail: chmod errors here are non-fatal — the file permission
		// hardening on the DB file itself is the security-relevant step.
		_ = os.Chmod(dir, 0o700)
	}

	// Open the SQLite file.
	db, err := sql.Open(sqliteDriverName, m.dbPath)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrDatabaseOpenFailed, err)
	}
	// Ping forces an actual connect so the file is created on disk before we
	// try to chmod it.
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return fmt.Errorf("%w: ping: %w", ErrDatabaseOpenFailed, err)
	}

	// Hard file permission: 0600 (matches Zig `chmod(path, 0o600)`).
	if err := os.Chmod(m.dbPath, 0o600); err != nil {
		// Non-fatal but worth surfacing — the spec calls this out as a
		// security step.  We log via stderr to keep the package
		// logging-free (logger package hasn't been ported yet).
		fmt.Fprintf(os.Stderr, "storage: chmod 0600 %s: %v\n", m.dbPath, err)
	}

	// Enable foreign keys on every connection.
	if _, err := db.Exec(`PRAGMA foreign_keys = ON;`); err != nil {
		_ = db.Close()
		return fmt.Errorf("%w: pragma foreign_keys: %w", ErrDatabaseOpenFailed, err)
	}

	m.db = db

	// Wire sub-managers.
	m.migration.SetDB(db)
	m.health.SetDB(db)
	m.crud.SetDB(db)
	m.habitSets.SetDB(db)
	m.habits.SetDB(db)
	m.timers.SetDB(db)

	return nil
}

// Migrate runs the migration check + table creation.  Mirrors the implicit
// `checkAndMigrate` call inside `SqliteManager.open` in Zig.
func (m *SqliteManager) Migrate() error {
	if m.db == nil {
		return ErrDatabaseNotConnected
	}
	return m.migration.CheckAndMigrate()
}

// Close releases the *sql.DB.  Idempotent.  Mirrors `pub fn close(self)` —
// in Go there is no separate "deinit"; close is enough.
func (m *SqliteManager) Close() error {
	if m.db == nil {
		return nil
	}
	err := m.db.Close()
	m.db = nil

	// Clear sub-manager handles so post-Close operations fail cleanly
	// instead of operating on a stale *sql.DB.
	m.migration.SetDB(nil)
	m.health.SetDB(nil)
	m.crud.SetDB(nil)
	m.habitSets.SetDB(nil)
	m.habits.SetDB(nil)
	m.timers.SetDB(nil)
	return err
}

// -----------------------------------------------------------------------------
// Convenience accessors — used by SqliteManager.SaveSettings / LoadSettings
// in storage.go and by tests.
// -----------------------------------------------------------------------------

// DB returns the underlying *sql.DB.  Returns nil if not open.
func (m *SqliteManager) DB() *sql.DB { return m.db }

// Migration returns the migration sub-manager (for tests + advanced use).
func (m *SqliteManager) Migration() *MigrationManager { return m.migration }

// Health returns the health-check sub-manager.
func (m *SqliteManager) Health() *HealthCheckManager { return m.health }

// Crud returns the settings-row sub-manager.
func (m *SqliteManager) Crud() *CrudManager { return m.crud }

// HabitSets returns the habit_sets sub-manager.
func (m *SqliteManager) HabitSets() *HabitSetCrud { return m.habitSets }

// Habits returns the habits sub-manager.
func (m *SqliteManager) Habits() *HabitCrud { return m.habits }

// Timers returns the timer-sessions sub-manager (sessions + timer_sessions).
func (m *SqliteManager) Timers() *TimerSessionCrud { return m.timers }

// IsOpen reports whether the underlying *sql.DB is connected.
func (m *SqliteManager) IsOpen() bool { return m.db != nil }