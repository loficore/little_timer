// 上游 webview.h 只在 debug==true 时才调用 webkit_settings_set_enable_developer_extras,
// 没有这个开关就看不到 WebKit 自带的 profiler,故用 LT_WEBKIT_DEBUG 显式控制。
package webview

import (
	"os"
	"strings"
)

const devtoolsEnvVar = "LT_WEBKIT_DEBUG"

func devtoolsEnabled() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(devtoolsEnvVar))) {
	case "1", "true", "on":
		return true
	default:
		return false
	}
}
