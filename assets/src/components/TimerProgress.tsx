/**
 * 计时器进度显示组件
 * 显示今日进度、目标、进度条和连胜信息
 */

import { memo } from "preact/compat";
import type { FunctionalComponent } from "preact";
import { formatDuration, calculateProgress } from "../utils/formatters";
import { StarIconComponent } from "../utils/icons";

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
  if (!isStopwatchMode || !habitDetail) {
    return null;
  }

  const todayProgress = habitDetail.today_seconds + elapsedSeconds;
  const progressPercent = calculateProgress(todayProgress, habitDetail.goal_seconds);

  return (
    <div className="w-full max-w-2xl mb-6 sm:mb-8">
      <div className="flex justify-between text-sm text-base-content/70 mb-1">
        <span>今日 {formatDuration(todayProgress)}</span>
        <span>目标 {formatDuration(habitDetail.goal_seconds)}</span>
      </div>
      <progress
        className={`progress w-full ${isFinished ? "progress-success" : "progress-primary"}`}
        value={Math.min(todayProgress, habitDetail.goal_seconds)}
        max={habitDetail.goal_seconds}
      />
      <div className="flex justify-between items-center mt-2 text-sm">
        <span className="text-base-content/60">
          进度 {progressPercent}%
        </span>
        {habitDetail.streak > 0 && (
          <span className="text-warning inline-flex items-center gap-1">
            <StarIconComponent />
            {habitDetail.streak} 天
          </span>
        )}
      </div>
    </div>
  );
});

TimerProgress.displayName = "TimerProgress";