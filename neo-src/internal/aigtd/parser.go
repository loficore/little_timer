package aigtd

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"little-timer/internal/domain"
)

// ErrParseFailed 标记 LLM 输出在校验/解析重试后仍失败。
var ErrParseFailed = errors.New("aigtd: parse failed after retries")

// ParseOnce 调用一次 Provider 并返回校验结果(单次,无重试)。
func ParseOnce(ctx context.Context, p Provider, req CompletionRequest) (*domain.AIDecompositionResult, []string, error) {
	resp, err := p.Complete(ctx, req)
	if err != nil {
		return nil, nil, err
	}
	return ValidateDecomposition(resp.Text)
}

// ParseOutcome 是 ParseWithRetryDetailed 的结果:解析结果 + 最后一次 Provider
// 响应的原始文本与 token 计数(供 ai_jobs 审计/成本分析用)。
type ParseOutcome struct {
	Result       *domain.AIDecompositionResult
	RawText      string
	InputTokens  int
	OutputTokens int
}

// ParseWithRetry 最多调用 Provider maxAttempts 次,首次失败时把错误反馈追加
// 到 user prompt 末尾再次调用。Provider 错误(ErrRetryable / ErrFatal)直接
// 向上抛出,不进入"解析重试"循环(那一层由 Worker 通过入队重做)。
//
// maxAttempts < 2 退化为单次调用。
func ParseWithRetry(ctx context.Context, p Provider, req CompletionRequest, maxAttempts int) (*domain.AIDecompositionResult, error) {
	out, err := ParseWithRetryDetailed(ctx, p, req, maxAttempts)
	if err != nil {
		return nil, err
	}
	return out.Result, nil
}

// ParseWithRetryDetailed 与 ParseWithRetry 逻辑相同,但额外返回最后一次
// Provider 响应的原始文本与 token 计数(即使解析失败也保留最后一次响应文本,
// 便于 ai_jobs.response_payload 留痕)。
func ParseWithRetryDetailed(ctx context.Context, p Provider, req CompletionRequest, maxAttempts int) (*ParseOutcome, error) {
	if maxAttempts < 1 {
		maxAttempts = 1
	}

	userPrompt := req.UserPrompt
	var lastErr error
	out := &ParseOutcome{}
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		r := req
		r.UserPrompt = userPrompt

		resp, err := p.Complete(ctx, r)
		if err != nil {
			// Provider 上游错误 — 不在 parse 重试里循环。
			return nil, err
		}
		out.RawText = resp.Text
		out.InputTokens = resp.InputTokens
		out.OutputTokens = resp.OutputTokens

		res, errs, perr := ValidateDecomposition(resp.Text)
		if perr == nil && len(errs) == 0 {
			out.Result = res
			return out, nil
		}

		if perr != nil {
			lastErr = perr
			userPrompt = req.UserPrompt + "\n\n你上次的输出无法解析为 JSON,错误:" +
				perr.Error() + "\n请重新输出合法 JSON。"
		} else {
			lastErr = fmt.Errorf("schema validation failed: %v", errs)
			userPrompt = req.UserPrompt + "\n\n你上次的输出不符合 schema,以下错误:\n- " +
				strings.Join(errs, "\n- ") + "\n请修正后重新输出合法 JSON。"
		}
	}

	if lastErr == nil {
		lastErr = ErrParseFailed
	}
	return out, fmt.Errorf("%w: %v", ErrParseFailed, lastErr)
}
