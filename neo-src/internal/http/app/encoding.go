package app

import "encoding/base64"

// base64Raw returns the URL-safe base64 encoding of b without padding.
// Used by GenerateToken to keep the auth token short enough for header
// use.
func base64Raw(b []byte) string {
	return base64.RawURLEncoding.EncodeToString(b)
}