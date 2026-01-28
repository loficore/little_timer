import type { FunctionalComponent } from "preact";
import { useEffect, useState } from "preact/hooks";
import { Header } from "./components/Header";
import { TabPanel } from "./components/TabPanel";
import { BasicSettings } from "./components/BasicSettings";
import { CountdownSettings } from "./components/CountdownSettings";
import { StopwatchSettings } from "./components/StopwatchSettings";
import { PresetSettings, type TimerPreset } from "./components/PresetSettings";
import { WorldClockSettings } from "./components/WorldClockSettings";
import { t, setLanguage } from "./utils/i18n";

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
  const [config, setConfig] = useState<SettingsConfig>(DEFAULT_CONFIG);
  const [presets, setPresets] = useState<TimerPreset[]>([]);
  const [activeTab, setActiveTab] = useState("basic");
  const [isSaving, setIsSaving] = useState(false);
  const [saveMessage, setSaveMessage] = useState("");

  // 根据默认模式切换标签页，确保模式与配置联动
  useEffect(() => {
    const modeTab =
      config.basic.default_mode === "countdown"
        ? "countdown"
        : config.basic.default_mode === "stopwatch"
          ? "stopwatch"
          : "world_clock";
    setActiveTab(modeTab);
  }, [config.basic.default_mode]);

  // 应用主题
  const applyTheme = (themeMode: string = "dark") => {
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

  const loadSettings = () => {
    // 从后端加载设置
    // 后端会通过 window.updateSettingsDisplay() 回调返回设置数据
    return new Promise<void>((resolve) => {
      try {
        // 设置超时：如果 2 秒内未收到回调，使用默认配置
        const timeoutId = setTimeout(() => {
          console.warn("⚠️ 加载设置超时，使用默认配置");
          setSaveMessage(t("errors.offline.message"));
          resolve();
        }, 2000);

        // 设置全局回调，用于接收后端发送的设置数据
        (window as any).updateSettingsDisplay = (settingsJson: string) => {
          clearTimeout(timeoutId);
          try {
            const parsedConfig = JSON.parse(settingsJson) as SettingsConfig;
            setConfig(parsedConfig);
            setPresets(parsedConfig.presets || []);
            console.log("✅ 设置已加载:", parsedConfig);
          } catch (parseError) {
            console.error("❌ 解析设置 JSON 失败:", parseError);
            setSaveMessage(
              t("validation.load_error", { error: "JSON 解析失败" }),
            );
          }
          resolve();
        };

        // 调用后端的 get_settings，后端会通过 window.run() 调用上面的回调
        window.webui?.call("get_settings");
      } catch (error) {
        setSaveMessage(
          t("validation.load_error", {
            error: error instanceof Error ? error.message : "未知错误",
          }),
        );
      }
    });
  };

  // 组件挂载时加载设置
  useEffect(() => {
    loadSettings();
  }, []);

  // 主题变化时应用主题
  useEffect(() => {
    applyTheme(config.basic.theme_mode || "dark");
  }, [config.basic.theme_mode]);

  // 语言变化时加载对应语言包
  useEffect(() => {
    setLanguage(config.basic.language).catch((err) =>
      console.error("加载语言失败", err),
    );
  }, [config.basic.language]);

  const handleSave = () => {
    setIsSaving(true);
    setSaveMessage("");

    try {
      // 调用后端保存设置
      const configJson = JSON.stringify({ ...config, presets });
      window.webui?.call("change_settings", configJson);

      setTimeout(() => {
        setIsSaving(false);
        setSaveMessage(t("common.save_success"));
        setTimeout(() => setSaveMessage(""), 3000);
      }, 500);
    } catch (error) {
      setIsSaving(false);
      const errorMessage = error instanceof Error ? error.message : "未知错误";
      setSaveMessage(t("validation.save_error", { error: errorMessage }));
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
      {/* 头部 */}
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

      {/* 标签页和内容 */}
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

                // 非倒计时模式下隐藏循环配置并重置为关闭
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
                      loop_interval_seconds:
                        preset.config.loop_interval_seconds ?? 0,
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
            }}
          />
        )}
      </TabPanel>

      {/* 操作按钮 */}
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
