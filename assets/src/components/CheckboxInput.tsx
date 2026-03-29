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
    <div className="form-control">
      <label className="label cursor-pointer">
        <span className="label-text">{label}</span>
        <input
          type="checkbox"
          checked={value}
          disabled={disabled}
          onChange={(e) => !disabled && onChange(e.currentTarget.checked)}
          className="toggle toggle-primary"
        />
      </label>
    </div>
  );
};
