// Package storage —— 健康检查 + 完整性检查。
package storage

import (
	"database/sql"
	"errors"
	"fmt"
)

// HealthCheckError 是健康检查的类型化哨兵错误。
type HealthCheckError string

const (
	ErrIntegrityCheckFailed HealthCheckError = "integrity check failed"
	ErrHealthCheckFailed    HealthCheckError = "health check failed"
)

func (e HealthCheckError) Error() string { return string(e) }

// HealthCheckInfo 是 health_check 行的快照。
type HealthCheckInfo struct {
	Status      string
	LastCheck   string
	RecordCount int64
}

// HealthCheckManager 执行健康检查。它不持有 *sql.DB；
// 由 SqliteManager 通过 SetDB 接线。
type HealthCheckManager struct {
	db *sql.DB
}

// NewHealthCheckManager 返回空 manager。
func NewHealthCheckManager() *HealthCheckManager {
	return &HealthCheckManager{}
}

// SetDB 接入 *sql.DB。
func (h *HealthCheckManager) SetDB(db *sql.DB) { h.db = db }

// Initialize 确保 `health_check` 中存在 id=1 行。
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

// PerformCheck 运行 PRAGMA integrity_check 并刷新 health 行。
// integrity_check 结果不是 "ok" 时返回 ErrIntegrityCheckFailed。
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

// UpdateRecord 统计 `sessions` 行数并覆盖写 health_check 行。
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

// GetInfo 读取当前 health_check 行。没有该行时返回哨兵值
// "unknown" / "never"。
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

// IsHealthy 当持久化的 health_check 行状态为 "healthy" 时返回 true。
func (h *HealthCheckManager) IsHealthy() (bool, error) {
	info, err := h.GetInfo()
	if err != nil {
		return false, err
	}
	return info.Status == "healthy", nil
}

// PerformDeepCheck 运行 PerformCheck 并额外统计
// sessions / settings / health_check 的行数。
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

	_ = healthRecords // 不属于返回的 info
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
