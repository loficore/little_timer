package handlers

import (
	"encoding/json"
	"strings"
)

// jsonUnmarshal 是 json.Unmarshal 的薄包装，把 encoding/json 的 import
// 收在本文件而不是每个 handler 里。
func jsonUnmarshal(raw []byte, dst any) error {
	return json.Unmarshal(raw, dst)
}

// validBackupName 拒绝 path-traversal 尝试和其他不安全文件名。
func validBackupName(name string) bool {
	if name == "" {
		return false
	}
	if strings.Contains(name, "..") {
		return false
	}
	if strings.ContainsAny(name, "/\\") {
		return false
	}
	for _, c := range name {
		if c < 0x20 {
			return false
		}
	}
	return true
}
