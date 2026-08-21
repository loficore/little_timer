// CLI 的 smoke test。目标：证明 Cobra 接线可编译、平台默认值正确，
// 且 serve 回调路径端到端打通（回调收到正确的 ServeOptions）。
package cli

import (
	"bytes"
	"runtime"
	"testing"
)

// TestDefaultsLinux 确认 Linux/BSD/macOS 默认是 http-only。
func TestDefaultsLinux(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skipf("linux test on %s", runtime.GOOS)
	}
	if !defaultHTTPOnly() {
		t.Errorf("defaultHTTPOnly on %s = false, want true", runtime.GOOS)
	}
}

// TestRunServeCallbackReceivesOptions 接上回调，断言
// `root --http-only=false --port 9090` 之后它看到正确的 ServeOptions。
// 覆盖 root.RunE → invokeServe → 回调 这条路径。
func TestRunServeCallbackReceivesOptions(t *testing.T) {
	var got *ServeOptions
	root := NewRootCmd(func(opts *ServeOptions) error {
		got = opts
		return nil
	})

	root.SetArgs([]string{"--http-only=false", "--port", "9090", "--db-path", "/tmp/cli-test.db"})
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if got == nil {
		t.Fatal("callback never invoked")
	}
	if got.HTTPOnly {
		t.Errorf("HTTPOnly = true, want false (user passed --http-only=false)")
	}
	if got.Port != 9090 {
		t.Errorf("Port = %d, want 9090", got.Port)
	}
	if got.DBPath != "/tmp/cli-test.db" {
		t.Errorf("DBPath = %q, want /tmp/cli-test.db", got.DBPath)
	}
}

// TestWebviewFlagResolves 确认 `--webview` 会把 HTTPOnly 置为 false。
func TestWebviewFlagResolves(t *testing.T) {
	var got *ServeOptions
	root := NewRootCmd(func(opts *ServeOptions) error {
		got = opts
		return nil
	})

	root.SetArgs([]string{"--webview"})
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if got == nil {
		t.Fatal("callback never invoked")
	}
	if got.HTTPOnly {
		t.Errorf("HTTPOnly = true after --webview, want false (--webview disables http-only)")
	}
}

// TestContradictoryFlagsErrors 确认 --http-only + --webview 会报错，
// 而不是让“后写的 flag 生效”这种静默行为迷惑用户。
func TestContradictoryFlagsErrors(t *testing.T) {
	root := NewRootCmd(nil)
	root.SetArgs([]string{"--http-only", "--webview"})
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))
	err := root.Execute()
	if err == nil {
		t.Fatal("Execute succeeded; want error for contradictory flags")
	}
}

// TestSubcommandServeWorks 确认 `serve --http-only --port 9090`
// 同样带着正确 options 抵达回调（子命令路径）。
func TestSubcommandServeWorks(t *testing.T) {
	var got *ServeOptions
	root := NewRootCmd(func(opts *ServeOptions) error {
		got = opts
		return nil
	})

	root.SetArgs([]string{"serve", "--http-only", "--port", "7777"})
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if got == nil {
		t.Fatal("serve subcommand did not invoke callback")
	}
	if got.Port != 7777 {
		t.Errorf("Port = %d, want 7777", got.Port)
	}
}

// TestVersionSubcommandJustPrints 确认 `version` 不会调用 serve 回调
// （能抓住误覆盖 root.RunE 的情况）。
func TestVersionSubcommandJustPrints(t *testing.T) {
	var called bool
	root := NewRootCmd(func(opts *ServeOptions) error {
		called = true
		return nil
	})

	root.SetArgs([]string{"version"})
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if called {
		t.Error("serve callback was invoked from `version` subcommand")
	}
}
