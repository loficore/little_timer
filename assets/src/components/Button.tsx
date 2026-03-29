import type { FunctionalComponent, ComponentChildren, VNode } from "preact";

type ButtonVariant = "primary" | "secondary" | "danger" | "ghost" | "success" | "warning" | "info";
type ButtonSize = "xs" | "sm" | "md" | "lg";

interface ButtonProps {
  /** 按钮样式变体 */
  variant?: ButtonVariant;
  /** 按钮尺寸 */
  size?: ButtonSize;
  /** 按钮图标（组件） */
  icon?: VNode;
  /** 是否禁用 */
  disabled?: boolean;
  /** 是否加载中 */
  loading?: boolean;
  /** 按钮内容 */
  children: ComponentChildren;
  /** 点击回调 */
  onClick?: () => void;
  /** 自定义 className */
  className?: string;
  /** 标题提示 */
  title?: string;
  /** 按钮类型 */
  type?: "button" | "submit" | "reset";
  /** 是否outline样式 */
  outline?: boolean;
  /** 是否block样式 */
  block?: boolean;
}

/**
 * 获取 DaisyUI 变体类名
 */
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

/**
 * 获取 DaisyUI 尺寸类名
 */
const getSizeClasses = (size: ButtonSize): string => {
  const sizes: Record<ButtonSize, string> = {
    xs: "btn-xs",
    sm: "btn-sm",
    md: "",
    lg: "btn-lg",
  };
  return sizes[size];
};

/**
 * 统一按钮组件 - 基于 DaisyUI btn
 */
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
  const variantClasses = getVariantClasses(variant, outline);
  const sizeClasses = getSizeClasses(size);

  const handleClick = () => {
    if (!disabled && !loading && onClick) {
      onClick();
    }
  };

  return (
    <button
      type={type}
      onClick={handleClick}
      disabled={disabled || loading}
      title={title}
      className={`btn ${variantClasses} ${sizeClasses} ${block ? "btn-block" : ""} ${className}`}
    >
      {loading && <span className="loading loading-spinner loading-sm" />}
      {!loading && icon && <span className="w-4 h-4">{icon}</span>}
      <span>{children}</span>
    </button>
  );
};
