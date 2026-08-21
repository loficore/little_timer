//go:build webview && embed_ui

// webview 生产 URL：指向 :8080 上的进程内 HTTP server，
// 由它提供内嵌的 HTML 构建产物。
//
// 编译时使用 `-tags "webview,embed_ui"` 启用此 URL。

package webview

const appURL = "http://127.0.0.1:8080/?runtime=webview"
