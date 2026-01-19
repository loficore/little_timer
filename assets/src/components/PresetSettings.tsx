import type { FunctionalComponent } from "preact";
import { useState } from "preact/hooks";
import { SettingItem } from "./SettingItem";
import { NumberInput } from "./NumberInput";
import { CheckboxInput } from "./CheckboxInput";
import { SelectInput } from "./SelectInput";
import { t } from "../utils/i18n";

export interface TimerPreset {
  name: string;
  mode: "countdown" | "stopwatch";
  config: {
    duration_seconds?: number;
    loop?: boolean;
    loop_count?: number;
    loop_interval_seconds?: number;
    max_seconds?: number;
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
  const [newPresetMode, setNewPresetMode] = useState<"countdown" | "stopwatch">(
    "countdown",
  );
  const [newPresetDurationMinutes, setNewPresetDurationMinutes] = useState(25);
  const [newPresetLoop, setNewPresetLoop] = useState(false);
  const [newPresetLoopCount, setNewPresetLoopCount] = useState(0);
  const [newPresetLoopIntervalSeconds, setNewPresetLoopIntervalSeconds] =
    useState(0);
  const [newPresetMaxHours, setNewPresetMaxHours] = useState(24);

  const handleAddPreset = () => {
    if (!newPresetName.trim()) {
      alert(t("validation.preset_name_required"));
      return;
    }

    // 检查名称是否已存在
    if (presets.some((p) => p.name === newPresetName.trim())) {
      alert(t("validation.preset_name_exists"));
      return;
    }

    const newPreset: TimerPreset = {
      name: newPresetName.trim(),
      mode: newPresetMode,
      config:
        newPresetMode === "countdown"
          ? {
              duration_seconds: newPresetDurationMinutes * 60,
              loop: newPresetLoop,
              loop_count: newPresetLoopCount,
              loop_interval_seconds: newPresetLoopIntervalSeconds,
            }
          : {
              max_seconds: newPresetMaxHours * 3600,
            },
    };

    onChange([...presets, newPreset]);

    // 重置表单
    setNewPresetName("");
    setNewPresetDurationMinutes(25);
    setNewPresetLoop(false);
    setNewPresetLoopCount(0);
    setNewPresetLoopIntervalSeconds(0);
    setNewPresetMaxHours(24);
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
    } else {
      const hours = Math.floor((preset.config.max_seconds || 0) / 3600);
      return `${t("settings.stopwatch.max_hours")}: ${hours}`;
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
                        : t("settings.presets.saved_badge_stopwatch")}
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
              onChange={(e) =>
                setNewPresetName((e.target as HTMLInputElement).value)
              }
              placeholder={t("settings.presets.name_placeholder")}
              maxLength={20}
              className="w-full px-3 py-2 rounded-lg bg-primary-dark border border-border-dark text-text-primary-dark focus:border-accent-dark outline-none"
            />
          </SettingItem>

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
              ]}
              onChange={(value) =>
                setNewPresetMode(value as "countdown" | "stopwatch")
              }
            />
          </SettingItem>

          {/* 倒计时配置 */}
          {newPresetMode === "countdown" && (
            <>
              <SettingItem label={t("settings.presets.duration_label")}>
                <NumberInput
                  value={newPresetDurationMinutes}
                  min={1}
                  max={1440}
                  onChange={(value) => setNewPresetDurationMinutes(value || 1)}
                  hint={t("settings.countdown.duration_hint", {
                    minutes: newPresetDurationMinutes,
                  })}
                />
              </SettingItem>

              <SettingItem label={t("settings.presets.loop_label")}>
                <CheckboxInput
                  checked={newPresetLoop}
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
                    <NumberInput
                      value={newPresetLoopIntervalSeconds}
                      min={0}
                      max={3600}
                      onChange={(value) =>
                        setNewPresetLoopIntervalSeconds(value || 0)
                      }
                      hint={t("settings.countdown.loop_interval_hint")}
                    />
                  </SettingItem>
                </>
              )}
            </>
          )}

          {/* 正计时配置 */}
          {newPresetMode === "stopwatch" && (
            <SettingItem label={t("settings.presets.max_hours_label")}>
              <NumberInput
                value={newPresetMaxHours}
                min={1}
                max={168}
                onChange={(value) => setNewPresetMaxHours(value || 1)}
                hint={t("settings.stopwatch.max_hours_hint", {
                  hours: newPresetMaxHours,
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
