import { useEffect, useState } from "preact/hooks";
import { useLLMSettings } from "../../hooks/useLLMSettings";
import { t } from "../../utils/i18n";

/**
 * LLMSettings —— 设置页 "AI / LLM" Tab 内容。
 *
 * 字段:Provider / Base URL / Model / Max Tokens / Timeout / API Key。
 * API Key 用密码框 + 末 4 位掩码;留空表示不动现有 Key;"清除" 显式删除。
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

  // cfg 加载完后填表单(一次性同步;之后用户编辑表单不覆盖 cfg)。
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

  return (
    <div className="space-y-3">
      <div className="form-control">
        <label className="label">
          <span className="label-text">{t("llm_settings.provider")}</span>
        </label>
        <select
          className="select select-bordered"
          value={provider}
          onChange={(e) => setProvider((e.currentTarget as HTMLSelectElement).value)}
        >
          <option value="openai_compat">{t("llm_settings.provider_openai_compat")}</option>
          <option value="anthropic">{t("llm_settings.provider_anthropic")}</option>
        </select>
      </div>

      <div className="form-control">
        <label className="label">
          <span className="label-text">{t("llm_settings.base_url")}</span>
        </label>
        <input
          className="input input-bordered"
          value={baseURL}
          placeholder="https://api.deepseek.com/v1"
          onInput={(e) => setBaseURL((e.currentTarget as HTMLInputElement).value)}
        />
      </div>

      <div className="form-control">
        <label className="label">
          <span className="label-text">{t("llm_settings.model")}</span>
        </label>
        <input
          className="input input-bordered"
          value={model}
          onInput={(e) => setModel((e.currentTarget as HTMLInputElement).value)}
        />
      </div>

      <div className="grid grid-cols-2 gap-3">
        <div className="form-control">
          <label className="label">
            <span className="label-text">{t("llm_settings.max_tokens")}</span>
          </label>
          <input
            type="number"
            className="input input-bordered"
            value={maxTokens}
            onInput={(e) => setMaxTokens(Number((e.currentTarget as HTMLInputElement).value))}
          />
        </div>
        <div className="form-control">
          <label className="label">
            <span className="label-text">{t("llm_settings.timeout_seconds")}</span>
          </label>
          <input
            type="number"
            className="input input-bordered"
            value={timeout}
            onInput={(e) => setTimeout(Number((e.currentTarget as HTMLInputElement).value))}
          />
        </div>
      </div>

      <div className="form-control">
        <label className="label">
          <span className="label-text">{t("llm_settings.api_key")}</span>
          {cfg.api_key_set && (
            <span className="label-text-alt">
              {t("llm_settings.api_key_set")} · {cfg.api_key_masked}
            </span>
          )}
        </label>
        <input
          type="password"
          className="input input-bordered"
          value={apiKey}
          placeholder={cfg.api_key_set ? "••••" : ""}
          onInput={(e) => setApiKey((e.currentTarget as HTMLInputElement).value)}
        />
        <p className="text-xs opacity-50">{t("llm_settings.api_key_masked_hint")}</p>
      </div>

      {cfg.api_key_set && (
        <label className="label cursor-pointer justify-start gap-2">
          <input
            type="checkbox"
            className="checkbox checkbox-sm"
            checked={clearKey}
            onChange={(e) => setClearKey((e.currentTarget as HTMLInputElement).checked)}
          />
          <span className="label-text">{t("llm_settings.clear_key")}</span>
        </label>
      )}

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
