//go:build webview

// 原生 webview 实现 —— 仅在传入 `-tags webview` 时参与编译。
// 没有该 tag 时，`stub.go` 提供同名 `run()` 函数并返回友好错误，
// 保证二进制仍能启动。

package webview

import (
	"fmt"

	"github.com/webview/webview_go"
)

func run() error {
	w := webview.New(false)
	if w == nil {
		return fmt.Errorf("webview: failed to create window — on Linux, install one of: gtk4+webkitgtk-6.0, gtk+-3.0+webkit2gtk-4.1, gtk+-3.0+webkit2gtk-4.0\n%s", missingDepsHint())
	}
	defer w.Destroy()

	w.SetTitle(Title)
	w.SetSize(DefaultWidth, DefaultHeight, webview.HintNone)
	w.Navigate(appURL)
	w.Run()
	return nil
}
