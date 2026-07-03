// Package storage — health check + integrity check.
//
// Port of `src/storage/storage_health.zig` (little_timer).
package storage

import (
	"database/sql"
	"errors"
	"fmt"
)

// HealthCheckError mirrors `pub const HealthCheckError = error{...}` in
// storage_health.zig.
type HealthCheckError string

const (
	ErrIntegrityCheckFailed HealthCheckError = "integrity check failed"
	ErrHealthCheckFailed    HealthCheckError = "health check failed"
)

func (e HealthCheckError) Error() string { return string(e) }

// HealthCheckInfo mirrors `pub const HealthCheckInfo = struct { status,
// last_check, record_count }`.  Go strings are immutable so no allocator is
// needed; ownership is the caller's.
type HealthCheckInfo struct {
	Status      string
	LastCheck   string
	RecordCount int64
}

// HealthCheckManager is the Go port of `pub const HealthCheckManager =
// struct { db, allocator }`.  It does not own the *sql.DB; the SqliteManager
// wires it via SetDB.
type HealthCheckManager struct {
	db *sql.DB
}

// NewHealthCheckManager returns an empty manager.  Mirrors
// `HealthCheckManager.init(allocator, null)`.
func NewHealthCheckManager() *HealthCheckManager {
	return &HealthCheckManager{}
}

// SetDB attaches a *sql.DB.  Mirrors `health_manager.db = self.db` in the
// Zig open() implementation.
func (h *HealthCheckManager) SetDB(db *sql.DB) { h.db = db }

// Initialize ensures a row exists in `health_check` (id=1).  Mirrors the
// Zig `fn initialize`.
func (h *HealthCheckManager) Initialize() error {
	if h.db == nil {
		return ErrHealthCheckFailed
	}
	var count int64
	if err := h.db.QueryRow(`SELECT COUNT(*) FROM health_check WHERE id = 1;`).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	_, err := h.db.Exec(`INSERT INTO health_check (id, status, record_count) VALUES (1, 'healthy', 0);`)
	return err
}

// PerformCheck runs PRAGMA integrity_check and refreshes the health row.
//
// Mirrors the Zig `fn performCheck`.  Returns ErrIntegrityCheckFailed when
// integrity_check yields anything other than "ok" — same as Zig.
func (h *HealthCheckManager) PerformCheck() error {
	if h.db == nil {
		return ErrHealthCheckFailed
	}

	var result string
	if err := h.db.QueryRow(`PRAGMA integrity_check;`).Scan(&result); err != nil {
		return fmt.Errorf("%w: %w", ErrHealthCheckFailed, err)
	}
	if result != "ok" {
		return fmt.Errorf("%w: %s", ErrIntegrityCheckFailed, result)
	}

	return h.UpdateRecord()
}

// UpdateRecord counts `sessions` rows and overwrites the health_check row.
// Mirrors `fn updateRecord` in storage_health.zig.
func (h *HealthCheckManager) UpdateRecord() error {
	if h.db == nil {
		return ErrHealthCheckFailed
	}
	var count int64
	if err := h.db.QueryRow(`SELECT COUNT(*) FROM sessions;`).Scan(&count); err != nil {
		return err
	}
	_, err := h.db.Exec(
		`INSERT OR REPLACE INTO health_check (id, last_check, status, record_count) VALUES (1, CURRENT_TIMESTAMP, 'healthy', ?);`,
		count,
	)
	return err
}

// GetInfo reads the current health_check row.  Mirrors `fn getInfo`; when
// no row exists, returns a sentinel "unknown" / "never" record (same as
// Zig).
func (h *HealthCheckManager) GetInfo() (HealthCheckInfo, error) {
	if h.db == nil {
		return HealthCheckInfo{}, ErrHealthCheckFailed
	}
	var info HealthCheckInfo
	err := h.db.QueryRow(
		`SELECT status, last_check, record_count FROM health_check WHERE id = 1;`,
	).Scan(&info.Status, &info.LastCheck, &info.RecordCount)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return HealthCheckInfo{
				Status:      "unknown",
				LastCheck:   "never",
				RecordCount: 0,
			}, nil
		}
		return HealthCheckInfo{}, err
	}
	return info, nil
}

// IsHealthy returns true iff the persisted health_check row status is
// "healthy".  Mirrors `fn isHealthy`.
func (h *HealthCheckManager) IsHealthy() (bool, error) {
	info, err := h.GetInfo()
	if err != nil {
		return false, err
	}
	return info.Status == "healthy", nil
}

// PerformDeepCheck is a Go port of `fn performDeepCheck` — runs PerformCheck
// and gathers extra counts from sessions / settings / health_check.  Mirrors
// the Zig helper without the allocator dance (Go strings are immutable).
func (h *HealthCheckManager) PerformDeepCheck() (HealthCheckInfo, error) {
	if err := h.PerformCheck(); err != nil {
		return HealthCheckInfo{}, err
	}

	var (
		sessionCount  int64
		settingsCount int64
		healthRecords int64
		lastCheck     sql.NullString
	)
	err := h.db.QueryRow(
		`SELECT
			(SELECT COUNT(*) FROM sessions),
			(SELECT COUNT(*) FROM settings),
			(SELECT COUNT(*) FROM health_check),
			(SELECT last_check FROM health_check WHERE id = 1)`,
	).Scan(&sessionCount, &settingsCount, &healthRecords, &lastCheck)
	if err != nil {
		return h.GetInfo()
	}

	_ = healthRecords // mirrors `_health_records` debug-only reference in Zig.
	status := "healthy"
	last := "never"
	if lastCheck.Valid {
		last = lastCheck.String
	}
	return HealthCheckInfo{
		Status:      status,
		LastCheck:   last,
		RecordCount: sessionCount + settingsCount,
	}, nil
}