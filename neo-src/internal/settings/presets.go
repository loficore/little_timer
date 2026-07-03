// Package settings — preset manager.
//
// Port of `src/settings/settings_presets.zig` (little_timer).
//
// The Zig source marks the entire module as deprecated and ships only
// stubs: every method is a no-op and `PresetsManager.add()` is
// intentionally inert.  We mirror that behaviour verbatim — the
// settings layer parses preset-shaped JSON in
// `SettingsManager.parseSettingsFromJson` directly without going through
// this manager, so the API exists for parity only.
//
// If a future wave re-enables preset persistence, the implementation
// hooks here without touching the public surface.
package settings

import (
	"little-timer/internal/domain"
)

// PresetsError mirrors `pub const PresetsError = error{...}`.  Kept for
// parity even though no call site currently returns it.
type PresetsError string

const (
	ErrPresetInvalidName PresetsError = "invalid preset name"
	ErrPresetNotFound    PresetsError = "preset not found"
)

func (e PresetsError) Error() string { return string(e) }

// MaxPresetCount mirrors the Zig `max_count: usize = 999` field.  Exposed
// so the validator and settings manager can use the same bound.
const MaxPresetCount = 999

// PresetsManager mirrors `pub const PresetsManager = struct { ... }`.
//
// The Zig source sets `max_count = 999` and exposes Add/Remove/Get/Count
// as no-ops.  We preserve that contract — the struct exists so callers
// can hold a "I have presets" reference, but the methods don't store
// anything.  Persistence of presets happens via the `presets` field on
// the JSON payload in `SettingsManager.parseSettingsFromJson`.
type PresetsManager struct {
	maxCount int
}

// NewPresetsManager returns a PresetsManager with the default limit.
func NewPresetsManager() *PresetsManager {
	return &PresetsManager{maxCount: MaxPresetCount}
}

// Add is a no-op.  Mirrors `pub fn add(_: *PresetsManager, preset)`.
func (*PresetsManager) Add(domain.TimerPreset) error { return nil }

// Remove is a no-op.  Mirrors `pub fn remove(_: *PresetsManager, _: usize)`.
func (*PresetsManager) Remove(int) {}

// Get always returns nil.  Mirrors `pub fn get(...) ?*const TimerPreset`.
func (*PresetsManager) Get(int) *domain.TimerPreset { return nil }

// GetAll returns an empty slice.  Mirrors `pub fn getAll(...) []const TimerPreset`.
func (*PresetsManager) GetAll() []domain.TimerPreset { return nil }

// GetByName always returns nil.  Mirrors `pub fn getByName(...) ?*const TimerPreset`.
func (*PresetsManager) GetByName(string) *domain.TimerPreset { return nil }

// Count always returns 0.  Mirrors `pub fn count(...) usize`.
func (*PresetsManager) Count() int { return 0 }

// Clear is a no-op.  Mirrors `pub fn clear(...)`.
func (*PresetsManager) Clear() {}

// Deinit is a no-op.  Mirrors `pub fn deinit(...)`.
func (*PresetsManager) Deinit() {}

// MaxCount returns the configured maximum (default 999).
func (p *PresetsManager) MaxCount() int {
	if p == nil {
		return MaxPresetCount
	}
	return p.maxCount
}