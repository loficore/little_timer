import { SettingItem } from "./SettingItem";
import { SelectInput } from "./SelectInput";
import { t } from "../utils/i18n";

interface BasicSettingsProps {
  config: {
    timezone: number;
    language: string;
    default_mode: string;
    theme_mode: string;
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

  return (
    <div
      className="space-y-4 sm:space-y-6 animate-slideUp"
      style={{ animationDelay: "0.3s", animationFillMode: "both" }}
    >
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
    </div>
  );
};
