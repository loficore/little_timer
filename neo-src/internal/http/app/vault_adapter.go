// Package app —— Phase 1 vault 适配器。
//
// appRuntime 让 *App 满足 vault.Runtime 而不改动 *App 现有的方法名
// （BackupManager / RebuildBackup / HasMasterPassword 等）。Phase 3a 会把
// *App 的 master password 方法搬进 vault.Operations，Phase 3c 会把
// *App 的方法改名为 Manager / Rebuild 并删除本文件。
//
// 见 .omo/plans/habit-vault-extraction.md 的 R5。
package app

import (
	"context"

	"little-timer/internal/storage/backup"
	"little-timer/internal/vault"
)

// appRuntime 把 *App 适配成 vault.Runtime。
//
// 嵌入 *App 让 HasMasterPassword / IsUnlocked / UnlockCredentials /
// SetMasterPassword / LockCredentials / GetMasterPasswordStatus 六个方法
// 直接继承；只需显式补上 vault 命名的 Manager / Rebuild。
type appRuntime struct{ *App }

// Manager 是 vault.BackupRuntime 的 Manager 适配器。
func (r appRuntime) Manager() *backup.BackupManager { return r.App.BackupManager() }

// Rebuild 是 vault.BackupRuntime 的 Rebuild 适配器。
func (r appRuntime) Rebuild(ctx context.Context) error { return r.App.RebuildBackup(ctx) }

// 编译期校验：appRuntime 满足 vault.Runtime。
var _ vault.Runtime = appRuntime{ nil }
