package app

import (
	"encoding/json"

	"little-timer/internal/domain"
)

// backupConfigToJSON 把 BackupConfig 序列化成可直接喂给
// SettingsManager.UpdateBackupConfigFromJSON 的 JSON 字符串。直接 marshal
// struct 是保留字段名的最简办法 —— BackupConfig 的 JSON tag 本来就和
// 解析器期望的 wire 形状一致。
func backupConfigToJSON(cfg domain.BackupConfig) string {
	b, err := json.Marshal(cfg)
	if err != nil {
		return "{}"
	}
	return string(b)
}
