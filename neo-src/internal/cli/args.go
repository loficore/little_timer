// Package cli builds the Cobra command tree for little-timer.
//
// Two ways to invoke serve:
//
//	$ little-timer serve --http-only --port 9090   # explicit subcommand
//	$ little-timer --http-only --port 9090         # implicit (no subcommand)
//
// Both work because the serve flags are PersistentFlags on the root
// command — the root's RunE dispatches to a caller-supplied serve
// callback when no subcommand is selected.  This matches the Zig
// source which had no subcommand layer at all (`parseArgs` in
// `src/main_entry.zig:31-45` only knew about `--http-only` and
// `--webview`).
//
// Defaults are platform-aware: Linux/macOS boot into `--http-only`
// (matches the Zig build), Windows boots into webview mode.
//
// Layering: this package knows nothing about storage, settings, or
// HTTP — it parses flags and calls the supplied ServeFunc.  Keeping
// the dependency one-way (cli → caller) means tests can exercise
// the CLI without touching SQLite.
package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/spf13/cobra"

	"little-timer/internal/app"
)

// ServeOptions is the parsed-result struct that the main entrypoint
// (cmd/server/main.go) consumes.  Populated from the root's
// PersistentFlags whenever serve is invoked (explicit subcommand or
// implicit root RunE).
type ServeOptions struct {
	HTTPOnly   bool
	Port       int
	DBPath     string
	CORSOrigin string
}

// ServeFunc is the callback the CLI invokes when the user wants to
// start the server.  The CLI knows nothing about storage or HTTP —
// it just parses flags and calls this.  main.go supplies the real
// implementation; tests can pass a stub that records the options.
type ServeFunc func(*ServeOptions) error

// DefaultPort is the Zig `port 8080` constant.
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

// defaultHTTPOnly reports the platform default for `--http-only`.
// Mirrors the Zig `builtin.os.tag` ternary: Windows → webview, every
// other OS → http-only (Linux / macOS / BSD / …).
func defaultHTTPOnly() bool {
	return runtime.GOOS != "windows"
}

// NewRootCmd builds the full command tree and wires the serve flags
// as PersistentFlags on root.  `serveFn` is called whenever the user
// invokes serve (explicit subcommand or implicit root RunE).  Pass
// nil during tests if you only care about parsing.
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

	// MarkFlagsMutuallyExclusive must run on the command that OWNS the
	// flags (root, where addServeFlags attached them).  Calling it on
	// the serve subcommand panics — PersistentFlags propagate down
	// but flag-group metadata is parent-local.
	root.MarkFlagsMutuallyExclusive("http-only", "webview")

	// Root RunE: if no subcommand was chosen, invoke the serve
	// callback with the current flag values.
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

// addServeFlags wires the four serve flags onto the given command.
// Both root and the `serve` subcommand share the same ServeOptions
// instance via root's PersistentFlags (which propagate down).  Only
// the root needs to call this — calling it on a subcommand would
// re-bind the variables to a different persistence context.
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

// resolveHTTPOnly collapses the mutually-exclusive --http-only /
// --webview flags into the single bool opts.HTTPOnly.  The two flags
// share the same target so callers can use whichever feels natural
// (`--http-only` vs `--webview`); we resolve here so the rest of the
// code only ever reads opts.HTTPOnly.
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

// invokeServe prints a one-line summary (useful for `--dry-run`
// style scripts and as a smoke-test target) then hands off to the
// caller-supplied callback.  When the callback is nil (test mode),
// we just print the summary and exit cleanly.
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

// -----------------------------------------------------------------------------
// serve — explicit subcommand alias.
//
// Inherits all serve flags from root via PersistentFlags.  Exists so
// `little-timer serve --http-only` reads naturally in shell history
// and CI scripts.  The actual work is delegated to invokeServe so the
// behaviour is identical whether the user typed `serve` or not.
// -----------------------------------------------------------------------------

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

// -----------------------------------------------------------------------------
// version — prints Version, BuildTime, GitCommit.
// -----------------------------------------------------------------------------

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

// -----------------------------------------------------------------------------
// backup — sub-subcommands: create / restore / list.
//
// The actual backup work is owned by `internal/storage/backup`.  These
// commands are wiring-only for now: the storage layer will fill in the
// real call sites in a later wave.  Until then they print "not yet
// implemented" and exit non-zero.
// -----------------------------------------------------------------------------

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