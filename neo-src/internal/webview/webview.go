// Package webview 封装 github.com/webview/webview_go，把
// Little Timer UI 承载在原生窗口中。
//
// 架构：
//
//	┌──────────────┐     Run()     ┌─────────────┐
//	│  cmd/server  │ ────────────► │  webview.Run│
//	│  (Cobra)     │               │   （阻塞）  │
//	└──────────────┘               └─────────────┘
//	                                       │
//	                                       ▼
//	                       webview_go → libwebview → GTK/WKWebView/Edge
//
// HTTP server 已在后台 goroutine 中运行（由 cmd/server 启动）—— 本包
// 只负责窗口。用户关闭窗口时 Run() 返回，main.go 随即停止 HTTP server。
//
// GTK 检测：Linux 上缺少 GTK/webkit 时，Go 绑定会让 `webview.New()`
// 抛出运行时错误。我们用它包装出带安装提示的错误，而不是在启动时调用
// `pkg-config` —— 对 99% 已装好依赖的用户来说，pkg-config 探测会拖慢
// 每次启动。
//
// Build tags：
//
//   - `webview` —— 启用原生 webview_go 实现（`native.go`）。没有此 tag
//     时 `Run()` 返回友好错误，二进制仍能启动（HTTP server 继续服务）。
//     这符合 Go 生态“CGO 显式启用”的惯例，也让没有 GTK 的 CI / 发布
//     机器仍能产出可用的二进制。
//   - `embed_ui` —— 把窗口目标 URL 从 Vite 开发服务器（:5173）切换到
//     内嵌 HTTP server（:8080）。定义在 `url_embed.go`，仅当 `webview`
//     tag 同时设置时才有意义。
package webview

import (
	"runtime"
)

// Title 是操作系统窗口栏显示的标题。
const Title = "Little Timer"

// DefaultSize 是初始窗口尺寸（CSS 像素）—— 特意设得紧凑
// （800x600），保证在 13 寸笔记本上可用。
const (
	DefaultWidth  = 800
	DefaultHeight = 600
)

// Run 打开 webview 窗口，导航到 appURL，并阻塞直到用户关闭窗口。
// 返回底层实现的错误 —— 最常见的是：
//
//   - "webview support not compiled in" —— 构建时未加 `-tags webview`
//   - "missing GTK/webkit" —— 加了 tag 但系统缺少 CGO 依赖
func Run() error { return run() }

// CheckLinuxDeps 是 Linux 上的尽力而为的预检查。非 Linux 平台返回
// (true, "")。我们不在启动时调用 pkg-config（webview.New() 的运行时
// 错误对常见场景已经足够）；但独立的 `little-timer doctor` 风格
// 子命令可以调用它，主动给出安装提示。
func CheckLinuxDeps() (bool, string) {
	if runtime.GOOS != "linux" {
		return true, ""
	}
	return false, missingDepsHint()
}

// missingDepsHint 返回 Linux 上 webview.New() 失败时显示的安装提示。
func missingDepsHint() string {
	return "❌ 未检测到可用 GTK/WebKit 组合。请安装任一组合：\n" +
		"  1) gtk4 + webkitgtk-6.0\n" +
		"  2) gtk+-3.0 + webkit2gtk-4.1\n" +
		"  3) gtk+-3.0 + webkit2gtk-4.0\n"
}
