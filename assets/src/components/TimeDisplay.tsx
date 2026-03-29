import type { FunctionalComponent } from "preact";

interface TimeDisplayProps {
  /** 显示的时间字符串 (HH:MM:SS 格式) */
  time: string;
  /** 计时器是否正在运行 */
  isRunning: boolean;
}

export const TimeDisplay: FunctionalComponent<TimeDisplayProps> = ({
  time,
  isRunning,
}) => {
  return (
    <div
      className={`text-4xl sm:text-6xl md:text-8xl font-light tracking-wider font-mono my-4 sm:my-6 md:my-6 text-center break-all time-transition ${
        isRunning ? "text-primary" : "text-base-content"
      }`}
    >
      {time}
    </div>
  );
};
