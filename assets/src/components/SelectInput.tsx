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
    <div className="form-control w-full">
      {label && (
        <label className="label">
          <span className="label-text">{label}</span>
        </label>
      )}
      <select
        value={value}
        disabled={disabled}
        onChange={(e) => onChange(e.currentTarget.value)}
        className={`my-select w-full ${disabled ? "disabled" : ""}`}
      >
        {options.map((option) => (
          <option key={option.value} value={option.value}>
            {option.label}
          </option>
        ))}
      </select>
      {hint && (
        <label className="label">
          <span className="label-text-alt">{hint}</span>
        </label>
      )}
    </div>
  );
};
