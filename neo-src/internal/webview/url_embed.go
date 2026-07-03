//go:build webview && embed_ui

// Production URL for the webview: point at the in-process HTTP server
// on :8080, which serves the embedded HTML bundle.  Mirrors the Zig
// `app_url` for the embed build (`src/core/webview_c.zig:9-12`).
//
// Compile with `-tags "webview,embed_ui"` to use this URL.

package webview

const appURL = "http://127.0.0.1:8080/?runtime=webview"