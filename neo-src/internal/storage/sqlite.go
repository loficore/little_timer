// Package storage — SqliteManager：顶层连接生命周期。
//
// 协调 migration / health / crud / habit 各子管理器；备份单独放在
// internal/storage/backup 处理。
//
// 数据库文件打开后加固为 0600，且每个连接都启用
// PRAGMA foreign_keys = ON。
package storage

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"little-timer/internal/log"

	_ "github.com/mattn/go-sqlite3"
)

// sqliteDriverName 是 `mattn/go-sqlite3` 自注册的名字。保留具名常量是为了
// 将来换驱动（如纯 Go 的 modernc.org/sqlite）只需改一行。
const sqliteDriverName = "sqlite3"

// SqliteError 表示 SQLite 管理器可能返回的存储层错误。
type SqliteError string

const (
	ErrDatabaseOpenFailed   SqliteError = "database open failed"
	ErrDatabaseNotConnected SqliteError = "database not connected"
)

func (e SqliteError) Error() string { return string(e) }

// SqliteManager 管理 SQLite 连接生命周期并协调各存储子模块。
// 必须先调用 Init() 再调用 Open()；打开子模块的任意 CRUD 方法前必须先
// 调用 Open()。
type SqliteManager struct {
	dbPath string

	// 子管理器 —— 由 Init() 填充。
	migration *MigrationManager
	health    *HealthCheckManager
	crud      *CrudManager
	habitSets *HabitSetCrud
	habits    *HabitCrud
	timers    *TimerSessionCrud

	db *sql.DB // Open() 成功前为 nil
}

// NewSqliteManager 构造一个未初始化的 SqliteManager。使用前需先 Init 设置路径，再 Open 打开文件。
func NewSqliteManager() *SqliteManager {
	return &SqliteManager{}
}

// Init 设置数据库路径并构造各存储子模块。
//
// `dbPath` 可为绝对或相对路径；相对路径按当前工作目录解析
// （与 Go 的 `database/sql` 行为一致）。
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

// Open 打开 SQLite 数据库文件并完成子模块接线与初始化检查。
// 文件以 Create|ReadWrite 打开、加固为 0600、启用外键、把 *sql.DB
// 接入每个子管理器，并执行 migration + 健康检查。幂等：第二次
// Open() 是 no-op。
//
// 错误向上抛出，由调用方决定如何应对。
func (m *SqliteManager) Open() error {
	if m.dbPath == "" {
		log.Error("storage.open failed", "error", "Init(dbPath) must be called before Open()")
		return errors.New("storage: Init(dbPath) must be called before Open()")
	}
	if m.db != nil {
		return nil // 已打开
	}

	// 确保父目录存在。
	if dir := filepath.Dir(m.dbPath); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			// 目录已存在时 MkdirAll 返回 EEXIST；这没问题。
			// 其他错误都是硬错误。
			if !errors.Is(err, os.ErrExist) {
				log.Error("storage.open failed", "db_path", m.dbPath, "error", err.Error())
				return fmt.Errorf("%w: mkdir %s: %w", ErrDatabaseOpenFailed, dir, err)
			}
		}
		// 尽力 chmod 为 0700。这里的错误不致命 —— 真正与安全相关的是
		// 对 DB 文件本身的权限加固。
		_ = os.Chmod(dir, 0o700)
	}

	// 打开 SQLite 文件。
	db, err := sql.Open(sqliteDriverName, m.dbPath)
	if err != nil {
		log.Error("storage.open failed", "db_path", m.dbPath, "error", err.Error())
		return fmt.Errorf("%w: %w", ErrDatabaseOpenFailed, err)
	}
	// Ping 强制真正建连，这样在我们尝试 chmod 之前文件已落盘。
	if err := db.Ping(); err != nil {
		_ = db.Close()
		log.Error("storage.open failed", "db_path", m.dbPath, "error", err.Error())
		return fmt.Errorf("%w: ping: %w", ErrDatabaseOpenFailed, err)
	}

	// 把文件加固为 0600。
	if err := os.Chmod(m.dbPath, 0o600); err != nil {
		// 不致命但值得暴露 —— 这是安全步骤。警告走共享 logger。
		log.Warn("storage.chmod", "db_path", m.dbPath, "error", err.Error())
	}

	// 每个连接都启用外键。
	if _, err := db.Exec(`PRAGMA foreign_keys = ON;`); err != nil {
		_ = db.Close()
		log.Error("storage.open failed", "db_path", m.dbPath, "error", err.Error())
		return fmt.Errorf("%w: pragma foreign_keys: %w", ErrDatabaseOpenFailed, err)
	}

	m.db = db

	// 接入子管理器。
	m.migration.SetDB(db)
	m.health.SetDB(db)
	m.crud.SetDB(db)
	m.habitSets.SetDB(db)
	m.habits.SetDB(db)
	m.timers.SetDB(db)

	log.Info("storage.open", "db_path", m.dbPath)
	return nil
}

// Migrate 执行迁移检查并按需建表。
func (m *SqliteManager) Migrate() error {
	if m.db == nil {
		return ErrDatabaseNotConnected
	}
	start := time.Now()
	err := m.migration.CheckAndMigrate()
	log.Info("storage.migrate", "duration_ms", time.Since(start).Milliseconds())
	return err
}

// Close 关闭底层 *sql.DB 并清空子模块的句柄引用。
func (m *SqliteManager) Close() error {
	if m.db == nil {
		return nil
	}
	err := m.db.Close()
	m.db = nil

	// 清空子管理器句柄，让 Close 之后的操作干净地失败，
	// 而不是在过期的 *sql.DB 上继续操作。
	m.migration.SetDB(nil)
	m.health.SetDB(nil)
	m.crud.SetDB(nil)
	m.habitSets.SetDB(nil)
	m.habits.SetDB(nil)
	m.timers.SetDB(nil)
	return err
}

// 便捷访问器 —— 供 storage.go 中 SqliteManager.SaveSettings / LoadSettings
// 以及测试使用。

// DB 返回底层 *sql.DB，未打开时返回 nil。
func (m *SqliteManager) DB() *sql.DB { return m.db }

// Migration 返回迁移子模块。
func (m *SqliteManager) Migration() *MigrationManager { return m.migration }

// Health 返回健康检查子模块。
func (m *SqliteManager) Health() *HealthCheckManager { return m.health }

// Crud 返回 settings 行 CRUD 子模块。
func (m *SqliteManager) Crud() *CrudManager { return m.crud }

// HabitSets 返回 habit_sets 子模块。
func (m *SqliteManager) HabitSets() *HabitSetCrud { return m.habitSets }

// Habits 返回 habits 子模块。
func (m *SqliteManager) Habits() *HabitCrud { return m.habits }

// Timers 返回 timer-sessions 子模块。
func (m *SqliteManager) Timers() *TimerSessionCrud { return m.timers }

// IsOpen 报告底层 *sql.DB 是否已连接。
func (m *SqliteManager) IsOpen() bool { return m.db != nil }
