//go:build bindings
// +build bindings

// Package main —— 供 bindings 代码生成使用的 Wails v3 service 注册。
//
// 本文件的存在仅仅是：在没有 android build tag 生成 TS bindings 时，让
// wails3 的静态分析（`just bindings` 步骤）能找到 application.NewService()
// 调用。它被门控在 `bindings` build tag 之后，这样正常的桌面二进制 ——
// 运行时从不用 Wails（它用 HTTP + webview_go 提供服务）—— 不会编译
// wails v3 application 包，那个包会拖进 gtk4 + webkitgtk-6.0 cgo 依赖，
// 而 EL9（AlmaLinux 9）和最小化/容器构建上没有。
//
// Android 专属注册在 main_android.go（`//go:build android`）；android
// 构建在 bootWails() 里注册真实 service，不需要本文件。
// `scripts/generate-bindings.sh` 在桌面模式下传 `-tags=bindings`，让
// 代码生成仍然能看到这些调用。

package main

import (
	"github.com/wailsapp/wails/v3/pkg/application"
	httpapp "little-timer/internal/http/app"
)

var _ = application.NewService(httpapp.NewTimerService(nil))
var _ = application.NewService(httpapp.NewHabitService(nil))
var _ = application.NewService(httpapp.NewSettingsService(nil))
var _ = application.NewService(httpapp.NewBackupService(nil))
