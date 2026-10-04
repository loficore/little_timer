/**
 * useLLMSettings —— LLM 配置读写 hook。
 */

import { useCallback, useEffect, useState } from "preact/hooks";
import { getAPIClient } from "../utils/apiClientSingleton";
import { logError } from "../utils/logger";
import type { LLMSettingsDTO } from "../types/aigtd";

export interface LLMSettingsPatch {
  provider?: "openai_compat" | "anthropic";
  base_url?: string;
  model?: string;
  max_tokens?: number;
  timeout_seconds?: number;
  api_key?: string;
  clear_key?: boolean;
}

export function useLLMSettings() {
  const [cfg, setCfg] = useState<LLMSettingsDTO | null>(null);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const refresh = useCallback(async () => {
    try {
      setCfg(await getAPIClient().getLLMSettings());
    } catch (err) {
      const msg = err instanceof Error ? err.message : String(err);
      setError(msg);
      logError("useLLMSettings.refresh failed", new Error(msg));
    }
  }, []);

  const save = useCallback(async (patch: LLMSettingsPatch) => {
    setSaving(true);
    setError(null);
    try {
      const next = await getAPIClient().updateLLMSettings(patch);
      setCfg(next);
    } catch (err) {
      const msg = err instanceof Error ? err.message : String(err);
      setError(msg);
      logError("useLLMSettings.save failed", new Error(msg));
      throw err;
    } finally {
      setSaving(false);
    }
  }, []);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  return { cfg, saving, error, refresh, save };
}
