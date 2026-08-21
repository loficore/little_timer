// 迁移验证 CLI 的测试。
//
// 我们直接驱动 `run` 函数覆盖三个契约用例（兼容 / 不匹配 / 需要迁移），
// 外加一次子进程调用，确认二进制确实以相同退出码退出。三个契约测试都经
// internal/storage（运行中 server 用的同一代码路径）构建真实 SQLite DB，
// 再删表 / 改表 / 原样保留来构造目标场景。
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

// buildCurrentSchemaDB 在给定路径开一个全新 SQLite DB、跑 v8 迁移
// （全部必需表 + 列就位）后关闭连接。返回该路径。这就是工具必须识别为
// 兼容的“快乐路径”。
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

// buildOldSchemaDB 构造一个模仿迁移前 v0–v2 数据库的 SQLite 文件：几张
// v8 表、没有 schema_version 行，并故意给 `habits` 一个很小的列集合
// （缺 goal_count / wallpaper），以覆盖列 diff 代码路径。
func buildOldSchemaDB(t *testing.T, path string) {
	t.Helper()
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		t.Fatalf("remove: %v", err)
	}
	dsn := filepath.Clean(path) // 普通文件 DSN，可写
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
		// 注意：没有 schema_version、没有 timer_sessions、没有 settings、
		// 没有 backup_config —— v8 形状减去几块。
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			t.Fatalf("exec %q: %v", s, err)
		}
	}
}

// buildIncompatibleSchemaDB 构造一个 schema 不匹配任何可识别版本的 DB：
// 一张怪表、没有必需的 v8 表。这就是“未知 / 不兼容”用例 → exit 1。
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

// 直调测试 —— 驱动 run() 并断言它要交给 os.Exit 的退出码。这是主要的
// 契约测试。

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

// run() 辅助函数的额外契约测试。覆盖三个具名契约测试没碰到的角落用例。

func TestRun_MissingFile_Exits2(t *testing.T) {
	// 文件缺失按“全新启动 —— 需要迁移”处理（规范用例 1），
	// 所以 exit 2 而不是 exit 1。
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
	// 全新 SQLite 文件（只被 `sqlite3` touch 过）零张表，但仍是合法的
	// 只读目标。inspect 应报告 DetectedVersion=0、没有缺列 diff，
	// FoundTables 应为空。
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
	// 我们构造的旧库在 habits 上故意缺 wallpaper + goal_count；
	// 检查器应把这两列标出来。
	if len(res.MissingColumns) == 0 {
		t.Errorf("MissingColumns = [], want non-empty diff for old-schema DB")
	}
	if res.Compatible() {
		t.Errorf("old-schema: Compatible() = true, want false")
	}
}

// 子进程 smoke 测试 —— 确认真实二进制以契约码退出。构建产物放在
// ./cmd/migrate 自己的目录里，CI 的 `go test ./cmd/migrate/...` 就不需要
// 额外的 `go build` 步骤。

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

// buildBinary 把 migrate 工具编译进临时二进制并返回其路径。传的是包路径
// （相对 module root），测试不依赖 cwd。
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

// findModuleRoot 从测试的 CWD 向上走直到找到 go.mod。migrate 工具坐在
// cmd/migrate，但 `go build` 要的是 module root（含 go.mod 的目录）。
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
		// exec.ExitError 通过 ExitCode 暴露退出码。
		if ee, ok := err.(*exec.ExitError); ok {
			t.Logf("binary stderr: %s", strings.TrimSpace(string(out)))
			return ee.ExitCode()
		}
		t.Fatalf("binary: %v\n%s", err, out)
	}
	return 0
}
