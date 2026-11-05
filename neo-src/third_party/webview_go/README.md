# webview_go（本地 fork）

上游：`github.com/webview/webview_go`，提交
`v0.0.0-20240831120633-6173450d4dd6`（MIT，见 `LICENSE`）。

改动有两处：

**一、Linux/BSD 的 `pkg-config` 模块**从 `webview.go` 的固定
`webkit2gtk-4.0` 拆成两个按 build tag 选择的文件：

| 构建方式 | 链接目标 |
| --- | --- |
| 默认（`-tags webview`） | `gtk+-3.0` + `webkit2gtk-4.0` |
| `-tags "webview,webkit41"` | `gtk+-3.0` + `webkit2gtk-4.1` |

**二、补一个 mingw 兼容头** `libs/mswebview2/include/EventToken.h`。

上游的 `WebView2.h` 里写着 `#include "EventToken.h"`，而那个头只随
Windows SDK 发布，**mingw-w64 不带**。上游 webview_go 之所以没暴露这个
问题，是因为它的 Windows 构建环境通常已经有 Windows SDK（MSVC 的
`INCLUDE`）；一旦换成 msys2 / zig 这类只有 mingw-w64 的环境，
`-tags webview` 就会 `fatal error: 'EventToken.h' file not found`。

兼容头取自上游 **`webview/webview`** 的
`compatibility/mingw/include/EventToken.h`（同为 MIT），只做了一件事：
按 Windows SDK 的布局声明 `EventRegistrationToken`（单个 64 位整数）。

Go API、`webview.h` 与 Windows 的 `WebView2.h` 均与上游一致。同步上游时
只需把新的上游文件覆盖进来，再重做上述两处改动：拆 pkg-config、补
`EventToken.h`。不要重新引入写死的 `webkit2gtk-4.0` pkg-config 行，
也不要删掉兼容头 —— 删了 msys2 上就编不过了。
