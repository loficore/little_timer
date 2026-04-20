import { memo } from "preact/compat";
import type { FunctionalComponent } from "preact";
import { useState, useEffect } from "preact/hooks";
import { PickerNumberInput } from "./PickerNumberInput";
import { t } from "../utils/i18n";

interface TimeInputProps {
  value: number;
  onChange: (totalSeconds: number) => void;
  label?: string;
  maxHours?: number;
  showHours?: boolean;
  showMinutes?: boolean;
  showSeconds?: boolean;
  hint?: string;
}

export const TimeInput: FunctionalComponent<TimeInputProps> = memo(({
  value,
  onChange,
  label,
  maxHours = 24,
  showHours = true,
  showMinutes = true,
  showSeconds = true,
  hint,
}) => {
  const [hours, setHours] = useState(0);
  const [minutes, setMinutes] = useState(0);
  const [seconds, setSeconds] = useState(0);

  useEffect(() => {
    const h = Math.floor(value / 3600);
    const m = Math.floor((value % 3600) / 60);
    const s = value % 60;
    setHours(h);
    setMinutes(m);
    setSeconds(s);
  }, [value]);

  const handleHoursChange = (h: number) => {
    const newHours = Math.max(0, Math.min(h, maxHours));
    setHours(newHours);
    onChange(newHours * 3600 + minutes * 60 + seconds);
  };

  const handleMinutesChange = (m: number) => {
    const newMinutes = Math.max(0, Math.min(m, 59));
    setMinutes(newMinutes);
    onChange(hours * 3600 + newMinutes * 60 + seconds);
  };

  const handleSecondsChange = (s: number) => {
    const newSeconds = Math.max(0, Math.min(s, 59));
    setSeconds(newSeconds);
    onChange(hours * 3600 + minutes * 60 + newSeconds);
  };

  return (
    <div className="form-control w-full">
      {label && (
        <label className="label">
          <span className="label-text">{label}</span>
        </label>
      )}
      <div className="flex items-center justify-center gap-3 sm:gap-4">
        {showHours && (
          <div className="flex flex-col items-center">
            <PickerNumberInput
              value={hours}
              min={0}
              max={maxHours}
              onChange={handleHoursChange}
            />
            <span className="label-text-alt mt-1">{t("common.hours")}</span>
          </div>
        )}
        <span className="text-lg font-bold -mt-4">:</span>
        {showMinutes && (
          <div className="flex flex-col items-center">
            <PickerNumberInput
              value={minutes}
              min={0}
              max={59}
              onChange={handleMinutesChange}
            />
            <span className="label-text-alt mt-1">{t("common.minutes")}</span>
          </div>
        )}
        <span className="text-lg font-bold -mt-4">:</span>
        {showSeconds && (
          <div className="flex flex-col items-center">
            <PickerNumberInput
              value={seconds}
              min={0}
              max={59}
              onChange={handleSecondsChange}
            />
            <span className="label-text-alt mt-1">{t("common.seconds")}</span>
          </div>
        )}
      </div>
      {hint && (
        <label className="label">
          <span className="label-text-alt">{hint}</span>
        </label>
      )}
    </div>
  );
});

TimeInput.displayName = "TimeInput";