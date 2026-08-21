// Package app 存放 little-timer 的进程级常量。
//
// Version、BuildTime、GitCommit 在链接期打戳，让 CLI 的 `version` 子命令
// 与未来的 `/api/version` handler 报告同一个事实来源。BuildTime 在 init 时
// 捕获（而不是走 `go build -ldflags`），这样 `go run` 或 `make build` 也能
// 产出有意义的时间戳，无需额外 flag。
package app

import (
	"os/exec"
	"strings"
	"time"
)

// Version 是人可读的发布 tag。每次发布递增；与 cliff.toml 的 `version`
// 字段保持一致。
var Version = "1.1.0"

// BuildTime 在包初始化时捕获。它实际是二进制被加载的时间 —— 对于
// “这个二进制是什么时候组装的”这一目的，足够接近构建时间了。
var BuildTime = time.Now().UTC().Format(time.RFC3339)

// GitCommit 是缩写版 HEAD commit hash。init 时通过 `git rev-parse --short
// HEAD` 读取一次；在 git 工作副本之外构建时（发布 tarball、Docker
// COPY-from-context 等）回退为 "unknown"。
//
// 该命令只读运行 —— 绝不修改工作树。
var GitCommit = resolveGitCommit()

// resolveGitCommit 调用 git 并返回短 HEAD hash。尽力而为：任何失败都返回
// "unknown"，保证二进制在非 git 环境也能启动。
func resolveGitCommit() string {
	out, err := exec.Command("git", "rev-parse", "--short", "HEAD").Output()
	if err != nil {
		return "unknown"
	}
	h := strings.TrimSpace(string(out))
	if h == "" {
		return "unknown"
	}
	return h
}
