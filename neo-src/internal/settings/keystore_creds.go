// 凭据状态清理层：重置路径（handlers.CredentialsReset）经本文件把 v1 时代的
// 遗留密文列与标记列复位，避免重置后 settings 仍声称"有凭据"。
//
// SettingsCreds 迁移适配层（LegacyCiphertext / PutSecret / 列名映射）已随
// v1 与一次性迁移链整体删除；backup_config 的旧密文列不再有写入方，
// 仅剩本文件的清理逻辑把它们归零。

package settings

import (
	"errors"
	"fmt"

	"little-timer/internal/crypto"
)

// ClearCredentialState 是"重置"路径用的清理：比 ClearLegacyState 多清三个
// has_* 标记列，把内存态一并复位。
//
// 与 ClearLegacyState 的语义差别是关键：迁移时代 has_* 必须**保持 1**——它
// 是"secret store 里有这个凭据"的信号。而重置是真把凭据删了，has_* 就该归 0。
func (sm *SettingsManager) ClearCredentialState() error {
	db := sm.sqlite.DB()
	if db == nil {
		return errors.New("settings: sqlite not open")
	}
	if err := sm.ClearLegacyState(); err != nil {
		return err
	}
	if _, err := db.Exec(`
        UPDATE backup_config SET
            has_webdav_password = 0,
            has_s3_access_key    = 0,
            has_s3_secret_key    = 0
        WHERE id = 1;`); err != nil {
		return fmt.Errorf("settings: 清理凭据标记失败: %w", err)
	}
	// 凭据明文也从内存态抹掉，避免本进程内还能读到已"重置"的凭据。
	sm.backupConfig.WebDAVPassword = ""
	sm.backupConfig.S3AccessKey = ""
	sm.backupConfig.S3SecretKey = ""
	return nil
}

// ClearLegacyState 复位 v1 遗留状态：三列密文置 NULL，四个主口令标记列置 0。
// **不动 has_webdav_password / has_s3_* —— 见 ClearCredentialState 的说明。**
//
// 注意这**不是**删列：Path B 决定保留 backup_config 表的存量 schema
// （cmd/migrate 的 expected columns 也列了它们），只是把值清成"没有"。
// 迁移链删除后这些列再无写入方，恒为默认值。
func (sm *SettingsManager) ClearLegacyState() error {
	db := sm.sqlite.DB()
	if db == nil {
		return errors.New("settings: sqlite not open")
	}
	_, err := db.Exec(`
        UPDATE backup_config SET
            webdav_password_encrypted = NULL,
            s3_access_key_encrypted    = NULL,
            s3_secret_key_encrypted    = NULL,
            has_master_password        = 0,
            credentials_unlock_time    = 0,
            credential_unlock_attempts = 0,
            credential_locked_until    = 0
        WHERE id = 1;`)
	if err != nil {
		return fmt.Errorf("settings: 清理旧凭据状态失败: %w", err)
	}
	// 内存态同步：主口令四列清零，避免本次进程内再把"设过主口令"当成事实。
	// has_webdav_password / has_s3_* 三列不在结构体里（凭据是否已配由
	// SecretStorage 内的条目决定，见 loadBackupConfigFromDB），故无需同步。
	sm.backupConfig.HasMasterPassword = false
	sm.backupConfig.CredentialsUnlockTime = 0
	sm.backupConfig.CredentialUnlockAttempts = 0
	sm.backupConfig.CredentialLockedUntil = 0
	return nil
}

// sm.secretStore 的类型来自 crypto.SecretStorage —— 显式标注一句，
// 避免将来 crypto 侧重构时这里静默失配。
var _ *crypto.SecretStorage
