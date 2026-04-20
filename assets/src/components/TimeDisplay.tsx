import type { FunctionalComponent } from "preact";
import { memo } from "preact/compat";
import { useMemo } from "preact/hooks";

interface TimeDisplayProps {
  time: string;
  isRunning: boolean;
}

export const TimeDisplay: FunctionalComponent<TimeDisplayProps> = memo(({ time, isRunning }) => {
  const className = useMemo(() => {
    const base = "text-4xl sm:text-6xl md:text-8xl font-light tracking-wider font-mono my-4 sm:my-6 md:my-6 text-center break-all time-transition";
    const colorClass = isRunning ? "text-primary" : "text-base-content";
    const glowClass = isRunning ? "time-running-glow" : "";
    return `${base} ${colorClass} ${glowClass}`.trim();
  }, [isRunning]);

  return (
    <div className={className}>
      <span className="time-value-swap">
        {time}
      </span>
    </div>
  );
});