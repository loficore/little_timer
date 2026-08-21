// Package middleware 包含 little-timer HTTP server 使用的 Gin 中间件。
package middleware

import (
	"github.com/gin-gonic/gin"
)

// CORS 返回一个 Gin 中间件：设置标准 CORS 头并短路 OPTIONS 预检请求。
//
// 前端可能从 `localhost:5173`（Vite dev）或任意其他 host 提供服务，
// 所以 HTTP 层必须放行这些 origin。
//
// `allowOrigin` 控制 `Access-Control-Allow-Origin`。用 "*" 表示
// “全部允许”；传具体 origin 即可收紧。空字符串按 "*" 处理。
func CORS(allowOrigin string) gin.HandlerFunc {
	if allowOrigin == "" {
		allowOrigin = "*"
	}
	return func(c *gin.Context) {
		header := c.Writer.Header()
		header.Set("Access-Control-Allow-Origin", allowOrigin)
		header.Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		header.Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Accept, Origin, X-Requested-With")
		header.Set("Access-Control-Expose-Headers", "Content-Length, Content-Type")
		header.Set("Access-Control-Max-Age", "86400")

		// CORS 规范禁止 Allow-Credentials 与通配 origin 同时使用。
		if allowOrigin != "*" {
			header.Set("Access-Control-Allow-Credentials", "true")
		}

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	}
}
