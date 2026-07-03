package app

import (
	"encoding/json"

	"little-timer/internal/domain"
)

// backupConfigToJSON serialises a BackupConfig to a JSON string suitable
// for SettingsManager.UpdateBackupConfigFromJSON.
//
// The Zig source's `updateBackupConfig` accepts a raw JSON body copied
// straight from the request; in Go we round-trip through
// `UpdateBackupConfigFromJSON` which expects the same shape.  Marshalling
// the BackupConfig struct directly is the simplest way to preserve field
// names — BackupConfig's tags already match the Zig field names.
func backupConfigToJSON(cfg domain.BackupConfig) string {
	b, err := json.Marshal(cfg)
	if err != nil {
		return "{}"
	}
	return string(b)
}