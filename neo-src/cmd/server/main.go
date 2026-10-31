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
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"little-timer/internal/aigtd"
	"little-timer/internal/cli"
	"little-timer/internal/cron"
	"little-timer/internal/crypto"
	"little-timer/internal/domain"
	httpx "little-timer/internal/http"
	httpapp "little-timer/internal/http/app"
	"little-timer/internal/http/handlers"
	"little-timer/internal/keystore"
	"little-timer/internal/log"
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
	// spec §9:打开 DB 之前完成 keystore fail-fast。SecretStorage 惰性构造、
	// Settings 凭据分流、备份重建全都依赖托管 master key 就绪,晚检会让
	// 半接线的 App 带病启动。
	ks := keystore.Current()
	if err := bootstrapKeystore(ks, secretStorePath(opts.DBPath)); err != nil {
		// err 已带 §9 指引文案(linux 后端 unavailable() 内嵌,%w 透传),
		// 由 Cobra Execute 打到 stderr,退出码非 0。日志只记错误文本,
		// 绝不含任何密钥材料。
		log.Error("server: keystore fail-fast", "error", err.Error())
		return err
	}

	app, cleanup, err := bootstrapApp(opts.DBPath)
	if err != nil {
		return fmt.Errorf("bootstrap: %w", err)
	}
	defer cleanup()

	router := httpx.NewRouterWithUI(app, opts.CORSOrigin, embeddedUIFS())
	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", opts.Port),
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
	}

	// AI GTD 异步 worker:启动 2 个 goroutine 从 ai_jobs 队列取任务。
	// 用户在设置页配好 API Key 后(凭据由平台 keystore 托管,无锁概念),
	// worker 就能真实调用 LLM。
	worker := aigtd.NewWorkerPool(app.SQLite.AITasks(), appSettingsAdapter{app: app}, 2)
	workerCtx, workerCancel := context.WithCancel(context.Background())
	worker.Start(workerCtx)
	defer func() {
		worker.Stop()
		worker.Wait()
		workerCancel()
	}()

	// GTD Daily Sweep cron:每分钟读 settings.daily_sweep_enabled/time,
	// 到点且当日未 sweep 时入队 review jobs。跨包解耦:cmd/server 注入
	// handlers.StartDailySweep 作为 starter。
	dailySweepCron := cron.NewDailySweepCron(app.SQLite.DailySweep(), func(ctx context.Context) ([]int64, error) {
		return handlers.StartDailySweep(ctx, app)
	})
	dailySweepCron.Start()
	defer dailySweepCron.Stop()

	// 调试用:把固定响应注入 worker(覆盖真实 LLM 调用)。仅用于 e2e。
	if opts.AIGtdTestPrompt != "" || opts.AIGtdTestFailWith != 0 {
		aigtd.SetTestOverride(opts.AIGtdTestPrompt, opts.AIGtdTestFailWith)
		defer aigtd.ClearTestOverride()
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

// appSettingsAdapter 把 httpapp.App 适配成 aigtd.SettingsGetter,
// 让 worker 不直接依赖 settings 包。
type appSettingsAdapter struct {
	app *httpapp.App
}

func (s appSettingsAdapter) CurrentDate() string {
	return domain.TodayString(s.app.Settings.Config().Basic.Timezone)
}

func (s appSettingsAdapter) LLMSettings() aigtd.LLMConfig {
	cfg := aigtd.LLMConfig{
		Provider:       "openai_compat",
		BaseURL:        "https://api.deepseek.com/v1",
		Model:          "deepseek-chat",
		MaxTokens:      2048,
		TimeoutSeconds: 30,
	}
	if s.app == nil || s.app.SQLite == nil || s.app.SQLite.DB() == nil {
		return cfg
	}
	row := s.app.SQLite.DB().QueryRow(
		`SELECT llm_provider, llm_base_url, llm_model, llm_max_tokens, llm_timeout_seconds
		 FROM settings WHERE id = 1;`,
	)
	_ = row.Scan(&cfg.Provider, &cfg.BaseURL, &cfg.Model, &cfg.MaxTokens, &cfg.TimeoutSeconds)
	if cfg.Provider == "" {
		cfg.Provider = "openai_compat"
	}
	if cfg.Model == "" {
		cfg.Model = "deepseek-chat"
	}
	if cfg.MaxTokens == 0 {
		cfg.MaxTokens = 2048
	}
	if cfg.TimeoutSeconds == 0 {
		cfg.TimeoutSeconds = 30
	}
	// 去锁化(Task 5):凭据由 keystore 托管,worker 直接取 Key,不再判锁。
	// 未配置(ErrSecretNotFound)属常态静默跳过;keystore 不可用
	// (ErrKeystoreUnavailable)记 Warn 提示运维,日志绝不包含 secret 值。
	key, err := s.app.Secrets().Retrieve([]byte(aigtd.APIKeySecretName))
	switch {
	case err == nil:
		cfg.APIKey = string(key)
	case errors.Is(err, crypto.ErrKeystoreUnavailable):
		log.Warn("LLMSettings: 凭据不可用,跳过 API Key 读取", "error", err.Error())
	case errors.Is(err, crypto.ErrSecretNotFound):
		log.Debug("LLMSettings: 未配置 API Key")
	default:
		log.Warn("LLMSettings: 读取 API Key 失败", "error", err.Error())
	}
	return cfg
}

// ---------------------------------------------------------------------------
// spec §9 —— 启动期 keystore fail-fast(Task 6;v1 与迁移链已整体删除)。
// ---------------------------------------------------------------------------

// legacySecretMagicV1 —— v1 主口令时代 secret 文件的 8 字节 magic(与
// internal/crypto 磁盘格式文档逐字一致;crypto 未导出该常量,cmd/server 侧
// 只做前缀嗅探,不解码内容,故在此镜像而非 import 弱化封装)。v2 magic
// "LTMSEKv2" 是本程序自己的文件,不构成告警理由。
var legacySecretMagicV1 = []byte("LTMSECv1")

// secretStorePath 与 httpapp 内部约定一致:secret blob 与 DB 同目录。
// app 层的同名函数未导出,这里独立推导,避免跨包耦合私有符号。
func secretStorePath(dbPath string) string {
	return filepath.Join(filepath.Dir(dbPath), "secret.db")
}

// isV1SecretFile 报告给定路径文件是否以 v1 magic 开头。读不到、太短或前缀
// 不符都返回 false —— 只有确证的 v1 遗留文件才触发提示,垃圾文件不打扰。
func isV1SecretFile(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer func() { _ = f.Close() }()
	head := make([]byte, len(legacySecretMagicV1))
	if _, err := io.ReadFull(f, head); err != nil {
		return false
	}
	return bytes.Equal(head, legacySecretMagicV1)
}

// bootstrapKeystore 是 spec §9 启动序列的决策函数(不退出),便于表驱动单测:
//   - Available() 失败 → fail-fast(Review Focus #4:没有迁移等待分支——v1
//     解密路径已随迁移链整体删除,磁盘上的 v1 文件不再可救回)。指引文案由
//     后端错误自带,%w 透传,绝不重写(Review Focus #5:sway/headless 机器
//     要看到可操作命令)。
//   - Available() 通过 → EnsureMasterKey();失败同样 fail-fast。
//   - 就绪成功后磁盘上仍是 v1("LTMSECv1")文件 → 只记一行 warn 提示用户去
//     设置里重置凭据;不退出、不删文件(清理由用户显式操作)。
//   - ks == nil(平台实现未注册)按 ErrUnavailable fail-fast。
//
// 错误文本只含原因,不含密钥材料;warn 行同样不含任何凭据内容。
func bootstrapKeystore(ks keystore.Keystore, secretDBPath string) error {
	if ks == nil {
		return fmt.Errorf("%w: 平台 keystore 未注册(当前构建目标没有可用的密钥托管实现)",
			keystore.ErrUnavailable)
	}
	if err := ks.Available(); err != nil {
		return fmt.Errorf("keystore 不可用: %w", err)
	}
	if err := ks.EnsureMasterKey(); err != nil {
		return fmt.Errorf("keystore 就绪但建立 master key 失败: %w", err)
	}
	if isV1SecretFile(secretDBPath) {
		log.Warn("server: 检测到不再支持的 v1 凭据文件,请在设置中重置凭据")
	}
	return nil
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
	// 尽早建凭据存储并接线 SettingsManager:Rebuild 及后续配置保存
	// 路径都需要已接线的 secret store(Task 4 的凭据分流)。
	_ = a.Secrets()
	if err := a.Rebuild(context.Background()); err != nil {
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
