// Smoke tests for the CLI.  Goal: prove the Cobra wiring compiles,
// the platform-aware defaults are correct, and the serve callback
// path is wired through end-to-end (callback receives the right
// ServeOptions).
package cli

import (
	"bytes"
	"runtime"
	"testing"
)

// TestDefaultsLinux confirms the Linux/BSD/macOS default is http-only.
// Mirrors the Zig `builtin.os.tag == .windows → webview, else http-only`
// ternary.
func TestDefaultsLinux(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skipf("linux test on %s", runtime.GOOS)
	}
	if !defaultHTTPOnly() {
		t.Errorf("defaultHTTPOnly on %s = false, want true", runtime.GOOS)
	}
}

// TestRunServeCallbackReceivesOptions wires a callback and asserts it
// sees the right ServeOptions after `root --http-only=false --port 9090`.
// Exercises the root.RunE → invokeServe → callback path.
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

// TestWebviewFlagResolves confirms `--webview` flips HTTPOnly=false.
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

// TestContradictoryFlagsErrors confirms --http-only + --webview errors
// instead of silently letting "last flag wins" confuse the user.
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

// TestSubcommandServeWorks confirms `serve --http-only --port 9090`
// also reaches the callback with the right options (subcommand path).
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

// TestVersionSubcommandJustPrints confirms `version` doesn't invoke
// the serve callback (catches accidental root.RunE override).
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