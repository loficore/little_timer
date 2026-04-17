import { SettingItem } from "./SettingItem";
import { NumberInput } from "./NumberInput";
import { t } from "../utils/i18n";

interface StopwatchSettingsProps {
  config: {
    max_seconds: number;
  };
  isAnimated?: boolean;
  onChange: (config: any) => void;
}

export const StopwatchSettings = ({
  config,
  onChange,
  isAnimated = true,
}: StopwatchSettingsProps) => {
  return (
    <div
      className={`space-y-4 sm:space-y-6 ${isAnimated ? "animate-slideUp" : ""}`}
      style={isAnimated ? { animationDelay: "0.3s", animationFillMode: "both" } : undefined}
    >
      <SettingItem label={t("settings.stopwatch.max_hours")}>
        <NumberInput
          value={Math.floor(config.max_seconds / 3600)}
          min={1}
          max={168}
          onChange={(value) =>
            onChange({ ...config, max_seconds: value * 3600 })
          }
          hint={t("settings.stopwatch.max_hours_hint", {
            hours: Math.floor(config.max_seconds / 3600),
          })}
        />
      </SettingItem>
    </div>
  );
};
