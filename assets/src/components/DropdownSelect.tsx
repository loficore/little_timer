import { useState, useRef, useEffect } from "preact/hooks";
import type { FunctionalComponent } from "preact";

interface DropdownOption {
  value: string | number;
  label: string;
}

interface DropdownSelectProps {
  value: string | number;
  options: DropdownOption[];
  onChange: (value: string | number) => void;
  disabled?: boolean;
  minWidth?: string;
}

export const DropdownSelect: FunctionalComponent<DropdownSelectProps> = ({
  value,
  options,
  onChange,
  disabled = false,
  minWidth = "130px",
}) => {
  const [isOpen, setIsOpen] = useState(false);
  const containerRef = useRef<HTMLDivElement>(null);

  // 关闭下拉菜单当点击外部时
  useEffect(() => {
    const handleClickOutside = (event: MouseEvent) => {
      if (
        containerRef.current &&
        !containerRef.current.contains(event.target as Node)
      ) {
        setIsOpen(false);
      }
    };

    if (isOpen) {
      document.addEventListener("mousedown", handleClickOutside);
      return () => {
        document.removeEventListener("mousedown", handleClickOutside);
      };
    }
  }, [isOpen]);

  const selectedOption = options.find((opt) => opt.value === value);

  const handleSelect = (optValue: string | number) => {
    onChange(optValue);
    setIsOpen(false);
  };

  return (
    <div
      ref={containerRef}
      className="relative inline-block"
      style={{ minWidth }}
    >
      {/* 选择器按钮 */}
      <button
        className={`timer-option-control dropdown-select-btn flex items-center justify-between w-full px-3.5 py-2.5 rounded-xl transition-all duration-200 ${
          disabled ? "opacity-50 cursor-not-allowed" : "cursor-pointer"
        }`}
        style={{
          background: isOpen
            ? "color-mix(in oklab, var(--my-surface) 70%, transparent)"
            : "color-mix(in oklab, var(--my-surface) 60%, transparent)",
          border: `1px solid ${
            isOpen
              ? "var(--accent-color)"
              : "color-mix(in oklab, var(--my-outline) 50%, transparent)"
          }`,
          color: "var(--my-on-surface)",
          boxShadow: isOpen
            ? `0 0 0 2px color-mix(in oklab, var(--accent-color) 25%, transparent)`
            : "none",
        }}
        onClick={() => !disabled && setIsOpen(!isOpen)}
        disabled={disabled}
      >
        <span className="flex-1 text-left text-sm">
          {selectedOption?.label || "选择"}
        </span>
        <svg
          xmlns="http://www.w3.org/2000/svg"
          className={`h-4 w-4 transition-transform duration-200 ${
            isOpen ? "rotate-180" : ""
          }`}
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
        >
          <path
            strokeLinecap="round"
            strokeLinejoin="round"
            strokeWidth={2}
            d="M19 9l-7 7-7-7"
          />
        </svg>
      </button>

      {/* 下拉菜单 */}
      {isOpen && (
        <div
          className="absolute top-full left-0 right-0 mt-2 bg-base-100 rounded-xl border shadow-lg z-50 overflow-hidden transition-all duration-200"
          style={{
            background: "color-mix(in oklab, var(--my-surface) 85%, transparent)",
            border: `1px solid color-mix(in oklab, var(--my-outline) 60%, transparent)`,
            backdropFilter: "blur(8px) saturate(110%)",
            WebkitBackdropFilter: "blur(8px) saturate(110%)",
            boxShadow: `0 8px 24px 0 rgba(0, 0, 0, 0.2)`,
            minWidth: minWidth,
          }}
        >
          {options.map((option) => (
            <button
              key={option.value}
              className={`w-full px-3.5 py-3 text-left text-sm transition-colors duration-150 border-b border-transparent ${
                value === option.value
                  ? "bg-primary/20"
                  : "hover:bg-base-200/50"
              }`}
              style={
                value === option.value
                  ? {
                      color: "var(--accent-color)",
                      background:
                        "color-mix(in oklab, var(--accent-color) 15%, transparent)",
                    }
                  : {
                      color: "var(--my-on-surface)",
                    }
              }
              onClick={() => handleSelect(option.value)}
            >
              <div className="flex items-center justify-between">
                <span>{option.label}</span>
                {value === option.value && (
                  <svg
                    xmlns="http://www.w3.org/2000/svg"
                    className="h-4 w-4"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                  >
                    <path
                      strokeLinecap="round"
                      strokeLinejoin="round"
                      strokeWidth={2}
                      d="M5 13l4 4L19 7"
                    />
                  </svg>
                )}
              </div>
            </button>
          ))}
        </div>
      )}
    </div>
  );
};
