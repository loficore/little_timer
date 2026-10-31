//go:build webview

// 原生 webview 实现 —— 仅在传入 `-tags webview` 时参与编译。
// 没有该 tag 时，`stub.go` 提供同名 `run()` 函数并返回友好错误，
// 保证二进制仍能启动。

package webview

import (
	"fmt"
	"os"

	"github.com/webview/webview_go"
)

func run() error {
	w := webview.New(devtoolsEnabled())
	if w == nil {
		return fmt.Errorf("webview: failed to create window — on Linux, install gtk3 + webkit2gtk-4.0 (default) or gtk3 + webkit2gtk-4.1 (build with -tags webkit41)\n%s", missingDepsHint())
	}
	defer w.Destroy()

	if devtoolsEnabled() {
		fmt.Fprintln(os.Stderr, "webview: devtools 已开。远程 inspector 由 WEBKIT_INSPECTOR_SERVER 决定地址(默认无,右键页面 → Inspect Element)。")
		// Navigate 之前绑定:webview.h 的 bind 走 user script(init),新文档加载时自动重放,
		// 页面里的 cadence.js 因此一定能看到 window.ltProfLog。
		if err := w.Bind(profBindingName, func(msg string) error {
			return writeProfLine(os.Stderr, msg)
		}); err != nil {
			fmt.Fprintln(os.Stderr, "webview: ltProfLog 绑定失败,[CAD] 行只留在页面 console:", err)
		}
	}
	w.SetTitle(Title)
	w.SetSize(DefaultWidth, DefaultHeight, webview.HintNone)
	w.Navigate(appURL)
	w.Run()
	return nil
}
