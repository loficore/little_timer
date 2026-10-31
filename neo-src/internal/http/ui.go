package http

import (
	"io/fs"
	"net/http"
	"path"
	"strings"

	"github.com/gin-gonic/gin"
)

// wailsRuntimeStub 给非 Wails 运行时（Linux HTTP 页面）提供最小 ES module。
// vite 产物把 `/wails/runtime.js` 保留为裸 import；没有这个桩，整个入口
// module 会加载失败并白屏。桌面端只用 HTTP API，不会调用这里的 Call。
const wailsRuntimeStub = `export const Call = {
  ByID: () => { throw new Error("Wails runtime not available in this build"); },
};
export const CancellablePromise = class {};
export const Create = {};
export default {};
`

func registerRoot(r *gin.Engine, uiFS fs.FS) {
	if uiFS == nil {
		r.GET("/", func(c *gin.Context) {
			c.Header("Content-Type", "text/html; charset=utf-8")
			c.String(http.StatusOK, "<html><body><h1>Little Timer</h1><p>Build the frontend (cd assets && pnpm run build) or open the dev server.</p></body></html>")
		})
		return
	}

	if _, err := fs.Stat(uiFS, "index.html"); err != nil {
		r.GET("/", func(c *gin.Context) {
			c.String(http.StatusInternalServerError, "embedded UI is missing index.html; run scripts/stage-embedded-ui.sh before go build -tags embed_ui")
		})
		return
	}

	r.GET("/wails/runtime.js", func(c *gin.Context) {
		c.Header("Content-Type", "text/javascript; charset=utf-8")
		c.String(http.StatusOK, wailsRuntimeStub)
	})

	r.NoRoute(func(c *gin.Context) {
		if c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead {
			c.Status(http.StatusNotFound)
			return
		}

		urlPath := c.Request.URL.Path
		if urlPath == "/api" || strings.HasPrefix(urlPath, "/api/") ||
			strings.HasPrefix(urlPath, "/wails/") {
			c.Status(http.StatusNotFound)
			return
		}

		name := strings.TrimPrefix(path.Clean(urlPath), "/")
		if name != "" {
			if st, err := fs.Stat(uiFS, name); err == nil && !st.IsDir() {
				http.ServeFileFS(c.Writer, c.Request, uiFS, name)
				return
			}
		}

		// SPA 路由兜底：/habits、/schedule 等都由前端处理。
		http.ServeFileFS(c.Writer, c.Request, uiFS, "index.html")
	})
}
