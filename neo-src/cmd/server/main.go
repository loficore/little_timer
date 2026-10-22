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

	"github.com/gin-gonic/gin"

	"little-timer/internal/aigtd"
	"little-timer/internal/cli"
	"little-timer/internal/cron"
	"little-timer/internal/crypto"
	"little-timer/internal/domain"
	httpx "little-timer/internal/http"
	httpapp "little-timer/internal/http/app"
	"little-timer/internal/http/handlers"
	"little-timer/internal/http/middleware"
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
	switch err := bootstrapKeystore(ks, secretStorePath(opts.DBPath)); {
	case err == nil:
		// 正常模式:keystore 就绪,继续既有引导序列。
	case errors.Is(err, errMigrationPending):
		// 迁移等待模式:不碰 DB、不起 worker/cron,只暴露 credentials
		// 端点的 501 占位(见 newMigrationWaitRouter 的 Task 9 面包屑)。
		log.Warn("server: keystore 不可用且检测到 v1 凭据文件,进入迁移等待模式",
			"error", err.Error())
		fmt.Fprintln(os.Stderr,
			"migration-wait mode: 检测到 v1 凭据文件但密钥服务不可用;"+
				"修复/启动系统密钥服务后重启即可完成迁移,"+
				"当前仅 /api/credentials/* 占位路由可用(501)。")
		return runMigrationWaitServer(opts)
	default:
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

	router := httpx.NewRouter(app, opts.CORSOrigin)
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
// spec §9 —— 启动期 keystore fail-fast 与迁移等待模式(Task 6)。
// ---------------------------------------------------------------------------

// errMigrationPending 是 bootstrapKeystore 的决策哨兵:密钥服务不可用且磁盘
// 上存在 v1 主口令时代的 secret 文件 —— 数据仍可救回,main 不退出而是进入
// "迁移等待"模式,由 Task 8 的迁移器 + Task 9 的 credentials/migrate 端点
// 接管。错误链同时保留 keystore 侧原因(%w 双链)。
var errMigrationPending = errors.New("server: 检测到 v1 凭据文件而密钥服务不可用,进入迁移等待模式")

// legacySecretMagicV1 —— v1 主口令时代 secret 文件的 8 字节 magic(与
// internal/crypto 磁盘格式文档及迁移器 Task 8 逐字一致;crypto 未导出该
// 常量,cmd/server 侧只做前缀嗅探,不解码内容,故在此镜像而非 import 弱化
// 封装)。v2 magic "LTMSEKv2" 是本程序自己的文件,不构成迁移等待理由。
var legacySecretMagicV1 = []byte("LTMSECv1")

// secretStorePath 与 httpapp 内部约定一致:secret blob 与 DB 同目录。
// app 层的同名函数未导出,这里独立推导,避免跨包耦合私有符号。
func secretStorePath(dbPath string) string {
	return filepath.Join(filepath.Dir(dbPath), "secret.db")
}

// isV1SecretFile 报告给定路径文件是否以 v1 magic 开头。读不到、太短或前缀
// 不符都返回 false —— "状态未知"按 fail-fast 处理:迁移等待只对确证的 v1
// 遗留数据开放,绝不为垃圾文件保留半死不活的进程。
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

// bootstrapKeystore 是 spec §9 启动序列的纯决策函数(不打日志、不退出),
// 便于表驱动单测:
//   - Available() 通过 → EnsureMasterKey();失败同样 fail-fast(指引文案由
//     后端错误自带,%w 透传,绝不重写)。
//   - Available() 失败 + v1 文件存在 → errMigrationPending(迁移等待)。
//   - Available() 失败 + 无 v1 文件 → 返回保留 ErrUnavailable 链与 §9 指引
//     文案的错误(Review Focus #5:sway/headless 机器要看到可操作命令)。
//   - ks == nil(平台实现未注册)按 ErrUnavailable fail-fast。
//
// 日志与模式分派留给调用方 runServer;错误文本只含原因,不含密钥材料。
func bootstrapKeystore(ks keystore.Keystore, secretDBPath string) error {
	if ks == nil {
		return fmt.Errorf("%w: 平台 keystore 未注册(当前构建目标没有可用的密钥托管实现)",
			keystore.ErrUnavailable)
	}
	if err := ks.Available(); err != nil {
		if isV1SecretFile(secretDBPath) {
			return fmt.Errorf("%w: %w", errMigrationPending, err)
		}
		return fmt.Errorf("keystore 不可用: %w", err)
	}
	if err := ks.EnsureMasterKey(); err != nil {
		return fmt.Errorf("keystore 就绪但建立 master key 失败: %w", err)
	}
	return nil
}

// newMigrationWaitRouter 构造迁移等待模式的最小路由:只挂载三条
// /api/credentials/* 端点(status/reset/migrate)的 501 占位。
//
// 面包屑(Task 6 → Task 9):Task 9 会把这些 stub 替换成真实 handler
// (GET status 报 CredentialStatus、POST reset 重置凭据、POST migrate 输入
// 最后一次主口令触发 Task 8 的迁移器)。在那之前本进程刻意不开 DB、不接
// Auth/ConflictGuard 中间件 —— 密钥服务不可用时任何凭据读写都无法 Seal,
// 统一 501 是最诚实的语义。
// 本进程是"迁移等待模式"专用,只挂三条 credentials 端点,不接
// Auth / ConflictGuard 中间件 —— 密钥服务不可用时任何凭据读写都无法 Seal,
// 让非凭据端点可被调用只会给出误导性的成功。
func newMigrationWaitRouter(a *httpapp.App, corsOrigin string) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.CORS(corsOrigin))
	r.Use(func(c *gin.Context) {
		c.Set("app", a)
		c.Next()
	})
	g := r.Group("/api/credentials")
	g.GET("/status", handlers.CredentialsStatus)
	g.POST("/reset", handlers.CredentialsReset)
	g.POST("/migrate", handlers.CredentialsMigrate)
	return r
}

// runKeystoreMigration 在正常模式下跑一次性迁移（Task 8 的 MigrateAll）。
//
// 放在 bootstrapKeystore 之后、打开 DB 之前：迁移要读 settings 的旧密文列，
// 所以 settings manager 必须已就绪；但要在起 worker/cron 之前完成，避免
// worker 抢在凭据就绪前就去读 API Key。
//
// password 为空表示"没有已知口令"——此时若磁盘上确无 v1 文件即无事可做；
// 若有 v1 文件，MigrateAll 会返回 ErrUnseal（缺口令），由调用方决定是
// 提示用户还是继续（headless 场景用户可以事后跑 migrate-keystore CLI）。
func runKeystoreMigration(ks keystore.Keystore, dbPath string, creds keystore.SettingsCreds, password []byte) error {
	secretPath := secretStorePath(dbPath)
	_, err := keystore.MigrateAll(ks, keystore.MigrateOptions{
		DBPath:       dbPath,
		SecretPath:   secretPath,
		SentinelPath: filepath.Join(filepath.Dir(secretPath), keystore.SentinelName),
		Password:     password,
	})
	return err
}

// runMigrationWaitServer 以迁移等待模式服务:HTTP 只暴露 501 占位路由,
// 等待运维修复密钥服务后重启(迁移逻辑见 Task 8)。webview/cron/worker
// 一概不启动。SIGINT/SIGTERM 走与正常模式相同的优雅关闭。
func runMigrationWaitServer(opts *cli.ServeOptions) error {
	// 迁移等待模式仍需 App：credentials 端点要从它拿 SecretStorage 与
	// SettingsManager 来完成迁移。但 keystore 此时不可用，故不跑
	// bootstrapApp 的常规引导（worker/cron/备份一律不启动）。
	migApp := newMigrationWaitApp(opts.DBPath)
	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", opts.Port),
		Handler:           newMigrationWaitRouter(migApp, opts.CORSOrigin),
		ReadHeaderTimeout: 10 * time.Second,
	}

	serverErr := make(chan error, 1)
	go func() {
		fmt.Fprintf(os.Stdout,
			"HTTP server listening on :%d (migration-wait mode, db=%s, cors-origin=%s)\n",
			opts.Port, opts.DBPath, opts.CORSOrigin)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	var srvErr error
	select {
	case sig := <-sigCh:
		fmt.Fprintf(os.Stdout, "\nreceived %s, shutting down...\n", sig)
	case srvErr = <-serverErr:
		fmt.Fprintf(os.Stderr, "HTTP server error: %v\n", srvErr)
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		fmt.Fprintf(os.Stderr, "HTTP shutdown error: %v\n", err)
	}
	return srvErr
}

// bootstrapApp 接线 SQLite → Settings → Clock → Backup → App 链。
// 返回 App 和一个 cleanup 函数，后者关闭 DB 并反初始化时钟管理器。
// newMigrationWaitApp 为迁移等待模式构造一个"瘦" App：只有 credentials 端点
// 需要的东西（SQLite / settings / keystore 接线），不建 backup manager，
// 也不跑备份重建——此时 keystore 不可用，任何凭据 Seal 都会失败。
func newMigrationWaitApp(dbPath string) *httpapp.App {
	sqlite := storage.NewSqliteManager().Init(dbPath)
	if err := sqlite.Open(); err != nil {
		fmt.Fprintf(os.Stderr, "migration-wait: open sqlite: %v\n", err)
		return httpapp.NewApp(domain.NewClockManager(domain.NewDefaultClockTaskConfig()),
			nil, sqlite, nil, dbPath)
	}
	_ = sqlite.Migrate()
	sm, err := settings.NewFromSqliteManager(sqlite, dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "migration-wait: settings: %v\n", err)
	}
	a := httpapp.NewApp(domain.NewClockManager(domain.NewDefaultClockTaskConfig()),
		sm, sqlite, nil, dbPath)
	_ = a.Secrets()
	return a
}

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
	// Task 11:在凭据存储就绪后、备份重建前跑一次性迁移。必须早于 Rebuild
	// ——备份适配器要用云端凭据，迁移没完成就读不到。errMigrationPending /
	// ErrUnseal 说明磁盘上有 v1 数据但没口令：不是致命错误（旧路径仍可用
	// 直到用户补跑 CLI），记 warn 后继续启动。
	if err := runKeystoreMigration(keystore.Current(), dbPath, sm, nil); err != nil {
		if errors.Is(err, errMigrationPending) || errors.Is(err, keystore.ErrUnseal) {
			log.Warn("server: 一次性凭据迁移未完成,旧凭据保持原样",
				"error", err.Error())
		} else {
			fmt.Fprintf(os.Stderr, "warning: credential migration failed (%v)\n", err)
		}
	}
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
