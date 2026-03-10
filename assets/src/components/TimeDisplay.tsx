import type { FunctionalComponent } from "preact";

interface TimeDisplayProps {
  /** 显示的时间字符串 (HH:MM:SS 格式) */
  time: string;
  /** 计时器是否正在运行 */
  isRunning: boolean;
}

/**
 * 时间显示组件 - 大号时间显示区域，极简设计，瞬间更新
 *
 * @example
 * ```tsx
 * <TimeDisplay
 *   time="25:30:45"
 *   isRunning={true}
 * />
 * ```
 */
export const TimeDisplay: FunctionalComponent<TimeDisplayProps> = ({
  time,
  isRunning,
}) => {
  return (
    <div
      className={`text-4xl sm:text-6xl md:text-8xl font-light tracking-wider text-text-primary-dark font-mono my-4 sm:my-6 md:my-6 text-center break-all time-transition ${
        isRunning ? "text-accent-dark" : ""
      }`}
    >
      {time}
    </div>
  );
};
