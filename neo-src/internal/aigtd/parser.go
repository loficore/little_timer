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
//
// 评分警告(spec §5.3 / task 7):以 "[scores] " 前缀标记的 errs 条目不触发
// 重试 —— ValidateDecomposition 已就地回退默认并把合法值放进 res,继续
// 重试只会浪费 token;真 schema 错误仍按原逻辑重试并把错误反馈进 prompt。
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
		if perr == nil && len(realParseErrs(errs)) == 0 {
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

// realParseErrs 过滤掉评分警告(spec §5.3 / task 7),只保留真正的 schema 错误。
// 评分警告以 "[scores] " 前缀标记 —— ValidateDecomposition 已就地回退默认值,
// 不应再触发解析重试。
func realParseErrs(errs []string) []string {
	out := errs[:0:0]
	for _, e := range errs {
		if strings.HasPrefix(e, scoreWarnPrefix) {
			continue
		}
		out = append(out, e)
	}
	return out
}

// ParseReviewResult 是 ValidateReviewResult 的薄封装,提供给 worker / handler
// 直接复用;LLM 原始文本解析 + change_id 兜底重写在这里完成。
func ParseReviewResult(taskID, jobID int64, llmText string) ([]domain.ReviewProposal, []string, error) {
	return ValidateReviewResult(taskID, jobID, llmText)
}

// reviewRetryWarnings 过滤掉良性 warning(change_id 兜底重写),只保留真正需要
// 让 LLM 重试的校验失败(action/op 非法、proposal 反序列化失败)。
//
// Ruling(见 ledger):delta §4.2 明确 change_id 缺失/重复由 ValidateReviewResult
// 兜底重写即可采纳,故此 warning 不得触发重试 —— 否则 LLM 偶发省略 change_id
// 会被反复拒绝,违背兜底设计。计划 Step 4 的 "len(warnings)==0" 表述与此冲突,
// 以 spec 为准。
func reviewRetryWarnings(warnings []string) []string {
	out := warnings[:0:0]
	for _, w := range warnings {
		if strings.Contains(w, "change_id") {
			continue
		}
		out = append(out, w)
	}
	return out
}

// ReviewParseOutcome 是 ParseReviewWithRetryDetailed 的结果:proposals + 校验
// warnings + 最后一次 Provider 响应的原始文本与 token 计数(供 ai_jobs 审计 /
// 成本分析用;即使最终解析失败,RawText 仍保留以便排查)。
type ReviewParseOutcome struct {
	Proposals    []domain.ReviewProposal
	Warnings     []string
	RawText      string
	InputTokens  int
	OutputTokens int
}

// ParseReviewWithRetryDetailed 与 ParseWithRetryDetailed 同构:最多调用
// Provider maxAttempts 次,首次失败时把 warnings 反馈进 prompt 末尾再次调用。
//
// 与分解路径的关键差异:成功判据是 `len(warnings)==0 && err==nil`(validate
// 不丢 proposal,只追加 warning);warnings 非空时算失败以触发重试,避免
// 把非法 action / op 静默落库。
//
// taskID/jobID 用于 ValidateReviewResult 的 change_id 兜底重写(Δ2) ——
// 计划 baseline 签名省略了这两个参数,delta §4.2 要求"对缺失/重复的
// change_id 用 taskID-jobID-index 兜底",故签名补上。详见 ledger。
func ParseReviewWithRetryDetailed(ctx context.Context, p Provider, req CompletionRequest, taskID, jobID int64, maxAttempts int) (*ReviewParseOutcome, error) {
	if maxAttempts < 1 {
		maxAttempts = 1
	}
	userPrompt := req.UserPrompt
	out := &ReviewParseOutcome{}
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		r := req
		r.UserPrompt = userPrompt
		resp, err := p.Complete(ctx, r)
		if err != nil {
			// Provider 上游错误,不进入 parse 重试循环。
			return nil, err
		}
		out.RawText = resp.Text
		out.InputTokens = resp.InputTokens
		out.OutputTokens = resp.OutputTokens

		props, warns, perr := ValidateReviewResult(taskID, jobID, resp.Text)
		// 成功:无解析错误,且没有需要重试的 warning(change_id 兜底属良性,
		// 不算失败 —— 见 reviewRetryWarnings)。
		retry := reviewRetryWarnings(warns)
		if perr == nil && len(retry) == 0 {
			out.Proposals = props
			out.Warnings = warns // 良性 warning 仍回报,便于审计
			return out, nil
		}
		// 失败:把真正的问题追加进 user prompt 末尾,让 LLM 下一轮修正。
		var msg string
		if perr != nil {
			msg = perr.Error()
		} else {
			msg = strings.Join(retry, "; ")
		}
		userPrompt = req.UserPrompt + "\n\n你上次的输出无法通过校验:" + msg + "\n请按规则重新输出合法 JSON。"
		out.Warnings = warns
	}
	if out.RawText == "" {
		return nil, ErrParseFailed
	}
	return out, fmt.Errorf("%w: review validation failed after %d attempts", ErrParseFailed, maxAttempts)
}
