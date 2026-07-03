// Tests for the migration verification CLI.
//
// We exercise the `run` function directly for the three contract cases
// (compatible / mismatch / migration-required), and add one subprocess
// invocation that confirms the binary actually exits with the same
// code.  All three contract tests build a real SQLite DB via
// internal/storage (the same code path the running server uses), then
// drop / mutate / leave-alone to produce the desired scenario.
package main

import (
	"database/sql"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/mattn/go-sqlite3"

	"little-timer/internal/storage"
)

// buildCurrentSchemaDB opens a fresh SQLite DB at the given path,
// runs the v8 migration (so all required tables + columns exist), and
// closes the connection.  Returns the path.  This is the "happy path"
// the tool must recognise as compatible.
func buildCurrentSchemaDB(t *testing.T, path string) {
	t.Helper()
	m := storage.NewSqliteManager().Init(path)
	if err := m.Open(); err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := m.Migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if err := m.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
}

// buildOldSchemaDB creates a SQLite file that mimics a pre-migration
// v0–v2 database: a couple of the v8 tables, no schema_version row,
// and an intentionally small column set on `habits` (missing
// goal_count / wallpaper) so we exercise the column-diff code path.
func buildOldSchemaDB(t *testing.T, path string) {
	t.Helper()
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		t.Fatalf("remove: %v", err)
	}
	dsn := filepath.Clean(path) // plain file DSN, writeable
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		t.Fatalf("ping: %v", err)
	}

	stmts := []string{
		`CREATE TABLE habit_sets (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			color TEXT NOT NULL DEFAULT '#6366f1'
		);`,
		`CREATE TABLE habits (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			set_id INTEGER NOT NULL,
			name TEXT NOT NULL,
			goal_seconds INTEGER NOT NULL DEFAULT 1500,
			color TEXT NOT NULL DEFAULT '#6366f1'
		);`,
		`CREATE TABLE sessions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			habit_id INTEGER NOT NULL,
			duration_seconds INTEGER NOT NULL DEFAULT 0,
			count INTEGER NOT NULL DEFAULT 0,
			date TEXT NOT NULL
		);`,
		// Note: no schema_version, no timer_sessions, no settings,
		// no backup_config — the v8 shape minus a few pieces.
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			t.Fatalf("exec %q: %v", s, err)
		}
	}
}

// buildIncompatibleSchemaDB creates a DB whose schema doesn't match
// any recognisable version: a single weird table, no required v8
// tables.  This is the "unknown / incompatible" case → exit 1.
func buildIncompatibleSchemaDB(t *testing.T, path string) {
	t.Helper()
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		t.Fatalf("remove: %v", err)
	}
	db, err := sql.Open("sqlite3", filepath.Clean(path))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		t.Fatalf("ping: %v", err)
	}
	if _, err := db.Exec(`CREATE TABLE not_a_little_timer_table (id INTEGER PRIMARY KEY, payload TEXT);`); err != nil {
		t.Fatalf("exec: %v", err)
	}
}

// -----------------------------------------------------------------------------
// Direct-call tests — exercise run() and assert the exit code it would
// hand to os.Exit.  This is the primary contract test.
// -----------------------------------------------------------------------------

func TestMigrationTool_CurrentSchema_Exits0(t *testing.T) {
	path := filepath.Join(t.TempDir(), "current.db")
	buildCurrentSchemaDB(t, path)

	code, err := run(path)
	if err != nil {
		t.Fatalf("run(%q) returned error: %v", path, err)
	}
	if code != ExitCompatible {
		t.Errorf("run(%q) = %d, want %d (ExitCompatible)", path, code, ExitCompatible)
	}
}

func TestMigrationTool_MissingTable_Exits1(t *testing.T) {
	path := filepath.Join(t.TempDir(), "incompatible.db")
	buildIncompatibleSchemaDB(t, path)

	code, err := run(path)
	if err != nil {
		t.Fatalf("run(%q) returned error: %v", path, err)
	}
	if code != ExitMismatch {
		t.Errorf("run(%q) = %d, want %d (ExitMismatch)", path, code, ExitMismatch)
	}
}

func TestMigrationTool_UnknownSchema_Exits2(t *testing.T) {
	path := filepath.Join(t.TempDir(), "old.db")
	buildOldSchemaDB(t, path)

	code, err := run(path)
	if err != nil {
		t.Fatalf("run(%q) returned error: %v", path, err)
	}
	if code != ExitMigrationReqd {
		t.Errorf("run(%q) = %d, want %d (ExitMigrationReqd)", path, code, ExitMigrationReqd)
	}
}

// -----------------------------------------------------------------------------
// Additional contract tests for the run() helper.  These cover the
// corner cases that aren't in the three named contract tests.
// -----------------------------------------------------------------------------

func TestRun_MissingFile_Exits2(t *testing.T) {
	// A missing file is treated as "fresh start — migration required"
	// (spec case 1), so exit 2, not exit 1.
	path := filepath.Join(t.TempDir(), "does-not-exist.db")
	code, err := run(path)
	if err != nil {
		t.Fatalf("run(%q): %v", path, err)
	}
	if code != ExitMigrationReqd {
		t.Errorf("missing file: code = %d, want %d", code, ExitMigrationReqd)
	}
}

func TestRun_EmptyDBPath_Exits1(t *testing.T) {
	code, err := run("")
	if err == nil {
		t.Errorf("expected error from empty path")
	}
	if code != ExitMismatch {
		t.Errorf("empty path: code = %d, want %d", code, ExitMismatch)
	}
}

func TestInspect_EmptyDatabase_ReturnsNoVersion(t *testing.T) {
	// A brand-new SQLite file (just `sqlite3 touched`) has zero tables
	// but is still a valid read-only target.  inspect should report
	// DetectedVersion=0, no missing-columns diff, and FoundTables
	// should be empty.
	path := filepath.Join(t.TempDir(), "empty.db")
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	f.Close()

	res, err := inspect(path)
	if err != nil {
		t.Fatalf("inspect: %v", err)
	}
	if res.DetectedVersion != 0 {
		t.Errorf("DetectedVersion = %d, want 0", res.DetectedVersion)
	}
	if len(res.FoundTables) != 0 {
		t.Errorf("FoundTables = %v, want []", res.FoundTables)
	}
	if res.Compatible() {
		t.Errorf("empty DB: Compatible() = true, want false")
	}
	if !res.MigrationRequired() {
		t.Errorf("empty DB: MigrationRequired() = false, want true")
	}
}

func TestInspect_CurrentSchema_CompatibleTrue(t *testing.T) {
	path := filepath.Join(t.TempDir(), "current.db")
	buildCurrentSchemaDB(t, path)

	res, err := inspect(path)
	if err != nil {
		t.Fatalf("inspect: %v", err)
	}
	if !res.Compatible() {
		t.Errorf("Compatible() = false, want true; missing tables=%v missing cols=%v",
			res.MissingTables, res.MissingColumns)
	}
	if res.DetectedVersion != storage.CurrentSchemaVersion {
		t.Errorf("DetectedVersion = %d, want %d", res.DetectedVersion, storage.CurrentSchemaVersion)
	}
}

func TestInspect_OldSchema_ReportsMissingColumns(t *testing.T) {
	path := filepath.Join(t.TempDir(), "old.db")
	buildOldSchemaDB(t, path)

	res, err := inspect(path)
	if err != nil {
		t.Fatalf("inspect: %v", err)
	}
	// The old DB we built intentionally lacks wallpaper + goal_count
	// on habits; the inspector should flag those.
	if len(res.MissingColumns) == 0 {
		t.Errorf("MissingColumns = [], want non-empty diff for old-schema DB")
	}
	if res.Compatible() {
		t.Errorf("old-schema: Compatible() = true, want false")
	}
}

// -----------------------------------------------------------------------------
// Subprocess smoke test — confirms the actual binary exits with the
// contract code.  Built into ./cmd/migrate's own directory so a
// `go test ./cmd/migrate/...` from CI doesn't require an extra
// `go build` step.
// -----------------------------------------------------------------------------

func TestBinaryExitCodes(t *testing.T) {
	binary := buildBinary(t)

	t.Run("compatible", func(t *testing.T) {
		db := filepath.Join(t.TempDir(), "c.db")
		buildCurrentSchemaDB(t, db)
		code := invokeBinary(t, binary, db)
		if code != ExitCompatible {
			t.Errorf("compatible: code = %d, want %d", code, ExitCompatible)
		}
	})
	t.Run("migration_required", func(t *testing.T) {
		db := filepath.Join(t.TempDir(), "o.db")
		buildOldSchemaDB(t, db)
		code := invokeBinary(t, binary, db)
		if code != ExitMigrationReqd {
			t.Errorf("old: code = %d, want %d", code, ExitMigrationReqd)
		}
	})
	t.Run("mismatch", func(t *testing.T) {
		db := filepath.Join(t.TempDir(), "x.db")
		buildIncompatibleSchemaDB(t, db)
		code := invokeBinary(t, binary, db)
		if code != ExitMismatch {
			t.Errorf("incompatible: code = %d, want %d", code, ExitMismatch)
		}
	})
}

// buildBinary compiles the migrate tool into a temp binary and returns
// its path.  We pass the package path (relative to the module root) so
// the test works regardless of cwd.
func buildBinary(t *testing.T) string {
	t.Helper()
	moduleRoot := findModuleRoot(t)
	dir := t.TempDir()
	bin := filepath.Join(dir, "little-timer-migrate")
	cmd := exec.Command("go", "build", "-o", bin, "./cmd/migrate")
	cmd.Dir = moduleRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}
	return bin
}

// findModuleRoot walks up from the test's CWD until it finds a
// go.mod.  The migrate tool sits in cmd/migrate, but `go build`
// wants the module root (the directory containing go.mod).
func findModuleRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	dir := wd
	for i := 0; i < 6; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Fatalf("could not find go.mod from %s", wd)
	return ""
}

func invokeBinary(t *testing.T, bin, dbPath string) int {
	t.Helper()
	cmd := exec.Command(bin, "--db-path", dbPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		// exec.ExitError surfaces the exit code via ExitCode.
		if ee, ok := err.(*exec.ExitError); ok {
			t.Logf("binary stderr: %s", strings.TrimSpace(string(out)))
			return ee.ExitCode()
		}
		t.Fatalf("binary: %v\n%s", err, out)
	}
	return 0
}