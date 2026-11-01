/**
 * useLLMSettings —— LLM 配置读写 hook。
 */

import { useCallback, useEffect, useState } from "preact/hooks";
import { getAPIClient } from "../utils/apiClientSingleton";
import { logError } from "../utils/logger";
import type { LLMSettingsDTO, TestConnectionResult } from "../types/aigtd";

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
  const [testing, setTesting] = useState(false);
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

  /** 用当前表单值(可含未保存改动)测一次连通性。 */
  const test = useCallback(
    async (
      patch: Omit<LLMSettingsPatch, "clear_key">,
    ): Promise<TestConnectionResult> => {
      setTesting(true);
      setError(null);
      try {
        return await getAPIClient().testLLMSettings(patch);
      } finally {
        setTesting(false);
      }
    },
    [],
  );

  useEffect(() => {
    void refresh();
  }, [refresh]);

  return { cfg, saving, testing, error, refresh, save, test };
}
