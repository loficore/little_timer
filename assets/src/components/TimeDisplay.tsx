import type { FunctionalComponent } from "preact";

interface TimeDisplayProps {
  /** 显示的时间字符串 (HH:MM:SS 格式) */
  time: string;
  /** 计时器是否正在运行 */
  isRunning: boolean;
  /** 是否正在执行动画 */
  isAnimating?: boolean;
  /** 动画延迟（可选） */
  animationDelay?: string;
}

/**
 * 时间显示组件 - 大号时间显示区域，支持动画效果
 *
 * @example
 * ```tsx
 * <TimeDisplay
 *   time="25:30:45"
 *   isRunning={true}
 *   isAnimating={isAnimating}
 *   animationDelay="0.2s"
 * />
 * ```
 */
export const TimeDisplay: FunctionalComponent<TimeDisplayProps> = ({
  time,
  isRunning,
  isAnimating = false,
  animationDelay = "0s",
}) => {
  return (
    <div
      className={`text-4xl sm:text-6xl md:text-8xl font-light tracking-wider text-text-primary-dark font-mono my-4 sm:my-6 md:my-6 text-center transition-all duration-300 animate-slideUp break-all time-transition ${
        isRunning ? "text-accent-dark" : ""
      } ${isAnimating ? "time-transition--active" : ""}`}
      style={{ animationDelay, animationFillMode: "both" }}
    >
      {time}
    </div>
  );
};
