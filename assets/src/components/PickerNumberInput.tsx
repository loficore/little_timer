import { memo } from "preact/compat";
import type { FunctionalComponent } from "preact";

interface PickerNumberInputProps {
  value: number;
  min?: number;
  max?: number;
  onChange: (value: number) => void;
  label?: string;
  unit?: string;
  hint?: string;
  disabled?: boolean;
}

export const PickerNumberInput: FunctionalComponent<PickerNumberInputProps> = memo(({
  value,
  min = 0,
  max = 99,
  onChange,
  label,
  unit,
  hint,
  disabled = false,
}) => {
  const handleIncrease = () => {
    if (value < max) {
      onChange(value + 1);
    }
  };

  const handleDecrease = () => {
    if (value > min) {
      onChange(value - 1);
    }
  };

  const handleInput = (e: Event) => {
    const input = e.target as HTMLInputElement;
    const rawValue = input.value.trim();
    if (rawValue === "") return;
    
    const numValue = parseInt(rawValue, 10);
    if (Number.isNaN(numValue)) return;
    
    const clampedValue = Math.max(min, Math.min(numValue, max));
    onChange(clampedValue);
  };

  return (
    <div className="form-control w-full">
      {label && (
        <label className="label">
          <span className="label-text">{label}</span>
        </label>
      )}
      <div className="flex items-center gap-1">
        <div className="relative flex items-center bg-[var(--my-surface)]/60 rounded-xl border border-[var(--my-outline)]/50 backdrop-blur-sm h-11 overflow-hidden">
          <input
            type="text"
            inputMode="numeric"
            value={value}
            onChange={handleInput}
            disabled={disabled}
            className={`w-16 h-full pl-3 text-center text-sm font-semibold bg-transparent border-none outline-none appearance-none ${
              disabled ? "opacity-50 cursor-not-allowed" : ""
            }`}
            style={{ WebkitAppearance: "none", appearance: "none" }}
          />
          <div className="flex flex-col flex-shrink-0 h-full">
            <button
              type="button"
              className="w-5 h-5 min-w-0 min-h-0 flex items-center justify-center text-[8px] text-[var(--my-on-surface-variant)] hover:text-[var(--my-on-surface)] hover:bg-[var(--my-primary-container)]/30 transition-colors"
              onClick={handleIncrease}
              disabled={disabled || value >= max}
            >
              ▲
            </button>
            <button
              type="button"
              className="w-5 h-5 min-w-0 min-h-0 flex items-center justify-center text-[8px] text-[var(--my-on-surface-variant)] hover:text-[var(--my-on-surface)] hover:bg-[var(--my-primary-container)]/30 transition-colors"
              onClick={handleDecrease}
              disabled={disabled || value <= min}
            >
              ▼
            </button>
          </div>
        </div>
        {unit && (
          <span className="text-sm text-[var(--my-on-surface-variant)] ml-1">{unit}</span>
        )}
      </div>
      <label className="label">
        {hint && <span className="label-text-alt">{hint}</span>}
      </label>
    </div>
  );
});

PickerNumberInput.displayName = "PickerNumberInput";