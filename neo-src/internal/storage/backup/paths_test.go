package backup

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

// windowsIsAbs 模拟 Windows 的 filepath.IsAbs —— 只有带卷名的路径才算绝对。
// 直接拿 Linux 的 filepath.IsAbs 测 Windows 语义会得出错误结论：
// 在 Linux 上 "/d/backup_test" 是绝对的，在 Windows 上它不是。
func windowsIsAbs(p string) bool {
	if len(p) >= 2 && p[0] == '\\' && p[1] == '\\' {
		return true // UNC
	}
	if len(p) >= 3 && p[1] == ':' && (p[2] == '\\' || p[2] == '/') {
		return true // 卷绝对
	}
	return false
}

func windowsOps() pathOps {
	return pathOps{
		isAbs: windowsIsAbs,
		abs:   func(p string) (string, error) { return filepath.Clean(p), nil },
	}
}

func unixOps() pathOps {
	return pathOps{
		isAbs: func(p string) bool { return strings.HasPrefix(p, "/") },
		abs:   func(p string) (string, error) { return filepath.Clean(p), nil },
	}
}

func TestResolveLocalDir_RejectsEmpty(t *testing.T) {
	for _, in := range []string{"", "   ", "\t"} {
		if _, err := resolveLocalDir(in, unixOps(), false); !errors.Is(err, ErrInvalidLocalPath) {
			t.Fatalf("resolveLocalDir(%q) err = %v, want ErrInvalidLocalPath", in, err)
		}
	}
}

// 相对路径在任何平台上都拒绝：它的落点取决于进程 cwd，用户无法预测。
func TestResolveLocalDir_RejectsRelative(t *testing.T) {
	cases := []string{"backup_test", "./backup_test", "../backup_test", "a/b"}
	for _, in := range cases {
		if _, err := resolveLocalDir(in, unixOps(), false); !errors.Is(err, ErrInvalidLocalPath) {
			t.Fatalf("unix: resolveLocalDir(%q) err = %v, want ErrInvalidLocalPath", in, err)
		}
		if _, err := resolveLocalDir(in, windowsOps(), true); !errors.Is(err, ErrInvalidLocalPath) {
			t.Fatalf("windows: resolveLocalDir(%q) err = %v, want ErrInvalidLocalPath", in, err)
		}
	}
}

// 这是本次 bug 的核心：Windows 上 "/d/backup_test" 会被解析成
// 「当前盘:\d\backup_test」，而不是 D:\backup_test。必须拒绝，并把这件事说清楚。
func TestResolveLocalDir_WindowsRejectsPOSIXStylePath(t *testing.T) {
	_, err := resolveLocalDir("/d/backup_test", windowsOps(), true)
	if !errors.Is(err, ErrInvalidLocalPath) {
		t.Fatalf("err = %v, want ErrInvalidLocalPath", err)
	}
	msg := err.Error()
	for _, want := range []string{"/d/backup_test", `D:\backup_test`, "当前盘"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error message %q 缺少 %q（用户需要看得懂该怎么改）", msg, want)
		}
	}
}

func TestResolveLocalDir_WindowsAcceptsVolumeAbsolute(t *testing.T) {
	for _, in := range []string{`D:\backup_test`, `C:/backup_test`, `\\server\share\b`} {
		got, err := resolveLocalDir(in, windowsOps(), true)
		if err != nil {
			t.Fatalf("resolveLocalDir(%q) err = %v, want nil", in, err)
		}
		if !windowsIsAbs(got) {
			t.Errorf("resolveLocalDir(%q) = %q, 不是卷绝对路径", in, got)
		}
	}
}

func TestResolveLocalDir_UnixAcceptsAbsoluteAndCleans(t *testing.T) {
	got, err := resolveLocalDir("/tmp/a/../b/", unixOps(), false)
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if got != "/tmp/b" {
		t.Errorf("resolveLocalDir = %q, want /tmp/b", got)
	}
}
