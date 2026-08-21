// Package backup —— 导出给跨包测试使用的测试辅助函数。
//
// 本文件导出内部类型与构造函数，让其他包（如 handlers）的测试能用自定义
// adapter 构造 BackupManager，而不必伸手碰非导出字段。
package backup

import (
	"os"

	"little-timer/internal/storage"
)

// FakeLocalAdapter 以可配置行为实现 BackupAdapter，供测试使用。
// 每个方法都有一个可选的错误字段；非 nil 时方法返回该错误而非正常行为。
//
// RestoreData 非 nil 时，Restore 写入它而不是 Backup() 期间记录的内容，
// 让测试可以模拟 SHA-256 不匹配。
type FakeLocalAdapter struct {
	backupDir   string
	backupData  []byte
	backupName  string
	backupErr   error
	restoreData []byte // 非 nil 时 Restore 写这个而不是 backupData
	restoreErr  error

	manifest     string
	manifestErr  error
	deletedNames []string
	deleteErr    error
}

// NewFakeLocalAdapter 返回一个 adapter：Backup() 记录上传的字节，
// Restore() 重放它们（若设置了 restoreData 则用它）。
func NewFakeLocalAdapter(backupDir string) *FakeLocalAdapter {
	return &FakeLocalAdapter{backupDir: backupDir}
}

// SetBackupError 让 Backup() 返回指定错误。
func (f *FakeLocalAdapter) SetBackupError(err error) { f.backupErr = err }

// SetRestoreData 让 Restore() 写入这些字节，而不是 Backup() 期间记录的内容。
func (f *FakeLocalAdapter) SetRestoreData(data []byte) { f.restoreData = data }

// Target 实现 BackupAdapter。
func (f *FakeLocalAdapter) Target() BackupTarget { return TargetLocal }

// TestConnection 实现 BackupAdapter。
func (f *FakeLocalAdapter) TestConnection() error { return nil }

// Backup 记录 srcPath 的内容。
func (f *FakeLocalAdapter) Backup(srcPath, backupName string) error {
	if f.backupErr != nil {
		return f.backupErr
	}
	data, err := os.ReadFile(srcPath)
	if err != nil {
		return err
	}
	f.backupData = data
	f.backupName = backupName
	return nil
}

// Restore 把 restoreData（若设置）或 backupData 写入 destPath。
func (f *FakeLocalAdapter) Restore(backupName, destPath string) error {
	if f.restoreErr != nil {
		return f.restoreErr
	}
	data := f.backupData
	if f.restoreData != nil {
		data = f.restoreData
	}
	return os.WriteFile(destPath, data, 0o600)
}

// List 实现 BackupAdapter。
func (f *FakeLocalAdapter) List() ([]BackupInfo, error) { return nil, nil }

// Delete 记录被删除的名字。
func (f *FakeLocalAdapter) Delete(backupName string) error {
	f.deletedNames = append(f.deletedNames, backupName)
	if f.deleteErr != nil {
		return f.deleteErr
	}
	return nil
}

// WriteManifest 保存 manifest JSON。
func (f *FakeLocalAdapter) WriteManifest(data string) error {
	if f.manifestErr != nil {
		return f.manifestErr
	}
	f.manifest = data
	return nil
}

// DeletedNames 返回传给 Delete 的名字列表。
func (f *FakeLocalAdapter) DeletedNames() []string { return f.deletedNames }

// NewManagerWithAdapter 构造接到给定 sqlite、dbPath、backupDir 与
// adapter 的 BackupManager。导出它是为了让其他包的测试能注入假 adapter。
func NewManagerWithAdapter(
	sqlite *storage.SqliteManager,
	dbPath, backupDir string,
	adapter BackupAdapter,
) *BackupManager {
	return &BackupManager{
		sqlite:     sqlite,
		dbPath:     dbPath,
		backupDir:  backupDir,
		maxBackups: MaxBackups,
		adapter:    adapter,
	}
}
