import type { FunctionalComponent } from "preact";
import { useEffect, useState, useRef } from "preact/hooks";
import { Header } from "./components/Header";
import { TabPanel } from "./components/TabPanel";
import { BasicSettings } from "./components/BasicSettings";
import { CountdownSettings } from "./components/CountdownSettings";
import { StopwatchSettings } from "./components/StopwatchSettings";
import { PresetSettings, type TimerPreset } from "./components/PresetSettings";
import { WorldClockSettings } from "./components/WorldClockSettings";
import { t, setLanguage } from "./utils/i18n";
import { APIClient } from "./utils/apiClient";

interface SettingsPageProps {
  onBackClick?: () => void;
}

interface SettingsConfig {
  basic: {
    timezone: number;
    language: string;
    default_mode: string;
    theme_mode: string;
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
  presets?: TimerPreset[];
}

const DEFAULT_CONFIG: SettingsConfig = {
  basic: {
    timezone: 8,
    language: "ZH",
    default_mode: "countdown",
    theme_mode: "dark",
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
  presets: [],
};

const TABS = [
  { id: "basic", labelKey: "settings.tabs.basic", icon: "⚙️" },
  { id: "countdown", labelKey: "settings.tabs.countdown", icon: "⏱️" },
  { id: "stopwatch", labelKey: "settings.tabs.stopwatch", icon: "⏲️" },
  { id: "world_clock", labelKey: "settings.tabs.world_clock", icon: "🌐" },
  { id: "presets", labelKey: "settings.tabs.presets", icon: "⭐" },
];

export const SettingsPage: FunctionalComponent<SettingsPageProps> = ({
  onBackClick,
}) => {
  const apiClientRef = useRef<APIClient | null>(null);
  const [config, setConfig] = useState<SettingsConfig>(DEFAULT_CONFIG);
  const [presets, setPresets] = useState<TimerPreset[]>([]);
  const [activeTab, setActiveTab] = useState("basic");
  const [isSaving, setIsSaving] = useState(false);
  const [saveMessage, setSaveMessage] = useState("");

  useEffect(() => {
    apiClientRef.current = new APIClient(window.location.origin);
  }, []);

  useEffect(() => {
    const modeTab =
      config.basic.default_mode === "countdown"
        ? "countdown"
        : config.basic.default_mode === "stopwatch"
          ? "stopwatch"
          : "world_clock";
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

  const loadSettings = async () => {
    if (!apiClientRef.current) {
      setSaveMessage(t("errors.offline.message"));
      return;
    }

    try {
      const settings = await apiClientRef.current.getSettings();
      
      const loadedConfig: SettingsConfig = {
        basic: {
          timezone: settings.basic.timezone,
          language: settings.basic.language,
          default_mode: settings.basic.default_mode,
          theme_mode: settings.basic.theme_mode,
        },
        clock_defaults: {
          countdown: settings.countdown as typeof DEFAULT_CONFIG.clock_defaults.countdown,
          stopwatch: settings.stopwatch as typeof DEFAULT_CONFIG.clock_defaults.stopwatch,
        },
        presets: [],
      };
      
      setConfig(loadedConfig);

      const loadedPresets = await apiClientRef.current.getPresets();
      if (Array.isArray(loadedPresets)) {
        setPresets(loadedPresets as TimerPreset[]);
      }
    } catch (error) {
      console.error("加载设置失败:", error);
      setSaveMessage(t("errors.offline.message"));
    }
  };

  useEffect(() => {
    void loadSettings();
  }, []);

  useEffect(() => {
    applyTheme(config.basic.theme_mode || "dark");
  }, [config.basic.theme_mode]);

  useEffect(() => {
    setLanguage(config.basic.language).catch((err) =>
      console.error("加载语言失败", err),
    );
  }, [config.basic.language]);

  useEffect(() => {
    if (presets.length === 0 && config.presets && config.presets.length > 0) {
      setPresets([...config.presets]);
    }
  }, [config]);

  const handleSave = () => {
    setIsSaving(true);
    setSaveMessage("");

    const configWithPresets = { ...config, presets: [...presets] };
    
    if (apiClientRef.current) {
      void apiClientRef.current.updateSettings(configWithPresets)
        .then(() => {
          setSaveMessage(t("common.save_success"));
          setTimeout(() => setSaveMessage(""), 3000);
        })
        .catch((error) => {
          const errorMessage = error instanceof Error ? error.message : "未知错误";
          setSaveMessage(t("validation.save_error", { error: errorMessage }));
        })
        .finally(() => {
          setIsSaving(false);
        });
    }
  };

  const handleReset = () => {
    if (confirm(t("common.reset_confirm"))) {
      setConfig(DEFAULT_CONFIG);
      setPresets([]);
      setSaveMessage(t("common.save_hint"));
    }
  };

  return (
    <div className="flex flex-col w-screen h-screen bg-primary-dark text-text-primary-dark transition-colors duration-300 animate-fadeIn overflow-hidden">
      <div
        className="flex justify-between items-center px-4 sm:px-6 md:px-8 py-3 sm:py-4 md:py-6 border-b border-border-dark animate-slideUp flex-shrink-0"
        style={{ animationDelay: "0.1s", animationFillMode: "both" }}
      >
        <Header
          title={`⚙ ${t("common.settings_title")}`}
          showBack={true}
          onBackClick={onBackClick}
        />
      </div>

      <TabPanel
        tabs={TABS.map((tab) => ({ ...tab, label: t(tab.labelKey) }))}
        activeTab={activeTab}
        onTabChange={setActiveTab}
        isAnimated={true}
      >
        {activeTab === "basic" && (
          <BasicSettings
            config={config.basic}
            onChange={(newBasic) => {
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
            onChange={(newCountdown) =>
              setConfig({
                ...config,
                clock_defaults: {
                  ...config.clock_defaults,
                  countdown: newCountdown,
                },
              })
            }
          />
        )}

        {activeTab === "stopwatch" && (
          <StopwatchSettings
            config={config.clock_defaults.stopwatch}
            onChange={(newStopwatch) =>
              setConfig({
                ...config,
                clock_defaults: {
                  ...config.clock_defaults,
                  stopwatch: newStopwatch,
                },
              })
            }
          />
        )}

        {activeTab === "world_clock" && (
          <WorldClockSettings
            timezone={config.basic.timezone}
            onTimezoneChange={(tz) =>
              setConfig({
                ...config,
                basic: { ...config.basic, timezone: tz },
              })
            }
          />
        )}

        {activeTab === "presets" && (
          <PresetSettings
            presets={presets}
            onChange={setPresets}
            onUsePreset={(preset) => {
              if (
                preset.mode === "countdown" &&
                preset.config.duration_seconds
              ) {
                setConfig({
                  ...config,
                  basic: { ...config.basic, default_mode: "countdown" },
                  clock_defaults: {
                    ...config.clock_defaults,
                    countdown: {
                      duration_seconds: preset.config.duration_seconds,
                      loop: !!preset.config.loop,
                      loop_count: preset.config.loop_count ?? 0,
                      loop_interval_seconds: preset.config.loop_interval_seconds ?? 0,
                    },
                  },
                });
              } else if (
                preset.mode === "stopwatch" &&
                preset.config.max_seconds
              ) {
                setConfig({
                  ...config,
                  basic: { ...config.basic, default_mode: "stopwatch" },
                  clock_defaults: {
                    ...config.clock_defaults,
                    stopwatch: {
                      max_seconds: preset.config.max_seconds,
                    },
                  },
                });
              } else if (preset.mode === "world_clock") {
                setConfig({
                  ...config,
                  basic: {
                    ...config.basic,
                    default_mode: "world_clock",
                    timezone: preset.config.timezone ?? config.basic.timezone,
                  },
                });
              }
              setSaveMessage(
                t("settings.presets.applied", { name: preset.name }),
              );
              setTimeout(() => setSaveMessage(""), 3000);
            }}
          />
        )}
      </TabPanel>

      <div
        className="flex gap-2 sm:gap-3 md:gap-4 items-center justify-center px-4 sm:px-6 md:px-8 py-4 sm:py-6 md:py-8 border-t border-border-dark bg-primary-dark flex-wrap animate-slideUp flex-shrink-0"
        style={{ animationDelay: "0.3s", animationFillMode: "both" }}
      >
        <button
          onClick={handleSave}
          disabled={isSaving}
          className="btn-primary"
        >
          {isSaving ? t("common.saving") : `💾 ${t("common.save")}`}
        </button>
        <button onClick={handleReset} className="btn-secondary">
          {`🔄 ${t("common.reset_default")}`}
        </button>
        {saveMessage && (
          <div className="px-4 sm:px-5 py-2 rounded-lg text-xs sm:text-sm font-medium bg-secondary-dark text-accent-dark animate-pulse">
            {saveMessage}
          </div>
        )}
      </div>
    </div>
  );
};
