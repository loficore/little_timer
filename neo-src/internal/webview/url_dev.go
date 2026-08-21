//go:build webview && !embed_ui

// webview 默认 URL：指向 Vite 开发服务器，让前端 HMR
// 在本地开发时生效。

package webview

const appURL = "http://localhost:5173/?runtime=webview"
