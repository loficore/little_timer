// Command little-timer-migrate — DB migration verification CLI.
//
// Port of the (informal) Zig-side migration checker.  This tool does NOT
// modify the SQLite file; it only inspects an existing database and
// reports whether the schema is compatible with the Go build, needs a
// migration ladder run, or is so different from the Go schema that no
// migration is possible (corruption / wrong app / hand-edited DB).
//
// Exit codes (matches the CLI contract):
//
//	0 — Schema compatible (v8, all required tables + columns present).
//	1 — Schema mismatch / unknown schema / read error.
//	2 — Schema migration required (v0–v7, can be brought up to v8).
//
// Usage:
//
//	$ little-timer-migrate --db-path /var/lib/little_timer.db
//	$ little-timer-migrate --db-path /tmp/missing.db
package main

import (
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	_ "github.com/mattn/go-sqlite3"

	"little-timer/internal/storage"
)

// Exit codes — surfaced as `os.Exit(...)` from main; tests assert them
// via `os.Exit` interception or subprocess invocations.
const (
	ExitCompatible    = 0
	ExitMismatch      = 1
	ExitMigrationReqd = 2
)

// expectedSchema describes the v8 schema shape the Go build understands.
// Sourced from `internal/storage/migration.go` (kept in sync with the
// Zig `src/storage/storage_migration.zig` source).  Order of columns is
// not significant for the diff; we sort both sides before comparing.
//
// ponytail: this duplicate definition is intentional — the migrate tool
// should be runnable on a DB the current app build can't open (e.g. a
// pre-migration DB), so it cannot depend on internal/storage's runtime
// state.  The schema shape is the spec; if migration.go drifts from
// this list, both must be updated together.
var expectedSchema = map[string][]string{
	"health_check": {"id", "last_check", "status", "checksum", "record_count"},
	"habit_sets":   {"id", "name", "description", "color", "wallpaper", "created_at"},
	"habits":       {"id", "set_id", "name", "goal_seconds", "goal_count", "color", "wallpaper", "created_at"},
	"sessions":     {"id", "habit_id", "duration_seconds", "count", "started_at", "date"},
	"timer_sessions": {
		"id", "habit_id", "mode", "started_at", "updated_at",
		"is_running", "is_finished", "is_paused",
		"elapsed_seconds", "paused_total_seconds", "pause_started_at", "last_synced_at",
		"remaining_seconds", "work_duration", "rest_duration",
		"loop_count", "current_round", "in_rest",
	},
	"settings": {
		"id", "timezone", "language", "default_mode", "theme_mode", "wallpaper",
		"duration_seconds",
		"countdown_loop", "countdown_loop_count", "countdown_loop_interval",
		"stopwatch_max_seconds",
		"log_level", "log_enable_timestamp", "log_tick_interval",
		"updated_at",
	},
	"backup_config": {
		"id", "target_type", "enabled", "auto_backup", "auto_backup_interval",
		"local_path", "webdav_url", "webdav_username", "webdav_password_encrypted",
		"s3_endpoint", "s3_bucket", "s3_region",
		"s3_access_key_encrypted", "s3_secret_key_encrypted", "s3_path_prefix",
		"has_master_password", "credentials_unlock_time",
		"credential_unlock_attempts", "credential_locked_until",
		"updated_at",
	},
}

// expectedRequiredTables is the subset of tables the Go runtime needs to
// boot without errors.  `schema_version` is intentionally excluded —
// its absence is itself a strong "old schema" signal that we want to
// report as exit 2, not exit 1.
var expectedRequiredTables = []string{
	"health_check",
	"habit_sets",
	"habits",
	"sessions",
	"timer_sessions",
	"settings",
	"backup_config",
}

// inspectResult is the structured outcome of a single verification run.
// Encapsulating the result makes the function unit-testable without
// reaching for `os.Exit`.
type inspectResult struct {
	DetectedVersion int      // 0 if no schema_version table present
	FoundTables     []string // every sqlite_master type='table' row
	MissingTables   []string // v8-required tables that are absent
	ExtraTables     []string // tables present but not in expectedSchema
	MissingColumns  []string // per-table: "table.col" not found
	SchemaErr       error    // I/O / parse failure, if any
}

// Compatible reports whether the result represents a clean v8 schema.
func (r *inspectResult) Compatible() bool {
	if r.SchemaErr != nil {
		return false
	}
	if r.DetectedVersion != storage.CurrentSchemaVersion {
		return false
	}
	if len(r.MissingTables) > 0 || len(r.MissingColumns) > 0 {
		return false
	}
	return true
}

// MigrationRequired reports whether the schema is older than v8 but
// not so badly malformed that it can't be brought forward.
//
// Classification rules:
//
//   - detected version > 0 AND < v8 → migration required (clear case).
//   - detected version == 0 (no schema_version row) AND at least one
//     v8-shaped table present → migration required (pre-v3 DB that
//     was created before schema_version existed).
//   - detected version == 0 AND no tables at all → migration
//     required (fresh DB, needs initial schema).
//   - detected version == 0 AND tables present but NONE of them are
//     v8-shaped → NOT migration required; that's an unknown / wrong
//     app's DB and should be reported as a mismatch (exit 1) instead.
func (r *inspectResult) MigrationRequired() bool {
	if r.SchemaErr != nil {
		return false
	}
	if r.DetectedVersion > 0 && r.DetectedVersion < storage.CurrentSchemaVersion {
		return true
	}
	if r.DetectedVersion == 0 && len(r.FoundTables) == 0 {
		// Empty DB (or freshly-touched file): needs initial schema.
		return true
	}
	if r.DetectedVersion == 0 && len(r.FoundTables) > 0 {
		// Any v8-shaped table means we recognise the schema lineage.
		for _, t := range r.FoundTables {
			if _, ok := expectedSchema[t]; ok {
				return true
			}
		}
	}
	return false
}

// inspect opens the SQLite DB read-only and computes the schema
// inspection result.  The DB doesn't have to be openable in write mode
// (e.g. permission errors mid-test) — read-only is enough to introspect
// the schema.
func inspect(dbPath string) (*inspectResult, error) {
	// _query_only=1 forces the connection to refuse writes.  We don't
	// pass file:<path>?mode=ro because mattn/go-sqlite3 only supports
	// mode=ro via the URL-form DSN which has its own quirks; the
	// _query_only pragma is the more portable knob.
	dsn := fmt.Sprintf("file:%s?mode=ro&_query_only=1", filepath.Clean(dbPath))
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("open: %w", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping: %w", err)
	}

	res := &inspectResult{}

	// 1. Read schema_version if present.
	if _, err := db.Exec(`SELECT 1 FROM schema_version LIMIT 1;`); err == nil {
		var v sql.NullInt64
		if scanErr := db.QueryRow(`SELECT MAX(version) FROM schema_version;`).Scan(&v); scanErr != nil {
			return nil, fmt.Errorf("read schema_version: %w", scanErr)
		}
		if v.Valid {
			res.DetectedVersion = int(v.Int64)
		}
	}

	// 2. List every table in the DB.
	rows, err := db.Query(
		`SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%';`,
	)
	if err != nil {
		return nil, fmt.Errorf("list tables: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		res.FoundTables = append(res.FoundTables, name)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows: %w", err)
	}
	sort.Strings(res.FoundTables)

	// 3. Diff against expected schema.
	foundSet := make(map[string]bool, len(res.FoundTables))
	for _, t := range res.FoundTables {
		foundSet[t] = true
	}
	for _, want := range expectedRequiredTables {
		if !foundSet[want] {
			res.MissingTables = append(res.MissingTables, want)
		}
	}
	for _, t := range res.FoundTables {
		if _, ok := expectedSchema[t]; !ok {
			// Either an unexpected / app-private table; flag but don't
			// treat as fatal.
			res.ExtraTables = append(res.ExtraTables, t)
		}
	}

	// 4. Per-table column diff.
	for table, wantCols := range expectedSchema {
		if !foundSet[table] {
			continue // missing-table diff is captured above.
		}
		got, err := readColumns(db, table)
		if err != nil {
			return nil, fmt.Errorf("read columns %s: %w", table, err)
		}
		gotSet := make(map[string]bool, len(got))
		for _, c := range got {
			gotSet[c] = true
		}
		for _, col := range wantCols {
			if !gotSet[col] {
				res.MissingColumns = append(res.MissingColumns, table+"."+col)
			}
		}
	}

	return res, nil
}

// readColumns queries `PRAGMA table_info(<table>)` and returns the
// column names in declaration order.
func readColumns(db *sql.DB, table string) ([]string, error) {
	// PRAGMA can't be parameterised via `?`, so we build the statement
	// with fmt.Sprintf.  `table` comes from our own list of v8 names
	// or from sqlite_master, both of which we already trust; an
	// injection here would require the DB to already be compromised.
	rows, err := db.Query(fmt.Sprintf(`PRAGMA table_info("%s");`, table))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var (
			cid     int
			name    string
			ctype   string
			notnull int
			dflt    sql.NullString
			pk      int
		)
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return nil, err
		}
		out = append(out, name)
	}
	return out, rows.Err()
}

// -----------------------------------------------------------------------------
// Reporting helpers.
// -----------------------------------------------------------------------------

// formatDiff prints a human-readable schema diff to stderr.
func formatDiff(res *inspectResult) string {
	var b strings.Builder
	if len(res.MissingTables) > 0 {
		fmt.Fprintf(&b, "missing tables: %s\n", strings.Join(res.MissingTables, ", "))
	}
	if len(res.MissingColumns) > 0 {
		fmt.Fprintf(&b, "missing columns: %s\n", strings.Join(res.MissingColumns, ", "))
	}
	if len(res.ExtraTables) > 0 {
		fmt.Fprintf(&b, "unexpected tables: %s\n", strings.Join(res.ExtraTables, ", "))
	}
	return strings.TrimRight(b.String(), "\n")
}

// run is the testable inner loop.  Returns the chosen exit code so the
// tests can assert it without spawning a subprocess.
func run(dbPath string) (int, error) {
	if dbPath == "" {
		return ExitMismatch, errors.New("--db-path is required")
	}
	if _, err := os.Stat(dbPath); err != nil {
		if os.IsNotExist(err) {
			// A missing file is the "fresh start" case from the spec;
			// treat it as "needs migration" so the operator knows to
			// run the app once to initialise the schema.
			fmt.Fprintf(os.Stderr, "Schema migration required: v0 -> v8 (file %s does not exist)\n", dbPath)
			return ExitMigrationReqd, nil
		}
		return ExitMismatch, fmt.Errorf("stat %s: %w", dbPath, err)
	}

	res, err := inspect(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Schema mismatch: %v\n", err)
		return ExitMismatch, nil
	}

	switch {
	case res.Compatible():
		fmt.Printf("Schema compatible: v%d\n", storage.CurrentSchemaVersion)
		return ExitCompatible, nil

	case res.MigrationRequired():
		fmt.Printf("Schema migration required: v%d -> v%d\n", res.DetectedVersion, storage.CurrentSchemaVersion)
		if diff := formatDiff(res); diff != "" {
			fmt.Fprintln(os.Stderr, diff)
		}
		return ExitMigrationReqd, nil

	default:
		// Detected version is > v8, or version is v8 but columns/tables
		// don't match.  Either way, the schema is too different for a
		// forward migration to apply cleanly.
		fmt.Fprintf(os.Stderr, "Schema mismatch: detected v%d, app supports v%d\n",
			res.DetectedVersion, storage.CurrentSchemaVersion)
		if diff := formatDiff(res); diff != "" {
			fmt.Fprintln(os.Stderr, diff)
		}
		return ExitMismatch, nil
	}
}

func main() {
	var (
		dbPath = flag.String("db-path", "little_timer.db", "Path to the SQLite database file to inspect")
	)
	flag.Parse()

	code, err := run(*dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "migrate: %v\n", err)
	}
	os.Exit(code)
}