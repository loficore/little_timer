interface CheckboxInputProps {
  checked: boolean;
  onChange: (checked: boolean) => void;
  label: string;
}

export const CheckboxInput = ({
  checked,
  onChange,
  label,
}: CheckboxInputProps) => {
  return (
    <label className="flex items-center gap-2 sm:gap-3 cursor-pointer font-normal user-select-none text-text-primary-dark text-sm sm:text-base min-h-[44px] sm:min-h-[48px]">
      <input
        type="checkbox"
        checked={checked}
        onChange={(e) => onChange(e.currentTarget.checked)}
        className="w-5 h-5 sm:w-6 sm:h-6 cursor-pointer accent-accent-dark border border-border-dark rounded bg-secondary-dark transition-all duration-200"
      />
      {label}
    </label>
  );
};
