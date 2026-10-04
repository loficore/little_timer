package aigtd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// AnthropicProvider 调用 Anthropic Messages API(独立实现,不走 OpenAI 协议)。
type AnthropicProvider struct {
	baseURL string
	apiKey  string
	model   string
	client  *http.Client
}

// NewAnthropicProvider 构造 Anthropic provider,使用默认 base URL。
func NewAnthropicProvider(apiKey, model string, timeout time.Duration) *AnthropicProvider {
	return NewAnthropicProviderWithBaseURL(apiKey, model, timeout, "https://api.anthropic.com")
}

// NewAnthropicProviderWithBaseURL 构造 Anthropic provider,允许自定义 base URL(测试用)。
func NewAnthropicProviderWithBaseURL(apiKey, model string, timeout time.Duration, baseURL string) *AnthropicProvider {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return &AnthropicProvider{
		baseURL: baseURL,
		apiKey:  apiKey,
		model:   model,
		client:  &http.Client{Timeout: timeout},
	}
}

// Name 实现 Provider。
func (p *AnthropicProvider) Name() string { return "anthropic" }

// Complete 实现 Provider。Anthropic 不支持 response_format,system prompt 中
// 已包含"只回 JSON"的硬约束。
func (p *AnthropicProvider) Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error) {
	model := req.Model
	if model == "" {
		model = p.model
	}
	maxTokens := req.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 2048
	}
	body := map[string]any{
		"model":      model,
		"system":     req.SystemPrompt,
		"messages":   []map[string]string{{"role": "user", "content": req.UserPrompt}},
		"max_tokens": maxTokens,
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return CompletionResponse{}, fmt.Errorf("%w: marshal: %v", ErrFatal, err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/v1/messages", bytes.NewReader(raw))
	if err != nil {
		return CompletionResponse{}, fmt.Errorf("%w: new request: %v", ErrFatal, err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", p.apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

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
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
		Usage struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return CompletionResponse{}, fmt.Errorf("%w: decode: %v", ErrRetryable, err)
	}
	if len(parsed.Content) == 0 {
		return CompletionResponse{}, fmt.Errorf("%w: empty content", ErrRetryable)
	}
	return CompletionResponse{
		Text:         parsed.Content[0].Text,
		InputTokens:  parsed.Usage.InputTokens,
		OutputTokens: parsed.Usage.OutputTokens,
	}, nil
}
