// Package cli 构建 little-timer 的 Cobra 命令树。
//
// 两种调用 serve 的方式：
//
//	$ little-timer serve --http-only --port 9090   # 显式子命令
//	$ little-timer --http-only --port 9090         # 隐式（无子命令）
//
// 两者都可行，是因为 serve 的 flag 是根命令上的 PersistentFlags —— 未选择
// 子命令时，根命令的 RunE 会分发给调用方提供的 serve 回调。
//
// 默认值区分平台：Linux/macOS 默认 `--http-only`，Windows 默认 webview 模式。
//
// 分层：本包对存储、设置、HTTP 一无所知 —— 它只解析 flag 并调用传入的
// ServeFunc。保持单向依赖（cli → 调用方）意味着测试无需接触 SQLite
// 就能验证 CLI。
package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/spf13/cobra"

	"little-timer/internal/app"
)

// ServeOptions 是供主入口（cmd/server/main.go）消费的解析结果 struct。
// 每当 serve 被调用（显式子命令或隐式根 RunE）时由根命令的
// PersistentFlags 填充。
type ServeOptions struct {
	HTTPOnly   bool
	Port       int
	DBPath     string
	CORSOrigin string
}

// ServeFunc 是用户要启动 server 时 CLI 调用的回调。CLI 对存储和 HTTP
// 一无所知 —— 只负责解析 flag 并调用它。main.go 提供真实实现；测试可传入
// 记录 options 的桩。
type ServeFunc func(*ServeOptions) error

// DefaultPort 是默认的 HTTP 监听端口。
const DefaultPort = 8080

func defaultDBPath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "little_timer.db"
	}
	subdir := "little_timer"
	if runtime.GOOS == "windows" {
		subdir = "LittleTimer"
	}
	full := filepath.Join(dir, subdir)
	if err := os.MkdirAll(full, 0o755); err != nil {
		return "little_timer.db"
	}
	return filepath.Join(full, "little_timer.db")
}

// defaultHTTPOnly 返回 `--http-only` 的平台默认值：
// Windows → webview，其他所有系统 → http-only（Linux / macOS / BSD / …）。
func defaultHTTPOnly() bool {
	return runtime.GOOS != "windows"
}

// NewRootCmd 构建完整命令树，并把 serve flags 注册为根命令的
// PersistentFlags。用户调用 serve 时（显式子命令或隐式根 RunE）都会执行
// `serveFn`。只关心解析的测试可传 nil。
func NewRootCmd(serveFn ServeFunc) *cobra.Command {
	opts := &ServeOptions{
		HTTPOnly:   defaultHTTPOnly(),
		Port:       DefaultPort,
		DBPath:     defaultDBPath(),
		CORSOrigin: "*",
	}

	root := &cobra.Command{
		Use:     "little-timer",
		Short:   "Little Timer — countdown/stopwatch with web UI",
		Long:    "Cross-platform timer app.  Boots a Gin HTTP server (and, on Windows, a webview window) backed by SQLite.",
		Version: app.Version,
	}

	addServeFlags(root, opts)

	// MarkFlagsMutuallyExclusive 必须在拥有这些 flag 的命令（root，
	// addServeFlags 挂载处）上调用。在 serve 子命令上调用会 panic ——
	// PersistentFlags 会向下传播，但 flag-group 元数据只属于父命令。
	root.MarkFlagsMutuallyExclusive("http-only", "webview")

	// 根 RunE：未选择子命令时，用当前 flag 值调用 serve 回调。
	root.RunE = func(cmd *cobra.Command, _ []string) error {
		if err := resolveHTTPOnly(cmd, opts); err != nil {
			return err
		}
		return invokeServe(serveFn, cmd, opts)
	}

	root.AddCommand(newServeCmd(opts, serveFn))
	root.AddCommand(newVersionCmd())
	root.AddCommand(newBackupCmd())

	return root
}

// addServeFlags 把四个 serve flag 挂到给定命令上。root 和 `serve`
// 子命令通过根命令的 PersistentFlags（向下传播）共享同一个 ServeOptions
// 实例。只有 root 需要调用 —— 在子命令上调用会把变量重新绑定到另一个
// persistence 上下文。
func addServeFlags(cmd *cobra.Command, opts *ServeOptions) {
	cmd.PersistentFlags().BoolVar(&opts.HTTPOnly, "http-only", opts.HTTPOnly,
		"Run HTTP server only (skip webview window)")
	cmd.PersistentFlags().Bool("webview", !opts.HTTPOnly,
		"Run with webview window (opposite of --http-only; mutually exclusive with --http-only)")
	cmd.PersistentFlags().IntVar(&opts.Port, "port", opts.Port,
		"HTTP port to listen on")
	cmd.PersistentFlags().StringVar(&opts.DBPath, "db-path", opts.DBPath,
		"SQLite database file path")
	cmd.PersistentFlags().StringVar(&opts.CORSOrigin, "cors-origin", opts.CORSOrigin,
		"Access-Control-Allow-Origin value")
}

// resolveHTTPOnly 把互斥的 --http-only / --webview flag 折叠为单个
// bool opts.HTTPOnly。两个 flag 指向同一目标，调用方可用更顺手的写法
// （`--http-only` 或 `--webview`）；在这里统一解析后，其余代码只读
// opts.HTTPOnly。
func resolveHTTPOnly(cmd *cobra.Command, opts *ServeOptions) error {
	webviewFlag, _ := cmd.Flags().GetBool("webview")
	httpOnlySet := cmd.Flags().Changed("http-only")
	webviewSet := cmd.Flags().Changed("webview")
	if httpOnlySet && webviewSet {
		return fmt.Errorf("--http-only and --webview are mutually exclusive")
	}
	if webviewSet {
		opts.HTTPOnly = !webviewFlag
	}
	return nil
}

// invokeServe 打印一行摘要（便于 `--dry-run` 风格脚本和 smoke-test），
// 然后交给调用方提供的回调。回调为 nil（测试模式）时只打印摘要并干净退出。
func invokeServe(fn ServeFunc, cmd *cobra.Command, opts *ServeOptions) error {
	fmt.Fprintf(cmd.OutOrStdout(),
		"serve: http-only=%v port=%d db-path=%q cors-origin=%q\n",
		opts.HTTPOnly, opts.Port, opts.DBPath, opts.CORSOrigin,
	)
	if fn == nil {
		return nil
	}
	return fn(opts)
}

// serve —— 显式子命令别名。
//
// 通过 PersistentFlags 从根命令继承全部 serve flag。存在它是为了让
// `little-timer serve --http-only` 在 shell 历史和 CI 脚本中读起来更自然。
// 实际工作委托给 invokeServe，因此用户是否输入 `serve` 行为完全一致。

func newServeCmd(opts *ServeOptions, fn ServeFunc) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the HTTP server (and, on Windows, the webview window)",
		Long: fmt.Sprintf(
			"Start the Little Timer HTTP server on --port (default %d) backed by SQLite at --db-path.\n\n"+
				"On Windows, also opens a native webview window unless --http-only is passed.\n"+
				"On Linux/macOS the default is --http-only; pass --webview to opt in to a window.",
			DefaultPort,
		),
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := resolveHTTPOnly(cmd, opts); err != nil {
				return err
			}
			return invokeServe(fn, cmd, opts)
		},
	}

	return cmd
}

// version —— 打印 Version、BuildTime、GitCommit。

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version, build time, and git commit",
		Run: func(cmd *cobra.Command, _ []string) {
			fmt.Fprintf(cmd.OutOrStdout(),
				"little-timer %s\n  build:  %s\n  commit: %s\n  go:     %s\n  os:     %s/%s\n",
				app.Version, app.BuildTime, app.GitCommit, runtime.Version(), runtime.GOOS, runtime.GOARCH,
			)
		},
	}
}

// backup —— 子命令：create / restore / list。
//
// 实际备份工作归属 `internal/storage/backup`。这些命令目前只做接线：
// 存储层会在后续阶段填入真实调用点。在那之前它们打印
// "not yet implemented" 并以非零码退出。

func newBackupCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "backup",
		Short: "Manage local backups (create / restore / list)",
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "create",
		Short: "Create a new backup",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return notImplemented("backup create")
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "restore <name>",
		Short: "Restore a named backup",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return notImplemented("backup restore " + args[0])
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List existing backups",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return notImplemented("backup list")
		},
	})

	return cmd
}

func notImplemented(name string) error {
	return fmt.Errorf("%s: not yet implemented (storage layer pending in a later wave)", name)
}
