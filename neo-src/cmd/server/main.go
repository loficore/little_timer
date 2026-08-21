// Package main — little-timer HTTP 服务器的入口。
//
// 取代 W1 stub。接线:
//
//	┌─────────┐  os.Args   ┌──────────┐  ServeOptions   ┌────────────┐
//	│  main() │ ─────────► │ cli.Cobra│ ──────────────► │ runServer  │
//	└─────────┘            └──────────┘                 └────────────┘
//	                                                           │
//	                                                           ▼
//	                                  SQLite → Settings → Clock → Backup → App
//	                                                           │
//	                                                           ▼
//	                                              Gin router + http.Server
//	                                              （后台 goroutine）
//
// 平台行为:
//
//   - Linux/macOS 默认: 仅 HTTP。webview 窗口需显式开启。
//   - Windows 默认:     Webview。传 --http-only 跳过窗口。
//   - webview 包由 `-tags webview` 门控;没有该 tag 时 Run() 返回错误，
//     HTTP 服务器独自继续服务。这保证在没有 GTK/webkit 开发包的机器
//     （CI、精简容器）上二进制仍可构建。
//
// 信号:
//
//   - SIGINT / SIGTERM: 优雅关闭（HTTP 服务器有 5s 截止时间）
//   - webview 窗口关闭: 在 webview 模式下等同关闭信号
package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"little-timer/internal/cli"
	"little-timer/internal/domain"
	httpx "little-timer/internal/http"
	httpapp "little-timer/internal/http/app"
	"little-timer/internal/settings"
	"little-timer/internal/storage"
	"little-timer/internal/webview"
)

// shutdownTimeout 是 HTTP 服务器排空在途请求的宽限期，超时后强制关闭。
const shutdownTimeout = 5 * time.Second

func main() {
	root := cli.NewRootCmd(runServer)
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

// runServer 是交给 CLI 的 serve 回调。它组装 App 束、启动 HTTP 服务器、
// 可选地在 webview 窗口中阻塞，然后执行优雅关闭。
//
// 返回 error 而非调用 os.Exit，让 Cobra 的 Execute() 能格式化并上报错误
// （与 CLI / settings 包其余部分使用的分层错误约定一致）。
func runServer(opts *cli.ServeOptions) error {
	app, cleanup, err := bootstrapApp(opts.DBPath)
	if err != nil {
		return fmt.Errorf("bootstrap: %w", err)
	}
	defer cleanup()

	router := httpx.NewRouter(app, opts.CORSOrigin)
	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", opts.Port),
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
	}

	serverErr := make(chan error, 1)
	go func() {
		fmt.Fprintf(os.Stdout, "HTTP server listening on :%d (db=%s, cors-origin=%s)\n", opts.Port, opts.DBPath, opts.CORSOrigin)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	// webview 调用在自己的 goroutine 里运行，因为 Run() 没有
	// "阻塞但收到信号即返回"的参数——即使用户还没关窗，我们也要能响应 SIGTERM。
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	var webviewErrCh chan error
	if !opts.HTTPOnly {
		webviewErrCh = make(chan error, 1)
		go func() {
			webviewErrCh <- webview.Run()
		}()
		fmt.Fprintln(os.Stdout, "webview mode: window opened (close it or send SIGTERM to quit)")
	} else {
		fmt.Fprintln(os.Stdout, "http-only mode: send SIGTERM (Ctrl+C) to quit")
	}

	var (
		sig           os.Signal
		srvErr        error
		winErr        error
		webviewClosed bool
	)
	select {
	case sig = <-sigCh:
		fmt.Fprintf(os.Stdout, "\nreceived %s, shutting down...\n", sig)
	case srvErr = <-serverErr:
		fmt.Fprintf(os.Stderr, "HTTP server error: %v\n", srvErr)
	case winErr = <-webviewErrCh:
		webviewClosed = true
		fmt.Fprintf(os.Stdout, "webview window closed, shutting down...\n")
		if winErr != nil {
			fmt.Fprintf(os.Stderr, "webview error: %v\n", winErr)
		}
	}

	_ = webviewClosed // 消除未使用警告

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		fmt.Fprintf(os.Stderr, "HTTP shutdown error: %v\n", err)
	}
	if srvErr != nil {
		return srvErr
	}
	return winErr
}

// bootstrapApp 接线 SQLite → Settings → Clock → Backup → App 链。
// 返回 App 和一个 cleanup 函数，后者关闭 DB 并反初始化时钟管理器。
func bootstrapApp(dbPath string) (*httpapp.App, func(), error) {
	sqlite := storage.NewSqliteManager().Init(dbPath)
	if err := sqlite.Open(); err != nil {
		return nil, nil, fmt.Errorf("open sqlite: %w", err)
	}
	if err := sqlite.Migrate(); err != nil {
		_ = sqlite.Close()
		return nil, nil, fmt.Errorf("migrate: %w", err)
	}

	sm, err := settings.NewFromSqliteManager(sqlite, dbPath)
	if err != nil {
		_ = sqlite.Close()
		return nil, nil, fmt.Errorf("settings: %w", err)
	}

	clk := domain.NewClockManager(sm.BuildClockConfig())

	// 备份可选——从持久化的 BackupConfig 重建;RebuildBackup 会回退到
	// local 适配器，只有回退也失败时才返回错误（本进程禁用备份）。
	a := httpapp.NewApp(clk, sm, sqlite, nil, dbPath)
	if err := a.RebuildBackup(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "warning: backup disabled (%v)\n", err)
	}

	cleanup := func() {
		clk.Deinit()
		if err := sqlite.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "sqlite close: %v\n", err)
		}
	}
	return a, cleanup, nil
}
