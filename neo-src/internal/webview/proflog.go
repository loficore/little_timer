// console 行的终端回传 —— 页面 console.log 默认到不了终端(WebKitGTK 不转发),
// cadence 矩阵因此没法自动留档。devtools 开启时用 webview 绑定把
// assets/devprobe/cadence.js 的 [CAD]/[PERF] 行写回 stderr;默认关闭时
// 绑定不存在,页面侧静默降级,发布行为不变。
package webview

import (
	"io"
	"strings"
)

const profBindingName = "ltProfLog"

func writeProfLine(w io.Writer, msg string) error {
	trimmed := strings.TrimRight(msg, "\n")
	if strings.TrimSpace(trimmed) == "" {
		return nil
	}
	_, err := io.WriteString(w, trimmed+"\n")
	return err
}
