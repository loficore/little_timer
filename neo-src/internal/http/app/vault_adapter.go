// Package app —— vault 适配器。
//
// appRuntime 让 *App 满足 vault.BackupRuntime 而不改动 *App 现有的方法名
// （BackupManager / RebuildBackup）。Phase 3c 会把 *App 的方法改名为
// Manager / Rebuild 并删除本文件。
//
// 见 .omo/plans/habit-vault-extraction.md 的 R5。
package app

import (
	"context"

	"little-timer/internal/storage/backup"
	"little-timer/internal/vault"
)

// appRuntime 把 *App 适配成 vault.BackupRuntime。
type appRuntime struct{ a *App }

// Manager 是 vault.BackupRuntime 的 Manager 适配器。
func (r appRuntime) Manager() *backup.BackupManager { return r.a.BackupManager() }

// Rebuild 是 vault.BackupRuntime 的 Rebuild 适配器。
func (r appRuntime) Rebuild(ctx context.Context) error { return r.a.RebuildBackup(ctx) }

// 编译期校验：appRuntime 满足 vault.BackupRuntime。
var _ vault.BackupRuntime = appRuntime{}
