package handlers

import (
	"encoding/json"
	"strings"
)

// jsonUnmarshal is a thin wrapper around json.Unmarshal that keeps the
// encoding/json import in this file rather than every handler.
func jsonUnmarshal(raw []byte, dst any) error {
	return json.Unmarshal(raw, dst)
}

// validBackupName rejects path-traversal attempts and other unsafe
// filenames.  Mirrors the Zig `if (backup_name.len == 0 || mem.indexOf
// ("..") != null || mem.indexOfAny ("/\\") != null)` check.
func validBackupName(name string) bool {
	if name == "" {
		return false
	}
	// Reject path traversal ".."
	if strings.Contains(name, "..") {
		return false
	}
	// Reject / or \ anywhere (no path separators allowed)
	if strings.ContainsAny(name, "/\\") {
		return false
	}
	// Reject control characters
	for _, c := range name {
		if c < 0x20 {
			return false
		}
	}
	return true
}