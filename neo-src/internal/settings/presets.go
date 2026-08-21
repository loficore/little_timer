// Package settings —— 预设 manager。
//
// 预设 manager 是有意为之的 no-op 桩：每个方法什么都不做，Add 刻意惰性。
// settings 层在 `SettingsManager.parseSettingsFromJson` 里直接解析预设形状的
// JSON，不经由本 manager，所以这个 API 只为形状兼容而存在。
//
// 若未来某轮重新启用预设持久化，实现挂在此处即可，不动公开接口。
package settings

import (
	"little-timer/internal/domain"
)

// PresetsError 为将来预留；目前没有调用点返回它。
type PresetsError string

const (
	ErrPresetInvalidName PresetsError = "invalid preset name"
	ErrPresetNotFound    PresetsError = "preset not found"
)

func (e PresetsError) Error() string { return string(e) }

// MaxPresetCount 是预设上限，与 validator 和 settings manager 共享。
const MaxPresetCount = 999

// PresetsManager 是一个方法全为 no-op 的桩。struct 存在是为了让调用方
// 能持有一个“我有预设”的引用，但什么都不存；持久化发生在
// `SettingsManager.parseSettingsFromJson` 里 JSON payload 的 `presets` 字段。
type PresetsManager struct {
	maxCount int
}

// NewPresetsManager 返回带默认上限的 PresetsManager。
func NewPresetsManager() *PresetsManager {
	return &PresetsManager{maxCount: MaxPresetCount}
}

// Add 是 no-op。
func (*PresetsManager) Add(domain.TimerPreset) error { return nil }

// Remove 是 no-op。
func (*PresetsManager) Remove(int) {}

// Get 恒返回 nil。
func (*PresetsManager) Get(int) *domain.TimerPreset { return nil }

// GetAll 返回空 slice。
func (*PresetsManager) GetAll() []domain.TimerPreset { return nil }

// GetByName 恒返回 nil。
func (*PresetsManager) GetByName(string) *domain.TimerPreset { return nil }

// Count 恒返回 0。
func (*PresetsManager) Count() int { return 0 }

// Clear 是 no-op。
func (*PresetsManager) Clear() {}

// Deinit 是 no-op。
func (*PresetsManager) Deinit() {}

// MaxCount 返回配置的最大值（默认 999）。
func (p *PresetsManager) MaxCount() int {
	if p == nil {
		return MaxPresetCount
	}
	return p.maxCount
}
