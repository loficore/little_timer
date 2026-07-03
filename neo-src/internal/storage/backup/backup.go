// Package backup — BackupManager coordinating adapters.
//
// Port of `src/storage/storage_backup.zig` (little_timer).  The Zig
// source owns its own zqlite connection and uses it to flush the WAL
// before copying the file.  In Go we delegate the "close / reopen"
// dance to the *storage.SqliteManager so the SQLite file is in a
// quiescent state when the adapter reads it — SQLite is single-writer
// and concurrent read+copy while writes are in flight is unsafe.
//
// The BackupManager dispatches every operation against a single
// BackupAdapter (constructed from a BackupConfig), which keeps the
// surface area tight.  Switching targets means constructing a new
// manager — there is no SetTarget mutator in the Zig source either.
package backup

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"little-timer/internal/domain"
	"little-timer/internal/storage"
)

// MaxBackups is the default retention cap.  Matches the Zig
// `max_backups: u32 = 10` field.
const MaxBackups = 10

// BackupManager is the Go port of `pub const BackupManager`.
type BackupManager struct {
	sqlite     *storage.SqliteManager
	dbPath     string
	backupDir  string
	maxBackups int
	adapter    BackupAdapter
}

// NewLocal returns a BackupManager wired to a LocalAdapter rooted at
// backupDir.  Mirrors `BackupManager.init`.
func NewLocal(sqliteMgr *storage.SqliteManager, dbPath, backupDir string) (*BackupManager, error) {
	if backupDir == "" {
		return nil, errors.New("backup: backupDir is required for local target")
	}
	if err := os.MkdirAll(backupDir, 0o700); err != nil {
		return nil, fmt.Errorf("backup: mkdir %s: %w", backupDir, err)
	}
	return &BackupManager{
		sqlite:     sqliteMgr,
		dbPath:     dbPath,
		backupDir:  backupDir,
		maxBackups: MaxBackups,
		adapter:    NewLocalAdapter(backupDir),
	}, nil
}

// NewFromConfig picks an adapter based on cfg.TargetType and wires it
// into a fresh BackupManager.  Mirrors `BackupManager.initWithConfig`.
func NewFromConfig(ctx context.Context, sqliteMgr *storage.SqliteManager, dbPath, backupDir string, cfg domain.BackupConfig) (*BackupManager, error) {
	mgr, err := NewLocal(sqliteMgr, dbPath, backupDir)
	if err != nil {
		return nil, err
	}
	adapter, err := buildAdapter(ctx, cfg, backupDir)
	if err != nil {
		return nil, err
	}
	mgr.adapter = adapter
	return mgr, nil
}

// buildAdapter picks the right adapter for the configured target type.
// webdav / s3 always need their full config; local falls back to
// backupDir when no path was supplied.
func buildAdapter(ctx context.Context, cfg domain.BackupConfig, backupDir string) (BackupAdapter, error) {
	switch cfg.TargetType {
	case domain.BackupTargetWebDAV:
		return NewWebDAVAdapter(WebDAVConfig{
			URL:      cfg.WebDAVURL,
			Username: cfg.WebDAVUsername,
			Password: cfg.WebDAVPassword,
			BasePath: "/",
		}), nil
	case domain.BackupTargetS3:
		return NewS3Adapter(ctx, S3Config{
			Endpoint:   cfg.S3Endpoint,
			Bucket:     cfg.S3Bucket,
			Region:     cfg.S3Region,
			AccessKey:  cfg.S3AccessKey,
			SecretKey:  cfg.S3SecretKey,
			PathPrefix: cfg.S3PathPrefix,
		})
	default:
		path := cfg.LocalPath
		if path == "" {
			path = backupDir
		}
		return NewLocalAdapter(path), nil
	}
}

// Adapter returns the underlying adapter (handy for tests).
func (m *BackupManager) Adapter() BackupAdapter { return m.adapter }

// MaxBackups returns the retention cap.  Mirrors `pub max_backups`.
func (m *BackupManager) MaxBackups() int { return m.maxBackups }

// SetMaxBackups adjusts the retention cap.
func (m *BackupManager) SetMaxBackups(n int) {
	if n > 0 {
		m.maxBackups = n
	}
}

// -----------------------------------------------------------------------------
// Backup / restore.
// -----------------------------------------------------------------------------

// CreateBackup generates a new backup file and uploads it via the
// configured adapter.  Mirrors `pub fn createBackup`.
func (m *BackupManager) CreateBackup() (string, error) {
	if m.sqlite == nil || !m.sqlite.IsOpen() {
		return "", fmt.Errorf("%w: sqlite not open", ErrBackupFailed)
	}
	ts := time.Now().Unix()
	name := fmt.Sprintf("%s%d%s", filenamePrefix, ts, filenameSuffix)

	// Flush WAL so the on-disk file is consistent.  database/sql doesn't
	// surface `wal_checkpoint` via stdlib; the SQL "PRAGMA wal_checkpoint
	// (TRUNCATE)" is the canonical hook.
	if _, err := m.sqlite.DB().Exec("PRAGMA wal_checkpoint(TRUNCATE);"); err != nil {
		return "", fmt.Errorf("%w: wal_checkpoint: %v", ErrBackupFailed, err)
	}

	if err := m.adapter.Backup(m.dbPath, name); err != nil {
		return "", err
	}
	if err := m.cleanupOldBackups(); err != nil {
		// Retention is best-effort; log but don't fail the backup.
		fmt.Fprintf(os.Stderr, "backup: cleanup: %v\n", err)
	}
	return name, nil
}

// RestoreFromBackup fetches a backup by name and overwrites the live DB.
// Mirrors `pub fn restoreFromBackup`.  The DB connection is closed
// during the swap and reopened afterward so the running app picks up
// the restored schema.
func (m *BackupManager) RestoreFromBackup(name string) error {
	if m.sqlite == nil || !m.sqlite.IsOpen() {
		return fmt.Errorf("%w: sqlite not open", ErrRestoreFailed)
	}
	tmp, err := os.CreateTemp(filepath.Dir(m.dbPath), "lt_restore_*.db")
	if err != nil {
		return fmt.Errorf("%w: tempfile: %v", ErrRestoreFailed, err)
	}
	tmpPath := tmp.Name()
	_ = tmp.Close()
	defer os.Remove(tmpPath)

	if err := m.adapter.Restore(name, tmpPath); err != nil {
		return err
	}
	if err := m.swapDatabase(tmpPath); err != nil {
		return err
	}
	return nil
}

// swapDatabase closes the SQLite connection, replaces the file, then
// reopens.  Mirrors the close/reopen dance in storage_backup.zig.
func (m *BackupManager) swapDatabase(src string) error {
	if err := m.sqlite.Close(); err != nil {
		return fmt.Errorf("%w: close before swap: %v", ErrRestoreFailed, err)
	}
	if err := os.Rename(src, m.dbPath); err != nil {
		return fmt.Errorf("%w: rename: %v", ErrRestoreFailed, err)
	}
	if err := m.sqlite.Open(); err != nil {
		return fmt.Errorf("%w: reopen after swap: %v", ErrRestoreFailed, err)
	}
	if err := m.sqlite.Migrate(); err != nil {
		return fmt.Errorf("%w: migrate after swap: %v", ErrRestoreFailed, err)
	}
	return nil
}

// DeleteBackup removes a single backup.  Mirrors `pub fn deleteBackup`.
func (m *BackupManager) DeleteBackup(name string) error {
	return m.adapter.Delete(name)
}

// ListBackups returns every backup known to the adapter.  Mirrors
// `pub fn listBackups`.
func (m *BackupManager) ListBackups() ([]BackupInfo, error) {
	return m.adapter.List()
}

// TestConnection validates the configured adapter is reachable.
func (m *BackupManager) TestConnection() error {
	return m.adapter.TestConnection()
}

// -----------------------------------------------------------------------------
// BackupInfo helpers — `getBackupInfo` / `freeBackupInfo` analogues.
// -----------------------------------------------------------------------------

// BackupSummary is the analogue of `getBackupInfo`'s anonymous struct.
type BackupSummary struct {
	TotalBackups   int    `json:"total_backups"`
	TotalSizeBytes uint64 `json:"total_size_bytes"`
	OldestBackup   string `json:"oldest_backup,omitempty"`
	NewestBackup   string `json:"newest_backup,omitempty"`
}

// Summary aggregates counts and size of the adapter's stored backups.
func (m *BackupManager) Summary() (BackupSummary, error) {
	items, err := m.adapter.List()
	if err != nil {
		return BackupSummary{}, err
	}
	if len(items) == 0 {
		return BackupSummary{}, nil
	}
	sorted := append([]BackupInfo(nil), items...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Timestamp < sorted[j].Timestamp })

	var totalSize uint64
	for _, it := range sorted {
		totalSize += it.SizeBytes
	}
	return BackupSummary{
		TotalBackups:   len(sorted),
		TotalSizeBytes: totalSize,
		OldestBackup:   sorted[0].Name,
		NewestBackup:   sorted[len(sorted)-1].Name,
	}, nil
}

// cleanupOldBackups trims the adapter down to m.maxBackups entries by
// deleting the oldest.  Local + WebDAV + S3 all support Delete so this
// is uniform across adapters.
func (m *BackupManager) cleanupOldBackups() error {
	items, err := m.adapter.List()
	if err != nil {
		return err
	}
	if len(items) <= m.maxBackups {
		return nil
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Timestamp < items[j].Timestamp })
	toDelete := items[:len(items)-m.maxBackups]
	for _, it := range toDelete {
		if err := m.adapter.Delete(it.Name); err != nil {
			fmt.Fprintf(os.Stderr, "backup: delete %s: %v\n", it.Name, err)
		}
	}
	return nil
}