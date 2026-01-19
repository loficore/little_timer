import { useState } from "preact/hooks";
import { t } from "../utils/i18n";

interface NumberInputProps {
  value: number;
  min?: number;
  max?: number;
  onChange: (value: number) => void;
  hint?: string;
}

export const NumberInput = ({
  value,
  min,
  max,
  onChange,
  hint,
}: NumberInputProps) => {
  const [error, setError] = useState<string>("");

  const handleChange = (e: Event) => {
    const input = e.currentTarget as HTMLInputElement;
    const rawValue = input.value.trim();

    // 清除错误状态（用户正在修改）
    setError("");

    // 空值检查
    if (rawValue === "") {
      setError(t("validation.input_required"));
      return;
    }

    const numValue = parseInt(rawValue, 10);

    // NaN 检查
    if (Number.isNaN(numValue)) {
      setError(t("validation.input_invalid"));
      return;
    }

    // 范围检查
    if (min !== undefined && numValue < min) {
      setError(t("validation.input_min_value", { min }));
      return;
    }

    if (max !== undefined && numValue > max) {
      setError(t("validation.input_max_value", { max }));
      return;
    }

    // 校验通过，调用回调
    onChange(numValue);
  };

  const handleBlur = () => {
    // 失焦时重新校验（可选）
    if (value === null || value === undefined) {
      setError(t("validation.input_required"));
    }
  };

  return (
    <>
      <input
        type="number"
        min={min}
        max={max}
        value={value}
        onChange={handleChange}
        onBlur={handleBlur}
        className={`form-input input-base px-3 sm:px-4 py-2 sm:py-3 border rounded-lg text-xs sm:text-sm bg-secondary-dark text-text-primary-dark focus:border-accent-dark transition-colors duration-200 ${
          error ? "border-red-500 focus:border-red-500" : "border-border-dark"
        }`}
      />
      {error && (
        <span className="text-xs text-red-500 font-medium">{error}</span>
      )}
      {!error && hint && (
        <span className="text-xs text-text-secondary-dark italic">{hint}</span>
      )}
    </>
  );
};
