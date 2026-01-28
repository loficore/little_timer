interface SelectOption {
  value: string | number;
  label: string;
}

interface SelectInputProps {
  value: string | number;
  options: SelectOption[];
  onChange: (value: string) => void;
  label?: string;
  hint?: string;
  disabled?: boolean;
}

export const SelectInput = ({
  value,
  options,
  onChange,
  label,
  hint,
  disabled = false,
}: SelectInputProps) => {
  return (
    <>
      {label && (
        <label className="block text-xs sm:text-sm font-medium text-text-primary-dark mb-2">
          {label}
        </label>
      )}
      <select
        value={value}
        disabled={disabled}
        onChange={(e) => onChange(e.currentTarget.value)}
        className="form-input input-base w-full px-3 sm:px-4 py-2 sm:py-3 border border-border-dark rounded-lg text-xs sm:text-sm bg-secondary-dark text-text-primary-dark focus:border-accent-dark transition-colors duration-200 disabled:opacity-50 disabled:cursor-not-allowed"
      >
        {options.map((option) => (
          <option key={option.value} value={option.value}>
            {option.label}
          </option>
        ))}
      </select>
      {hint && (
        <span className="text-xs text-text-secondary-dark italic">{hint}</span>
      )}
    </>
  );
};
