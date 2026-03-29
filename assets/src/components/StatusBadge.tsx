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
 * 获取 DaisyUI badge 变体类名
 */
const getStatusVariant = (status: StatusType): string => {
  const variants: Record<StatusType, string> = {
    running: "badge-primary",
    paused: "badge-neutral",
    finished: "badge-success",
  };
  return variants[status];
};

/**
 * 状态徽章组件 - 基于 DaisyUI badge
 */
export const StatusBadge: FunctionalComponent<StatusBadgeProps> = ({
  status,
  label,
  animationDelay = "0s",
}) => {
  const variant = getStatusVariant(status);

  return (
    <span
      className={`badge ${variant} gap-2 text-xs sm:text-sm animate-slideUp`}
      style={{ animationDelay, animationFillMode: "both" }}
    >
      {status === "running" && <span className="badge badge-xs badge-primary animate-pulse" />}
      {label}
    </span>
  );
};
