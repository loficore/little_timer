import type { FunctionalComponent } from "preact";
import { useState } from "preact/hooks";
import { SettingItem } from "./SettingItem";
import { TimeInput } from "./TimeInput";
import { NumberInput } from "./NumberInput";
import { CheckboxInput } from "./CheckboxInput";
import { SelectInput } from "./SelectInput";
import { t } from "../utils/i18n";

export interface TimerPreset {
  name: string;
  mode: "countdown" | "stopwatch" | "world_clock";
  config: {
    duration_seconds?: number;
    loop?: boolean;
    loop_count?: number;
    loop_interval_seconds?: number;
    max_seconds?: number;
    timezone?: number;
  };
}

interface PresetSettingsProps {
  presets: TimerPreset[];
  onChange: (presets: TimerPreset[]) => void;
  onUsePreset?: (preset: TimerPreset) => void;
}

export const PresetSettings: FunctionalComponent<PresetSettingsProps> = ({
  presets,
  onChange,
  onUsePreset,
}) => {
  const [isAdding, setIsAdding] = useState(false);
  const [newPresetName, setNewPresetName] = useState("");
  const [newPresetMode, setNewPresetMode] = useState<
    "countdown" | "stopwatch" | "world_clock"
  >("countdown");
  const [formError, setFormError] = useState("");
  const [newPresetDurationSeconds, setNewPresetDurationSeconds] = useState(1500);
  const [newPresetLoop, setNewPresetLoop] = useState(false);
  const [newPresetLoopCount, setNewPresetLoopCount] = useState(0);
  const [newPresetLoopIntervalSeconds, setNewPresetLoopIntervalSeconds] =
    useState(0);
  const [newPresetMaxSeconds, setNewPresetMaxSeconds] = useState(86400);
  const [newPresetTimezone, setNewPresetTimezone] = useState(8);

  // 复用时区选项
  const timezones = Array.from({ length: 27 }, (_, i) => i - 12).map((tz) => ({
    value: tz,
    label: `UTC${tz >= 0 ? "+" : ""}${tz}`,
  }));

  const handleAddPreset = () => {
    if (!newPresetName.trim()) {
      setFormError(t("validation.preset_name_required"));
      return;
    }

    // 检查名称是否已存在
    if (presets.some((p) => p.name === newPresetName.trim())) {
      setFormError(t("validation.preset_name_exists"));
      return;
    }

    const newPreset: TimerPreset = {
      name: newPresetName.trim(),
      mode: newPresetMode,
      config:
        newPresetMode === "countdown"
          ? {
              duration_seconds: newPresetDurationSeconds,
              loop: newPresetLoop,
              loop_count: newPresetLoopCount,
              loop_interval_seconds: newPresetLoopIntervalSeconds,
            }
          : newPresetMode === "stopwatch"
            ? {
                max_seconds: newPresetMaxSeconds,
              }
            : {
                timezone: newPresetTimezone,
              },
    };

    onChange([...presets, newPreset]);

    // 重置表单
    setNewPresetName("");
    setFormError("");
    setNewPresetDurationSeconds(1500);
    setNewPresetLoop(false);
    setNewPresetLoopCount(0);
    setNewPresetLoopIntervalSeconds(0);
    setNewPresetMaxSeconds(86400);
    setNewPresetTimezone(8);
    setIsAdding(false);
  };

  const handleDeletePreset = (index: number) => {
    if (
      confirm(
        t("settings.presets.confirm_delete", { name: presets[index].name }),
      )
    ) {
      const newPresets = presets.filter((_, i) => i !== index);
      onChange(newPresets);
    }
  };

  const handleUsePreset = (preset: TimerPreset) => {
    if (onUsePreset) {
      onUsePreset(preset);
    }
  };

  const formatPresetInfo = (preset: TimerPreset): string => {
    if (preset.mode === "countdown") {
      const minutes = Math.floor((preset.config.duration_seconds || 0) / 60);
      const loopInfo = preset.config.loop
        ? ` | ${t("settings.countdown.loop_count")} ${
            preset.config.loop_count || "∞"
          }`
        : "";
      return `${minutes} ${t("settings.countdown.duration_minutes")} ${loopInfo}`;
    } else if (preset.mode === "stopwatch") {
      const hours = Math.floor((preset.config.max_seconds || 0) / 3600);
      return `${t("settings.stopwatch.max_hours")}: ${hours}`;
    } else {
      const tz = preset.config.timezone ?? 8;
      return `${t("settings.world_clock.timezone")}: UTC${tz >= 0 ? "+" : ""}${tz}`;
    }
  };

  return (
    <div
      className="space-y-4 sm:space-y-6 animate-slideUp"
      style={{ animationDelay: "0.3s", animationFillMode: "both" }}
    >
      {/* 预设列表 */}
      <div className="space-y-3">
        <h3 className="text-xs sm:text-sm font-semibold text-text-secondary-dark uppercase tracking-wider">
          {t("settings.presets.list_count", { count: presets.length })}
        </h3>

        {presets.length === 0 ? (
          <div className="text-center py-8 text-text-secondary-dark">
            <p className="mb-2">{t("settings.presets.none_title")}</p>
            <p className="text-xs">{t("settings.presets.none_desc")}</p>
          </div>
        ) : (
          <div className="space-y-2">
            {presets.map((preset, index) => (
              <div
                key={index}
                className="flex items-center justify-between p-4 rounded-lg bg-secondary-dark border border-border-dark hover:border-accent-dark transition-all"
              >
                <div className="flex-1">
                  <div className="flex items-center gap-2 mb-1">
                    <span className="font-medium text-text-primary-dark">
                      {preset.name}
                    </span>
                    <span className="px-2 py-1 text-xs rounded bg-tertiary-dark text-text-secondary-dark">
                      {preset.mode === "countdown"
                        ? t("settings.presets.saved_badge_countdown")
                        : preset.mode === "stopwatch"
                          ? t("settings.presets.saved_badge_stopwatch")
                          : t("settings.tabs.world_clock")}
                    </span>
                  </div>
                  <p className="text-xs text-text-secondary-dark">
                    {formatPresetInfo(preset)}
                  </p>
                </div>
                <div className="flex gap-2">
                  <button
                    onClick={() => handleUsePreset(preset)}
                    className="px-3 py-2 text-sm rounded-lg bg-accent-dark text-white hover:opacity-80 transition-all"
                    title={t("settings.presets.use")}
                  >
                    {t("settings.presets.use")}
                  </button>
                  <button
                    onClick={() => handleDeletePreset(index)}
                    className="px-3 py-2 text-sm rounded-lg bg-transparent border border-border-dark text-text-secondary-dark hover:border-red-500 hover:text-red-500 transition-all"
                    title={t("settings.presets.delete")}
                  >
                    {t("settings.presets.delete")}
                  </button>
                </div>
              </div>
            ))}
          </div>
        )}
      </div>

      {/* 添加预设按钮/表单 */}
      {!isAdding ? (
        <button
          onClick={() => setIsAdding(true)}
          disabled={presets.length >= 10}
          className="w-full py-3 rounded-lg border-2 border-dashed border-border-dark text-text-secondary-dark hover:border-accent-dark hover:text-accent-dark transition-all disabled:opacity-50 disabled:cursor-not-allowed"
        >
          {t("settings.presets.add_new")}
        </button>
      ) : (
        <div className="p-4 rounded-lg bg-secondary-dark border border-accent-dark space-y-4">
          <h4 className="font-semibold text-text-primary-dark">
            {t("settings.presets.new_title")}
          </h4>

          {/* 预设名称 */}
          <SettingItem label={t("settings.presets.name_label")}>
            <input
              type="text"
              value={newPresetName}
              onChange={(e) => {
                setNewPresetName((e.target as HTMLInputElement).value);
                setFormError("");
              }}
              placeholder={t("settings.presets.name_placeholder")}
              maxLength={20}
              className="w-full px-3 py-2 rounded-lg bg-primary-dark border border-border-dark text-text-primary-dark focus:border-accent-dark outline-none"
            />
          </SettingItem>
          {formError && (
            <div className="text-xs text-red-500 font-medium">{formError}</div>
          )}

          {/* 模式选择 */}
          <SettingItem label={t("settings.presets.mode_label")}>
            <SelectInput
              value={newPresetMode}
              options={[
                {
                  value: "countdown",
                  label: t("settings.presets.saved_badge_countdown"),
                },
                {
                  value: "stopwatch",
                  label: t("settings.presets.saved_badge_stopwatch"),
                },
                {
                  value: "world_clock",
                  label: t("settings.tabs.world_clock"),
                },
              ]}
              onChange={(value) =>
                setNewPresetMode(
                  value as "countdown" | "stopwatch" | "world_clock",
                )
              }
            />
          </SettingItem>

          {/* 倒计时配置 */}
          {newPresetMode === "countdown" && (
            <>
              <SettingItem label={t("settings.presets.duration_label")}>
                <TimeInput
                  value={newPresetDurationSeconds}
                  maxHours={24}
                  onChange={(value) => setNewPresetDurationSeconds(value)}
                  hint={t("settings.countdown.duration_hint", {
                    minutes: Math.floor(newPresetDurationSeconds / 60),
                  })}
                />
              </SettingItem>

              <SettingItem label={t("settings.presets.loop_label")}>
                <CheckboxInput
                  value={newPresetLoop}
                  onChange={(checked) => setNewPresetLoop(checked)}
                  label={t("settings.countdown.loop_enable")}
                />
              </SettingItem>

              {newPresetLoop && (
                <>
                  <SettingItem label={t("settings.presets.loop_count_label")}>
                    <NumberInput
                      value={newPresetLoopCount}
                      min={0}
                      max={100}
                      onChange={(value) => setNewPresetLoopCount(value || 0)}
                      hint={t("settings.countdown.loop_count_hint")}
                    />
                  </SettingItem>

                  <SettingItem
                    label={t("settings.presets.loop_interval_label")}
                  >
                    <TimeInput
                      value={newPresetLoopIntervalSeconds}
                      maxHours={1}
                      showHours={false}
                      onChange={(value) =>
                        setNewPresetLoopIntervalSeconds(value)
                      }
                      hint={t("settings.countdown.loop_interval_hint")}
                    />
                  </SettingItem>
                </>
              )}
            </>
          )}

          {/* 世界时钟配置 */}
          {newPresetMode === "world_clock" && (
            <SettingItem label={t("settings.world_clock.timezone")}>
              <SelectInput
                value={newPresetTimezone}
                options={timezones}
                onChange={(value) => setNewPresetTimezone(parseInt(value, 10))}
              />
            </SettingItem>
          )}

          {/* 正计时配置 */}
          {newPresetMode === "stopwatch" && (
            <SettingItem label={t("settings.presets.max_hours_label")}>
              <TimeInput
                value={newPresetMaxSeconds}
                maxHours={168}
                onChange={(value) => setNewPresetMaxSeconds(value)}
                hint={t("settings.stopwatch.max_hours_hint", {
                  hours: Math.floor(newPresetMaxSeconds / 3600),
                })}
              />
            </SettingItem>
          )}

          {/* 操作按钮 */}
          <div className="flex gap-2 pt-2">
            <button
              onClick={handleAddPreset}
              className="flex-1 py-2 rounded-lg bg-accent-dark text-white hover:opacity-80 transition-all"
            >
              {t("settings.presets.save_preset")}
            </button>
            <button
              onClick={() => setIsAdding(false)}
              className="flex-1 py-2 rounded-lg bg-transparent border border-border-dark text-text-secondary-dark hover:border-accent-dark transition-all"
            >
              {t("settings.presets.cancel")}
            </button>
          </div>
        </div>
      )}
    </div>
  );
};
