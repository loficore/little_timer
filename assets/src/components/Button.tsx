import type { FunctionalComponent, ComponentChildren, VNode } from "preact";
import { useState, useRef, useCallback } from "preact/hooks";

type ButtonVariant = "primary" | "secondary" | "danger" | "ghost" | "success" | "warning" | "info";
type ButtonSize = "xs" | "sm" | "md" | "lg";

interface ButtonProps {
  variant?: ButtonVariant;
  size?: ButtonSize;
  icon?: VNode;
  disabled?: boolean;
  loading?: boolean;
  children: ComponentChildren;
  onClick?: () => void;
  className?: string;
  title?: string;
  type?: "button" | "submit" | "reset";
  outline?: boolean;
  block?: boolean;
}

const getVariantClasses = (variant: ButtonVariant, outline: boolean): string => {
  const variants: Record<ButtonVariant, string> = {
    primary: outline ? "btn-primary btn-outline" : "btn-primary",
    secondary: outline ? "btn-secondary btn-outline" : "btn-secondary",
    danger: outline ? "btn-error btn-outline" : "btn-error",
    ghost: "btn-ghost",
    success: outline ? "btn-success btn-outline" : "btn-success",
    warning: outline ? "btn-warning btn-outline" : "btn-warning",
    info: outline ? "btn-info btn-outline" : "btn-info",
  };
  return variants[variant];
};

const getSizeClasses = (size: ButtonSize): string => {
  const sizes: Record<ButtonSize, string> = {
    xs: "btn-xs",
    sm: "btn-sm",
    md: "",
    lg: "btn-lg",
  };
  return sizes[size];
};

export const Button: FunctionalComponent<ButtonProps> = ({
  variant = "primary",
  size = "md",
  icon,
  disabled = false,
  loading = false,
  children,
  onClick,
  className = "",
  title,
  type = "button",
  outline = false,
  block = false,
}) => {
  const [ripples, setRipples] = useState<{ x: number; y: number; id: number }[]>([]);
  const buttonRef = useRef<HTMLButtonElement>(null);
  const rippleIdRef = useRef(0);

  const handleClick = useCallback((e: MouseEvent) => {
    if (!disabled && !loading && onClick) {
      const button = buttonRef.current;
      if (button) {
        const rect = button.getBoundingClientRect();
        const x = e.clientX - rect.left;
        const y = e.clientY - rect.top;
        const id = ++rippleIdRef.current;
        setRipples((prev) => [...prev, { x, y, id }]);
        setTimeout(() => {
          setRipples((prev) => prev.filter((r) => r.id !== id));
        }, 600);
      }
      onClick();
    }
  }, [disabled, loading, onClick]);

  const variantClasses = getVariantClasses(variant, outline);
  const sizeClasses = getSizeClasses(size);

  return (
    <button
      ref={buttonRef}
      type={type}
      onClick={handleClick}
      disabled={disabled || loading}
      title={title}
      className={`btn ${variantClasses} ${sizeClasses} ${block ? "btn-block" : ""} ${className} relative overflow-hidden`}
    >
      {ripples.map((ripple) => (
        <span
          key={ripple.id}
          className="ripple-effect"
          style={{
            left: `${ripple.x}px`,
            top: `${ripple.y}px`,
          }}
        />
      ))}
      {loading && <span className="loading loading-spinner loading-sm" />}
      {!loading && icon && <span className="w-4 h-4">{icon}</span>}
      <span>{children}</span>
    </button>
  );
};
