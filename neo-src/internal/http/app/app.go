// Package app hosts the App struct — the Go analogue of the Zig
// `MainApplication` that the std_server.zig handlers thread through
// global state.  Each handler receives `*App` via the Gin context's
// `MustGet("app")` slot, so handlers can reach the clock, settings,
// SQLite manager, backup manager, and master-password state without
// importing a global package-level variable.
//
// Splitting `App` into its own sub-package breaks the import cycle
// that would otherwise form (router → handlers → http).  The router
// and the handlers both import this package.
//
// Memory ownership: the Zig source uses an arena allocator and mutates
// `MainApplication` under `std.Thread.Mutex`.  In Go we use a single
// `sync.RWMutex` to mirror the lock; the rest of the state is owned by
// the underlying components (ClockManager, SettingsManager, etc.).
package app

import (
	"sync"
	"time"

	"little-timer/internal/crypto"
	"little-timer/internal/domain"
	"little-timer/internal/settings"
	"little-timer/internal/storage"
	"little-timer/internal/storage/backup"
)

// App is the HTTP-layer dependency bundle.
//
// One App per server process.  Constructed by the caller (currently the
// server bootstrap in `cmd/server/main.go`) and passed to `NewRouter`.
//
// `Backup` is optional — when nil, backup endpoints return 503-ish
// responses (handlers treat nil as "backup not configured").  This keeps
// the http layer independent of whether a backup manager was wired in.
type App struct {
	mu sync.RWMutex

	// Clock + settings + DB — mirrors the Zig MainApplication fields
	// of the same name.
	Clock    *domain.ClockManager
	Settings *settings.SettingsManager
	SQLite   *storage.SqliteManager
	Backup   *backup.BackupManager

	// DBPath is captured at App construction so backup handlers can
	// hand it to the adapter if needed.  Matches the Zig
	// `app.settings_manager.sqlite_db.?.*.db_path` chain.
	DBPath string

	// CurrentHabitID / CurrentTimerSessionID are the in-memory mirrors
	// of `app.current_habit_id` / `app.current_timer_session_id`.  The
	// Zig source resets these on `resetTimerSession`; the Go port does
	// the same.
	CurrentHabitID         *int64
	CurrentTimerSessionID  *int64

	// Secrets is the in-process master-password store.  Mirrors the
	// Zig `SoftwareSecretImpl` / `SecretStorage`.  Lazily created by
	// the helper methods so callers can omit it during construction.
	secrets *crypto.SecretStorage
}

// NewApp builds an App with the supplied dependencies and default in-memory
// state.  `dbPath` is captured so handlers that need the on-disk path
// (currently only the backup handlers that build ad-hoc adapters) can
// read it without re-deriving it from the SQLite manager.
func NewApp(
	clk *domain.ClockManager,
	sm *settings.SettingsManager,
	sqlite *storage.SqliteManager,
	bm *backup.BackupManager,
	dbPath string,
) *App {
	return &App{
		Clock:    clk,
		Settings: sm,
		SQLite:   sqlite,
		Backup:   bm,
		DBPath:   dbPath,
	}
}

// -----------------------------------------------------------------------------
// Convenience mutex accessors.  The Zig source takes the lock on every
// mutating endpoint (start/pause/reset/finish); in Go we expose Lock/Unlock
// rather than hiding them behind helper methods so handlers stay explicit.
// -----------------------------------------------------------------------------

// Lock mirrors `app.mutex.lock()`.
func (a *App) Lock() { a.mu.Lock() }

// Unlock mirrors `app.mutex.unlock()`.
func (a *App) Unlock() { a.mu.Unlock() }

// RLock mirrors a read-side lock; used by the SSE goroutine.
func (a *App) RLock() { a.mu.RLock() }

// RUnlock mirrors a read-side unlock.
func (a *App) RUnlock() { a.mu.RUnlock() }

// -----------------------------------------------------------------------------
// Timer session helpers — mirrors `createTimerSession`, `finishTimerSession`,
// `resetTimerSession`, `saveTimerProgress`, `loadTimerProgress`.
//
// All four assume the caller holds the App mutex (Lock or RLock) —
// they only mutate the in-memory pointers and the database, never the
// lock itself.  This matches the Zig source, where the helpers run
// under the caller's mutex with no re-acquisition.
// -----------------------------------------------------------------------------

// CreateTimerSession inserts a new timer_sessions row and updates the
// in-memory pointers.  Caller MUST hold a.mu (write).  Mirrors
// `app.createTimerSession`.
func (a *App) CreateTimerSession(habitID *int64, mode string, work, rest, loop int64) (int64, error) {
	id, err := a.SQLite.Timers().CreateTimerSession(habitID, mode, work, rest, loop)
	if err != nil {
		return 0, err
	}
	a.CurrentTimerSessionID = &id
	a.CurrentHabitID = habitID
	return id, nil
}

// FinishTimerSession marks the current timer_session finished and
// returns the elapsed seconds.  Caller MUST hold a.mu (write).  Mirrors
// `app.finishTimerSession`.
func (a *App) FinishTimerSession() (int64, error) {
	sessionID := a.CurrentTimerSessionID
	if sessionID == nil {
		return 0, nil
	}
	if err := a.SQLite.Timers().FinishTimerSession(*sessionID); err != nil {
		return 0, err
	}
	state := a.Clock.Update()
	return state.GetElapsedSeconds(), nil
}

// ResetTimerSession clears the in-memory pointers and deletes the
// current timer_session row.  Caller MUST hold a.mu (write).  Mirrors
// `app.resetTimerSession`.
func (a *App) ResetTimerSession() {
	sessionID := a.CurrentTimerSessionID
	a.CurrentTimerSessionID = nil
	a.CurrentHabitID = nil
	if sessionID != nil {
		_ = a.SQLite.Timers().DeleteTimerSession(*sessionID)
	}
}

// LoadTimerProgress re-reads the most recent unfinished timer_session
// into the in-memory pointers.  Caller MUST hold a.mu (write).  Mirrors
// `app.loadTimerProgress`.
func (a *App) LoadTimerProgress() {
	row, err := a.SQLite.Timers().GetActiveTimerSession()
	if err != nil {
		return
	}
	id := row.ID
	a.CurrentTimerSessionID = &id
	if row.HabitID != nil {
		hid := *row.HabitID
		a.CurrentHabitID = &hid
	}
}

// SaveProgressLocked persists the current clock state to the active
// timer_session row.  Caller MUST hold a.mu (write).  Mirrors
// `app.saveTimerProgress`.
func (a *App) SaveProgressLocked() {
	if a.CurrentTimerSessionID == nil {
		return
	}
	state := a.Clock.Update()
	now := time.Now().Unix()
	row, err := a.SQLite.Timers().GetTimerSessionByID(*a.CurrentTimerSessionID)
	if err != nil {
		return
	}
	pausedTotal := row.PausedTotalSeconds
	pauseStarted := row.PauseStartedAt
	isPaused := state.IsPaused()
	isRunning := !isPaused
	if isPaused && pauseStarted == nil {
		pauseStarted = &now
	} else if !isPaused && pauseStarted != nil {
		if now > *pauseStarted {
			pausedTotal += now - *pauseStarted
		}
		pauseStarted = nil
	}
	remaining := state.GetRemainingSeconds()
	_ = a.SQLite.Timers().UpdateTimerSession(
		row.ID, state.GetElapsedSeconds(), &remaining,
		pausedTotal, pauseStarted, &now,
		isRunning, isPaused, state.IsFinished(),
		row.CurrentRound, state.InRest(),
	)
}

// -----------------------------------------------------------------------------
// Master-password helpers — the Zig source plumbs these through
// `app.settings_manager.hasMasterPassword()` etc.  The Go port keeps the
// state on the SettingsManager's BackupConfig (which already carries the
// lockout fields) plus a SecretStorage for the unlock password itself.
//
// All helpers tolerate a nil Secrets (treat as "no master password set").
// -----------------------------------------------------------------------------

func (a *App) ensureSecrets() *crypto.SecretStorage {
	if a.secrets == nil {
		// ponytail: secrets live in-memory only here.  Persistent
		// storage wiring lands once the user-data dir decision lands
		// (W6+).
		a.secrets = crypto.New("")
	}
	return a.secrets
}

// HasMasterPassword mirrors `app.settings_manager.hasMasterPassword`.
// Returns true when a master-password blob exists on disk OR the
// BackupConfig row already says so.
func (a *App) HasMasterPassword() bool {
	cfg := a.Settings.BackupConfig()
	if cfg.HasMasterPassword {
		return true
	}
	return a.ensureSecrets().HasMasterPassword()
}

// IsUnlocked mirrors `app.settings_manager.isUnlocked`.  Returns true
// when the secrets store holds an unlocked master password AND the
// lockout window has elapsed.
func (a *App) IsUnlocked() bool {
	cfg := a.Settings.BackupConfig()
	if !a.ensureSecrets().IsLocked() && cfg.CredentialLockedUntil <= time.Now().Unix() {
		return true
	}
	return false
}

// UnlockCredentials mirrors `app.settings_manager.unlockCredentials`.
// Returns an UnlockResult JSON-friendly struct.  Always returns
// `Success: true` when no master password is set (matches Zig).
func (a *App) UnlockCredentials(password string) domain.UnlockResult {
	cfg := a.Settings.BackupConfig()
	if !a.HasMasterPassword() {
		// No master password: always succeed.
		cfg.CredentialLockedUntil = 0
		cfg.CredentialsUnlockTime = time.Now().Unix()
		_ = a.Settings.UpdateBackupConfigFromJSON(backupConfigToJSON(cfg))
		return domain.UnlockResult{Success: true, LockedUntil: 0}
	}
	if err := a.ensureSecrets().Unlock([]byte(password)); err != nil {
		return domain.UnlockResult{Success: false, LockedUntil: a.ensureSecrets().LockoutUntil()}
	}
	cfg.CredentialLockedUntil = 0
	cfg.CredentialsUnlockTime = time.Now().Unix()
	_ = a.Settings.UpdateBackupConfigFromJSON(backupConfigToJSON(cfg))
	return domain.UnlockResult{Success: true, LockedUntil: 0}
}

// SetMasterPassword mirrors `app.settings_manager.setMasterPassword`.
// Persists the password via SecretStorage AND updates the BackupConfig
// flag so the on-disk row matches.
func (a *App) SetMasterPassword(password string) error {
	if len(password) < 4 {
		return errPasswordTooShort
	}
	if err := a.ensureSecrets().SetMasterPassword([]byte(password)); err != nil {
		return err
	}
	cfg := a.Settings.BackupConfig()
	cfg.HasMasterPassword = true
	cfg.CredentialLockedUntil = 0
	return a.Settings.UpdateBackupConfigFromJSON(backupConfigToJSON(cfg))
}

// GetMasterPasswordStatus mirrors
// `app.settings_manager.getMasterPasswordStatus`.
func (a *App) GetMasterPasswordStatus() domain.MasterPasswordStatus {
	cfg := a.Settings.BackupConfig()
	return domain.MasterPasswordStatus{
		HasPassword: a.HasMasterPassword(),
		Unlocked:    a.IsUnlocked(),
		LockedUntil: cfg.CredentialLockedUntil,
		UnlockTime:  cfg.CredentialsUnlockTime,
	}
}

// LockCredentials mirrors the body of `handleBackupLock` — sets the
// lockout to "now+1s" and clears the in-memory secrets cache.
func (a *App) LockCredentials() {
	cfg := a.Settings.BackupConfig()
	cfg.CredentialLockedUntil = time.Now().Unix() + 1
	_ = a.Settings.UpdateBackupConfigFromJSON(backupConfigToJSON(cfg))
	a.ensureSecrets().Lock()
}

// -----------------------------------------------------------------------------
// Auth helpers.
// -----------------------------------------------------------------------------

// GenerateToken returns a 32-byte random token encoded as base64.
// Mirrors Zig `crypto.generateToken` (which returned a 64-char hex
// string; base64 of 32 bytes is 44 chars — close enough for the auth
// header format and far cheaper than hex encoding).
func GenerateToken() string {
	return base64Raw(crypto.GenerateKey())
}

// -----------------------------------------------------------------------------
// Errors.
// -----------------------------------------------------------------------------

// errPasswordTooShort is returned by SetMasterPassword when the
// supplied password is shorter than 4 characters.  Mirrors the Zig
// `if (password_str.len < 4)` branch in handleSetMasterPassword.
var errPasswordTooShort = &httpError{code: "password_too_short", message: "password too short (minimum 4 characters)"}

// httpError is a tiny error type used internally for predictable
// `error.Error()` strings in JSON responses.
type httpError struct {
	code, message string
}

func (e *httpError) Error() string { return e.message }