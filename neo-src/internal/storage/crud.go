// Package storage — settings row CRUD.
//
// Port of `src/storage/storage_crud.zig` (little_timer), specifically the
// saveSettings / loadSettings pair.  Backup-config encryption lives in
// internal/storage/backup in this port — credential encryption needs the
// secret-storage helper that hasn't been ported yet, so we leave the
// encrypted-column handling for a later wave and keep this file focused on
// the settings round-trip.
package storage

import (
	"database/sql"
	"errors"
	"fmt"

	"little-timer/internal/domain"
)

// CrudError mirrors `pub const CrudError = error{...}` in storage_crud.zig.
type CrudError string

const (
	ErrSettingsNotFound   CrudError = "settings not found"
	ErrSettingsSaveFailed CrudError = "settings save failed"
	ErrQueryFailed        CrudError = "query failed"
	ErrCrudNoDatabase     CrudError = "database open failed"
)

func (e CrudError) Error() string { return string(e) }

// SettingsRow is the Go port of `pub const SettingsRow = struct {...}` in
// storage_crud.zig.  It's a low-level view of a single `settings` row — the
// higher-level domain.SettingsConfig is what callers actually pass in.
type SettingsRow struct {
	ID                    int64
	Timezone              int8
	Language              string
	DefaultMode           string
	ThemeMode             string
	Wallpaper             string
	DurationSeconds       int64
	CountdownLoop         bool
	CountdownLoopCount    int64
	CountdownLoopInterval int64
	StopwatchMaxSeconds   int64
	LogLevel              string
	LogEnableTimestamp    bool
	LogTickInterval       int64
}

// CrudManager owns the *sql.DB handle for settings row reads + writes.
type CrudManager struct {
	db *sql.DB
}

// NewCrudManager returns an empty CrudManager.  Mirrors
// `CrudManager.init(allocator, null)`.
func NewCrudManager() *CrudManager {
	return &CrudManager{}
}

// SetDB attaches a *sql.DB.  Mirrors `crud_manager.db = self.db` in the
// Zig SqliteManager.open.
func (c *CrudManager) SetDB(db *sql.DB) { c.db = db }

// saveSettingsSQL is the UPSERT statement, byte-for-byte from
// storage_crud.zig:saveSettings.  Note the trailing space before `wallpaper`
// — preserved from the Zig source.
const saveSettingsSQL = `INSERT OR REPLACE INTO settings (id, timezone, language, default_mode, theme_mode, wallpaper, duration_seconds, countdown_loop, countdown_loop_count, countdown_loop_interval, stopwatch_max_seconds, log_level, log_enable_timestamp, log_tick_interval) VALUES (1, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);`

// SaveSettings persists a SettingsConfig to the settings row.
//
// Mirrors `pub fn saveSettings(self, config)`.  Translation notes:
//
//   - Zig DefaultMode enum → Go int constant + String() lookup.
//   - Zig `bool` is stored as INTEGER 0/1 by SQLite; the column is BOOLEAN
//     NOT NULL DEFAULT 0/1 in the schema.  We write 0/1 explicitly.
func (c *CrudManager) SaveSettings(config domain.SettingsConfig) error {
	if c.db == nil {
		return ErrCrudNoDatabase
	}

	defaultModeStr := config.Basic.DefaultMode.String()
	themeMode := config.Basic.ThemeMode
	if themeMode == "" {
		themeMode = "dark" // matches schema DEFAULT 'dark' and Zig defaults.
	}
	logLevel := config.Logging.Level
	if logLevel == "" {
		logLevel = "INFO"
	}
	lang := config.Basic.Language
	if lang == "" {
		lang = "ZH"
	}

	_, err := c.db.Exec(saveSettingsSQL,
		config.Basic.Timezone,
		lang,
		defaultModeStr,
		themeMode,
		config.Basic.Wallpaper,
		int64(config.ClockDefaults.Countdown.DurationSeconds),
		boolToInt(config.ClockDefaults.Countdown.Loop),
		int64(config.ClockDefaults.Countdown.LoopCount),
		int64(config.ClockDefaults.Countdown.LoopIntervalSeconds),
		int64(config.ClockDefaults.Stopwatch.MaxSeconds),
		logLevel,
		boolToInt(config.Logging.EnableTimestamp),
		config.Logging.TickIntervalMs,
	)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrSettingsSaveFailed, err)
	}
	return nil
}

// LoadSettings reads the settings row and returns a populated SettingsConfig.
//
// Mirrors `pub fn loadSettings(self, allocator)`.  When no row exists, the
// Zig source returns `SettingsConfig{}` (zero-valued); we return
// NewDefaultSettingsConfig() instead — same effect for any consumer that
// only reads fields, and safer for callers that forget to apply defaults.
func (c *CrudManager) LoadSettings() (domain.SettingsConfig, error) {
	if c.db == nil {
		return domain.SettingsConfig{}, ErrCrudNoDatabase
	}

	const query = `SELECT timezone, language, default_mode, theme_mode, COALESCE(wallpaper, ''), duration_seconds, countdown_loop, countdown_loop_count, countdown_loop_interval, stopwatch_max_seconds, log_level, log_enable_timestamp, log_tick_interval FROM settings WHERE id = 1;`

	var (
		timezone              int64
		language              string
		defaultModeStr        string
		themeMode             string
		wallpaper             string
		durationSeconds       int64
		countdownLoop         bool
		countdownLoopCount    int64
		countdownLoopInterval int64
		stopwatchMaxSeconds   int64
		logLevel              string
		logEnableTimestamp    bool
		logTickInterval       int64
	)
	err := c.db.QueryRow(query).Scan(
		&timezone, &language, &defaultModeStr, &themeMode, &wallpaper,
		&durationSeconds, &countdownLoop, &countdownLoopCount,
		&countdownLoopInterval, &stopwatchMaxSeconds, &logLevel,
		&logEnableTimestamp, &logTickInterval,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.NewDefaultSettingsConfig(), nil
		}
		return domain.SettingsConfig{}, fmt.Errorf("%w: %w", ErrQueryFailed, err)
	}

	return domain.SettingsConfig{
		Basic: domain.SettingsBasic{
			Timezone:    int8(timezone),
			Language:    language,
			DefaultMode: parseDefaultMode(defaultModeStr),
			ThemeMode:   themeMode,
			Wallpaper:   wallpaper,
		},
		ClockDefaults: domain.ClockTaskConfig{
			Countdown: domain.CountdownConfig{
				DurationSeconds:     uint64(durationSeconds),
				Loop:                countdownLoop,
				LoopCount:           uint32(countdownLoopCount),
				LoopIntervalSeconds: uint64(countdownLoopInterval),
			},
			Stopwatch: domain.StopwatchConfig{
				MaxSeconds: uint64(stopwatchMaxSeconds),
			},
		},
		Logging: domain.SettingsLogging{
			Level:           logLevel,
			EnableTimestamp: logEnableTimestamp,
			TickIntervalMs:  logTickInterval,
		},
	}, nil
}

// LoadSettingsRow returns the raw row view (kept for parity with Zig).
func (c *CrudManager) LoadSettingsRow() (SettingsRow, error) {
	if c.db == nil {
		return SettingsRow{}, ErrCrudNoDatabase
	}

	const query = `SELECT id, timezone, language, default_mode, theme_mode, COALESCE(wallpaper, ''), duration_seconds, countdown_loop, countdown_loop_count, countdown_loop_interval, stopwatch_max_seconds, log_level, log_enable_timestamp, log_tick_interval FROM settings WHERE id = 1;`

	var row SettingsRow
	err := c.db.QueryRow(query).Scan(
		&row.ID, &row.Timezone, &row.Language, &row.DefaultMode,
		&row.ThemeMode, &row.Wallpaper, &row.DurationSeconds,
		&row.CountdownLoop, &row.CountdownLoopCount,
		&row.CountdownLoopInterval, &row.StopwatchMaxSeconds,
		&row.LogLevel, &row.LogEnableTimestamp, &row.LogTickInterval,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return SettingsRow{}, ErrSettingsNotFound
		}
		return SettingsRow{}, fmt.Errorf("%w: %w", ErrQueryFailed, err)
	}
	return row, nil
}

// boolToInt mirrors Zig's `@intFromBool`.  SQLite BOOLEAN stores 0/1.
func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// parseDefaultMode converts the persisted TEXT back to the DefaultMode enum.
// Mirrors the Zig if/else that checked for "countdown" and assumed
// "stopwatch" otherwise.
func parseDefaultMode(s string) domain.DefaultMode {
	if s == "countdown" {
		return domain.DefaultModeCountdown
	}
	// Zig treats everything-not-"countdown" as stopwatch; preserve that.
	// The schema's CHECK also allows "world_clock" but no consumer uses it.
	return domain.DefaultModeStopwatch
}