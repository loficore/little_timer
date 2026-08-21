package app

import "encoding/base64"

// base64Raw 返回 b 的 URL-safe base64 编码（无 padding）。GenerateToken
// 用它把 auth token 控制在够短、能放进 header。
func base64Raw(b []byte) string {
	return base64.RawURLEncoding.EncodeToString(b)
}
