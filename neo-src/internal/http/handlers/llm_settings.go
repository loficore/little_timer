// Package handlers —— /api/settings/llm:LLM provider 配置读写。
//
// 非敏感字段(Provider/BaseURL/Model/MaxTokens/TimeoutSeconds)直接写在
// `settings` 表的 llm_* 列;API Key 加密后存在 SecretStorage(受平台 keystore
// 托管,不再受主口令保护)。
package handlers

import (
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"little-timer/internal/aigtd"
	"little-timer/internal/http/app"
)

const llmAPIKeySecret = aigtd.APIKeySecretName

// LLMSettingsDTO 是 GET /api/settings/llm 的出参。
type LLMSettingsDTO struct {
	Provider       string `json:"provider"`
	BaseURL        string `json:"base_url"`
	Model          string `json:"model"`
	MaxTokens      int    `json:"max_tokens"`
	TimeoutSeconds int    `json:"timeout_seconds"`
	APIKeySet      bool   `json:"api_key_set"`
	APIKeyMasked   string `json:"api_key_masked,omitempty"`
}

// LLMSettingsGet GET /api/settings/llm
func LLMSettingsGet(c *gin.Context) {
	a := appFromCtx(c)
	dto, err := readLLMSettings(a)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "read failed"})
		return
	}
	c.JSON(http.StatusOK, dto)
}

// LLMSettingsUpdate PUT /api/settings/llm
//
// body 字段(全部可选):provider / base_url / model / max_tokens /
// timeout_seconds / api_key(非空=更新或设置) / clear_key(bool=删除已存的 Key)。
func LLMSettingsUpdate(c *gin.Context) {
	a := appFromCtx(c)
	var body map[string]any
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid json"})
		return
	}

	if v, ok := body["provider"].(string); ok && v != "" {
		if v != "openai_compat" && v != "anthropic" {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid provider"})
			return
		}
	}
	if v, ok := body["max_tokens"].(float64); ok {
		if v < 1 || v > 32000 {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "max_tokens out of range"})
			return
		}
	}
	if v, ok := body["timeout_seconds"].(float64); ok {
		if v < 1 || v > 300 {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "timeout_seconds out of range"})
			return
		}
	}

	current, err := readLLMSettings(a)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "read failed"})
		return
	}

	provider := pickStr(body, "provider", current.Provider)
	if provider == "" {
		provider = "openai_compat"
	}
	baseURL := pickStr(body, "base_url", current.BaseURL)
	model := pickStr(body, "model", current.Model)
	if model == "" {
		model = "deepseek-chat"
	}
	maxTokens := pickInt(body, "max_tokens", current.MaxTokens)
	if maxTokens == 0 {
		maxTokens = 2048
	}
	timeout := pickInt(body, "timeout_seconds", current.TimeoutSeconds)
	if timeout == 0 {
		timeout = 30
	}

	// 判定是否涉及秘钥字段(api_key / clear_key)。去锁化(Task 5):不再有
	// 锁定态,凭据由 keystore 托管,直接进入写入/删除分支。
	hasAPIKey := false
	bodyClearKey := false
	if v, ok := body["api_key"].(string); ok && v != "" {
		hasAPIKey = true
	}
	if clear, _ := body["clear_key"].(bool); clear {
		bodyClearKey = true
	}

	secrets := a.Secrets()

	if _, err := a.SQLite.DB().Exec(
		`UPDATE settings
		 SET llm_provider=?, llm_base_url=?, llm_model=?, llm_max_tokens=?, llm_timeout_seconds=?
		 WHERE id=1;`,
		provider, baseURL, model, maxTokens, timeout,
	); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "update failed"})
		return
	}

	if hasAPIKey {
		if err := secrets.Store([]byte(llmAPIKeySecret), []byte(body["api_key"].(string))); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "store key failed"})
			return
		}
	}
	if bodyClearKey {
		if err := secrets.Delete([]byte(llmAPIKeySecret)); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "clear key failed"})
			return
		}
	}

	out, _ := readLLMSettings(a)
	c.JSON(http.StatusOK, out)
}

func readLLMSettings(a *app.App) (*LLMSettingsDTO, error) {
	row := a.SQLite.DB().QueryRow(
		`SELECT llm_provider, llm_base_url, llm_model, llm_max_tokens, llm_timeout_seconds
		 FROM settings WHERE id = 1;`,
	)
	var d LLMSettingsDTO
	if err := row.Scan(&d.Provider, &d.BaseURL, &d.Model, &d.MaxTokens, &d.TimeoutSeconds); err != nil {
		return nil, err
	}

	secrets := a.Secrets()
	// 去锁化(Task 5):直接读取;未配置(ErrSecretNotFound)或 keystore 不可用
	// (ErrKeystoreUnavailable)时保持默认未设置态,不谎报。
	if key, err := secrets.Retrieve([]byte(llmAPIKeySecret)); err == nil && len(key) > 0 {
		d.APIKeySet = true
		d.APIKeyMasked = maskAPIKey(string(key))
	}

	if d.Provider == "" {
		d.Provider = "openai_compat"
	}
	if d.Model == "" {
		d.Model = "deepseek-chat"
	}
	if d.MaxTokens == 0 {
		d.MaxTokens = 2048
	}
	if d.TimeoutSeconds == 0 {
		d.TimeoutSeconds = 30
	}
	return &d, nil
}

// LLMTestResult 是 POST /api/settings/llm/test 的出参。
//
// 语义:HTTP 200 + ok=false 表示"请求发出去了,但配置或上游有问题",前端把
// 它当可读的业务反馈渲染;HTTP 4xx/5xx 只留给请求本身非法(JSON 坏了、DB
// 读失败)——这样「Key 错了」「模型名不存在」不会和「后端挂了」混为一谈。
type LLMTestResult struct {
	OK        bool   `json:"ok"`
	LatencyMS int64  `json:"latency_ms"`
	Model     string `json:"model"`
	Error     string `json:"error,omitempty"`
}

// LLMSettingsTest POST /api/settings/llm/test
//
// 用「已保存配置 + 本次表单覆盖」发一次最小补全,验证 Base URL / 模型 /
// API Key / 网络是否真的可用。body 字段与 PUT 相同且全部可选;api_key 留空
// 表示沿用已保存的 Key(用户可以改完表单先测、再保存)。
//
// 探针固定只要 64 token 以内,不跟随用户配置的 32000 上限——测试本身不应该
// 变慢或变贵。超时沿用配置值(上限 300s),因为"配置的超时撑不撑得住"也是
// 要验证的一部分。
func LLMSettingsTest(c *gin.Context) {
	a := appFromCtx(c)
	var body map[string]any
	if err := c.ShouldBindJSON(&body); err != nil && !errors.Is(err, io.EOF) {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid json"})
		return
	}

	cur, err := readLLMSettings(a)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "read failed"})
		return
	}

	provider := pickStr(body, "provider", cur.Provider)
	switch provider {
	case "", "openai_compat":
		provider = "openai_compat"
	case "anthropic":
	default:
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid provider"})
		return
	}

	baseURL := pickStr(body, "base_url", cur.BaseURL)
	model := pickStr(body, "model", cur.Model)
	maxTokens := pickInt(body, "max_tokens", cur.MaxTokens)
	timeoutSec := pickInt(body, "timeout_seconds", cur.TimeoutSeconds)
	if timeoutSec <= 0 || timeoutSec > 300 {
		timeoutSec = 30
	}
	if maxTokens <= 0 || maxTokens > 64 {
		maxTokens = 64
	}

	apiKey := ""
	if v, ok := body["api_key"].(string); ok {
		apiKey = v
	}
	if apiKey == "" {
		if key, kerr := a.Secrets().Retrieve([]byte(llmAPIKeySecret)); kerr == nil {
			apiKey = string(key)
		}
	}

	result := LLMTestResult{Model: model}
	if apiKey == "" {
		result.Error = "未配置 API Key"
		c.JSON(http.StatusOK, result)
		return
	}

	timeout := time.Duration(timeoutSec) * time.Second
	var p aigtd.Provider
	switch provider {
	case "anthropic":
		p = aigtd.NewAnthropicProvider(apiKey, model, timeout)
	default:
		if baseURL == "" {
			baseURL = "https://api.deepseek.com/v1"
		}
		p = aigtd.NewOpenAICompatProvider(baseURL, apiKey, model, timeout)
	}

	start := time.Now()
	_, perr := p.Complete(c.Request.Context(), aigtd.CompletionRequest{
		SystemPrompt: "You are a connectivity probe. Reply with JSON only.",
		UserPrompt:   `{"ping":true}`,
		Model:        model,
		MaxTokens:    maxTokens,
	})
	result.LatencyMS = time.Since(start).Milliseconds()
	if perr != nil {
		result.Error = perr.Error()
	} else {
		result.OK = true
	}
	c.JSON(http.StatusOK, result)
}

func maskAPIKey(s string) string {
	if len(s) <= 4 {
		return "***"
	}
	return "***" + s[len(s)-4:]
}

func pickStr(body map[string]any, key, current string) string {
	if v, ok := body[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return current
}

func pickInt(body map[string]any, key string, current int) int {
	if v, ok := body[key]; ok {
		if f, ok := v.(float64); ok {
			return int(f)
		}
	}
	return current
}
