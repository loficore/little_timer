// EventToken.h —— mingw-w64 缺 Windows SDK 的 um/EventToken.h 时的兼容头。
//
// 来源：https://github.com/webview/webview/blob/master/compatibility/mingw/include/EventToken.h
// 许可：MIT（与本 fork 的 webview_go 一致，见 ../LICENSE）。
//
// 为什么需要它：上游 webview_go 的 WebView2.h 直接 `#include "EventToken.h"`，
// 而该头只随 Windows SDK 发布，mingw-w64 各发行版都不带（本仓库直接在
// zig 自带的 mingw-w64 头集里逐个查过）。于是「交叉编译 Windows webview」
// 会 fatal error: 'EventToken.h' file not found —— 除非构建机上恰好装了
// Windows SDK 并以 MSVC 的 INCLUDE 形式暴露出来（GitHub runner 就是这种情况，
// 所以 release job 一直是绿的，而普通开发机 / msys2 上编不过）。
// 上游 webview/webview 为同一问题提供了这个兼容头，此处 vendored 一份。
//
// 结构体布局与 Windows SDK 的一致：单个 64 位整数。

#ifndef WEBVIEW_COMPAT_EVENTTOKEN_H
#define WEBVIEW_COMPAT_EVENTTOKEN_H
#ifdef _WIN32

// This compatibility header provides types used by MS WebView2. This header can
// be used as an alternative to the "EventToken.h" header normally provided by
// the Windows SDK. Depending on the MinGW distribution, this header may not be
// present, or it may be present with the name "eventtoken.h". The letter casing
// matters when cross-compiling on a system with case-sensitive file names.

#ifndef __eventtoken_h__

#ifdef __cplusplus
#include <cstdint>
#else
#include <stdint.h>
#endif

typedef struct EventRegistrationToken {
  int64_t value;
} EventRegistrationToken;
#endif // __eventtoken_h__

#endif // _WIN32
#endif // WEBVIEW_COMPAT_EVENTTOKEN_H
