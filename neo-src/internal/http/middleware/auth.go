// Package middleware —— Bearer token 鉴权。
//
// 通过 `Auth(app)` 暴露鉴权检查，并用 per-handler 豁免（`Public()`）让
// 某些 endpoint 即使在鉴权开启时也永不设卡。公开路径直接列在下方，
// 方便后来的维护者一眼看全。
package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"crypto/subtle"
	"little-timer/internal/http/app"
)

// 无论 `auth_enabled` 与否都绕过鉴权的路径前缀 / 精确匹配。
var publicPathSet = map[string]bool{
	"/api/events":      true,
	"/api/auth/status": true,
}

// isPublic 报告请求路径是否绕过鉴权（精确匹配）。
func isPublic(path string) bool {
	return publicPathSet[path]
}

// Auth 返回一个 Gin 中间件：针对所给 App 的 SettingsManager 的 auth 块
// 执行 Bearer-token 鉴权。读取：
//
//   - Header: `Authorization: Bearer <token>`（优先）
//   - Query:  `?auth_token=<token>`（旧版回退）
//
// 失败时响应 401 + `{"err":"Unauthorized: Invalid or missing token"}`。
// 成功时设置 "app" 上下文键（下游 handler 即可取 `c.MustGet("app")`）。
//
// Settings 为 nil 的行为：当 a.Settings 为 nil（仅出现在没接存储栈的
// smoke 测试里）时，中间件放行一切请求 —— 鉴权是选择性启用，不是强制。
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

		// Header 优先于 URL query 参数。
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

// extractBearer 从 header 值中剥掉 "Bearer " 前缀。header 缺失或不符合
// `Bearer <token>` 形状时返回 ""。
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
