import { SettingItem } from "./SettingItem";
import { SelectInput } from "./SelectInput";
import { WallpaperSelector } from "./WallpaperSelector";
import { t } from "../utils/i18n";

interface BasicSettingsProps {
  config: {
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
  };
  onChange: (config: any) => void;
}

export const BasicSettings = ({ config, onChange }: BasicSettingsProps) => {
  const timezones = Array.from({ length: 27 }, (_, i) => i - 12).map((tz) => ({
    value: tz,
    label: `UTC${tz >= 0 ? "+" : ""}${tz}`,
  }));

  const languages = [
    { value: "ZH", label: t("settings.basic.lang_zh") },
    { value: "EN", label: t("settings.basic.lang_en") },
    { value: "JP", label: t("settings.basic.lang_jp") },
  ];

  const modes = [
    { value: "countdown", label: t("settings.basic.mode_countdown") },
    { value: "stopwatch", label: t("settings.basic.mode_stopwatch") },
    { value: "world_clock", label: t("settings.basic.mode_world_clock") },
  ];

  const themeOptions = [
    { value: "auto", label: t("settings.basic.theme_auto") },
    { value: "light", label: t("settings.basic.theme_light") },
    { value: "dark", label: t("settings.basic.theme_dark") },
  ];

  const densityOptions = [
    { value: "compact", label: t("settings.basic.layout_compact") },
    { value: "normal", label: t("settings.basic.layout_normal") },
    { value: "spacious", label: t("settings.basic.layout_spacious") },
  ];

  const timeDisplayStyleOptions = [
    { value: "classic", label: t("settings.basic.time_display_classic") },
    { value: "seven_segment", label: t("settings.basic.time_display_seven_segment") },
  ];

  return (
    <div
      className="space-y-4 sm:space-y-6 animate-slideUp"
      style={{ animationDelay: "0.3s", animationFillMode: "both" }}
    >
      <WallpaperSelector
        value={config.wallpaper || ""}
        onChange={(wallpaper) => onChange({ ...config, wallpaper })}
      />

      <SettingItem label={t("settings.basic.timezone")}>
        <SelectInput
          value={config.timezone}
          options={timezones}
          onChange={(value) =>
            onChange({ ...config, timezone: parseInt(value) })
          }
          hint={t("settings.basic.timezone_hint", {
            offset: `${config.timezone >= 0 ? "+" : ""}${config.timezone}`,
          })}
        />
      </SettingItem>

      <SettingItem label={t("settings.basic.language")}>
        <SelectInput
          value={config.language}
          options={languages}
          onChange={(value) => onChange({ ...config, language: value })}
        />
      </SettingItem>

      <SettingItem label={t("settings.basic.default_mode")}>
        <SelectInput
          value={config.default_mode}
          options={modes}
          onChange={(value) => onChange({ ...config, default_mode: value })}
        />
      </SettingItem>

      <SettingItem label={t("settings.basic.theme_mode")}>
        <SelectInput
          value={config.theme_mode || "dark"}
          options={themeOptions}
          onChange={(value) => onChange({ ...config, theme_mode: value })}
        />
      </SettingItem>

      <SettingItem label={t("settings.basic.layout_density")}>
        <SelectInput
          value={config.layout_density || "normal"}
          options={densityOptions}
          onChange={(value) => onChange({ ...config, layout_density: value })}
        />
      </SettingItem>

      <SettingItem label={t("settings.basic.time_display_style")}>
        <SelectInput
          value={config.time_display_style || "classic"}
          options={timeDisplayStyleOptions}
          onChange={(value) => onChange({ ...config, time_display_style: value })}
        />
      </SettingItem>

      <SettingItem label={t("settings.basic.sound_enabled")}>
        <div className="space-y-3">
          <label className="label cursor-pointer rounded-lg px-3 py-2 border border-base-300 bg-base-200/50">
            <span className="label-text">{t("settings.basic.sound_finish")}</span>
            <input
              type="checkbox"
              className="toggle toggle-primary"
              checked={config.sound_finish}
              disabled={!config.sound_enabled}
              onChange={(e) =>
                onChange({
                  ...config,
                  sound_finish: e.currentTarget.checked,
                })
              }
            />
          </label>

          <label className="label cursor-pointer rounded-lg px-3 py-2 border border-base-300 bg-base-200/50">
            <span className="label-text">{t("settings.basic.sound_tick")}</span>
            <input
              type="checkbox"
              className="toggle toggle-secondary"
              checked={config.sound_tick}
              disabled={!config.sound_enabled}
              onChange={(e) =>
                onChange({
                  ...config,
                  sound_tick: e.currentTarget.checked,
                })
              }
            />
          </label>

          <label className="label cursor-pointer rounded-lg px-3 py-2 border border-base-300 bg-base-200/50">
            <span className="label-text font-medium">{t("settings.basic.sound_master_switch")}</span>
            <input
              type="checkbox"
              className="toggle toggle-accent"
              checked={config.sound_enabled}
              onChange={(e) =>
                onChange({
                  ...config,
                  sound_enabled: e.currentTarget.checked,
                })
              }
            />
          </label>

          <div className="rounded-lg px-3 py-3 border border-base-300 bg-base-200/30">
            <div className="flex items-center justify-between text-sm mb-2">
              <span>{t("settings.basic.sound_volume")}</span>
              <span className="font-medium">{config.sound_volume}%</span>
            </div>
            <input
              type="range"
              min={0}
              max={100}
              step={1}
              value={config.sound_volume}
              disabled={!config.sound_enabled}
              onInput={(e) => {
                const target = e.currentTarget;
                onChange({ ...config, sound_volume: parseInt(target.value, 10) });
              }}
              className="range range-primary range-sm"
            />
          </div>
        </div>
      </SettingItem>
    </div>
  );
};
