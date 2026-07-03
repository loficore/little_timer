// Package storage — facade combining SqliteManager with the higher-level
// domain types.
//
// The spec calls for a thin facade (sqlite.go already coordinates the
// sub-modules).  This file adds the domain-shaped convenience methods
// (`SaveSettings(domain.SettingsConfig)` / `LoadSettings() domain.SettingsConfig`)
// so callers from the http layer don't have to drill through .Crud().
package storage

import "little-timer/internal/domain"

// SaveSettings persists a SettingsConfig to the settings row.
func (m *SqliteManager) SaveSettings(config domain.SettingsConfig) error {
	return m.crud.SaveSettings(config)
}

// LoadSettings reads the settings row and returns a populated SettingsConfig.
func (m *SqliteManager) LoadSettings() (domain.SettingsConfig, error) {
	return m.crud.LoadSettings()
}

// PerformHealthCheck is a thin wrapper around Health().PerformCheck().
func (m *SqliteManager) PerformHealthCheck() error {
	return m.health.PerformCheck()
}

// IsHealthy returns true iff the persisted health_check row says "healthy".
func (m *SqliteManager) IsHealthy() (bool, error) {
	return m.health.IsHealthy()
}

// GetHealthInfo returns the current health_check row.
func (m *SqliteManager) GetHealthInfo() (HealthCheckInfo, error) {
	return m.health.GetInfo()
}