// Package webview wraps github.com/webview/webview_go to host the
// Little Timer UI in a native window.
//
// Architecture:
//
//	┌──────────────┐     Run()     ┌─────────────┐
//	│  cmd/server  │ ────────────► │  webview.Run│
//	│  (Cobra)     │               │  (blocks)   │
//	└──────────────┘               └─────────────┘
//	                                       │
//	                                       ▼
//	                       webview_go → libwebview → GTK/WKWebView/Edge
//
// The HTTP server is already running in a background goroutine
// (started by cmd/server) — this package only owns the window.  When
// the user closes the window, Run() returns and main.go stops the
// HTTP server.
//
// GTK detection (the equivalent of `build.zig:17-97`): the Go binding
// surfaces a runtime error from `webview.New()` when GTK/webkit is
// missing on Linux.  We wrap that error with a helpful install hint
// so the user gets the same message the Zig build would print, but
// without us shelling out to `pkg-config` at startup — pkg-config
// errors would slow down every boot for the 99% of users who DO have
// the deps installed.
//
// Build tags:
//
//   - `webview` — enables the native webview_go implementation
//     (`native.go`).  Without this tag, `Run()` returns a friendly
//     error and the binary still boots (HTTP server keeps serving).
//     This matches the Go ecosystem's "CGO is opt-in" convention and
//     means CI / release builds on machines without GTK can still
//     produce a working binary.
//   - `embed_ui` — switches the window's target URL from the Vite
//     dev server (:5173) to the embedded HTTP server (:8080).
//     Defined in `url_embed.go`.  Only meaningful when the `webview`
//     tag is also set.
package webview

import (
	"runtime"
)

// Title is the window title shown in the OS chrome.  Matches the
// Zig `win.setTitle("Little Timer")` call.
const Title = "Little Timer"

// DefaultSize is the initial window size in CSS pixels.  Matches the
// spec's "800x600 HintNone" — note this is a deliberate shrink from
// the Zig original (1200x780) to keep the new Go-native window
// snug on a 13" laptop.
const (
	DefaultWidth  = 800
	DefaultHeight = 600
)

// Run opens a webview window, navigates to appURL, and blocks until
// the user closes the window.  Returns any error from the underlying
// implementation — most commonly:
//
//   - "webview support not compiled in" — built without `-tags webview`
//   - "missing GTK/webkit" — built with the tag but the OS lacks the
//     CGO dependencies (mirrors the Zig `@panic("missing Linux webview
//     dependencies")` path)
//
// Mirrors `webview.Window.openDefault` + `win.run()` in the Zig
// source (`src/core/webview_c.zig:84-93` and `src/main_entry.zig:138-165`).
func Run() error { return run() }

// CheckLinuxDeps is a best-effort pre-flight for Linux.  On non-Linux
// platforms it returns (true, "").  We don't shell out to pkg-config
// at boot (the runtime error from webview.New() is enough for the
// common case); but a separate `little-timer doctor` style subcommand
// can call this to surface install hints proactively.
//
// The Zig build's `linkWebviewDesktopDeps` does the same check
// (build.zig:17-97) — we mirror the candidate set here.
func CheckLinuxDeps() (bool, string) {
	if runtime.GOOS != "linux" {
		return true, ""
	}
	return false, missingDepsHint()
}

// missingDepsHint returns the install hint shown when webview.New()
// fails on Linux.  Identical wording to the Zig `@panic` message.
func missingDepsHint() string {
	return "❌ 未检测到可用 GTK/WebKit 组合。请安装任一组合：\n" +
		"  1) gtk4 + webkitgtk-6.0\n" +
		"  2) gtk+-3.0 + webkit2gtk-4.1\n" +
		"  3) gtk+-3.0 + webkit2gtk-4.0\n"
}