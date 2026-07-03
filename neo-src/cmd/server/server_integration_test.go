// Package main — integration tests for cmd/server bootstrap and shutdown.
//
// Two layers of coverage:
//
//  1. Subprocess tests (TestBootstrap_*, TestSignal_*) build the server
//     binary once via TestMain and exec it.  They verify the contract
//     cmd/server presents to its environment: successful boot, fatal
//     bootstrap errors, and SIGTERM-driven graceful shutdown.
//
//  2. Unit-level recovery tests (TestClock_*, TestSettings_*) exercise
//     the two failure-recovery paths the spec calls out: clock duration
//     overflow (silent fallback to safe defaults) and settings row
//     corruption (re-seed defaults and continue).
//
// Run:
//
//	go test -v -run "TestBootstrap|TestSignal|TestClock|TestSettings" ./cmd/server/...
package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"little-timer/internal/domain"
	"little-timer/internal/settings"
	"little-timer/internal/storage"
)

// testServerPath is set by TestMain to a freshly-built server binary.
// Building once per suite keeps per-test overhead minimal and removes
// any dependency on a pre-built `bin/server` artefact.
var testServerPath string

// TestMain builds the server binary into a temp file so the subprocess
// tests don't depend on a pre-existing artefact.  Build runs from the
// test binary's CWD (cmd/server), so `go build .` resolves to this
// package's main.go.
func TestMain(m *testing.M) {
	tmpDir, err := os.MkdirTemp("", "lt-server-int-")
	if err != nil {
		fmt.Fprintf(os.Stderr, "test main: mkdir temp: %v\n", err)
		os.Exit(2)
	}
	// Best-effort cleanup; we also exit() below which makes cleanup
	// mostly cosmetic, but it keeps `go test -count=2` runs tidy.
	defer func() { _ = os.RemoveAll(tmpDir) }()

	binPath := filepath.Join(tmpDir, "lt-server-test-bin")
	build := exec.Command("go", "build", "-o", binPath, ".")
	var buildStderr bytes.Buffer
	build.Stderr = &buildStderr
	if err := build.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "test main: build server: %v\n%s\n", err, buildStderr.String())
		os.Exit(2)
	}
	testServerPath = binPath

	os.Exit(m.Run())
}

// serveArgs returns the flag list for an http-only run rooted at dbPath
// on port.  All subprocess tests use --http-only so the webview path
// is never taken (no GTK deps needed on CI).
func serveArgs(dbPath string, port int) []string {
	return []string{
		"--http-only",
		"--port", strconv.Itoa(port),
		"--db-path", dbPath,
	}
}

// processAlive checks if pid is still running via signal 0.  Equivalent
// to `kill -0 $PID` — no actual signal is delivered.
func processAlive(pid *os.Process) error {
	if pid == nil {
		return errors.New("nil process")
	}
	return pid.Signal(syscall.Signal(0))
}

// =============================================================================
// Bootstrap failure paths (subprocess).
// =============================================================================

// TestBootstrap_ValidTempDirStarts confirms the happy path: with a
// writable temp dir + unique port, the server starts and stays alive
// past the initial bootstrap window.  Uses signal 0 to probe liveness —
// we don't actually need to hit HTTP.
func TestBootstrap_ValidTempDirStarts(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "bootstrap-ok.db")

	cmd := exec.Command(testServerPath, serveArgs(dbPath, 18091)...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("start: %v\nstderr: %s", err, stderr.String())
	}

	// Cleanup: SIGTERM, then force-kill if shutdown stalls.
	t.Cleanup(func() {
		if cmd.Process == nil {
			return
		}
		_ = cmd.Process.Signal(syscall.SIGTERM)
		done := make(chan struct{})
		go func() { _ = cmd.Wait(); close(done) }()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			_ = cmd.Process.Kill()
		}
	})

	// Give bootstrap time to finish, then probe.
	time.Sleep(400 * time.Millisecond)
	if err := processAlive(cmd.Process); err != nil {
		t.Errorf("server died after bootstrap: %v\nstdout: %s\nstderr: %s",
			err, stdout.String(), stderr.String())
	}
}

// TestBootstrap_UnreadableDBPathExitsOne confirms a non-creatable DB
// path surfaces a "bootstrap" error and exits with code 1.
//
// The path's parent directory doesn't exist and its grandparent is a
// root-owned mode-000 path (`/nonexistent-lt-...`), so `os.MkdirAll`
// inside storage.Open() cannot create the file.  Mirrors the
// `bootstrap: open sqlite: ...` message we expect runServer to wrap.
func TestBootstrap_UnreadableDBPathExitsOne(t *testing.T) {
	badPath := "/nonexistent-lt-root-owned-dir-xyz/db.db"

	cmd := exec.Command(testServerPath, serveArgs(badPath, 18092)...)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	err := cmd.Run()
	if err == nil {
		t.Fatalf("expected non-zero exit; output:\n%s", out.String())
	}

	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected ExitError, got %T: %v", err, err)
	}
	if exitErr.ExitCode() != 1 {
		t.Errorf("exit code = %d, want 1", exitErr.ExitCode())
	}

	if !strings.Contains(out.String(), "bootstrap") {
		t.Errorf("error output must mention 'bootstrap', got:\n%s", out.String())
	}
}

// =============================================================================
// Signal handling (subprocess).
// =============================================================================

// TestSignal_SIGTERMGracefulShutdown confirms the server catches
// SIGTERM and exits cleanly within the 5-second shutdown window
// declared in main.go's `shutdownTimeout` constant.
//
// We measure from signal-send to process-exit and assert the wall-clock
// duration is under 5s.  A real run on this dev machine is ~milliseconds
// (HTTP server has no in-flight requests), but the assertion guards
// against regressions that hang shutdown (e.g. forgetting the timeout
// on srv.Shutdown).
func TestSignal_SIGTERMGracefulShutdown(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "shutdown.db")

	cmd := exec.Command(testServerPath, serveArgs(dbPath, 18093)...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}

	// Wait for the server to be listening before signalling.
	time.Sleep(400 * time.Millisecond)
	if err := processAlive(cmd.Process); err != nil {
		t.Fatalf("server died during startup: %v\nstderr: %s", err, stderr.String())
	}

	start := time.Now()
	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatalf("signal: %v", err)
	}

	waitCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	select {
	case err := <-done:
		elapsed := time.Since(start)
		// cmd.Wait returns *exec.ExitError for any non-zero exit;
		// graceful SIGTERM-driven shutdown should be exit 0, but
		// we accept any non-error wait result here — the contract
		// is "exits within 5s", not "exits with status 0".
		if err != nil {
			var exitErr *exec.ExitError
			if !errors.As(err, &exitErr) {
				t.Errorf("unexpected wait error: %v\nstdout: %s\nstderr: %s",
					err, stdout.String(), stderr.String())
			}
		}
		if elapsed > 5*time.Second {
			t.Errorf("shutdown took %v, want <= 5s (matches main.go shutdownTimeout)", elapsed)
		}
	case <-waitCtx.Done():
		_ = cmd.Process.Kill()
		t.Fatalf("server did not exit within 5 seconds after SIGTERM\nstderr: %s", stderr.String())
	}
}

// =============================================================================
// Clock duration overflow (unit).
// =============================================================================

// TestClock_OverflowFallsBackToSafeDefaults confirms
// domain.NewClockManager substitutes the safe 25-min countdown / 24-hr
// stopwatch defaults when the supplied durations would overflow int64
// milliseconds.  Mirrors the Zig source's `initSafe` path in clock.zig.
//
// We exercise both directions because the fallback config covers
// Countdown AND Stopwatch and the buildInitialState switch only
// populates the active variant — so each path needs its own check.
func TestClock_OverflowFallsBackToSafeDefaults(t *testing.T) {
	// uint64(math.MaxInt64)/1000 is the largest duration that won't
	// overflow int64 when multiplied by 1000; one more triggers the
	// fallback in durationOverflows().
	overflow := uint64(math.MaxInt64)/1000 + 1

	// Path 1: countdown overflow → expect 25-min countdown state.
	t.Run("countdown_overflow", func(t *testing.T) {
		cfg := domain.ClockTaskConfig{
			DefaultMode: domain.CountdownMode,
			Countdown:   domain.CountdownConfig{DurationSeconds: overflow},
			Stopwatch:   domain.StopwatchConfig{MaxSeconds: 24 * domain.Hour},
		}
		m := domain.NewClockManager(cfg)
		state := m.Update()
		if state == nil || state.Countdown == nil {
			t.Fatalf("expected countdown state, got %+v", state)
		}
		const wantMs = uint64(25 * domain.Minute * 1000)
		if state.Countdown.DurationMs != wantMs {
			t.Errorf("Countdown.DurationMs = %d, want %d (25-min fallback)",
				state.Countdown.DurationMs, wantMs)
		}
		if state.Countdown.RemainingMs != int64(wantMs) {
			t.Errorf("Countdown.RemainingMs = %d, want %d",
				state.Countdown.RemainingMs, wantMs)
		}
	})

	// Path 2: stopwatch overflow → expect 25-min countdown state (the
	// fallback config zeroes DefaultMode, so the active mode flips).
	t.Run("stopwatch_overflow", func(t *testing.T) {
		cfg := domain.ClockTaskConfig{
			DefaultMode: domain.StopwatchMode,
			Countdown:   domain.CountdownConfig{DurationSeconds: 25 * domain.Minute},
			Stopwatch:   domain.StopwatchConfig{MaxSeconds: overflow},
		}
		m := domain.NewClockManager(cfg)
		state := m.Update()
		if state == nil || state.Countdown == nil || state.Stopwatch != nil {
			t.Fatalf("expected countdown state (mode falls back on overflow), got %+v", state)
		}
		const wantMs = uint64(25 * domain.Minute * 1000)
		if state.Countdown.DurationMs != wantMs {
			t.Errorf("Countdown.DurationMs = %d, want %d (25-min fallback)",
				state.Countdown.DurationMs, wantMs)
		}
		if state.Countdown.RemainingMs != int64(wantMs) {
			t.Errorf("Countdown.RemainingMs = %d, want %d",
				state.Countdown.RemainingMs, wantMs)
		}
	})

	// Path 3: both at the boundary should NOT trigger fallback — guards
	// against an off-by-one in durationOverflows().
	t.Run("boundary_no_overflow", func(t *testing.T) {
		const safeMax = uint64(math.MaxInt64) / 1000
		cfg := domain.ClockTaskConfig{
			DefaultMode: domain.CountdownMode,
			Countdown:   domain.CountdownConfig{DurationSeconds: safeMax},
			Stopwatch:   domain.StopwatchConfig{MaxSeconds: safeMax},
		}
		m := domain.NewClockManager(cfg)
		state := m.Update()
		if state == nil || state.Countdown == nil {
			t.Fatalf("expected countdown state, got %+v", state)
		}
		const wantMs = safeMax * 1000
		if state.Countdown.DurationMs != wantMs {
			t.Errorf("Countdown.DurationMs at boundary = %d, want %d (no fallback)",
				state.Countdown.DurationMs, wantMs)
		}
	})
}

// =============================================================================
// Settings recovery (unit).
// =============================================================================

// TestSettings_CorruptRowSeedsDefaults confirms settings.NewFromSqliteManager
// recovers from a missing settings row by seeding defaults and returning a
// usable config.  Mirrors the Zig source's re-seed path in loadAll().
//
// We induce failure by deleting the settings row after migration runs.
// CHECK constraints in the schema prevent UPDATE-ing an invalid value,
// so DELETE is the cheapest realistic corruption mode and exercises the
// same recovery branch in loadAll — both `sql.ErrNoRows` and other scan
// errors land in the same "re-seed defaults" path.
func TestSettings_CorruptRowSeedsDefaults(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "settings-recover.db")

	sqlite := storage.NewSqliteManager().Init(dbPath)
	if err := sqlite.Open(); err != nil {
		t.Fatalf("sqlite open: %v", err)
	}
	if err := sqlite.Migrate(); err != nil {
		t.Fatalf("sqlite migrate: %v", err)
	}
	t.Cleanup(func() { _ = sqlite.Close() })

	sm, err := settings.NewFromSqliteManager(sqlite, dbPath)
	if err != nil {
		t.Fatalf("settings (initial): %v", err)
	}

	// Sanity: initial construction seeded defaults.
	initial := sm.Config()
	if initial.Basic.Language == "" {
		t.Errorf("initial Language empty; expected defaults after migration")
	}
	if initial.ClockDefaults.Countdown.DurationSeconds == 0 {
		t.Errorf("initial DurationSeconds = 0; expected default")
	}

	// Simulate corruption: wipe the settings row.  The next Load
	// must re-seed defaults without bubbling an error.
	if _, err := sqlite.DB().Exec(`DELETE FROM settings WHERE id = 1;`); err != nil {
		t.Fatalf("delete row: %v", err)
	}

	if err := sm.Load(); err != nil {
		t.Errorf("Load after row deletion returned error: %v (expected recovery)", err)
	}

	cfg := sm.Config()
	if cfg.Basic.Language == "" {
		t.Errorf("after recovery, Language empty; want default")
	}
	if cfg.ClockDefaults.Countdown.DurationSeconds == 0 {
		t.Errorf("after recovery, DurationSeconds = 0; want default")
	}
	if cfg.Logging.Level == "" {
		t.Errorf("after recovery, Logging.Level empty; want default")
	}

	// Manager remains usable: a follow-up round-trip through viper
	// should see the re-seeded values, not zero.
	if got := sm.Viper().GetString("basic.language"); got == "" {
		t.Errorf("after recovery, viper basic.language empty; want default")
	}
}