// Package handlers —— /api/settings/llm:LLM provider 配置读写。
//
// 非敏感字段(Provider/BaseURL/Model/MaxTokens/TimeoutSeconds)直接写在
// `settings` 表的 llm_* 列;API Key 加密后存在 SecretStorage(受主密码保护)。
package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"little-timer/internal/http/app"
)

const llmAPIKeySecret = "llm_api_key"

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

	if _, err := a.SQLite.DB().Exec(
		`UPDATE settings
		 SET llm_provider=?, llm_base_url=?, llm_model=?, llm_max_tokens=?, llm_timeout_seconds=?
		 WHERE id=1;`,
		provider, baseURL, model, maxTokens, timeout,
	); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "update failed"})
		return
	}

	secrets := a.Secrets()
	if v, ok := body["api_key"].(string); ok && v != "" {
		if secrets.IsLocked() {
			c.JSON(http.StatusLocked, gin.H{"success": false, "code": "auth.master_password_required", "error": "master password required"})
			return
		}
		if err := secrets.Store([]byte(llmAPIKeySecret), []byte(v)); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "store key failed"})
			return
		}
	}
	if clear, _ := body["clear_key"].(bool); clear {
		_ = secrets.Delete([]byte(llmAPIKeySecret))
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
	if !secrets.IsLocked() {
		if key, err := secrets.Retrieve([]byte(llmAPIKeySecret)); err == nil && len(key) > 0 {
			d.APIKeySet = true
			d.APIKeyMasked = maskAPIKey(string(key))
		}
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
