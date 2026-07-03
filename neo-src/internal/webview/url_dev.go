//go:build webview && !embed_ui

// Default URL for the webview: point at the Vite dev server so the
// frontend's HMR works during local development.  Matches the Zig
// `app_url` for the non-embed build (`src/core/webview_c.zig:9-12`).

package webview

const appURL = "http://localhost:5173/?runtime=webview"