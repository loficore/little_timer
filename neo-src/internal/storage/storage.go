// Package storage —— 把 SqliteManager 与高层 domain 类型组合起来的门面。
//
// 规范要求一个薄门面（sqlite.go 已协调各子模块）。本文件补充领域形状的
// 便捷方法（`SaveSettings(domain.SettingsConfig)` /
// `LoadSettings() domain.SettingsConfig`），让 http 层调用方不必穿过 .Crud() 层层取数。
package storage

import "little-timer/internal/domain"

// SaveSettings 把 SettingsConfig 持久化到 settings 行。
func (m *SqliteManager) SaveSettings(config domain.SettingsConfig) error {
	return m.crud.SaveSettings(config)
}

// LoadSettings 读取 settings 行并返回填充好的 SettingsConfig。
func (m *SqliteManager) LoadSettings() (domain.SettingsConfig, error) {
	return m.crud.LoadSettings()
}

// PerformHealthCheck 是 Health().PerformCheck() 的薄包装。
func (m *SqliteManager) PerformHealthCheck() error {
	return m.health.PerformCheck()
}

// IsHealthy 当持久化的 health_check 行显示 "healthy" 时返回 true。
func (m *SqliteManager) IsHealthy() (bool, error) {
	return m.health.IsHealthy()
}

// GetHealthInfo 返回当前 health_check 行。
func (m *SqliteManager) GetHealthInfo() (HealthCheckInfo, error) {
	return m.health.GetInfo()
}
