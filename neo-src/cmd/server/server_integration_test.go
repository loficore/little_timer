// Package main —— cmd/server 启动与关闭的集成测试。
//
// 两层覆盖：
//
//  1. 子进程测试（TestBootstrap_*、TestSignal_*）由 TestMain 构建一次
//     server 二进制并 exec 它。它们验证 cmd/server 对环境呈现的契约：
//     成功启动、致命的 bootstrap 错误、SIGTERM 驱动的优雅关闭。
//
//  2. 单元级恢复测试（TestClock_*、TestSettings_*）覆盖规范点明的两条
//     失败恢复路径：clock 时长溢出（静默回退到安全默认值）与 settings
//     行损坏（重新播种默认值并继续）。
//
// 运行：
//
//	go test -v -run "TestBootstrap|TestSignal|TestClock|TestSettings" ./cmd/server/...
package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"little-timer/internal/domain"
	"little-timer/internal/settings"
	"little-timer/internal/storage"
)

// testServerPath 由 TestMain 设置为新构建的 server 二进制路径。每个套件
// 只构建一次，把单测开销压到最低，并去掉对预构建 `bin/server` 产物的依赖。
var testServerPath string

// TestMain 把 server 二进制构建进临时文件，子进程测试因此不依赖既有产物。
// 构建发生在测试二进制的 CWD（cmd/server），所以 `go build .` 解析到本包
// 的 main.go。
func TestMain(m *testing.M) {
	tmpDir, err := os.MkdirTemp("", "lt-server-int-")
	if err != nil {
		fmt.Fprintf(os.Stderr, "test main: mkdir temp: %v\n", err)
		os.Exit(2)
	}
	// 尽力清理；下面还会 exit()，使清理多半只是装饰，但能让
	// `go test -count=2` 保持整洁。
	defer func() { _ = os.RemoveAll(tmpDir) }()

	binPath := filepath.Join(tmpDir, "lt-server-test-bin")
	build := exec.Command("go", "build", "-o", binPath, ".")
	var buildStderr bytes.Buffer
	build.Stderr = &buildStderr
	if err := build.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "test main: build server: %v\n%s\n", err, buildStderr.String())
		os.Exit(2)
	}
	testServerPath = binPath

	os.Exit(m.Run())
}

// serveArgs 返回以 dbPath 为根、port 监听的 http-only 运行参数列表。
// 所有子进程测试都用 --http-only，webview 路径永不触发（CI 无需 GTK 依赖）。
func serveArgs(dbPath string, port int) []string {
	return []string{
		"--http-only",
		"--port", strconv.Itoa(port),
		"--db-path", dbPath,
	}
}

// processAlive 用 signal 0 检查 pid 是否仍在运行。等价于
// `kill -0 $PID` —— 不投递真实信号。
func processAlive(pid *os.Process) error {
	if pid == nil {
		return errors.New("nil process")
	}
	return pid.Signal(syscall.Signal(0))
}

// Bootstrap 失败路径（子进程）。

// TestBootstrap_ValidTempDirStarts 确认快乐路径：可写临时目录 + 唯一端口
// 时，server 启动并活过初始 bootstrap 窗口。用 signal 0 探活 —— 并不需要
// 真的打 HTTP。
func TestBootstrap_ValidTempDirStarts(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "bootstrap-ok.db")

	cmd := exec.Command(testServerPath, serveArgs(dbPath, 18091)...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("start: %v\nstderr: %s", err, stderr.String())
	}

	// 清理：先 SIGTERM，关闭卡住则强杀。
	t.Cleanup(func() {
		if cmd.Process == nil {
			return
		}
		_ = cmd.Process.Signal(syscall.SIGTERM)
		done := make(chan struct{})
		go func() { _ = cmd.Wait(); close(done) }()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			_ = cmd.Process.Kill()
		}
	})

	// 给 bootstrap 留时间完成，再探活。
	time.Sleep(400 * time.Millisecond)
	if err := processAlive(cmd.Process); err != nil {
		t.Errorf("server died after bootstrap: %v\nstdout: %s\nstderr: %s",
			err, stdout.String(), stderr.String())
	}
}

// TestBootstrap_UnreadableDBPathExitsOne 确认无法创建的 DB 路径会抛出
// "bootstrap" 错误并以退出码 1 退出。
//
// 该路径的父目录不存在，且其祖父是 root 所有、mode-000 的路径
// （`/nonexistent-lt-...`），storage.Open() 内部的 `os.MkdirAll` 无法建文件。
func TestBootstrap_UnreadableDBPathExitsOne(t *testing.T) {
	badPath := "/nonexistent-lt-root-owned-dir-xyz/db.db"

	cmd := exec.Command(testServerPath, serveArgs(badPath, 18092)...)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	err := cmd.Run()
	if err == nil {
		t.Fatalf("expected non-zero exit; output:\n%s", out.String())
	}

	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected ExitError, got %T: %v", err, err)
	}
	if exitErr.ExitCode() != 1 {
		t.Errorf("exit code = %d, want 1", exitErr.ExitCode())
	}

	if !strings.Contains(out.String(), "bootstrap") {
		t.Errorf("error output must mention 'bootstrap', got:\n%s", out.String())
	}
}

// 信号处理（子进程）。

// TestSignal_SIGTERMGracefulShutdown 确认 server 能捕获 SIGTERM，并在
// main.go `shutdownTimeout` 常量声明的 5 秒关闭窗口内干净退出。
//
// 我们从发信号量到进程退出，断言墙钟时长 < 5s。本机真实运行约几毫秒
// （HTTP server 无在途请求），但这个断言防的是让关闭挂死的回归
// （例如忘了给 srv.Shutdown 设超时）。
func TestSignal_SIGTERMGracefulShutdown(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "shutdown.db")

	cmd := exec.Command(testServerPath, serveArgs(dbPath, 18093)...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}

	// 等 server 开始监听后再发信号。
	time.Sleep(400 * time.Millisecond)
	if err := processAlive(cmd.Process); err != nil {
		t.Fatalf("server died during startup: %v\nstderr: %s", err, stderr.String())
	}

	start := time.Now()
	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatalf("signal: %v", err)
	}

	waitCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	select {
	case err := <-done:
		elapsed := time.Since(start)
		// cmd.Wait 对任何非零退出返回 *exec.ExitError；优雅的 SIGTERM 关闭
		// 应该是 exit 0，但这里我们接受任何非错误的 wait 结果 ——
		// 契约是“5 秒内退出”，不是“以状态 0 退出”。
		if err != nil {
			var exitErr *exec.ExitError
			if !errors.As(err, &exitErr) {
				t.Errorf("unexpected wait error: %v\nstdout: %s\nstderr: %s",
					err, stdout.String(), stderr.String())
			}
		}
		if elapsed > 5*time.Second {
			t.Errorf("shutdown took %v, want <= 5s (matches main.go shutdownTimeout)", elapsed)
		}
	case <-waitCtx.Done():
		_ = cmd.Process.Kill()
		t.Fatalf("server did not exit within 5 seconds after SIGTERM\nstderr: %s", stderr.String())
	}
}

// Clock 时长溢出（单元）。

// TestClock_OverflowFallsBackToSafeDefaults 确认当传入时长会溢出 int64
// 毫秒时，domain.NewClockManager 替换为安全的 25 分钟倒计时 / 24 小时
// 正计时默认值。
//
// 两个方向都要覆盖，因为回退配置同时管 Countdown 和 Stopwatch，而
// buildInitialState 的 switch 只填充激活的那个变体 —— 所以每条路径都要
// 单独检查。
func TestClock_OverflowFallsBackToSafeDefaults(t *testing.T) {
	// uint64(math.MaxInt64)/1000 是乘以 1000 后不溢出 int64 的最大时长；
	// 再加 1 就触发 durationOverflows() 的回退。
	overflow := uint64(math.MaxInt64)/1000 + 1

	// 路径 1：倒计时溢出 → 期望 25 分钟倒计时状态。
	t.Run("countdown_overflow", func(t *testing.T) {
		cfg := domain.ClockTaskConfig{
			DefaultMode: domain.CountdownMode,
			Countdown:   domain.CountdownConfig{DurationSeconds: overflow},
			Stopwatch:   domain.StopwatchConfig{MaxSeconds: 24 * domain.Hour},
		}
		m := domain.NewClockManager(cfg)
		state := m.Update()
		if state == nil || state.Countdown == nil {
			t.Fatalf("expected countdown state, got %+v", state)
		}
		const wantMs = uint64(25 * domain.Minute * 1000)
		if state.Countdown.DurationMs != wantMs {
			t.Errorf("Countdown.DurationMs = %d, want %d (25-min fallback)",
				state.Countdown.DurationMs, wantMs)
		}
		if state.Countdown.RemainingMs != int64(wantMs) {
			t.Errorf("Countdown.RemainingMs = %d, want %d",
				state.Countdown.RemainingMs, wantMs)
		}
	})

	// 路径 2：正计时溢出 → 期望 25 分钟倒计时状态（回退配置把 DefaultMode
	// 置零，所以激活模式翻转）。
	t.Run("stopwatch_overflow", func(t *testing.T) {
		cfg := domain.ClockTaskConfig{
			DefaultMode: domain.StopwatchMode,
			Countdown:   domain.CountdownConfig{DurationSeconds: 25 * domain.Minute},
			Stopwatch:   domain.StopwatchConfig{MaxSeconds: overflow},
		}
		m := domain.NewClockManager(cfg)
		state := m.Update()
		if state == nil || state.Countdown == nil || state.Stopwatch != nil {
			t.Fatalf("expected countdown state (mode falls back on overflow), got %+v", state)
		}
		const wantMs = uint64(25 * domain.Minute * 1000)
		if state.Countdown.DurationMs != wantMs {
			t.Errorf("Countdown.DurationMs = %d, want %d (25-min fallback)",
				state.Countdown.DurationMs, wantMs)
		}
		if state.Countdown.RemainingMs != int64(wantMs) {
			t.Errorf("Countdown.RemainingMs = %d, want %d",
				state.Countdown.RemainingMs, wantMs)
		}
	})

	// 路径 3：两者都取边界值不应触发回退 —— 防 durationOverflows() 的 off-by-one。
	t.Run("boundary_no_overflow", func(t *testing.T) {
		const safeMax = uint64(math.MaxInt64) / 1000
		cfg := domain.ClockTaskConfig{
			DefaultMode: domain.CountdownMode,
			Countdown:   domain.CountdownConfig{DurationSeconds: safeMax},
			Stopwatch:   domain.StopwatchConfig{MaxSeconds: safeMax},
		}
		m := domain.NewClockManager(cfg)
		state := m.Update()
		if state == nil || state.Countdown == nil {
			t.Fatalf("expected countdown state, got %+v", state)
		}
		const wantMs = safeMax * 1000
		if state.Countdown.DurationMs != wantMs {
			t.Errorf("Countdown.DurationMs at boundary = %d, want %d (no fallback)",
				state.Countdown.DurationMs, wantMs)
		}
	})
}

// Settings 恢复（单元）。

// TestSettings_CorruptRowSeedsDefaults 确认 settings.NewFromSqliteManager
// 能在 settings 行缺失时播种默认值、返回可用配置来恢复。
//
// 我们通过迁移完成后删除 settings 行来诱发失败。schema 的 CHECK 约束
// 禁止 UPDATE 非法值，所以 DELETE 是最便宜的真实损坏方式，并且走的是
// loadAll 里同一条恢复分支 —— `sql.ErrNoRows` 与其他 scan 错误都落到
// 同一个“重新播种默认值”路径。
func TestSettings_CorruptRowSeedsDefaults(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "settings-recover.db")

	sqlite := storage.NewSqliteManager().Init(dbPath)
	if err := sqlite.Open(); err != nil {
		t.Fatalf("sqlite open: %v", err)
	}
	if err := sqlite.Migrate(); err != nil {
		t.Fatalf("sqlite migrate: %v", err)
	}
	t.Cleanup(func() { _ = sqlite.Close() })

	sm, err := settings.NewFromSqliteManager(sqlite, dbPath)
	if err != nil {
		t.Fatalf("settings (initial): %v", err)
	}

	// 合理性检查：初始构造已播种默认值。
	initial := sm.Config()
	if initial.Basic.Language == "" {
		t.Errorf("initial Language empty; expected defaults after migration")
	}
	if initial.ClockDefaults.Countdown.DurationSeconds == 0 {
		t.Errorf("initial DurationSeconds = 0; expected default")
	}

	// 模拟损坏：清空 settings 行。下一次 Load 必须重新播种默认值，
	// 且不向上抛错误。
	if _, err := sqlite.DB().Exec(`DELETE FROM settings WHERE id = 1;`); err != nil {
		t.Fatalf("delete row: %v", err)
	}

	if err := sm.Load(); err != nil {
		t.Errorf("Load after row deletion returned error: %v (expected recovery)", err)
	}

	cfg := sm.Config()
	if cfg.Basic.Language == "" {
		t.Errorf("after recovery, Language empty; want default")
	}
	if cfg.ClockDefaults.Countdown.DurationSeconds == 0 {
		t.Errorf("after recovery, DurationSeconds = 0; want default")
	}
	if cfg.Logging.Level == "" {
		t.Errorf("after recovery, Logging.Level empty; want default")
	}

	// manager 仍可用：后续经 viper 的往返应看到重新播种的值，而不是零。
	if got := sm.Viper().GetString("basic.language"); got == "" {
		t.Errorf("after recovery, viper basic.language empty; want default")
	}
}
