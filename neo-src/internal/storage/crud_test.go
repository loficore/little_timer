package storage

import (
	"testing"

	"little-timer/internal/domain"
)

// TestSaveSettingsKeepsForeignColumns 验证 SaveSettings 只更新它负责的基础/
// 计时/日志列,不会把同一行里别的模块维护的列带回 schema DEFAULT。
//
// 历史 bug:保存语句是 `INSERT OR REPLACE INTO settings (...)` 且列清单停在
// log_tick_interval。REPLACE 会先删旧行再插新行,未列出的列(llm_*、
// daily_sweep_*)于是取建表 DEFAULT —— 用户改一次主题就把 Base URL / 模型
// 重置成默认值,随后拿中转站 Key 去请求官方端点,得到 401。
func TestSaveSettingsKeepsForeignColumns(t *testing.T) {
	m := openTempSqlite(t)

	type foreign struct {
		Provider     string
		BaseURL      string
		Model        string
		MaxTokens    int64
		Timeout      int64
		SweepEnabled int64
		SweepTime    string
	}
	want := foreign{
		Provider:     "openai_compat",
		BaseURL:      "https://relay.example/v1",
		Model:        "gpt-4o",
		MaxTokens:    8000,
		Timeout:      120,
		SweepEnabled: 0,
		SweepTime:    "10:30",
	}

	// 模拟 LLM 设置页 / Daily Sweep 各自写过的值(不经过 SaveSettings)。
	if _, err := m.DB().Exec(
		`UPDATE settings SET
			llm_provider=?, llm_base_url=?, llm_model=?, llm_max_tokens=?, llm_timeout_seconds=?,
			daily_sweep_enabled=?, daily_sweep_time=?
		 WHERE id = 1;`,
		want.Provider, want.BaseURL, want.Model, want.MaxTokens, want.Timeout,
		want.SweepEnabled, want.SweepTime,
	); err != nil {
		t.Fatalf("预置 llm_/daily_sweep_ 列: %v", err)
	}

	cfg := domain.NewDefaultSettingsConfig()
	cfg.Basic.ThemeMode = "light"
	if err := m.SaveSettings(cfg); err != nil {
		t.Fatalf("SaveSettings: %v", err)
	}

	var got foreign
	if err := m.DB().QueryRow(
		`SELECT llm_provider, llm_base_url, llm_model, llm_max_tokens, llm_timeout_seconds,
		        daily_sweep_enabled, daily_sweep_time
		 FROM settings WHERE id = 1;`,
	).Scan(
		&got.Provider, &got.BaseURL, &got.Model, &got.MaxTokens, &got.Timeout,
		&got.SweepEnabled, &got.SweepTime,
	); err != nil {
		t.Fatalf("读回 llm_/daily_sweep_ 列: %v", err)
	}

	if got != want {
		t.Errorf("SaveSettings 覆盖了不属于它的列:\n got  %+v\n want %+v", got, want)
	}
}
