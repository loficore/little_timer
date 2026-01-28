import type { FunctionalComponent } from "preact";

type StatusType = "running" | "paused" | "finished";

interface StatusBadgeProps {
  /** 状态类型 */
  status: StatusType;
  /** 状态标签文本 */
  label: string;
  /** 动画延迟（可选） */
  animationDelay?: string;
}

/**
 * 获取状态样式类
 */
const getStatusClasses = (status: StatusType): string => {
  const classes: Record<StatusType, string> = {
    running:
      "bg-accent-dark text-white border border-accent-dark animate-pulse",
    paused:
      "bg-secondary-dark text-text-secondary-dark border border-border-dark",
    finished: "bg-green-600 text-white border border-green-600",
  };
  return classes[status];
};

/**
 * 状态徽章组件 - 显示计时器运行状态
 *
 * @example
 * ```tsx
 * <StatusBadge
 *   status="running"
 *   label="运行中"
 * />
 *
 * <StatusBadge
 *   status="finished"
 *   label="已完成"
 * />
 * ```
 */
export const StatusBadge: FunctionalComponent<StatusBadgeProps> = ({
  status,
  label,
  animationDelay = "0s",
}) => {
  const statusClasses = getStatusClasses(status);

  return (
    <span
      className={`px-3 sm:px-4 py-1.5 sm:py-2 rounded-full text-xs sm:text-sm font-medium transition-all duration-200 whitespace-nowrap animate-slideUp ${statusClasses}`}
      style={{ animationDelay, animationFillMode: "both" }}
    >
      {label}
    </span>
  );
};
