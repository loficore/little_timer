import { useState } from "preact/hooks";
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

export const NumberInput = ({
  value,
  min,
  max,
  onChange,
  label,
  unit,
  hint,
  disabled = false,
}: NumberInputProps) => {
  const [error, setError] = useState<string>("");

  const handleChange = (e: Event) => {
    const input = e.currentTarget as HTMLInputElement;
    const rawValue = input.value.trim();

    setError("");

    if (rawValue === "") {
      setError(t("validation.input_required"));
      return;
    }

    const numValue = parseInt(rawValue, 10);

    if (Number.isNaN(numValue)) {
      setError(t("validation.input_invalid"));
      return;
    }

    if (min !== undefined && numValue < min) {
      onChange(min);
      return;
    }

    if (max !== undefined && numValue > max) {
      onChange(max);
      return;
    }

    onChange(numValue);
  };

  const handleBlur = () => {
    if (value === null || value === undefined) {
      setError(t("validation.input_required"));
    }
  };

  return (
    <div className="form-control w-full">
      {label && (
        <label className="label">
          <span className="label-text">{label}</span>
        </label>
      )}
      <div className="flex items-center gap-2">
        <input
          type="number"
          min={min}
          max={max}
          value={value}
          disabled={disabled}
          onChange={handleChange}
          onBlur={handleBlur}
          className={`input input-bordered w-full ${error ? "input-error" : ""} ${
            disabled ? "disabled" : ""
          }`}
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
};
