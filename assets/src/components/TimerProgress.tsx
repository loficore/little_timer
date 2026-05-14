/**
 * 计时器进度显示组件
 * 显示今日进度、目标、进度条和连胜信息
 */

import { memo } from "preact/compat";
import type { FunctionalComponent } from "preact";
import { formatDuration, calculateProgress } from "../utils/formatters";
import { StarIconComponent } from "../utils/icons";
import { useEffect, useState } from "preact/hooks";

interface HabitDetailData {
  today_seconds: number;
  goal_seconds: number;
  streak: number;
}

interface TimerProgressProps {
  habitDetail: HabitDetailData | null;
  elapsedSeconds: number;
  isFinished: boolean;
  isStopwatchMode: boolean;
}

export const TimerProgress: FunctionalComponent<TimerProgressProps> = memo(({
  habitDetail,
  elapsedSeconds,
  isFinished,
  isStopwatchMode,
}) => {
  const [displayProgress, setDisplayProgress] = useState(0);

  if (!isStopwatchMode || !habitDetail) {
    return null;
  }

  const todayProgress = habitDetail.today_seconds + elapsedSeconds;
  const progressPercent = calculateProgress(todayProgress, habitDetail.goal_seconds);

  useEffect(() => {
    const timer = setTimeout(() => {
      setDisplayProgress(progressPercent);
    }, 100);
    return () => clearTimeout(timer);
  }, [progressPercent]);

  return (
    <div className="w-full max-w-2xl mb-6 sm:mb-8">
      <div className="flex justify-between text-sm text-base-content/70 mb-1">
        <span>今日 {formatDuration(todayProgress)}</span>
        <span>目标 {formatDuration(habitDetail.goal_seconds)}</span>
      </div>
      <div className="progress-container w-full h-2 rounded-full overflow-hidden bg-[var(--my-field-bg)]">
        <div
          className={`progress-bar h-full rounded-full transition-all duration-500 ease-out ${
            isFinished ? "bg-gradient-to-r from-success to-success/80" : "bg-gradient-to-r from-[var(--accent-color)] to-[var(--my-primary-container)]"
          }`}
          style={{ width: `${Math.min(displayProgress, 100)}%` }}
        />
      </div>
      <div className="flex justify-between items-center mt-2 text-sm">
        <span className="text-base-content/60">
          进度 {progressPercent}%
        </span>
        {habitDetail.streak > 0 && (
          <span className="text-warning inline-flex items-center gap-1 animate-pulse">
            <StarIconComponent />
            {habitDetail.streak} 天
          </span>
        )}
      </div>
    </div>
  );
});

TimerProgress.displayName = "TimerProgress";