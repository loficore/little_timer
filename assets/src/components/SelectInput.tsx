import { DropdownSelect } from "./DropdownSelect";

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
      <DropdownSelect
        value={value}
        options={options}
        onChange={(val) => onChange(String(val))}
        disabled={disabled}
        minWidth="100%"
      />
      {hint && (
        <label className="label">
          <span className="label-text-alt">{hint}</span>
        </label>
      )}
    </div>
  );
};
