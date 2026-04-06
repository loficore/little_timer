import { memo } from "preact/compat";
import type { FunctionalComponent } from "preact";
import { useState } from "preact/hooks";
import { PickerNumberInput } from "./PickerNumberInput";
import { t } from "../utils/i18n";

interface NumberInputProps {
  value: number;
  min?: number;
  max?: number;
  onChange: (value: number) => void;
  label?: string;
  unit?: string;
  hint?: string;
  disabled?: boolean;
}

export const NumberInput: FunctionalComponent<NumberInputProps> = memo(({
  value,
  min = 0,
  max = 9999,
  onChange,
  label,
  unit,
  hint,
  disabled = false,
}) => {
  const [error, setError] = useState<string>("");

  const handleChange = (newValue: number) => {
    setError("");
    
    if (newValue === null || newValue === undefined) {
      setError(t("validation.input_required"));
      return;
    }

    if (min !== undefined && newValue < min) {
      onChange(min);
      return;
    }

    if (max !== undefined && newValue > max) {
      onChange(max);
      return;
    }

    onChange(newValue);
  };

  return (
    <div className="form-control w-full">
      {label && (
        <label className="label">
          <span className="label-text">{label}</span>
        </label>
      )}
      <div className="flex items-center gap-2">
        <PickerNumberInput
          value={value}
          min={min ?? 0}
          max={max ?? 9999}
          onChange={handleChange}
          disabled={disabled}
        />
        {unit && (
          <span className="label-text-alt">{unit}</span>
        )}
      </div>
      <label className="label">
        {error && <span className="label-text-alt text-error">{error}</span>}
        {!error && hint && <span className="label-text-alt">{hint}</span>}
      </label>
    </div>
  );
});

NumberInput.displayName = "NumberInput";