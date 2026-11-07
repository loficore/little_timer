package backup

import (
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
)

// ErrInvalidLocalPath 表示配置的本地备份目录不可用（空、相对路径，或
// Windows 上非卷绝对的路径）。
//
// 为什么要在接受配置时就拦住：Windows 把「以 / 开头、但不带盘符」的路径
// 解释成「当前盘的根」。用户按 msys2 习惯填 "/d/backup_test" 时，实际会落到
// <进程 cwd 所在盘>:\d\backup_test；而 MkdirAll 会成功建目录、探针也读写正常，
// 于是界面报"备份成功"，用户在 D:\backup_test 里却什么都看不到。
var ErrInvalidLocalPath = errors.New("backup: invalid local path")

// pathOps 把与操作系统相关的路径判定抽出来，便于在 Linux 上覆盖 Windows 语义
// （Windows 与 Unix 对"绝对路径"的定义不同，而这个差异正是上面那个 bug 的来源）。
type pathOps struct {
	isAbs func(string) bool
	abs   func(string) (string, error)
}

var osPath = pathOps{isAbs: filepath.IsAbs, abs: filepath.Abs}

// ResolveLocalDir 校验并归一化用户配置的本地备份目录，返回绝对、清理过的路径。
// 空路径、相对路径、Windows 上非卷绝对的路径一律返回 ErrInvalidLocalPath。
func ResolveLocalDir(p string) (string, error) {
	return resolveLocalDir(p, osPath, runtime.GOOS == "windows")
}

func resolveLocalDir(p string, ops pathOps, windows bool) (string, error) {
	p = strings.TrimSpace(p)
	if p == "" {
		return "", fmt.Errorf("%w: 本地备份目录不能为空", ErrInvalidLocalPath)
	}
	if !ops.isAbs(p) {
		return "", fmt.Errorf("%w: %q 不是绝对路径。%s", ErrInvalidLocalPath, p, localPathHint(p, windows))
	}
	abs, err := ops.abs(p)
	if err != nil {
		return "", fmt.Errorf("%w: %q: %v", ErrInvalidLocalPath, p, err)
	}
	return abs, nil
}

func localPathHint(p string, windows bool) string {
	if !windows {
		return "请填写绝对路径（如 /var/backups/little_timer）"
	}
	hint := `请填写卷绝对路径（如 D:\backup_test）`
	drive, ok := posixStyleDrive(p)
	if !ok {
		return hint
	}
	// "/d/backup_test" 在 Windows 上是「当前盘根下的 d\backup_test」。
	rest := strings.TrimPrefix(p, "/")
	resolved := `当前盘:\` + strings.ReplaceAll(rest, "/", `\`)
	intended := drive + `:\` + strings.ReplaceAll(strings.TrimPrefix(rest, string(p[1])+"/"), "/", `\`)
	return fmt.Sprintf(`%s。%q 在 Windows 上会被解析成「%s」，而不是 %s`, hint, p, resolved, intended)
}

// posixStyleDrive 识别 "/d/foo" 这种 msys2 风格的盘符前缀，返回盘符字母。
func posixStyleDrive(p string) (string, bool) {
	if len(p) < 4 || p[0] != '/' || p[2] != '/' {
		return "", false
	}
	c := p[1]
	if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') {
		return strings.ToUpper(string(c)), true
	}
	return "", false
}
