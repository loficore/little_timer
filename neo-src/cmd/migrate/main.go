// Command little-timer-migrate —— DB 迁移验证 CLI。
//
// 本工具不修改 SQLite 文件；它只检查既有数据库，报告 schema 与 Go 构建
// 是否兼容、是否需要跑迁移阶梯，或与 Go schema 差异大到无法迁移
// （损坏 / 拿错 app 的库 / 手改过的库）。
//
// 退出码（与 CLI 契约一致）：
//
//	0 —— schema 兼容（v8，全部必需表 + 列都在）。
//	1 —— schema 不匹配 / 未知 schema / 读错误。
//	2 —— 需要 schema 迁移（v0–v7，可升级到 v8）。
//
// 用法：
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

// 退出码 —— 由 main 以 `os.Exit(...)` 暴露；测试通过拦截 `os.Exit` 或
// 子进程调用来断言。
const (
	ExitCompatible    = 0
	ExitMismatch      = 1
	ExitMigrationReqd = 2
)

// expectedSchema 描述 Go 构建理解的 v8 schema 形状。取自
// `internal/storage/migration.go`。列顺序对 diff 不重要；比较前两边都排序。
//
// 这份重复定义是故意的 —— migrate 工具必须能跑在当前 app 构建打不开的
// 库上（如迁移前的 DB），所以不能依赖 internal/storage 的运行时状态。
// schema 形状即规范；migration.go 若与本列表漂移，两边必须一起更新。
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

// expectedRequiredTables 是 Go 运行时正常启动所需的表子集。
// `schema_version` 有意排除 —— 它缺失本身就是强烈的“旧 schema”信号，
// 我们想报成 exit 2 而不是 exit 1。
var expectedRequiredTables = []string{
	"health_check",
	"habit_sets",
	"habits",
	"sessions",
	"timer_sessions",
	"settings",
	"backup_config",
}

// inspectResult 是单次验证运行的结构化结果。把结果封装起来，函数就能
// 单元测试而不用碰 `os.Exit`。
type inspectResult struct {
	DetectedVersion int      // schema_version 表不存在时为 0
	FoundTables     []string // 全部 sqlite_master type='table' 行
	MissingTables   []string // v8 必需但缺失的表
	ExtraTables     []string // 存在但不在 expectedSchema 里的表
	MissingColumns  []string // 逐表："table.col" 未找到
	SchemaErr       error    // I/O / 解析错误（若有）
}

// Compatible 报告结果是否代表干净的 v8 schema。
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

// MigrationRequired 报告 schema 是否比 v8 旧、但坏到还能向前迁移。
//
// 分类规则：
//
//   - 检出版本 > 0 且 < v8 → 需要迁移（明确情况）。
//   - 检出版本 == 0（无 schema_version 行）且至少存在一张 v8 形状的表
//     → 需要迁移（早于 schema_version 机制建立的 v3 之前的库）。
//   - 检出版本 == 0 且一张表都没有 → 需要迁移（全新库，需要初始 schema）。
//   - 检出版本 == 0、有表但没有一张是 v8 形状 → 不需要迁移；那是未知 /
//     拿错 app 的库，应报不匹配（exit 1）。
func (r *inspectResult) MigrationRequired() bool {
	if r.SchemaErr != nil {
		return false
	}
	if r.DetectedVersion > 0 && r.DetectedVersion < storage.CurrentSchemaVersion {
		return true
	}
	if r.DetectedVersion == 0 && len(r.FoundTables) == 0 {
		// 空库（或刚 touch 的文件）：需要初始 schema。
		return true
	}
	if r.DetectedVersion == 0 && len(r.FoundTables) > 0 {
		// 任一 v8 形状的表都说明我们认得这条 schema 血脉。
		for _, t := range r.FoundTables {
			if _, ok := expectedSchema[t]; ok {
				return true
			}
		}
	}
	return false
}

// inspect 以只读方式打开 SQLite DB 并计算 schema 检查结果。DB 不必能以
// 写模式打开（例如测试中途的权限错误）—— 只读就够 introspect schema 了。
func inspect(dbPath string) (*inspectResult, error) {
	// _query_only=1 是双保险：即使 mode=ro 未被遵守，也强制连接拒写。
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

	if _, err := db.Exec(`SELECT 1 FROM schema_version LIMIT 1;`); err == nil {
		var v sql.NullInt64
		if scanErr := db.QueryRow(`SELECT MAX(version) FROM schema_version;`).Scan(&v); scanErr != nil {
			return nil, fmt.Errorf("read schema_version: %w", scanErr)
		}
		if v.Valid {
			res.DetectedVersion = int(v.Int64)
		}
	}

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
			// 要么是没见过的 / app 私有的表；标记但不视为致命。
			res.ExtraTables = append(res.ExtraTables, t)
		}
	}

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

// readColumns 按声明顺序返回 table 的列名。
func readColumns(db *sql.DB, table string) ([]string, error) {
	// PRAGMA 不能用 `?` 参数化，所以用 fmt.Sprintf 拼语句。`table` 来自
	// 我们自己的 v8 名字列表或 sqlite_master，两者都已可信；想在这里注入
	// 得先让 DB 本身被攻破。
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

// 报告辅助函数。

// formatDiff 向 stderr 打印人可读的 schema diff。
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

// run 是可测试的内部主循环。返回选定的退出码，测试无需起子进程即可断言。
func run(dbPath string) (int, error) {
	if dbPath == "" {
		return ExitMismatch, errors.New("--db-path is required")
	}
	if _, err := os.Stat(dbPath); err != nil {
		if os.IsNotExist(err) {
			// 文件缺失是规范里的“全新启动”场景；按“需要迁移”处理，
			// 让运维知道跑一次 app 即可初始化 schema。
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
		// 检出版本 > v8，或版本是 v8 但列/表对不上。无论哪种，schema
		// 差异都大到无法干净地向前迁移。
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
