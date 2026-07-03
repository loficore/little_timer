// Package settings — input validation.
//
// Port of `src/settings/settings_validator.zig` (little_timer).  Every
// rule from the Zig source is preserved verbatim — same names, same
// range bounds, same error semantics.  The functions return a
// `ValidationError` (a typed sentinel) rather than panicking so the
// caller can choose how to surface the failure.
//
// Mapping notes:
//
//   - Zig `ValidationError` error set → Go typed-sentinel `ValidationError`
//     with `Error()` for use with `errors.Is`/`errors.As`.
//   - `safeI8FromJson`/`safeU32FromJson`/`safeU64FromJson`/`safeI64FromJson`
//     become methods on the `Validator` zero-value type so callers can
//     `validator.SafeI8FromJson(...)` without importing the package twice.
package settings

import (
	"fmt"

	"little-timer/internal/domain"
)

// ValidationError mirrors `pub const ValidationError = error{...}`.
type ValidationError string

const (
	ErrInvalidTimezone     ValidationError = "invalid timezone"
	ErrInvalidLanguage     ValidationError = "invalid language"
	ErrInvalidDuration     ValidationError = "invalid duration"
	ErrInvalidLoopCount    ValidationError = "invalid loop count"
	ErrInvalidLoopInterval ValidationError = "invalid loop interval"
	ErrInvalidMaxSeconds   ValidationError = "invalid max seconds"
	ErrInvalidTickInterval ValidationError = "invalid tick interval"
	ErrInvalidPresetName    ValidationError = "invalid preset name"
	ErrPresetLimitExceeded  ValidationError = "preset limit exceeded"
	ErrInvalidToken         ValidationError = "invalid auth token"
)

func (e ValidationError) Error() string { return string(e) }

// Time / interval bounds — mirror constants from interface.zig.
const (
	minTimezone = -12
	maxTimezone = 14

	minLanguageLen = 1
	maxLanguageLen = 10

	minDurationSec = uint64(1)
	maxDurationSec = domain.Day // 86400

	maxLoopCount    = uint32(1000)
	maxLoopInterval = uint64(3600)

	minMaxSeconds = uint64(1)
	maxMaxSeconds = domain.Year * 365 // DEFAULT_MAX_YEAR_SECONDS

	minTickIntervalMs = domain.MinTickIntervalMs // 100
	maxTickIntervalMs = domain.MaxTickIntervalMs // 5000

	maxPresetNameLen = 64
	maxPresetCount   = 999

	minAuthTokenLen = 32
	maxAuthTokenLen = 256
)

// Validator is a zero-value struct holding the validation rules.  Exists
// for API discoverability — callers write `Validator.ValidateTimezone(tz)`
// rather than reaching for a free function.
type Validator struct{}

// New returns a Validator.  All state is in the type; the value is
// stateless, but a constructor makes future extension (e.g. pluggable
// rules) non-breaking.
func NewValidator() Validator { return Validator{} }

// -----------------------------------------------------------------------------
// Range checks.
// -----------------------------------------------------------------------------

// ValidateTimezone accepts [-12, 14].  Mirrors `pub fn validateTimezone`.
func (Validator) ValidateTimezone(tz int8) error {
	if tz < minTimezone || tz > maxTimezone {
		return fmt.Errorf("%w: %d not in [%d, %d]",
			ErrInvalidTimezone, tz, minTimezone, maxTimezone)
	}
	return nil
}

// ValidateLanguage accepts 1..10 characters.  Mirrors
// `pub fn validateLanguage`.
func (Validator) ValidateLanguage(lang string) error {
	if len := len(lang); len < minLanguageLen || len > maxLanguageLen {
		return fmt.Errorf("%w: length %d not in [%d, %d]",
			ErrInvalidLanguage, len, minLanguageLen, maxLanguageLen)
	}
	return nil
}

// ValidateDuration accepts [1, 86400].  Mirrors `pub fn validateDuration`.
func (Validator) ValidateDuration(seconds uint64) error {
	if seconds < minDurationSec || seconds > maxDurationSec {
		return fmt.Errorf("%w: %d not in [%d, %d]",
			ErrInvalidDuration, seconds, minDurationSec, maxDurationSec)
	}
	return nil
}

// ValidateLoopCount accepts [0, 1000].  0 means "infinite".
func (Validator) ValidateLoopCount(count uint32) error {
	if count > maxLoopCount {
		return fmt.Errorf("%w: %d > %d",
			ErrInvalidLoopCount, count, maxLoopCount)
	}
	return nil
}

// ValidateLoopInterval accepts [0, 3600].  Mirrors `pub fn validateLoopInterval`.
func (Validator) ValidateLoopInterval(seconds uint64) error {
	if seconds > maxLoopInterval {
		return fmt.Errorf("%w: %d > %d",
			ErrInvalidLoopInterval, seconds, maxLoopInterval)
	}
	return nil
}

// ValidateMaxSeconds accepts (0, 31_536_000].  Mirrors
// `pub fn validateMaxSeconds`.
func (Validator) ValidateMaxSeconds(maxSeconds uint64) error {
	if maxSeconds < minMaxSeconds || maxSeconds > maxMaxSeconds {
		return fmt.Errorf("%w: %d not in (%d, %d]",
			ErrInvalidMaxSeconds, maxSeconds, minMaxSeconds, maxMaxSeconds)
	}
	return nil
}

// ValidateTickInterval accepts [100, 5000] ms.
func (Validator) ValidateTickInterval(intervalMs int64) error {
	if intervalMs < minTickIntervalMs || intervalMs > maxTickIntervalMs {
		return fmt.Errorf("%w: %d not in [%d, %d]",
			ErrInvalidTickInterval, intervalMs, minTickIntervalMs, maxTickIntervalMs)
	}
	return nil
}

// ValidatePresetName accepts 1..64 characters.
func (Validator) ValidatePresetName(name string) error {
	if len := len(name); len == 0 || len > maxPresetNameLen {
		return fmt.Errorf("%w: length %d not in [1, %d]",
			ErrInvalidPresetName, len, maxPresetNameLen)
	}
	return nil
}

// ValidatePresetCount accepts [0, 999].
func (Validator) ValidatePresetCount(count int) error {
	if count > maxPresetCount {
		return fmt.Errorf("%w: %d > %d",
			ErrPresetLimitExceeded, count, maxPresetCount)
	}
	return nil
}

// ValidateAuthToken accepts [32, 256] characters.
func (Validator) ValidateAuthToken(token string) error {
	if len := len(token); len < minAuthTokenLen || len > maxAuthTokenLen {
		return fmt.Errorf("%w: length %d not in [%d, %d]",
			ErrInvalidToken, len, minAuthTokenLen, maxAuthTokenLen)
	}
	return nil
}

// -----------------------------------------------------------------------------
// Safe conversions — `safeXFromJson` ports.
// -----------------------------------------------------------------------------

// SafeI8FromJson mirrors `pub fn safeI8FromJson` — returns nil if the
// JSON-supplied integer falls outside [min, max] or doesn't fit in int8.
func (Validator) SafeI8FromJson(jsonInt int64, min, max int8) *int8 {
	if jsonInt < int64(min) || jsonInt > int64(max) {
		return nil
	}
	v := int8(jsonInt)
	return &v
}

// SafeU32FromJson mirrors `pub fn safeU32FromJson` — rejects negatives
// and values > max.
func (Validator) SafeU32FromJson(jsonInt int64, max uint32) *uint32 {
	if jsonInt < 0 || jsonInt > int64(max) {
		return nil
	}
	v := uint32(jsonInt)
	return &v
}

// SafeU64FromJson mirrors `pub fn safeU64FromJson`.
func (Validator) SafeU64FromJson(jsonInt, min, max uint64) *uint64 {
	if jsonInt < min || jsonInt > max {
		return nil
	}
	return &jsonInt
}

// SafeI64FromJson mirrors `pub fn safeI64FromJson`.
func (Validator) SafeI64FromJson(jsonInt, min, max int64) *int64 {
	if jsonInt < min || jsonInt > max {
		return nil
	}
	return &jsonInt
}

// -----------------------------------------------------------------------------
// Package-level convenience — so callers don't have to allocate a Validator.
// -----------------------------------------------------------------------------

var defaultValidator = NewValidator()

// ValidateTimezone wraps the package-level validator.
func ValidateTimezone(tz int8) error { return defaultValidator.ValidateTimezone(tz) }

// ValidateLanguage wraps the package-level validator.
func ValidateLanguage(lang string) error { return defaultValidator.ValidateLanguage(lang) }

// ValidateDuration wraps the package-level validator.
func ValidateDuration(seconds uint64) error { return defaultValidator.ValidateDuration(seconds) }

// ValidateLoopCount wraps the package-level validator.
func ValidateLoopCount(count uint32) error { return defaultValidator.ValidateLoopCount(count) }

// ValidateLoopInterval wraps the package-level validator.
func ValidateLoopInterval(seconds uint64) error { return defaultValidator.ValidateLoopInterval(seconds) }

// ValidateMaxSeconds wraps the package-level validator.
func ValidateMaxSeconds(maxSeconds uint64) error {
	return defaultValidator.ValidateMaxSeconds(maxSeconds)
}

// ValidateTickInterval wraps the package-level validator.
func ValidateTickInterval(intervalMs int64) error {
	return defaultValidator.ValidateTickInterval(intervalMs)
}

// ValidatePresetName wraps the package-level validator.
func ValidatePresetName(name string) error { return defaultValidator.ValidatePresetName(name) }

// ValidatePresetCount wraps the package-level validator.
func ValidatePresetCount(count int) error { return defaultValidator.ValidatePresetCount(count) }

// ValidateAuthToken wraps the package-level validator.
func ValidateAuthToken(token string) error { return defaultValidator.ValidateAuthToken(token) }