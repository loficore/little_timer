interface CheckboxInputProps {
  value: boolean;
  onChange: (checked: boolean) => void;
  label: string;
  disabled?: boolean;
}

export const CheckboxInput = ({
  value,
  onChange,
  label,
  disabled = false,
}: CheckboxInputProps) => {
  return (
    <label className="flex items-center gap-2 sm:gap-3 cursor-pointer font-normal user-select-none text-text-primary-dark text-sm sm:text-base min-h-[44px] sm:min-h-[48px]">
      <input
        type="checkbox"
        checked={value}
        disabled={disabled}
        onChange={(e) => !disabled && onChange(e.currentTarget.checked)}
        className="w-5 h-5 sm:w-6 sm:h-6 cursor-pointer accent-accent-dark border border-border-dark rounded bg-secondary-dark transition-all duration-200 disabled:opacity-50 disabled:cursor-not-allowed"
      />
      {label}
    </label>
  );
};
