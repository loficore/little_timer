// Package settings — tests for the validator + SettingsManager.
//
// Validator tests assert every Zig rule verbatim (timezone, language,
// duration, loop count, loop interval, max-seconds, tick-interval,
// preset name, preset count, auth token).  Manager tests round-trip
// through a temporary SQLite DB so we exercise the full Load/Save
// pathway end-to-end.
package settings

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"little-timer/internal/domain"
)

// -----------------------------------------------------------------------------
// Validator — every Zig rule tested at the boundary.
// -----------------------------------------------------------------------------

func TestValidateTimezone(t *testing.T) {
	v := NewValidator()
	cases := []struct {
		tz   int8
		want bool
	}{
		{-12, true}, {-11, true}, {0, true}, {8, true}, {14, true},
		{-13, false}, {15, false}, {127, false}, {-128, false},
	}
	for _, c := range cases {
		err := v.ValidateTimezone(c.tz)
		if (err == nil) != c.want {
			t.Errorf("ValidateTimezone(%d) = %v, want ok=%v", c.tz, err, c.want)
		}
	}
}

func TestValidateLanguage(t *testing.T) {
	v := NewValidator()
	if err := v.ValidateLanguage("ZH"); err != nil {
		t.Errorf("ValidateLanguage(\"ZH\"): %v", err)
	}
	if err := v.ValidateLanguage(""); !errors.Is(err, ErrInvalidLanguage) {
		t.Errorf("ValidateLanguage(\"\"): want ErrInvalidLanguage, got %v", err)
	}
	if err := v.ValidateLanguage(strings.Repeat("x", 11)); !errors.Is(err, ErrInvalidLanguage) {
		t.Errorf("ValidateLanguage(long): want ErrInvalidLanguage, got %v", err)
	}
	if err := v.ValidateLanguage(strings.Repeat("x", 10)); err != nil {
		t.Errorf("ValidateLanguage(10 chars): %v", err)
	}
}

func TestValidateDuration(t *testing.T) {
	v := NewValidator()
	if err := v.ValidateDuration(1); err != nil {
		t.Errorf("ValidateDuration(1): %v", err)
	}
	if err := v.ValidateDuration(domain.Day); err != nil {
		t.Errorf("ValidateDuration(86400): %v", err)
	}
	if err := v.ValidateDuration(0); !errors.Is(err, ErrInvalidDuration) {
		t.Errorf("ValidateDuration(0): want ErrInvalidDuration, got %v", err)
	}
	if err := v.ValidateDuration(domain.Day + 1); !errors.Is(err, ErrInvalidDuration) {
		t.Errorf("ValidateDuration(86401): want ErrInvalidDuration, got %v", err)
	}
}

func TestValidateLoopCount(t *testing.T) {
	v := NewValidator()
	if err := v.ValidateLoopCount(0); err != nil {
		t.Errorf("ValidateLoopCount(0): %v", err)
	}
	if err := v.ValidateLoopCount(1000); err != nil {
		t.Errorf("ValidateLoopCount(1000): %v", err)
	}
	if err := v.ValidateLoopCount(1001); !errors.Is(err, ErrInvalidLoopCount) {
		t.Errorf("ValidateLoopCount(1001): want ErrInvalidLoopCount, got %v", err)
	}
}

func TestValidateLoopInterval(t *testing.T) {
	v := NewValidator()
	if err := v.ValidateLoopInterval(0); err != nil {
		t.Errorf("ValidateLoopInterval(0): %v", err)
	}
	if err := v.ValidateLoopInterval(3600); err != nil {
		t.Errorf("ValidateLoopInterval(3600): %v", err)
	}
	if err := v.ValidateLoopInterval(3601); !errors.Is(err, ErrInvalidLoopInterval) {
		t.Errorf("ValidateLoopInterval(3601): want ErrInvalidLoopInterval, got %v", err)
	}
}

func TestValidateMaxSeconds(t *testing.T) {
	v := NewValidator()
	if err := v.ValidateMaxSeconds(1); err != nil {
		t.Errorf("ValidateMaxSeconds(1): %v", err)
	}
	if err := v.ValidateMaxSeconds(domain.Year * 365); err != nil {
		t.Errorf("ValidateMaxSeconds(year): %v", err)
	}
	if err := v.ValidateMaxSeconds(0); !errors.Is(err, ErrInvalidMaxSeconds) {
		t.Errorf("ValidateMaxSeconds(0): want ErrInvalidMaxSeconds, got %v", err)
	}
	if err := v.ValidateMaxSeconds(domain.Year*365 + 1); !errors.Is(err, ErrInvalidMaxSeconds) {
		t.Errorf("ValidateMaxSeconds(year+1): want ErrInvalidMaxSeconds, got %v", err)
	}
}

func TestValidateTickInterval(t *testing.T) {
	v := NewValidator()
	if err := v.ValidateTickInterval(domain.MinTickIntervalMs); err != nil {
		t.Errorf("ValidateTickInterval(min): %v", err)
	}
	if err := v.ValidateTickInterval(domain.MaxTickIntervalMs); err != nil {
		t.Errorf("ValidateTickInterval(max): %v", err)
	}
	if err := v.ValidateTickInterval(domain.MinTickIntervalMs - 1); !errors.Is(err, ErrInvalidTickInterval) {
		t.Errorf("ValidateTickInterval(below min): want ErrInvalidTickInterval, got %v", err)
	}
	if err := v.ValidateTickInterval(domain.MaxTickIntervalMs + 1); !errors.Is(err, ErrInvalidTickInterval) {
		t.Errorf("ValidateTickInterval(above max): want ErrInvalidTickInterval, got %v", err)
	}
}

func TestValidatePresetName(t *testing.T) {
	v := NewValidator()
	if err := v.ValidatePresetName("Workout"); err != nil {
		t.Errorf("ValidatePresetName: %v", err)
	}
	if err := v.ValidatePresetName(""); !errors.Is(err, ErrInvalidPresetName) {
		t.Errorf("ValidatePresetName(\"\"): want ErrInvalidPresetName, got %v", err)
	}
	if err := v.ValidatePresetName(strings.Repeat("x", 65)); !errors.Is(err, ErrInvalidPresetName) {
		t.Errorf("ValidatePresetName(too long): want ErrInvalidPresetName, got %v", err)
	}
}

func TestValidatePresetCount(t *testing.T) {
	v := NewValidator()
	if err := v.ValidatePresetCount(0); err != nil {
		t.Errorf("ValidatePresetCount(0): %v", err)
	}
	if err := v.ValidatePresetCount(999); err != nil {
		t.Errorf("ValidatePresetCount(999): %v", err)
	}
	if err := v.ValidatePresetCount(1000); !errors.Is(err, ErrPresetLimitExceeded) {
		t.Errorf("ValidatePresetCount(1000): want ErrPresetLimitExceeded, got %v", err)
	}
}

func TestValidateAuthToken(t *testing.T) {
	v := NewValidator()
	if err := v.ValidateAuthToken(strings.Repeat("x", 32)); err != nil {
		t.Errorf("ValidateAuthToken(32): %v", err)
	}
	if err := v.ValidateAuthToken(strings.Repeat("x", 256)); err != nil {
		t.Errorf("ValidateAuthToken(256): %v", err)
	}
	if err := v.ValidateAuthToken(strings.Repeat("x", 31)); !errors.Is(err, ErrInvalidToken) {
		t.Errorf("ValidateAuthToken(31): want ErrInvalidToken, got %v", err)
	}
	if err := v.ValidateAuthToken(strings.Repeat("x", 257)); !errors.Is(err, ErrInvalidToken) {
		t.Errorf("ValidateAuthToken(257): want ErrInvalidToken, got %v", err)
	}
}

func TestSafeConversions(t *testing.T) {
	v := NewValidator()

	if got := v.SafeI8FromJson(8, -12, 14); got == nil || *got != 8 {
		t.Errorf("SafeI8FromJson(8): got %v want 8", got)
	}
	if got := v.SafeI8FromJson(20, -12, 14); got != nil {
		t.Errorf("SafeI8FromJson(20): got %v want nil", got)
	}

	if got := v.SafeU32FromJson(42, 1000); got == nil || *got != 42 {
		t.Errorf("SafeU32FromJson(42): got %v want 42", got)
	}
	if got := v.SafeU32FromJson(-1, 1000); got != nil {
		t.Errorf("SafeU32FromJson(-1): got %v want nil", got)
	}
	if got := v.SafeU32FromJson(2000, 1000); got != nil {
		t.Errorf("SafeU32FromJson(2000): got %v want nil", got)
	}

	if got := v.SafeU64FromJson(100, 1, 200); got == nil || *got != 100 {
		t.Errorf("SafeU64FromJson(100): got %v want 100", got)
	}
	if got := v.SafeU64FromJson(300, 1, 200); got != nil {
		t.Errorf("SafeU64FromJson(300): got %v want nil", got)
	}

	if got := v.SafeI64FromJson(0, -1, 1); got == nil || *got != 0 {
		t.Errorf("SafeI64FromJson(0): got %v want 0", got)
	}
	if got := v.SafeI64FromJson(5, -1, 1); got != nil {
		t.Errorf("SafeI64FromJson(5): got %v want nil", got)
	}
}

// -----------------------------------------------------------------------------
// SettingsManager — round-trip + JSON parsing.
// -----------------------------------------------------------------------------

func newTestManager(t *testing.T) *SettingsManager {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "settings.db")
	mgr, err := New(dbPath)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { _ = mgr.Close() })
	return mgr
}

func TestManagerLoadsDefaultsOnFreshDB(t *testing.T) {
	mgr := newTestManager(t)

	cfg := mgr.Config()
	if cfg.Basic.Timezone != 8 {
		t.Errorf("default timezone = %d, want 8", cfg.Basic.Timezone)
	}
	if cfg.Basic.Language != "ZH" {
		t.Errorf("default language = %q, want ZH", cfg.Basic.Language)
	}
	if cfg.Basic.ThemeMode != "dark" {
		t.Errorf("default theme_mode = %q, want dark", cfg.Basic.ThemeMode)
	}
	if cfg.Logging.Level != "INFO" {
		t.Errorf("default log level = %q, want INFO", cfg.Logging.Level)
	}
}

func TestManagerUpdateBasicValidates(t *testing.T) {
	mgr := newTestManager(t)
	if err := mgr.UpdateBasic(BasicConfig{Timezone: 100, Language: "ZH"}); !errors.Is(err, ErrInvalidTimezone) {
		t.Errorf("UpdateBasic(bad tz) = %v, want ErrInvalidTimezone", err)
	}
	if err := mgr.UpdateBasic(BasicConfig{Timezone: 5, Language: ""}); !errors.Is(err, ErrInvalidLanguage) {
		t.Errorf("UpdateBasic(empty lang) = %v, want ErrInvalidLanguage", err)
	}
	if err := mgr.UpdateBasic(BasicConfig{Timezone: 5, Language: "EN", DefaultMode: domain.DefaultModeCountdown}); err != nil {
		t.Errorf("UpdateBasic(good): %v", err)
	}
	if got := mgr.Config().Basic.Language; got != "EN" {
		t.Errorf("Language = %q, want EN", got)
	}
}

func TestManagerSaveAndReload(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "settings.db")

	mgr, err := New(dbPath)
	if err != nil {
		t.Fatalf("New #1: %v", err)
	}
	if err := mgr.UpdateBasic(BasicConfig{Timezone: -5, Language: "EN", DefaultMode: domain.DefaultModeStopwatch}); err != nil {
		t.Fatalf("UpdateBasic: %v", err)
	}
	if err := mgr.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if err := mgr.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	mgr2, err := New(dbPath)
	if err != nil {
		t.Fatalf("New #2: %v", err)
	}
	defer mgr2.Close()
	if got := mgr2.Config().Basic.Timezone; got != -5 {
		t.Errorf("reloaded timezone = %d, want -5", got)
	}
	if got := mgr2.Config().Basic.Language; got != "EN" {
		t.Errorf("reloaded language = %q, want EN", got)
	}
	if got := mgr2.Config().Basic.DefaultMode; got != domain.DefaultModeStopwatch {
		t.Errorf("reloaded default_mode = %v, want stopwatch", got)
	}
}

func TestManagerViperPopulatedFromSQLite(t *testing.T) {
	mgr := newTestManager(t)
	v := mgr.Viper()
	if got := v.GetString("basic.language"); got != "ZH" {
		t.Errorf("viper basic.language = %q, want ZH", got)
	}
	if got := v.GetInt("basic.timezone"); got != 8 {
		t.Errorf("viper basic.timezone = %d, want 8", got)
	}
	if got := v.GetString("logging.level"); got != "INFO" {
		t.Errorf("viper logging.level = %q, want INFO", got)
	}
	if got := v.GetBool("clock_defaults.countdown.loop"); got {
		t.Errorf("viper clock_defaults.countdown.loop = true, want false")
	}
}

func TestManagerUpdateBackupConfigFromJSON(t *testing.T) {
	mgr := newTestManager(t)

	payload, _ := json.Marshal(map[string]any{
		"enabled":              true,
		"auto_backup":          true,
		"auto_backup_interval": 7200,
		"target_type":          "webdav",
		"webdav_url":           "https://dav.example.com/",
		"webdav_username":      "alice",
		"webdav_password":      "s3cret",
		"s3_path_prefix":       "alt_prefix/",
	})
	if err := mgr.UpdateBackupConfigFromJSON(string(payload)); err != nil {
		t.Fatalf("UpdateBackupConfigFromJSON: %v", err)
	}
	bc := mgr.BackupConfig()
	if !bc.Enabled || !bc.AutoBackup {
		t.Errorf("enabled/auto_backup flags not set: %+v", bc)
	}
	if bc.AutoBackupSecs != 7200 {
		t.Errorf("AutoBackupSecs = %d, want 7200", bc.AutoBackupSecs)
	}
	if bc.TargetType != domain.BackupTargetWebDAV {
		t.Errorf("TargetType = %v, want webdav", bc.TargetType)
	}
	if bc.WebDAVURL != "https://dav.example.com/" {
		t.Errorf("WebDAVURL = %q", bc.WebDAVURL)
	}
	if bc.WebDAVUsername != "alice" || bc.WebDAVPassword != "s3cret" {
		t.Errorf("webdav creds: %+v", bc)
	}
	if bc.S3PathPrefix != "alt_prefix/" {
		t.Errorf("S3PathPrefix = %q, want alt_prefix/", bc.S3PathPrefix)
	}
}

func TestManagerBackupConfigCredentialsEncryptedAtRest(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "settings.db")
	mgr, err := New(dbPath)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer mgr.Close()

	payload, _ := json.Marshal(map[string]any{
		"target_type":     "s3",
		"s3_endpoint":     "https://s3.amazonaws.com",
		"s3_bucket":       "mybucket",
		"s3_region":       "us-east-1",
		"s3_access_key":   "AKIA-PLAINTEXT-CANARY",
		"s3_secret_key":   "secret-PLAINTEXT-CANARY",
		"s3_path_prefix":  "lt/",
	})
	if err := mgr.UpdateBackupConfigFromJSON(string(payload)); err != nil {
		t.Fatalf("UpdateBackupConfigFromJSON: %v", err)
	}

	// Inspect the on-disk SQLite blob — the access key literal must NOT
	// appear in the encrypted BLOB column.
	data, err := os.ReadFile(dbPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if strings.Contains(string(data), "AKIA-PLAINTEXT-CANARY") {
		t.Error("access key plaintext leaked into SQLite file")
	}
	if strings.Contains(string(data), "secret-PLAINTEXT-CANARY") {
		t.Error("secret key plaintext leaked into SQLite file")
	}

	// Reload and confirm the decryption round-trips.
	mgr2, err := New(dbPath)
	if err != nil {
		t.Fatalf("New #2: %v", err)
	}
	defer mgr2.Close()
	bc := mgr2.BackupConfig()
	if bc.S3AccessKey != "AKIA-PLAINTEXT-CANARY" {
		t.Errorf("reloaded S3AccessKey = %q, want AKIA-PLAINTEXT-CANARY", bc.S3AccessKey)
	}
	if bc.S3SecretKey != "secret-PLAINTEXT-CANARY" {
		t.Errorf("reloaded S3SecretKey = %q, want secret-PLAINTEXT-CANARY", bc.S3SecretKey)
	}
}

func TestManagerHandleSettingsChangeEvent(t *testing.T) {
	mgr := newTestManager(t)
	payload := `{"basic":{"timezone":3,"language":"EN"}}`
	if err := mgr.HandleSettingsEvent(domain.SettingsChangeEvent{JSON: payload}); err != nil {
		t.Fatalf("HandleSettingsEvent: %v", err)
	}
	if got := mgr.Config().Basic.Timezone; got != 3 {
		t.Errorf("Timezone = %d, want 3", got)
	}
	if got := mgr.Config().Basic.Language; got != "EN" {
		t.Errorf("Language = %q, want EN", got)
	}
}

func TestManagerResetToDefaults(t *testing.T) {
	mgr := newTestManager(t)
	if err := mgr.UpdateBasic(BasicConfig{Timezone: -7, Language: "EN"}); err != nil {
		t.Fatalf("UpdateBasic: %v", err)
	}
	if err := mgr.ResetToDefaults(); err != nil {
		t.Fatalf("ResetToDefaults: %v", err)
	}
	cfg := mgr.Config()
	if cfg.Basic.Timezone != 8 {
		t.Errorf("after reset Timezone = %d, want 8", cfg.Basic.Timezone)
	}
	if cfg.Basic.Language != "ZH" {
		t.Errorf("after reset Language = %q, want ZH", cfg.Basic.Language)
	}
}

func TestManagerBuildClockConfig(t *testing.T) {
	mgr := newTestManager(t)
	cc := mgr.BuildClockConfig()
	if cc.DefaultMode != domain.CountdownMode {
		t.Errorf("BuildClockConfig mode = %v, want countdown", cc.DefaultMode)
	}
	if cc.Countdown.DurationSeconds != domain.DefaultWorkDurationSeconds {
		t.Errorf("Countdown.DurationSeconds = %d, want %d",
			cc.Countdown.DurationSeconds, domain.DefaultWorkDurationSeconds)
	}
}

// -----------------------------------------------------------------------------
// PresetsManager — Zig source makes it a near-no-op; assert that.
// -----------------------------------------------------------------------------

func TestPresetsManagerIsNoop(t *testing.T) {
	p := NewPresetsManager()
	if p.Count() != 0 {
		t.Errorf("Count = %d, want 0", p.Count())
	}
	if p.GetAll() != nil {
		t.Errorf("GetAll = %v, want nil", p.GetAll())
	}
	if err := p.Add(domain.TimerPreset{Name: "x", Mode: domain.CountdownMode, Config: domain.ClockTaskConfig{}}); err != nil {
		t.Errorf("Add error: %v", err)
	}
	if p.Count() != 0 {
		t.Errorf("Count after Add = %d, want 0", p.Count())
	}
	if p.MaxCount() != MaxPresetCount {
		t.Errorf("MaxCount = %d, want %d", p.MaxCount(), MaxPresetCount)
	}
}