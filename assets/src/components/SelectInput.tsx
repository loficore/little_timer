interface SelectOption {
  value: string | number;
  label: string;
}

interface SelectInputProps {
  value: string | number;
  options: SelectOption[];
  onChange: (value: string) => void;
  hint?: string;
}

export const SelectInput = ({
  value,
  options,
  onChange,
  hint,
}: SelectInputProps) => {
  return (
    <>
      <select
        value={value}
        onChange={(e) => onChange(e.currentTarget.value)}
        className="form-input input-base px-3 sm:px-4 py-2 sm:py-3 border border-border-dark rounded-lg text-xs sm:text-sm bg-secondary-dark text-text-primary-dark focus:border-accent-dark transition-colors duration-200"
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
