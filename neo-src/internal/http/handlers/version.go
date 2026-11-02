// Package handlers —— /api/version:进程级构建信息。
//
// 前端 UI 的版本号只允许来自这里(链接期由 ldflags 注入 app.Version),
// 组件里不得再写死版本字符串。
package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"little-timer/internal/app"
)

// VersionDTO 是 GET /api/version 的出参。
type VersionDTO struct {
	// Version 是人可读的发布版本(已去掉 tag 的 v 前缀)。
	Version string `json:"version"`
	// GitCommit 是构建时的短 commit hash,非 git 环境为 "unknown"。
	GitCommit string `json:"git_commit"`
	// BuildTime 是二进制加载时捕获的 RFC3339 时间。
	BuildTime string `json:"build_time"`
}

// VersionGet GET /api/version
//
// 只读接口,无乐观锁语义;ConflictGuard 只拦 mutation,因此无需 If-Match。
func VersionGet(c *gin.Context) {
	c.JSON(http.StatusOK, VersionDTO{
		Version:   app.Version,
		GitCommit: app.GitCommit,
		BuildTime: app.BuildTime,
	})
}
