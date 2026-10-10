import { useEffect, useState } from "preact/hooks";
import { useLLMSettings } from "../../hooks/useLLMSettings";
import { SettingItem } from "../SettingItem";
import { SelectInput } from "../SelectInput";
import { t } from "../../utils/i18n";

/**
 * LLMSettings —— 设置页 "AI / LLM" Tab 内容。
 *
 * 字段:Provider / Base URL / Model / Max Tokens / Timeout / API Key。
 * API Key 用密码框 + 末 4 位掩码;留空表示不动现有 Key;"清除" 显式删除。
 *
 * 布局沿用 BasicSettings/CountdownSettings 的 SettingItem 模式(左标签 + 右控件),
 * 与其他设置子选项保持一致。
 */
export function LLMSettings() {
  const { cfg, saving, error, save } = useLLMSettings();
  const [provider, setProvider] = useState<string>("");
  const [baseURL, setBaseURL] = useState("");
  const [model, setModel] = useState("");
  const [maxTokens, setMaxTokens] = useState<number>(2048);
  const [timeout, setTimeout] = useState<number>(30);
  const [apiKey, setApiKey] = useState("");
  const [clearKey, setClearKey] = useState(false);
  const [feedback, setFeedback] = useState<string | null>(null);

  useEffect(() => {
    if (cfg) {
      setProvider(cfg.provider);
      setBaseURL(cfg.base_url);
      setModel(cfg.model);
      setMaxTokens(cfg.max_tokens);
      setTimeout(cfg.timeout_seconds);
    }
  }, [cfg]);

  const onSave = async () => {
    setFeedback(null);
    try {
      await save({
        provider: provider as "openai_compat" | "anthropic",
        base_url: baseURL,
        model,
        max_tokens: maxTokens,
        timeout_seconds: timeout,
        api_key: apiKey || undefined,
        clear_key: clearKey,
      });
      setApiKey("");
      setClearKey(false);
      setFeedback(t("llm_settings.saved"));
    } catch (e) {
      setFeedback(e instanceof Error ? e.message : String(e));
    }
  };

  if (!cfg) {
    return <p className="opacity-60">…</p>;
  }

  const providerOptions = [
    { value: "openai_compat", label: t("llm_settings.provider_openai_compat") },
    { value: "anthropic", label: t("llm_settings.provider_anthropic") },
  ];

  return (
    <div className="space-y-4 sm:space-y-6">
      <SettingItem label={t("llm_settings.provider")}>
        <SelectInput
          value={provider}
          options={providerOptions}
          onChange={(v) => setProvider(v)}
        />
      </SettingItem>

      <SettingItem label={t("llm_settings.base_url")}>
        <input
          className="input input-bordered w-full"
          value={baseURL}
          placeholder="https://api.deepseek.com/v1"
          onInput={(e) => setBaseURL((e.currentTarget as HTMLInputElement).value)}
        />
      </SettingItem>

      <SettingItem label={t("llm_settings.model")}>
        <input
          className="input input-bordered w-full"
          value={model}
          onInput={(e) => setModel((e.currentTarget as HTMLInputElement).value)}
        />
      </SettingItem>

      <SettingItem label={t("llm_settings.max_tokens")}>
        <input
          type="number"
          className="input input-bordered w-full"
          value={maxTokens}
          onInput={(e) => setMaxTokens(Number((e.currentTarget as HTMLInputElement).value))}
        />
      </SettingItem>

      <SettingItem label={t("llm_settings.timeout_seconds")}>
        <input
          type="number"
          className="input input-bordered w-full"
          value={timeout}
          onInput={(e) => setTimeout(Number((e.currentTarget as HTMLInputElement).value))}
        />
      </SettingItem>

      <SettingItem label={t("llm_settings.api_key")}>
        <input
          type="password"
          className="input input-bordered w-full"
          value={apiKey}
          placeholder={cfg.api_key_set ? "••••" : ""}
          onInput={(e) => setApiKey((e.currentTarget as HTMLInputElement).value)}
        />
        <p className="text-xs opacity-50">{t("llm_settings.api_key_masked_hint")}</p>
        {cfg.api_key_set && (
          <span className="text-xs opacity-70">
            {t("llm_settings.api_key_set")} · {cfg.api_key_masked}
          </span>
        )}
        {cfg.api_key_set && (
          <label className="mt-1 flex cursor-pointer items-center gap-2">
            <input
              type="checkbox"
              className="checkbox checkbox-sm"
              checked={clearKey}
              onChange={(e) => setClearKey((e.currentTarget as HTMLInputElement).checked)}
            />
            <span className="text-sm">{t("llm_settings.clear_key")}</span>
          </label>
        )}
      </SettingItem>

      <div className="flex items-center gap-2">
        <button className="btn btn-primary" disabled={saving} onClick={() => void onSave()}>
          {saving ? "..." : t("common.save")}
        </button>
        {feedback && <span className="text-sm opacity-70">{feedback}</span>}
        {error && <span className="text-sm text-error">{error}</span>}
      </div>
    </div>
  );
}