//go:build android
// +build android

// little-timer 的 Android 入口。
//
// Android 构建时取代桌面版 main.go（例如
// `GOOS=android GOARCH=arm64 go build -tags android`）。在 Android 上
// Go runloop 不归我们所有 —— main.go 的 `func main()` 依然参与链接
// （保证包能为 Android 编译），但生命周期由 Wails Android 宿主驱动：
//
//   - Kotlin/Java Activity 创建 `WailsBridge` 实例。
//   - bridge 的 `nativeInit` JNI 入口保存 JavaVM + bridge 全局引用，
//     然后在一个 goroutine 里调用我们在此通过 `application.RegisterAndroidMain`
//     注册的函数（`pkg/application/application_android.go`）。
//   - 之后的 bridge 回调（`nativeOnStart`、`nativeOnResume`、
//     `nativeOnPause`、`nativeHandleRuntimeCall` 等）分发给 Wails 消息
//     处理器并进入我们的 service。
//
// 因为 runloop 归宿主所有，我们刻意不调用 `wailsApp.Run()` —— 它会在
// 这里永远阻塞、永不返回。
//
// 为什么不写 `func main()`？包里已有 main.go 的 `main()`；再为 Android
// 加第二个会冲突。所以我们把 Wails 安装做成 `init()` 注册的回调 ——
// 与 gomobile 系库把启动延迟给宿主的套路相同。
package main

import (
	"context"
	"embed"
	"fmt"
	"path/filepath"

	"github.com/wailsapp/wails/v3/pkg/application"
	"little-timer/internal/domain"
	httpapp "little-timer/internal/http/app"
	"little-timer/internal/log"
	"little-timer/internal/settings"
	"little-timer/internal/storage"
)

//go:embed all:assets
var assets embed.FS

// wailsApp 是 Android JNI bridge 用来转交传入 runtime 调用的
// *application.App。由 bootWails 构建一次；WebView 发出 runtime 调用时，
// Wails 消息处理器读取它（见 `application_android.go` 的
// `handleRuntimeCallForAndroid`）。
var wailsApp *application.App

// bootWails 构建 Wails App + service 包装器。由 Android JNI bridge 在
// `nativeInit` 保存 bridge 全局引用之后调用 —— 见
// `Java_com_wails_app_WailsBridge_nativeInit`。service 注册列表与
// `bindings/.../wailsbindings.ts` 严格对应 —— Wails 客户端调用的每个方法
// 都必须能在这些类型上找到对应的导出方法。新增方法请加到
// `wails_services.go`，不要加在这里。
func bootWails() {
	storagePath := application.Android.StoragePath()

	if err := log.Init(filepath.Join(storagePath, "logs")); err != nil {
		log.Error("log.Init failed", "error", err.Error())
	}

	log.Info(fmt.Sprintf("[bootWails] StoragePath=%q", storagePath))
	log.Info("[bootWails] starting")

	dbPath := filepath.Join(storagePath, "little_timer.db")
	log.Info(fmt.Sprintf("[bootWails] dbPath=%q backupDir=%q", dbPath, filepath.Join(storagePath, "backups")))

	sqlite := storage.NewSqliteManager().Init(dbPath)
	log.Debug("[bootWails] sqlite manager created")
	if err := sqlite.Open(); err != nil {
		log.Error(fmt.Sprintf("[bootWails] sqlite.Open FAILED: %v", err))
		panic(fmt.Sprintf("open sqlite: %v", err))
	}
	log.Debug("[bootWails] sqlite opened")
	if err := sqlite.Migrate(); err != nil {
		log.Error(fmt.Sprintf("[bootWails] migrate FAILED: %v", err))
		panic(fmt.Sprintf("migrate: %v", err))
	}
	log.Debug("[bootWails] sqlite migrated")

	sm, err := settings.NewFromSqliteManager(sqlite, dbPath)
	if err != nil {
		log.Error(fmt.Sprintf("[bootWails] settings FAILED: %v", err))
		panic(fmt.Sprintf("settings: %v", err))
	}
	log.Debug("[bootWails] settings created")

	clk := domain.NewClockManager(sm.BuildClockConfig())
	log.Debug("[bootWails] clock created")

	// RebuildBackup 从 dbPath 推导备份目录（`<storage>/backups`）并遵循
	// 持久化的 BackupConfig，失败时回退到 local。
	a := httpapp.NewApp(clk, sm, sqlite, nil, dbPath)
	if err := a.RebuildBackup(context.Background()); err != nil {
		log.Info(fmt.Sprintf("[bootWails] backup disabled: %v", err))
	}

	log.Debug(fmt.Sprintf("[bootWails] app created a=%p sqlite=%p sm=%p clk=%p bm=%p",
		a, sqlite, sm, clk, a.BackupManager()))

	wailsApp = application.New(application.Options{
		Services: []application.Service{
			application.NewService(httpapp.NewTimerService(a)),
			application.NewService(httpapp.NewHabitService(a)),
			application.NewService(httpapp.NewSettingsService(a)),
			application.NewService(httpapp.NewBackupService(a)),
		},
		Assets: application.AssetOptions{
			Handler: application.BundledAssetFileServer(assets),
		},
	})
	log.Debug("[bootWails] wailsApp created")

	go func() {
		log.Info("[bootWails] wailsApp.Run starting")
		if err := wailsApp.Run(); err != nil {
			log.Error("wails runtime error", "error", err.Error())
		}
		log.Info("[bootWails] wailsApp.Run exited")
	}()
	log.Info("[bootWails] done, goroutine started")
}

// init 在其他任何代码运行之前把 `bootWails` 接入 Wails Android 生命周期。
// 宿主在 `nativeInit` 之后于一个 goroutine 里调用我们注册的函数；从 Go 的
// 角度看不需要再做别的。
func init() {
	application.RegisterAndroidMain(bootWails)
}
