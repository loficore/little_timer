//go:build !webview

// Stub implementation — compiled when `-tags webview` is NOT passed.
// Lets the binary link cleanly on machines without GTK/webkit dev
// packages installed, so `little-timer serve --http-only` still works
// (the webview path simply returns an error at runtime).

package webview

import "fmt"

func run() error {
	return fmt.Errorf("webview support not compiled in — rebuild with `-tags webview` (requires gtk+-3.0+webkit2gtk-4.0 dev packages)")
}