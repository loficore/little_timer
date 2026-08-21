//go:build !webview

// 桩实现 —— 未传入 `-tags webview` 时参与编译。
// 让二进制在没有 GTK/webkit 开发包的机器上也能干净链接，
// `little-timer serve --http-only` 照常工作（webview 路径只在
// 运行时返回错误）。

package webview

import "fmt"

func run() error {
	return fmt.Errorf("webview support not compiled in — rebuild with `-tags webview` (requires gtk+-3.0+webkit2gtk-4.0 dev packages)")
}
