import type { FunctionalComponent, ComponentChildren } from "preact";

type ButtonVariant = "primary" | "secondary" | "danger" | "ghost";
type ButtonSize = "sm" | "md" | "lg";

interface ButtonProps {
  /** 按钮样式变体 */
  variant?: ButtonVariant;
  /** 按钮尺寸 */
  size?: ButtonSize;
  /** 按钮图标（emoji 或文本） */
  icon?: string;
  /** 是否禁用 */
  disabled?: boolean;
  /** 按钮内容 */
  children: ComponentChildren;
  /** 点击回调 */
  onClick?: () => void;
  /** 自定义 className */
  className?: string;
  /** 标题提示 */
  title?: string;
}

/**
 * 获取变体样式类
 */
const getVariantClasses = (variant: ButtonVariant): string => {
  const variants: Record<ButtonVariant, string> = {
    primary:
      "bg-accent-dark text-white border border-accent-dark hover:bg-accent-dark/90 hover:border-accent-dark/90",
    secondary:
      "bg-secondary-dark text-text-secondary-dark border border-border-dark hover:bg-tertiary-dark hover:text-text-primary-dark hover:border-accent-dark",
    danger:
      "bg-red-600 text-white border border-red-600 hover:bg-red-700 hover:border-red-700",
    ghost:
      "bg-transparent text-text-secondary-dark border border-transparent hover:bg-secondary-dark hover:text-text-primary-dark",
  };
  return variants[variant];
};

/**
 * 获取尺寸样式类
 */
const getSizeClasses = (size: ButtonSize): string => {
  const sizes: Record<ButtonSize, string> = {
    sm: "px-2 sm:px-3 py-1.5 sm:py-2 text-xs sm:text-sm",
    md: "px-3 sm:px-4 py-2 sm:py-3 text-sm sm:text-base",
    lg: "px-4 sm:px-6 py-3 sm:py-4 text-base sm:text-lg",
  };
  return sizes[size];
};

/**
 * 统一按钮组件 - 提供一致的按钮样式和交互
 *
 * @example
 * ```tsx
 * <Button variant="primary" size="md" icon="▶" onClick={handleStart}>
 *   开始
 * </Button>
 *
 * <Button variant="danger" size="sm">
 *   删除
 * </Button>
 *
 * <Button variant="ghost">返回</Button>
 * ```
 */
export const Button: FunctionalComponent<ButtonProps> = ({
  variant = "primary",
  size = "md",
  icon,
  disabled = false,
  children,
  onClick,
  className = "",
  title,
}) => {
  const baseClasses =
    "flex items-center justify-center gap-1 sm:gap-2 rounded-xl font-medium transition-all duration-200 hover:scale-105 active:scale-95 cursor-pointer disabled:opacity-50 disabled:cursor-not-allowed disabled:hover:scale-100";

  const variantClasses = getVariantClasses(variant);
  const sizeClasses = getSizeClasses(size);

  const handleClick = () => {
    if (!disabled && onClick) {
      onClick();
    }
  };

  return (
    <button
      onClick={handleClick}
      disabled={disabled}
      title={title}
      className={`${baseClasses} ${variantClasses} ${sizeClasses} ${className}`}
    >
      {icon && <span className="text-lg sm:text-xl">{icon}</span>}
      <span>{children}</span>
    </button>
  );
};
