/**
 * 设置管理 Hook
 * 统一管理应用设置、持久化和同步
 */

import { useState, useEffect, useRef, useCallback } from "preact/hooks";
import { getAPIClient } from "../utils/apiClientSingleton";
import {
  DEFAULT_AUDIO_PREFERENCES,
  loadAudioPreferences,
  normalizeAudioPreferences,
  saveAudioPreferences,
  type AudioPreferences,
} from "../utils/audio";
import { t } from "../utils/i18n";
import { logError } from "../utils/logger";
import { STORAGE_KEYS } from "../utils/constants";

export interface BasicSettings {
  timezone: number;
  language: string;
  default_mode: string;
  theme_mode: string;
  wallpaper?: string;
  sound_enabled: boolean;
  sound_tick: boolean;
  sound_finish: boolean;
  sound_volume: number;
  layout_density?: string;
  time_display_style?: string;
  debug_mode?: boolean;
}

export interface ClockDefaults {
  countdown: {
    duration_seconds: number;
    loop: boolean;
    loop_count: number;
    loop_interval_seconds: number;
  };
  stopwatch: {
    max_seconds: number;
  };
}

export interface UseSettingsReturn {
  settings: BasicSettings;
  clockDefaults: ClockDefaults;
  audioPreferences: AudioPreferences;
  isSaving: boolean;
  saveMessage: string;
  updateSettings: (settings: Partial<BasicSettings>) => void;
  updateClockDefaults: (defaults: Partial<ClockDefaults>) => void;
  save: () => Promise<void>;
  reset: () => void;
}

const DEFAULT_SETTINGS: BasicSettings = {
  timezone: 8,
  language: "ZH",
  default_mode: "countdown",
  theme_mode: "dark",
  wallpaper: "",
  sound_enabled: DEFAULT_AUDIO_PREFERENCES.sound_enabled,
  sound_tick: DEFAULT_AUDIO_PREFERENCES.sound_tick,
  sound_finish: DEFAULT_AUDIO_PREFERENCES.sound_finish,
  sound_volume: DEFAULT_AUDIO_PREFERENCES.sound_volume,
  layout_density: "normal",
  time_display_style: "classic",
  debug_mode: false,
};

const DEBUG_STORAGE_KEY = "lt_debug_perf";

const loadDebugMode = (): boolean => {
  if (typeof window === "undefined") return false;
  try {
    return localStorage.getItem(DEBUG_STORAGE_KEY) === "1";
  } catch {
    return false;
  }
};

const DEFAULT_CLOCK_DEFAULTS: ClockDefaults = {
  countdown: {
    duration_seconds: 1500,
    loop: false,
    loop_count: 0,
    loop_interval_seconds: 0,
  },
  stopwatch: {
    max_seconds: 86400,
  },
};

export const useSettings = (): UseSettingsReturn => {
  const apiClientRef = useRef(getAPIClient());
  const [settings, setSettings] = useState<BasicSettings>(DEFAULT_SETTINGS);
  const [clockDefaults, setClockDefaults] = useState<ClockDefaults>(DEFAULT_CLOCK_DEFAULTS);
  const [audioPreferences, setAudioPreferences] = useState<AudioPreferences>(loadAudioPreferences());
  const [isSaving, setIsSaving] = useState(false);
  const [saveMessage, setSaveMessage] = useState("");

  const applyTheme = useCallback((themeMode = "dark") => {
    const html = document.documentElement;
    const theme =
      themeMode === "auto"
        ? window.matchMedia("(prefers-color-scheme: light)").matches
          ? "light"
          : "dark"
        : themeMode;

    if (theme === "light") {
      html.classList.add("light-mode");
      document.body.classList.add("light-mode");
    } else {
      html.classList.remove("light-mode");
      document.body.classList.remove("light-mode");
    }
  }, []);

  const loadSettings = useCallback(async () => {
    try {
      const serverSettings = await apiClientRef.current.getSettings();
      const localAudio = loadAudioPreferences();
      
      const basic = serverSettings?.basic as Record<string, unknown> || {};
      const countdown = serverSettings?.countdown as Record<string, unknown> || {};
      const stopwatch = serverSettings?.stopwatch as Record<string, unknown> || {};

      const audioPrefs = normalizeAudioPreferences({
        sound_enabled: (basic.sound_enabled as boolean) ?? localAudio.sound_enabled,
        sound_tick: (basic.sound_tick as boolean) ?? localAudio.sound_tick,
        sound_finish: (basic.sound_finish as boolean) ?? localAudio.sound_finish,
        sound_volume: (basic.sound_volume as number) ?? localAudio.sound_volume,
      });
      saveAudioPreferences(audioPrefs);
      setAudioPreferences(audioPrefs);

      const localLayoutDensity = localStorage.getItem(STORAGE_KEYS.LAYOUT_DENSITY) || "normal";
      const localTimeDisplayStyle = localStorage.getItem(STORAGE_KEYS.TIME_DISPLAY_STYLE) || "classic";

      setSettings({
        timezone: (basic.timezone as number) ?? 8,
        language: (basic.language as string) ?? "ZH",
        default_mode: (basic.default_mode as string) ?? "countdown",
        theme_mode: (basic.theme_mode as string) ?? "dark",
        wallpaper: (basic.wallpaper as string) ?? "",
        sound_enabled: audioPrefs.sound_enabled,
        sound_tick: audioPrefs.sound_tick,
        sound_finish: audioPrefs.sound_finish,
        sound_volume: audioPrefs.sound_volume,
        layout_density: localLayoutDensity,
        time_display_style: localTimeDisplayStyle,
        debug_mode: loadDebugMode(),
      });

      setClockDefaults({
        countdown: {
          duration_seconds: (countdown.duration_seconds as number) ?? 1500,
          loop: (countdown.loop as boolean) ?? false,
          loop_count: (countdown.loop_count as number) ?? 0,
          loop_interval_seconds: (countdown.loop_interval_seconds as number) ?? 0,
        },
        stopwatch: {
          max_seconds: (stopwatch.max_seconds as number) ?? 86400,
        },
      });

      applyTheme((basic.theme_mode as string) ?? "dark");
    } catch (e) {
      logError(`加载设置失败: ${e}`);
    }
  }, [applyTheme]);

  useEffect(() => {
    void loadSettings();
  }, [loadSettings]);

  useEffect(() => {
    applyTheme(settings.theme_mode || "dark");
  }, [settings.theme_mode, applyTheme]);

  const updateSettings = useCallback((newSettings: Partial<BasicSettings>) => {
    setSettings((prev) => ({ ...prev, ...newSettings }));
  }, []);

  const updateClockDefaults = useCallback((newDefaults: Partial<ClockDefaults>) => {
    setClockDefaults((prev) => ({
      countdown: { ...prev.countdown, ...(newDefaults.countdown || {}) },
      stopwatch: { ...prev.stopwatch, ...(newDefaults.stopwatch || {}) },
    }));
  }, []);

  const save = useCallback(async () => {
    setIsSaving(true);
    setSaveMessage("");

    const audioPrefs = normalizeAudioPreferences({
      sound_enabled: settings.sound_enabled,
      sound_tick: settings.sound_tick,
      sound_finish: settings.sound_finish,
      sound_volume: settings.sound_volume,
    });
    saveAudioPreferences(audioPrefs);
    setAudioPreferences(audioPrefs);

    localStorage.setItem(STORAGE_KEYS.LAYOUT_DENSITY, settings.layout_density || "normal");
    localStorage.setItem(STORAGE_KEYS.TIME_DISPLAY_STYLE, settings.time_display_style || "classic");

    const { sound_enabled: _sound_enabled, sound_tick: _sound_tick, sound_finish: _sound_finish, sound_volume: _sound_volume, ...serverBasic } = settings;
    const serverConfig = {
      basic: serverBasic,
      ...clockDefaults,
    };

    try {
      await apiClientRef.current.updateSettings(serverConfig);
      setSaveMessage(t("common.save_success"));
      setTimeout(() => setSaveMessage(""), 3000);
    } catch (e) {
      const errorMessage = e instanceof Error ? e.message : "未知错误";
      setSaveMessage(t("validation.save_error", { error: errorMessage }));
    } finally {
      setIsSaving(false);
    }
  }, [settings, clockDefaults]);

  const reset = useCallback(() => {
    setSettings(DEFAULT_SETTINGS);
    setClockDefaults(DEFAULT_CLOCK_DEFAULTS);
    saveAudioPreferences(DEFAULT_AUDIO_PREFERENCES);
    setAudioPreferences(DEFAULT_AUDIO_PREFERENCES);
    localStorage.setItem(STORAGE_KEYS.LAYOUT_DENSITY, "normal");
    localStorage.setItem(STORAGE_KEYS.TIME_DISPLAY_STYLE, "classic");
    setSaveMessage(t("common.save_hint"));
  }, []);

  return {
    settings,
    clockDefaults,
    audioPreferences,
    isSaving,
    saveMessage,
    updateSettings,
    updateClockDefaults,
    save,
    reset,
  };
};
