// Package middleware — Bearer token auth.
//
// File `auth.go` ports the Zig `validateAuth` function (std_server.zig).
// The Zig code:
//
//  1. Checks if auth is enabled at all (`auth_enabled` bool from settings);
//     if disabled, accepts every request.
//  2. If `auth_token` is empty, accepts every request (matches the Zig
//     "no token configured → no auth required" branch).
//  3. Reads `Authorization: Bearer <token>` first, then falls back to
//     `?auth_token=<token>` (legacy form).
//  4. Sends a 401 with `{"err":"Unauthorized: Invalid or missing token"}`
//     on failure.
//
// The Go port exposes the same behaviour via `Auth(app)` and uses a
// per-handler opt-out (`Public()`) for endpoints that should never be
// gated, even when auth is enabled.  The Zig source has an explicit
// `public_paths` list — we keep it inline below so future maintainers
// can see them all in one place.
package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"little-timer/internal/http/app"
	"crypto/subtle"
)

// Public path prefixes / exact matches that bypass auth regardless of
// `auth_enabled`.  Mirrors the Zig `public_paths = [_][]const u8{ "/",
// "/api/log", "/api/events" }` list (note: the Zig list also includes
// "/" but we treat that as handled separately by the SPA fallback in
// the router; "/" is therefore not enforced here).
var publicPathSet = map[string]bool{
	"/api/events":      true,
	"/api/auth/status": true,
}

// isPublic reports whether the request path bypasses auth.  Exact-match
// only — Zig's list was also exact-match.
func isPublic(path string) bool {
	return publicPathSet[path]
}

// Auth returns a Gin middleware that enforces Bearer-token auth against
// the auth block on the supplied App's SettingsManager.  Reads:
//
//   - Header: `Authorization: Bearer <token>` (preferred)
//   - Query:  `?auth_token=<token>`           (legacy fallback)
//
// On failure responds with 401 + `{"err":"Unauthorized: Invalid or missing token"}`
// matching the Zig error shape exactly.  On success sets the "app"
// context key (so downstream handlers can pull `c.MustGet("app")`).
//
// Nil-Settings behaviour: when a.Settings is nil (only in smoke tests
// where the storage stack isn't wired up) the middleware lets every
// request through.  This matches what would happen in production when
// auth_enabled is false — i.e. auth is opt-in, not enforced.
func Auth(a *app.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set("app", a)

		if isPublic(c.Request.URL.Path) {
			c.Next()
			return
		}

		if a.Settings == nil {
			c.Next()
			return
		}

		auth := a.Settings.Config().Auth
		if !auth.AuthEnabled {
			c.Next()
			return
		}
		if auth.AuthToken == "" {
			c.Next()
			return
		}

		// Header takes priority — Zig checks the header first, then
		// the URL query param.
		provided := extractBearer(c.GetHeader("Authorization"))
		if provided == "" {
			provided = c.Query("auth_token")
		}
		if provided != "" && subtle.ConstantTimeCompare([]byte(provided), []byte(auth.AuthToken)) == 1 {
			c.Next()
			return
		}

		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
			"err": "Unauthorized: Invalid or missing token",
		})
	}
}

// extractBearer strips the "Bearer " prefix from a header value.
// Returns "" if the header is missing or doesn't follow the
// `Bearer <token>` shape.
func extractBearer(header string) string {
	const prefix = "Bearer "
	if len(header) <= len(prefix) {
		return ""
	}
	if header[:len(prefix)] != prefix {
		return ""
	}
	return header[len(prefix):]
}