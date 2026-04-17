import type { FunctionalComponent, VNode } from "preact";
import { useEffect, useState, useRef, useCallback } from "preact/hooks";
import { Header } from "./components/Header";
import { TabPanel } from "./components/TabPanel";
import { BasicSettings } from "./components/BasicSettings";
import { CountdownSettings } from "./components/CountdownSettings";
import { StopwatchSettings } from "./components/StopwatchSettings";
import { t, setLanguage } from "./utils/i18n";
import { getAPIClient } from "./utils/apiClientSingleton";
import { ClockIconComponent, CheckIconComponent, ResetIcon } from "./utils/icons";
import {
  DEFAULT_AUDIO_PREFERENCES,
  loadAudioPreferences,
  normalizeAudioPreferences,
  saveAudioPreferences,
} from "./utils/audio";
import { STORAGE_KEYS } from "./utils/constants";
import { isPerfDebugEnabled, isWebViewRuntime, logPerf } from "./utils/logger";

interface SettingsPageProps {
  onBackClick?: () => void;
  wallpaper?: string;
  onWallpaperChange?: (wallpaper: string) => void;
}

interface SettingsConfig {
  basic: {
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
    light_style?: string;
  };
  clock_defaults: {
    countdown: {
      duration_seconds: number;
      loop: boolean;
      loop_count: number;
      loop_interval_seconds: number;
    };
    stopwatch: {
      max_seconds: number;
    };
  };
}

const DEFAULT_CONFIG: SettingsConfig = {
  basic: {
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
    light_style: "paper",
  },
  clock_defaults: {
    countdown: {
      duration_seconds: 1500,
      loop: false,
      loop_count: 0,
      loop_interval_seconds: 0,
    },
    stopwatch: {
      max_seconds: 86400,
    },
  },
};

const TABS: { id: string; labelKey: string; icon?: VNode }[] = [
  { id: "basic", labelKey: "settings.tabs.basic" },
  { id: "countdown", labelKey: "settings.tabs.countdown", icon: <ClockIconComponent /> },
  { id: "stopwatch", labelKey: "settings.tabs.stopwatch", icon: <ClockIconComponent /> },
];

const LIGHT_STYLE_STORAGE_KEY = "lt_light_style";
const THEME_MODE_STORAGE_KEY = "lt_theme_mode";

export const SettingsPage: FunctionalComponent<SettingsPageProps> = ({
  onBackClick,
  wallpaper,
  onWallpaperChange,
}) => {
  const apiClientRef = useRef<ReturnType<typeof getAPIClient> | null>(null);
  const [config, setConfig] = useState<SettingsConfig>(DEFAULT_CONFIG);
  const [activeTab, setActiveTab] = useState("basic");
  const [isSaving, setIsSaving] = useState(false);
  const [saveMessage, setSaveMessage] = useState("");
  const animationsEnabled = !isWebViewRuntime();
  const pendingInteractionLabelRef = useRef<string | null>(null);
  const pendingInteractionStartRef = useRef<number>(0);

  const markInteraction = useCallback((label: string) => {
    if (!isPerfDebugEnabled()) return;
    pendingInteractionLabelRef.current = label;
    pendingInteractionStartRef.current = performance.now();
  }, []);

  const handleTabChange = useCallback((tabId: string) => {
    markInteraction(`tab.change:${tabId}`);
    setActiveTab(tabId);
  }, [markInteraction]);

  useEffect(() => {
    apiClientRef.current = getAPIClient();
  }, []);

  useEffect(() => {
    const modeTab = config.basic.default_mode === "countdown" ? "countdown" : "stopwatch";
    setActiveTab(modeTab);
  }, [config.basic.default_mode]);

  const applyTheme = (themeMode = "dark") => {
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
  };

  const applyLightStyle = (lightStyle = "paper") => {
    const html = document.documentElement;
    html.classList.remove("light-style-mist");
    if (lightStyle === "mist") {
      html.classList.add("light-style-mist");
    }
  };

  const loadSettings = async () => {
    const startAt = performance.now();
    if (!apiClientRef.current) {
      setSaveMessage(t("errors.offline.message"));
      return;
    }

    try {
      const apiStartAt = performance.now();
      const settings = await apiClientRef.current.getSettings();
      const apiDurationMs = Math.round(performance.now() - apiStartAt);

      const normalizeStartAt = performance.now();
      const s = settings as any;
      const localAudioPreferences = loadAudioPreferences();
      const audioPreferences = normalizeAudioPreferences({
        sound_enabled: s?.basic?.sound_enabled ?? localAudioPreferences.sound_enabled,
        sound_tick: s?.basic?.sound_tick ?? localAudioPreferences.sound_tick,
        sound_finish: s?.basic?.sound_finish ?? localAudioPreferences.sound_finish,
        sound_volume: s?.basic?.sound_volume ?? localAudioPreferences.sound_volume,
      });
      saveAudioPreferences(audioPreferences);
      
      const localLayoutDensity = localStorage.getItem(STORAGE_KEYS.LAYOUT_DENSITY) || "normal";
      const localTimeDisplayStyle = localStorage.getItem(STORAGE_KEYS.TIME_DISPLAY_STYLE) || "classic";
      const localLightStyle = localStorage.getItem(LIGHT_STYLE_STORAGE_KEY) || "paper";
      
      const loadedConfig: SettingsConfig = {
        basic: {
          timezone: s?.basic?.timezone ?? 8,
          language: s?.basic?.language ?? "ZH",
          default_mode: s?.basic?.default_mode ?? "countdown",
          theme_mode: s?.basic?.theme_mode ?? "dark",
          wallpaper: s?.basic?.wallpaper ?? "",
          sound_enabled: audioPreferences.sound_enabled,
          sound_tick: audioPreferences.sound_tick,
          sound_finish: audioPreferences.sound_finish,
          sound_volume: audioPreferences.sound_volume,
          layout_density: localLayoutDensity,
          time_display_style: localTimeDisplayStyle,
          light_style: localLightStyle,
        },
        clock_defaults: {
          countdown: s?.countdown ? {
            duration_seconds: s.countdown?.duration_seconds ?? 1500,
            loop: s.countdown?.loop ?? false,
            loop_count: s.countdown?.loop_count ?? 0,
            loop_interval_seconds: s.countdown?.loop_interval_seconds ?? 0,
          } : DEFAULT_CONFIG.clock_defaults.countdown,
          stopwatch: s?.stopwatch ? {
            max_seconds: s.stopwatch?.max_seconds ?? 86400,
          } : DEFAULT_CONFIG.clock_defaults.stopwatch,
        },
      };
      const normalizeDurationMs = Math.round(performance.now() - normalizeStartAt);
      
      const setStateStartAt = performance.now();
      setConfig(loadedConfig);
      logPerf("Settings.load.success", {
        durationMs: Math.round(performance.now() - startAt),
        apiDurationMs,
        normalizeDurationMs,
        setStateScheduleMs: Math.round(performance.now() - setStateStartAt),
        defaultMode: loadedConfig.basic.default_mode,
        language: loadedConfig.basic.language,
      });
    } catch (error) {
      console.error("加载设置失败:", error);
      setSaveMessage(t("errors.offline.message"));
      logPerf("Settings.load.error", {
        durationMs: Math.round(performance.now() - startAt),
      });
    }
  };

  useEffect(() => {
    void loadSettings();
  }, []);

  useEffect(() => {
    const startAt = performance.now();
    const themeMode = config.basic.theme_mode || "dark";
    applyTheme(themeMode);
    localStorage.setItem(THEME_MODE_STORAGE_KEY, themeMode);
    logPerf("Settings.theme.applied", {
      themeMode,
      durationMs: Math.round(performance.now() - startAt),
    });
  }, [config.basic.theme_mode]);

  useEffect(() => {
    const startAt = performance.now();
    applyLightStyle(config.basic.light_style || "paper");
    logPerf("Settings.lightStyle.applied", {
      lightStyle: config.basic.light_style || "paper",
      durationMs: Math.round(performance.now() - startAt),
    });
  }, [config.basic.light_style]);

  useEffect(() => {
    const startAt = performance.now();
    setLanguage(config.basic.language).catch((err) =>
      console.error("加载语言失败", err),
    );
    logPerf("Settings.language.changed", {
      language: config.basic.language,
      scheduleMs: Math.round(performance.now() - startAt),
    });
  }, [config.basic.language]);

  useEffect(() => {
    onWallpaperChange?.(config.basic.wallpaper || "");
  }, [config.basic.wallpaper, onWallpaperChange]);

  const handleSave = () => {
    const startAt = performance.now();
    setIsSaving(true);
    setSaveMessage("");

    const audioPreferences = normalizeAudioPreferences({
      sound_enabled: config.basic.sound_enabled,
      sound_tick: config.basic.sound_tick,
      sound_finish: config.basic.sound_finish,
      sound_volume: config.basic.sound_volume,
    });
    saveAudioPreferences(audioPreferences);
    
    // 保存布局密度到localStorage
    localStorage.setItem(STORAGE_KEYS.LAYOUT_DENSITY, config.basic.layout_density || "normal");
    localStorage.setItem(STORAGE_KEYS.TIME_DISPLAY_STYLE, config.basic.time_display_style || "classic");
    localStorage.setItem(LIGHT_STYLE_STORAGE_KEY, config.basic.light_style || "paper");

    const {
      sound_enabled,
      sound_tick,
      sound_finish,
      sound_volume,
      layout_density,
      time_display_style,
      light_style,
      ...serverBasic
    } =
      config.basic;
    const serverConfig = {
      ...config,
      basic: serverBasic,
    };

    void sound_enabled;
    void sound_tick;
    void sound_finish;
    void sound_volume;
    void layout_density;
    void time_display_style;
    void light_style;

    if (apiClientRef.current) {
      void apiClientRef.current.updateSettings(serverConfig)
        .then(() => {
          setSaveMessage(t("common.save_success"));
          setTimeout(() => setSaveMessage(""), 3000);
          logPerf("Settings.save.success", {
            durationMs: Math.round(performance.now() - startAt),
          });
        })
        .catch((error) => {
          const errorMessage = error instanceof Error ? error.message : "未知错误";
          setSaveMessage(t("validation.save_error", { error: errorMessage }));
          logPerf("Settings.save.error", {
            durationMs: Math.round(performance.now() - startAt),
            error: errorMessage,
          });
        })
        .finally(() => {
          setIsSaving(false);
        });
    }
  };

  const handleReset = () => {
    if (confirm(t("common.reset_confirm"))) {
      const startAt = performance.now();
      setConfig(DEFAULT_CONFIG);
      saveAudioPreferences(DEFAULT_AUDIO_PREFERENCES);
      localStorage.setItem(STORAGE_KEYS.LAYOUT_DENSITY, "normal");
      localStorage.setItem(STORAGE_KEYS.TIME_DISPLAY_STYLE, "classic");
      localStorage.setItem(LIGHT_STYLE_STORAGE_KEY, "paper");
      setSaveMessage(t("common.save_hint"));
      logPerf("Settings.reset", {
        durationMs: Math.round(performance.now() - startAt),
      });
    }
  };

  useEffect(() => {
    if (!isPerfDebugEnabled()) return;
    logPerf("Settings.tab.changed", {
      activeTab,
      defaultMode: config.basic.default_mode,
    });
  }, [activeTab, config.basic.default_mode]);

  useEffect(() => {
    if (!isPerfDebugEnabled()) return;
    if (!pendingInteractionLabelRef.current) return;

    const interactionLabel = pendingInteractionLabelRef.current;
    const interactionStartAt = pendingInteractionStartRef.current;
    pendingInteractionLabelRef.current = null;

    requestAnimationFrame(() => {
      logPerf("Settings.interaction.frame", {
        label: interactionLabel,
        durationMs: Math.round(performance.now() - interactionStartAt),
        activeTab,
      });
    });
  }, [activeTab, config]);

  useEffect(() => {
    if (!isPerfDebugEnabled()) return;
    if (typeof PerformanceObserver === "undefined") return;

    const observer = new PerformanceObserver((list) => {
      const entries = list.getEntries();
      for (const entry of entries) {
        logPerf("Settings.longtask", {
          name: entry.name,
          durationMs: Math.round(entry.duration),
          activeTab,
        });
      }
    });

    try {
      observer.observe({ type: "longtask", buffered: true } as PerformanceObserverInit);
    } catch {
      // 部分 WebView 不支持 longtask，忽略即可。
    }

    return () => {
      observer.disconnect();
    };
  }, [activeTab]);

  return (
    <div
      className={`flex flex-col flex-1 text-base-content transition-colors duration-300 overflow-hidden bg-transparent ${
        animationsEnabled ? "animate-fadeIn" : ""
      }`}
    >
      <div
        className="flex flex-col w-full h-full"
        style={(config.basic.wallpaper || wallpaper) ? { backgroundColor: "rgba(0,0,0,0.15)" } : {}}
      >
        <Header
          title={t("common.settings_title")}
          showBack={true}
          onBackClick={onBackClick}
          showSettings={false}
        />

        <TabPanel
          tabs={TABS.map((tab) => ({ ...tab, label: t(tab.labelKey) }))}
          activeTab={activeTab}
          onTabChange={handleTabChange}
          isAnimated={animationsEnabled}
        >
          {activeTab === "basic" && (
            <BasicSettings
              config={config.basic}
              isAnimated={animationsEnabled}
              onChange={(newBasic) => {
                markInteraction("basic.config.change");
                setConfig((prev) => {
                  const next = { ...prev, basic: newBasic };

                  if (newBasic.default_mode !== "countdown") {
                    next.clock_defaults = {
                      ...prev.clock_defaults,
                      countdown: {
                        ...prev.clock_defaults.countdown,
                        loop: false,
                        loop_count: 0,
                        loop_interval_seconds: 0,
                      },
                    };
                  }

                  return next;
                });
              }}
            />
          )}

          {activeTab === "countdown" && (
            <CountdownSettings
              config={config.clock_defaults.countdown}
              showLoopControls={config.basic.default_mode === "countdown"}
              isAnimated={animationsEnabled}
              onChange={(newCountdown) => {
                markInteraction("countdown.config.change");
                setConfig({
                  ...config,
                  clock_defaults: {
                    ...config.clock_defaults,
                    countdown: newCountdown,
                  },
                });
              }}
            />
          )}

          {activeTab === "stopwatch" && (
            <StopwatchSettings
              config={config.clock_defaults.stopwatch}
              isAnimated={animationsEnabled}
              onChange={(newStopwatch) => {
                markInteraction("stopwatch.config.change");
                setConfig({
                  ...config,
                  clock_defaults: {
                    ...config.clock_defaults,
                    stopwatch: newStopwatch,
                  },
                });
              }}
            />
          )}
        </TabPanel>

        <div
          className={`my-surface-panel flex gap-2 sm:gap-3 md:gap-4 items-center justify-center px-4 sm:px-6 md:px-8 py-4 sm:py-6 md:py-8 flex-wrap flex-shrink-0 ${
            animationsEnabled ? "animate-slideUp" : ""
          }`}
          style={animationsEnabled ? { animationDelay: "0.3s", animationFillMode: "both" } : undefined}
        >
          <button
            onClick={handleSave}
            disabled={isSaving}
            className="btn-primary inline-flex items-center gap-2"
          >
            {!isSaving && <CheckIconComponent />}
            {isSaving ? t("common.saving") : t("common.save")}
          </button>
          <button onClick={handleReset} className="my-btn-secondary inline-flex items-center gap-2">
            <ResetIcon />
            {t("common.reset_default")}
          </button>
          {saveMessage && (
            <div className="px-4 sm:px-5 py-2 rounded-lg text-xs sm:text-sm font-medium bg-secondary-dark text-accent-dark animate-pulse">
              {saveMessage}
            </div>
          )}
        </div>
      </div>
    </div>
  );
};
