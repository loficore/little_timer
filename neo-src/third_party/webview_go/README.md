# webview_go（本地 fork）

上游：`github.com/webview/webview_go`，提交
`v0.0.0-20240831120633-6173450d4dd6`（MIT，见 `LICENSE`）。

改动只有一处：Linux/BSD 的 `pkg-config` 模块从 `webview.go` 的固定
`webkit2gtk-4.0` 拆成两个按 build tag 选择的文件：

| 构建方式 | 链接目标 |
| --- | --- |
| 默认（`-tags webview`） | `gtk+-3.0` + `webkit2gtk-4.0` |
| `-tags "webview,webkit41"` | `gtk+-3.0` + `webkit2gtk-4.1` |

Go API、`webview.h` 与 Windows 的 `WebView2.h` 均与上游一致。同步上游时
只需把新的上游文件覆盖进来，再重做上述拆分；不要重新引入写死的
`webkit2gtk-4.0` pkg-config 行。
