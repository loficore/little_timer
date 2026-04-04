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
      <label className="label cursor-pointer gap-3">
        <input
          type="checkbox"
          checked={value}
          disabled={disabled}
          onChange={(e) => !disabled && onChange(e.currentTarget.checked)}
          className="my-checkbox"
        />
        <span className="label-text">{label}</span>
      </label>
    </div>
  );
};
