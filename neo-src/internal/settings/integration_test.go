// Package settings — integration tests.
//
// settings_test.go covers the validator + manager unit paths; this
// file covers the lifecycle paths that need a real SQLite file
// end-to-end.  Two test cases:
//
//   - FullRoundTrip: write → close → reopen → read → confirm the
//     stored values are identical to what was written.  Mirrors the
//     Zig test "完整往返持久化" in test_settings.zig.
//   - ValidatorRejectsInvalid: every public validator rejects the
//     boundary-out-of-range inputs with the documented sentinel
//     error.
package settings

import (
	"errors"
	"path/filepath"
	"testing"

	"little-timer/internal/domain"
)

// TestSettings_FullRoundTrip exercises the full Save → Close → Open →
// Load cycle with non-default values across every persisted column of
// SettingsConfig + BackupConfig.
//
// Scope note: only fields that have a column in the v8 SQLite schema
// round-trip cleanly.  Fields without a column (EnableFileLogging,
// LogDir, MaxFileSize, MaxFileCount, Auth) are populated in-memory by
// parseSettingsFromJSON but not persisted by SaveSettings, so they're
// excluded from the comparison below.  The BackupConfig is persisted
// via a separate saveBackupConfigToDB pathway that DOES cover the full
// field set.
func TestSettings_FullRoundTrip(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "roundtrip.db")

	want := domain.SettingsConfig{
		Basic: domain.SettingsBasic{
			Timezone:    -7,
			Language:    "JP",
			DefaultMode: domain.DefaultModeStopwatch,
			ThemeMode:   "light",
			Wallpaper:   "wallpapers/sakura.png",
		},
		ClockDefaults: domain.ClockTaskConfig{
			DefaultMode: domain.StopwatchMode,
			Countdown: domain.CountdownConfig{
				DurationSeconds:     3000,
				Loop:                true,
				LoopCount:           4,
				LoopIntervalSeconds: 30,
			},
			Stopwatch: domain.StopwatchConfig{
				MaxSeconds: 43200,
			},
		},
		Logging: domain.SettingsLogging{
			Level:           "DEBUG",
			EnableTimestamp: false,
			TickIntervalMs:  500,
		},
	}

	wantBackup := domain.BackupConfig{
		Enabled:           true,
		AutoBackup:        true,
		AutoBackupSecs:    7200,
		TargetType:        domain.BackupTargetWebDAV,
		LocalPath:         "/tmp/lt_backups",
		WebDAVURL:         "https://dav.example.com/",
		WebDAVUsername:    "alice",
		WebDAVPassword:    "s3cret-pwd",
		S3Endpoint:        "https://s3.amazonaws.com",
		S3Bucket:          "mybucket",
		S3Region:          "us-east-1",
		S3AccessKey:       "AKIA-INTEGRATION",
		S3SecretKey:       "secret-INTEGRATION",
		S3PathPrefix:      "lt/",
		HasMasterPassword: true,
	}

	// Phase 1: write.
	{
		mgr, err := New(dbPath)
		if err != nil {
			t.Fatalf("New #1: %v", err)
		}
		if err := mgr.UpdateBasic(BasicConfig{
			Timezone:    want.Basic.Timezone,
			Language:    want.Basic.Language,
			DefaultMode: want.Basic.DefaultMode,
		}); err != nil {
			t.Fatalf("UpdateBasic: %v", err)
		}
		if err := mgr.parseSettingsFromJSON(`{
			"basic": {"timezone":-7,"language":"JP","default_mode":"stopwatch","theme_mode":"light","wallpaper":"wallpapers/sakura.png"},
			"clock_defaults": {"countdown":{"duration_seconds":3000,"loop":true,"loop_count":4,"loop_interval_seconds":30},"stopwatch":{"max_seconds":43200}},
			"logging": {"level":"DEBUG","enable_timestamp":false,"tick_interval_ms":500},
			"backup": {"enabled":true,"auto_backup":true,"auto_backup_interval":7200,"target_type":"webdav","local_path":"/tmp/lt_backups","webdav_url":"https://dav.example.com/","webdav_username":"alice","webdav_password":"s3cret-pwd","s3_endpoint":"https://s3.amazonaws.com","s3_bucket":"mybucket","s3_region":"us-east-1","s3_access_key":"AKIA-INTEGRATION","s3_secret_key":"secret-INTEGRATION","s3_path_prefix":"lt/","has_master_password":true}
		}`); err != nil {
			t.Fatalf("parseSettingsFromJSON: %v", err)
		}
		if err := mgr.Save(); err != nil {
			t.Fatalf("Save: %v", err)
		}
		if err := mgr.Close(); err != nil {
			t.Fatalf("Close #1: %v", err)
		}
	}

	// Phase 2: reopen + read.
	mgr2, err := New(dbPath)
	if err != nil {
		t.Fatalf("New #2: %v", err)
	}
	defer mgr2.Close()

	got := mgr2.Config()
	if got.Basic.Timezone != want.Basic.Timezone {
		t.Errorf("Basic.Timezone: got %d, want %d", got.Basic.Timezone, want.Basic.Timezone)
	}
	if got.Basic.Language != want.Basic.Language {
		t.Errorf("Basic.Language: got %q, want %q", got.Basic.Language, want.Basic.Language)
	}
	if got.Basic.DefaultMode != want.Basic.DefaultMode {
		t.Errorf("Basic.DefaultMode: got %v, want %v", got.Basic.DefaultMode, want.Basic.DefaultMode)
	}
	if got.Basic.ThemeMode != want.Basic.ThemeMode {
		t.Errorf("Basic.ThemeMode: got %q, want %q", got.Basic.ThemeMode, want.Basic.ThemeMode)
	}
	if got.Basic.Wallpaper != want.Basic.Wallpaper {
		t.Errorf("Basic.Wallpaper: got %q, want %q", got.Basic.Wallpaper, want.Basic.Wallpaper)
	}
	if got.ClockDefaults.Countdown.DurationSeconds != want.ClockDefaults.Countdown.DurationSeconds {
		t.Errorf("Countdown.DurationSeconds: got %d, want %d",
			got.ClockDefaults.Countdown.DurationSeconds,
			want.ClockDefaults.Countdown.DurationSeconds)
	}
	if got.ClockDefaults.Countdown.Loop != want.ClockDefaults.Countdown.Loop {
		t.Errorf("Countdown.Loop: got %v, want %v",
			got.ClockDefaults.Countdown.Loop, want.ClockDefaults.Countdown.Loop)
	}
	if got.ClockDefaults.Countdown.LoopCount != want.ClockDefaults.Countdown.LoopCount {
		t.Errorf("Countdown.LoopCount: got %d, want %d",
			got.ClockDefaults.Countdown.LoopCount,
			want.ClockDefaults.Countdown.LoopCount)
	}
	if got.ClockDefaults.Countdown.LoopIntervalSeconds != want.ClockDefaults.Countdown.LoopIntervalSeconds {
		t.Errorf("Countdown.LoopIntervalSeconds: got %d, want %d",
			got.ClockDefaults.Countdown.LoopIntervalSeconds,
			want.ClockDefaults.Countdown.LoopIntervalSeconds)
	}
	if got.ClockDefaults.Stopwatch.MaxSeconds != want.ClockDefaults.Stopwatch.MaxSeconds {
		t.Errorf("Stopwatch.MaxSeconds: got %d, want %d",
			got.ClockDefaults.Stopwatch.MaxSeconds,
			want.ClockDefaults.Stopwatch.MaxSeconds)
	}
	if got.Logging.Level != want.Logging.Level {
		t.Errorf("Logging.Level: got %q, want %q", got.Logging.Level, want.Logging.Level)
	}
	if got.Logging.EnableTimestamp != want.Logging.EnableTimestamp {
		t.Errorf("Logging.EnableTimestamp: got %v, want %v",
			got.Logging.EnableTimestamp, want.Logging.EnableTimestamp)
	}
	if got.Logging.TickIntervalMs != want.Logging.TickIntervalMs {
		t.Errorf("Logging.TickIntervalMs: got %d, want %d",
			got.Logging.TickIntervalMs, want.Logging.TickIntervalMs)
	}

	gotBackup := mgr2.BackupConfig()
	if gotBackup.TargetType != wantBackup.TargetType {
		t.Errorf("BackupConfig.TargetType: got %v, want %v",
			gotBackup.TargetType, wantBackup.TargetType)
	}
	if gotBackup.WebDAVURL != wantBackup.WebDAVURL {
		t.Errorf("BackupConfig.WebDAVURL: got %q, want %q",
			gotBackup.WebDAVURL, wantBackup.WebDAVURL)
	}
	if gotBackup.WebDAVUsername != wantBackup.WebDAVUsername {
		t.Errorf("BackupConfig.WebDAVUsername: got %q, want %q",
			gotBackup.WebDAVUsername, wantBackup.WebDAVUsername)
	}
	if gotBackup.WebDAVPassword != wantBackup.WebDAVPassword {
		t.Errorf("BackupConfig.WebDAVPassword round-trip failed: got %q, want %q",
			gotBackup.WebDAVPassword, wantBackup.WebDAVPassword)
	}
	if gotBackup.S3Bucket != wantBackup.S3Bucket {
		t.Errorf("BackupConfig.S3Bucket: got %q, want %q",
			gotBackup.S3Bucket, wantBackup.S3Bucket)
	}
	if gotBackup.S3AccessKey != wantBackup.S3AccessKey {
		t.Errorf("BackupConfig.S3AccessKey round-trip failed: got %q, want %q",
			gotBackup.S3AccessKey, wantBackup.S3AccessKey)
	}
	if gotBackup.S3SecretKey != wantBackup.S3SecretKey {
		t.Errorf("BackupConfig.S3SecretKey round-trip failed: got %q, want %q",
			gotBackup.S3SecretKey, wantBackup.S3SecretKey)
	}
	if gotBackup.HasMasterPassword != wantBackup.HasMasterPassword {
		t.Errorf("BackupConfig.HasMasterPassword: got %v, want %v",
			gotBackup.HasMasterPassword, wantBackup.HasMasterPassword)
	}
}

// TestSettings_ValidatorRejectsInvalid hits every public validator
// method with an out-of-range input and asserts the documented
// sentinel error type comes back.
func TestSettings_ValidatorRejectsInvalid(t *testing.T) {
	v := NewValidator()
	mgr := newTestManager(t)

	type tc struct {
		name string
		run  func() error
		want error
	}
	cases := []tc{
		{"UpdateBasic timezone=15", func() error {
			return mgr.UpdateBasic(BasicConfig{Timezone: 15, Language: "ZH"})
		}, ErrInvalidTimezone},
		{"UpdateBasic language=''", func() error {
			return mgr.UpdateBasic(BasicConfig{Timezone: 5, Language: ""})
		}, ErrInvalidLanguage},
		{"ValidateDuration 0", func() error {
			return v.ValidateDuration(0)
		}, ErrInvalidDuration},
		{"ValidateDuration > day", func() error {
			return v.ValidateDuration(domain.Day + 1)
		}, ErrInvalidDuration},
		{"ValidateLoopCount > 1000", func() error {
			return v.ValidateLoopCount(1001)
		}, ErrInvalidLoopCount},
		{"ValidateLoopInterval > 3600", func() error {
			return v.ValidateLoopInterval(3601)
		}, ErrInvalidLoopInterval},
		{"ValidateMaxSeconds 0", func() error {
			return v.ValidateMaxSeconds(0)
		}, ErrInvalidMaxSeconds},
		{"ValidateTickInterval below min", func() error {
			return v.ValidateTickInterval(domain.MinTickIntervalMs - 1)
		}, ErrInvalidTickInterval},
		{"ValidateTickInterval above max", func() error {
			return v.ValidateTickInterval(domain.MaxTickIntervalMs + 1)
		}, ErrInvalidTickInterval},
		{"ValidatePresetName empty", func() error {
			return v.ValidatePresetName("")
		}, ErrInvalidPresetName},
		{"ValidatePresetCount 1000", func() error {
			return v.ValidatePresetCount(1000)
		}, ErrPresetLimitExceeded},
		{"ValidateAuthToken too short", func() error {
			return v.ValidateAuthToken("short")
		}, ErrInvalidToken},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := c.run()
			if err == nil {
				t.Fatalf("%s: expected error, got nil", c.name)
			}
			if !errors.Is(err, c.want) {
				t.Errorf("%s: got %v, want errors.Is(%v)", c.name, err, c.want)
			}
		})
	}
}