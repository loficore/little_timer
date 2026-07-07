//go:build android
// +build android

// Android entrypoint for little-timer.
//
// Replaces the desktop main.go on Android builds (e.g.
// `GOOS=android GOARCH=arm64 go build -tags android`).  We do not
// own the Go runloop on Android — main.go's `func main()` still
// links in (so the package compiles for Android), but the Wails
// Android host drives the lifecycle:
//
//   - The Kotlin/Java Activity creates a `WailsBridge` instance.
//   - The bridge's `nativeInit` JNI entrypoint stores the JavaVM +
//     bridge global reference, then invokes the function we register
//     here via `application.RegisterAndroidMain` in a goroutine
//     (`pkg/application/application_android.go`).
//   - Subsequent bridge callbacks (`nativeOnStart`, `nativeOnResume`,
//     `nativeOnPause`, `nativeHandleRuntimeCall`, ...) dispatch to
//     the Wails message processor and into our services.
//
// Because the host owns the runloop, we deliberately do NOT call
// `wailsApp.Run()` — it would block here forever and never return.
//
// Why no `func main()`?  The package already has `main()` in
// main.go; adding a second one for Android would conflict.  Instead
// we install the Wails setup as an `init()`-registered callback —
// the same trick gomobile-based libraries use to defer startup to
// the host.
package main

import (
	"embed"

	"github.com/wailsapp/wails/v3/pkg/application"
	httpapp "little-timer/internal/http/app"
)

//go:embed all:assets
var assets embed.FS

// wailsApp is the *application.App the Android JNI bridge hands
// incoming runtime calls to.  Built once by bootWails; read by the
// Wails message processor when the WebView posts a runtime call
// (see `handleRuntimeCallForAndroid` in `application_android.go`).
var wailsApp *application.App

// bootWails builds the Wails App + service wrappers.  Invoked by
// the Android JNI bridge after `nativeInit` stores the bridge
// global ref — see `Java_com_wails_app_WailsBridge_nativeInit`.
//
// We pass a zero-value `*httpapp.App{}` for the tooling-only build
// (the package must link even without a bootstrapped DB / clock /
// settings / backup).  A real Android runtime wires those in
// before this point — that lands once the Android user-data dir
// decision is finalised.  The service constructors themselves only
// store the *App pointer; they don't dereference its fields, so
// the zero App compiles cleanly.
//
// ponytail: the service registration list mirrors `bindings/.../
// wailsbindings.ts` exactly — every method the Wails client calls
// must have a corresponding exported method on one of these types.
// Add new methods to `wails_services.go`, not here.
func bootWails() {
	a := &httpapp.App{}

	wailsApp = application.New(application.Options{
		Services: []application.Service{
			application.NewService(httpapp.NewTimerService(a)),
			application.NewService(httpapp.NewHabitService(a)),
			application.NewService(httpapp.NewSettingsService(a)),
			application.NewService(httpapp.NewBackupService(a)),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
	})
}

// init wires `bootWails` into the Wails Android lifecycle before
// any other code runs.  The host calls our registered func in a
// goroutine after `nativeInit`; we don't need to do anything else
// from Go's perspective.
func init() {
	application.RegisterAndroidMain(bootWails)
}