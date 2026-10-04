package aigtd

import (
	"context"
	"sync/atomic"
)

// testOverride 是测试用的 LLM 响应覆盖(text + 可选 failWith)。worker
// 启动新 job 前读它,如果有就跳过真实 provider,返回固定响应或失败。
// 生产环境(SetTestOverride 未被调用)此字段始终为零值,nil。
var testOverride atomic.Pointer[testOverrideV]

type testOverrideV struct {
	text     string
	failWith int // 0 = 正常返回 text;非 0 = 返回 ErrRetryable(模拟 5xx)
}

// SetTestOverride 设置全局测试覆盖。`failWith == 0` 时 worker 把 text 作为
// LLM 响应;`failWith != 0` 时 worker 直接返 ErrRetryable(用于测试重试)。
// 传空 text + failWith=0 等价于 ClearTestOverride。
func SetTestOverride(text string, failWith int) {
	testOverride.Store(&testOverrideV{text: text, failWith: failWith})
}

// ClearTestOverride 移除测试覆盖。
func ClearTestOverride() {
	testOverride.Store(nil)
}

// currentTestOverride 读取当前覆盖(原子)。供 worker 内部使用。
func currentTestOverride() *testOverrideV {
	return testOverride.Load()
}

// staticProvider 是测试用的固定返回 provider(Complete 忽略请求,直接返 text 或错)。
type staticProvider struct {
	text     string
	failWith int
}

func (p *staticProvider) Name() string { return "test_override" }
func (p *staticProvider) Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error) {
	if p.failWith != 0 {
		return CompletionResponse{}, ErrRetryable
	}
	return CompletionResponse{Text: p.text}, nil
}
