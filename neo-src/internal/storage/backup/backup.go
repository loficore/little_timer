// Package backup —— 协调各 adapter 的 BackupManager。
//
// manager 把“关闭 / 重开”这套动作委托给 *storage.SqliteManager，
// 保证 adapter 读取时 SQLite 文件处于静默状态 —— SQLite 是单写者，
// 写入进行中还并发 read+copy 并不安全。
//
// BackupManager 把每个操作都派发到单个 BackupAdapter（由 BackupConfig
// 构造），从而收紧 API 面。切换 target 意味着构造新 manager ——
// 没有 SetTarget 变更器。
package backup

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"little-timer/internal/domain"
	"little-timer/internal/log"
	"little-timer/internal/storage"
)

// MaxBackups 是默认的保留上限。
const MaxBackups = 10

type BackupManager struct {
	sqlite     *storage.SqliteManager
	dbPath     string
	backupDir  string
	maxBackups int
	adapter    BackupAdapter
}

// NewLocal 返回接到 LocalAdapter（根目录为 backupDir）的 BackupManager。
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

// NewFromConfig 根据 cfg.TargetType 选择 adapter，并接入全新的
// BackupManager。
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

// buildAdapter 为配置的 target 类型挑选正确的 adapter。
// webdav / s3 一定需要完整配置；local 在未提供路径时回退到 backupDir。
func buildAdapter(ctx context.Context, cfg domain.BackupConfig, backupDir string) (BackupAdapter, error) {
	switch cfg.TargetType {
	case domain.BackupTargetWebDAV:
		return NewWebDAVAdapter(WebDAVConfig{
			URL:      cfg.WebDAVURL,
			Username: cfg.WebDAVUsername,
			Password: cfg.WebDAVPassword,
			BasePath: cfg.WebDAVPathPrefix,
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

// Adapter 返回底层 adapter（对测试方便）。
func (m *BackupManager) Adapter() BackupAdapter { return m.adapter }

// MaxBackups 返回保留上限。
func (m *BackupManager) MaxBackups() int { return m.maxBackups }

// SetMaxBackups 调整保留上限。
func (m *BackupManager) SetMaxBackups(n int) {
	if n > 0 {
		m.maxBackups = n
	}
}

// CreateBackup 生成新备份文件并通过配置的 adapter 上传。
//
// 用 VACUUM INTO 做热快照（要求 SQLite >= 3.27.0），替代旧的
// wal_checkpoint + 直接拷贝。SHA-256 在上传前从快照计算，随后通过
// 下载已上传的副本比对摘要来验证。
//
// manifest 写入是尽力而为：已上传的 .db 文件才是事实来源（已通过
// SHA-256 验证），所以 manifest 构建/写入失败只记警告，不会让备份失败。
func (m *BackupManager) CreateBackup() (string, error) {
	if m.sqlite == nil || !m.sqlite.IsOpen() {
		return "", fmt.Errorf("%w: sqlite not open", ErrBackupFailed)
	}
	ts := time.Now().Unix()
	name := fmt.Sprintf("%s%d%s", filenamePrefix, ts, filenameSuffix)

	// 为 VACUUM INTO 热备份创建临时文件。
	tmp, err := os.CreateTemp(filepath.Dir(m.dbPath), "lt_backup_*.db")
	if err != nil {
		return "", fmt.Errorf("%w: tempfile: %v", ErrBackupFailed, err)
	}
	tmpPath := tmp.Name()
	if err := tmp.Close(); err != nil {
		log.Warn("CreateBackup: temp close failed", "error", err.Error())
	}
	defer os.Remove(tmpPath)

	// 热备份：VACUUM INTO 把一致性快照写入临时文件。需要短暂的排他锁；
	// 对低频备份可以接受。SQLite 不支持 VACUUM INTO 的绑定参数，
	// 所以路径里的单引号必须翻倍转义。
	escapedPath := strings.ReplaceAll(tmpPath, "'", "''")
	if _, err := m.sqlite.DB().Exec(fmt.Sprintf("VACUUM INTO '%s'", escapedPath)); err != nil {
		log.Error("CreateBackup: vacuum failed", "error", err.Error())
		return "", fmt.Errorf("%w: vacuum into: %v", ErrBackupFailed, err)
	}

	// 上传前从临时文件计算 SHA-256。
	hash, err := sha256File(tmpPath)
	if err != nil {
		log.Error("CreateBackup: sha256 failed", "error", err.Error())
		return "", fmt.Errorf("%w: sha256: %v", ErrBackupFailed, err)
	}

	// 取文件大小用于 manifest。
	fi, err := os.Stat(tmpPath)
	if err != nil {
		log.Error("CreateBackup: stat failed", "error", err.Error())
		return "", fmt.Errorf("%w: stat: %v", ErrBackupFailed, err)
	}
	sizeBytes := uint64(fi.Size())

	// 上传临时文件。
	if err := m.adapter.Backup(tmpPath, name); err != nil {
		log.Error("CreateBackup: upload failed", "error", err.Error())
		return "", err
	}

	// GET-back 校验：下载已上传的备份并比对 SHA-256。
	tmpVerify, err := os.CreateTemp(filepath.Dir(m.dbPath), "lt_verify_*.db")
	if err != nil {
		log.Error("CreateBackup: verify tempfile failed", "error", err.Error())
		return "", fmt.Errorf("%w: verify temp: %v", ErrBackupFailed, err)
	}
	tmpVerifyPath := tmpVerify.Name()
	if err := tmpVerify.Close(); err != nil {
		log.Warn("CreateBackup: verify temp close failed", "error", err.Error())
	}
	defer os.Remove(tmpVerifyPath)

	if err := m.adapter.Restore(name, tmpVerifyPath); err != nil {
		log.Error("CreateBackup: verify restore failed", "error", err.Error())
		return "", fmt.Errorf("%w: verify restore: %v", ErrBackupFailed, err)
	}

	verifyHash, err := sha256File(tmpVerifyPath)
	if err != nil {
		log.Error("CreateBackup: verify sha256 failed", "error", err.Error())
		return "", fmt.Errorf("%w: verify sha256: %v", ErrBackupFailed, err)
	}

	if hash != verifyHash {
		log.Error("CreateBackup: sha256 mismatch",
			"expected", hash, "got", verifyHash)
		// 尽力删除损坏的备份。
		if delErr := m.adapter.Delete(name); delErr != nil {
			log.Error("CreateBackup: delete after mismatch failed", "error", delErr.Error())
		}
		return "", fmt.Errorf("%w: sha256 mismatch", ErrBackupFailed)
	}

	// 为所有 target 写 manifest。尽力而为：已上传的 .db 已通过
	// SHA-256 验证，manifest 失败只会降低索引的便利性 ——
	// 记警告，不让备份失败。
	manifest, err := m.buildManifest(name, ts, hash, sizeBytes)
	if err != nil {
		log.Warn("CreateBackup: build manifest failed", "error", err.Error())
	} else if err := m.adapter.WriteManifest(manifest); err != nil {
		log.Warn("CreateBackup: write manifest failed", "error", err.Error())
	}

	if err := m.cleanupOldBackups(); err != nil {
		// 保留策略是尽力而为；记日志但不让备份失败。
		log.Error("CreateBackup: cleanup failed", "error", err.Error())
	}
	log.Info("CreateBackup: success", "name", name, "sha256", hash)
	return name, nil
}

// RestoreFromBackup 按名字取回备份并覆盖活动 DB。交换期间会关闭 DB
// 连接、完成后重开，让运行中的应用拿到恢复后的 schema。
func (m *BackupManager) RestoreFromBackup(name string) error {
	if m.sqlite == nil || !m.sqlite.IsOpen() {
		return fmt.Errorf("%w: sqlite not open", ErrRestoreFailed)
	}
	tmp, err := os.CreateTemp(filepath.Dir(m.dbPath), "lt_restore_*.db")
	if err != nil {
		return fmt.Errorf("%w: tempfile: %v", ErrRestoreFailed, err)
	}
	tmpPath := tmp.Name()
	if err := tmp.Close(); err != nil {
		log.Warn("RestoreFromBackup: temp close failed", "error", err.Error())
	}
	defer os.Remove(tmpPath)

	if err := m.adapter.Restore(name, tmpPath); err != nil {
		return err
	}
	if err := m.swapDatabase(tmpPath); err != nil {
		return err
	}
	return nil
}

// buildManifest 为 WriteManifest 生成 manifest JSON 字符串。
// sha256 与 sizeBytes 取自刚创建的备份文件，使每个条目都带完整性元数据。
func (m *BackupManager) buildManifest(backupName string, timestamp int64, sha256hash string, sizeBytes uint64) (string, error) {
	backups, err := m.adapter.List()
	if err != nil {
		return "", fmt.Errorf("list backups: %w", err)
	}

	type manifestBackup struct {
		Name      string `json:"name"`
		Timestamp int64  `json:"timestamp"`
		SizeBytes uint64 `json:"size_bytes"`
		SHA256    string `json:"sha256"`
	}

	manifestBackups := make([]manifestBackup, 0, len(backups)+1)
	for _, b := range backups {
		if b.Name == backupName {
			continue // 跳过刚创建的备份；下面会带 SHA256 追加
		}
		manifestBackups = append(manifestBackups, manifestBackup{
			Name:      b.Name,
			Timestamp: b.Timestamp,
			SizeBytes: b.SizeBytes,
		})
	}
	manifestBackups = append(manifestBackups, manifestBackup{
		Name:      backupName,
		Timestamp: timestamp,
		SizeBytes: sizeBytes,
		SHA256:    sha256hash,
	})

	manifest := struct {
		Version   int              `json:"version"`
		Backups   []manifestBackup `json:"backups"`
		DBVersion string           `json:"db_version"`
	}{
		Version:   1,
		Backups:   manifestBackups,
		DBVersion: "1.0",
	}

	data, err := json.Marshal(manifest)
	if err != nil {
		return "", fmt.Errorf("marshal manifest: %w", err)
	}

	return string(data), nil
}

// swapDatabase 关闭 SQLite 连接、替换文件，然后重开。
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

// DeleteBackup 删除单个备份。
func (m *BackupManager) DeleteBackup(name string) error {
	return m.adapter.Delete(name)
}

// ListBackups 返回 adapter 已知的所有备份。
func (m *BackupManager) ListBackups() ([]BackupInfo, error) {
	return m.adapter.List()
}

// TestConnection 校验配置的 adapter 可达。
func (m *BackupManager) TestConnection() error {
	return m.adapter.TestConnection()
}

// BackupSummary 汇总 adapter 内备份的数量与总大小。
type BackupSummary struct {
	TotalBackups   int    `json:"total_backups"`
	TotalSizeBytes uint64 `json:"total_size_bytes"`
	OldestBackup   string `json:"oldest_backup,omitempty"`
	NewestBackup   string `json:"newest_backup,omitempty"`
}

// Summary 聚合 adapter 所存备份的数量与大小。
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

// sha256File 以流式方式计算文件的 SHA-256 十六进制摘要。
func sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// cleanupOldBackups 通过删除最旧的条目把 adapter 修剪到 m.maxBackups
// 条。Local + WebDAV + S3 都支持 Delete，因此各 adapter 行为一致。
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
