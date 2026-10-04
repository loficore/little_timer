// Package aigtd 是 AI GTD 拆解引擎:Provider 抽象(OpenAI 兼容 + Anthropic)、
// Prompt / JSON Schema / 校验、异步 Worker。
//
// 设计原则:本包不直接接触数据库,Storage 通过 SqliteManager 传入;LLM API Key
// 由调用方解密后传入,本包绝不写日志。
package aigtd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ErrRetryable 标记可重试的上游错误(5xx / 网络 / 超时)。
var ErrRetryable = errors.New("aigtd: retryable provider error")

// ErrFatal 标记不可重试的上游错误(4xx:配额、鉴权、模型名错误)。
var ErrFatal = errors.New("aigtd: fatal provider error")

// Provider 是所有 LLM 后端的统一接口。
type Provider interface {
	Name() string
	Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error)
}

// CompletionRequest 是一次补全请求。
type CompletionRequest struct {
	SystemPrompt string
	UserPrompt   string
	Model        string
	MaxTokens    int
	// Temperature 默认 0.2(需要稳定的 JSON 输出)。
	Temperature float64
}

// CompletionResponse 是一次补全结果。
type CompletionResponse struct {
	Text         string
	InputTokens  int
	OutputTokens int
}

// OpenAICompatProvider 调用 OpenAI 兼容的 /chat/completions 端点。
// DeepSeek / OpenAI / OpenRouter / Azure OpenAI / Ollama / LM Studio /
// vLLM / llama.cpp server / LocalAI 等均可通过它接入。
type OpenAICompatProvider struct {
	baseURL string
	apiKey  string
	model   string
	client  *http.Client
}

// NewOpenAICompatProvider 构造 OpenAI 兼容 provider。
//
// `baseURL` 应包含版本段(如 `https://api.deepseek.com/v1`),endpoint 会在其后
// 追加 `/chat/completions`。
func NewOpenAICompatProvider(baseURL, apiKey, model string, timeout time.Duration) *OpenAICompatProvider {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return &OpenAICompatProvider{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		model:   model,
		client:  &http.Client{Timeout: timeout},
	}
}

// Name 实现 Provider。
func (p *OpenAICompatProvider) Name() string { return "openai_compat" }

// Complete 实现 Provider。
func (p *OpenAICompatProvider) Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error) {
	model := req.Model
	if model == "" {
		model = p.model
	}
	temp := req.Temperature
	if temp == 0 {
		temp = 0.2
	}
	body := map[string]any{
		"model": model,
		"messages": []map[string]string{
			{"role": "system", "content": req.SystemPrompt},
			{"role": "user", "content": req.UserPrompt},
		},
		"response_format": map[string]string{"type": "json_object"},
		"temperature":     temp,
	}
	if req.MaxTokens > 0 {
		body["max_tokens"] = req.MaxTokens
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return CompletionResponse{}, fmt.Errorf("%w: marshal: %v", ErrFatal, err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/chat/completions", bytes.NewReader(raw))
	if err != nil {
		return CompletionResponse{}, fmt.Errorf("%w: new request: %v", ErrFatal, err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if p.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
	}

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return CompletionResponse{}, fmt.Errorf("%w: %v", ErrRetryable, err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if resp.StatusCode >= 500 {
		return CompletionResponse{}, fmt.Errorf("%w: http %d", ErrRetryable, resp.StatusCode)
	}
	if resp.StatusCode >= 400 {
		return CompletionResponse{}, fmt.Errorf("%w: http %d: %s", ErrFatal, resp.StatusCode, truncate(respBody, 300))
	}

	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return CompletionResponse{}, fmt.Errorf("%w: decode: %v", ErrRetryable, err)
	}
	if len(parsed.Choices) == 0 {
		return CompletionResponse{}, fmt.Errorf("%w: empty choices", ErrRetryable)
	}
	return CompletionResponse{
		Text:         parsed.Choices[0].Message.Content,
		InputTokens:  parsed.Usage.PromptTokens,
		OutputTokens: parsed.Usage.CompletionTokens,
	}, nil
}

// truncate 截断错误正文,避免把超长响应塞进日志/错误。
func truncate(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n]) + "…"
}
