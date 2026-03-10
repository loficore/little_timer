import { SettingItem } from "./SettingItem";
import { TimeInput } from "./TimeInput";
import { NumberInput } from "./NumberInput";
import { CheckboxInput } from "./CheckboxInput";
import { t } from "../utils/i18n";

interface CountdownSettingsProps {
  config: {
    duration_seconds: number;
    loop: boolean;
    loop_count: number;
    loop_interval_seconds: number;
  };
  onChange: (config: any) => void;
  /** 当非倒计时模式时隐藏循环相关配置 */
  showLoopControls?: boolean;
}

export const CountdownSettings = ({
  config,
  onChange,
  showLoopControls = true,
}: CountdownSettingsProps) => {
  return (
    <div
      className="space-y-4 sm:space-y-6 animate-slideUp"
      style={{ animationDelay: "0.3s", animationFillMode: "both" }}
    >
      <SettingItem label={t("settings.countdown.duration")}>
        <TimeInput
          value={config.duration_seconds}
          maxHours={24}
          onChange={(totalSeconds) =>
            onChange({ ...config, duration_seconds: totalSeconds })
          }
          hint={t("settings.countdown.duration_hint", {
            minutes: Math.floor(config.duration_seconds / 60),
          })}
        />
      </SettingItem>

      {showLoopControls && (
        <>
          <SettingItem label={t("settings.countdown.loop_mode")}>
            <CheckboxInput
              value={config.loop}
              onChange={(checked) => onChange({ ...config, loop: checked })}
              label={t("settings.countdown.loop_enable")}
            />
          </SettingItem>

          {config.loop && (
            <>
              <SettingItem label={t("settings.countdown.loop_count")}>
                <NumberInput
                  value={config.loop_count}
                  min={0}
                  max={100}
                  onChange={(value) =>
                    onChange({ ...config, loop_count: value })
                  }
                  hint={t("settings.countdown.loop_count_hint")}
                />
              </SettingItem>

              <SettingItem label={t("settings.countdown.loop_interval")}>
                <TimeInput
                  value={config.loop_interval_seconds}
                  maxHours={1}
                  showHours={false}
                  onChange={(totalSeconds) =>
                    onChange({ ...config, loop_interval_seconds: totalSeconds })
                  }
                  hint={t("settings.countdown.loop_interval_hint")}
                />
              </SettingItem>
            </>
          )}
        </>
      )}
    </div>
  );
};
