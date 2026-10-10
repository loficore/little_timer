/**
 * Timeline —— 一日调度计划视图。
 *
 * 结构:
 *  - 上方轴:dayStartHour..dayEndHour(默认 6..23)的整点 tick 标签
 *  - 中间轨:绝对定位的 TaskBlock;左右/宽度百分比按窗口换算
 *  - 下方习惯轨:HabitBlock 占位(habit_schedules 属于 Plan 3,v1 空白)
 *
 * 注意:
 *  - 入参 `scheduled_start`/`scheduled_end` 是 unix 秒,需用本地时区解析到「当日
 *    几点几分」;窗口本身也是分钟(0-1440)。这种"日期 + 当天 hour window"
 *    的混合由 `localMinuteOfDay` 处理。
 *  - 测试用例 scheduled_start=0(epoch)在 UTC 环境下为 0:00,落在 dayStart(6:00)
 *    之前,会被 clamp 到窗口左边界,仍能渲染为最小宽度的占位(测 1/3 已覆盖)。
 */

import { TaskBlock } from "./TaskBlock";
import { HabitBlock } from "./HabitBlock";
import type { TimelineTask } from "./types";
import { t } from "../../utils/i18n";

export interface TimelineProps {
  tasks: TimelineTask[];
  dayStartHour?: number; // 默认 6
  dayEndHour?: number; // 默认 23
}

function buildHourTicks(startHour: number, endHour: number): number[] {
  const out: number[] = [];
  for (let h = startHour; h <= endHour; h++) out.push(h);
  return out;
}

function pctOfWindow(min: number, startMin: number, endMin: number): number {
  const span = endMin - startMin;
  if (span <= 0) return 0;
  return ((min - startMin) / span) * 100;
}

export function Timeline({ tasks, dayStartHour = 6, dayEndHour = 23 }: TimelineProps) {
  const windowStartMin = dayStartHour * 60;
  const windowEndMin = dayEndHour * 60;
  const ticks = buildHourTicks(dayStartHour, dayEndHour);

  return (
    <div
      className="schedule-timeline relative w-full rounded border border-base-300 bg-base-100 p-3"
      data-testid="timeline"
    >
      {/* 小时轴 */}
      <div className="timeline-axis relative mb-2 h-6 border-b border-base-300">
        {ticks.map((h) => (
          <span
            key={h}
            className="timeline-tick absolute top-0 -translate-x-1/2 text-[10px] opacity-60"
            style={{ left: `${pctOfWindow(h * 60, windowStartMin, windowEndMin)}%` }}
          >
            {String(h).padStart(2, "0")}:00
          </span>
        ))}
      </div>

      {/* Task 轨 */}
      <div
        className="timeline-track relative h-16 rounded bg-base-200/40"
        data-testid="timeline-track"
      >
        {/* 网格线 */}
        {ticks.map((h) => (
          <span
            key={`g-${h}`}
            className="timeline-gridline pointer-events-none absolute top-0 bottom-0 w-px bg-base-300/70"
            style={{ left: `${pctOfWindow(h * 60, windowStartMin, windowEndMin)}%` }}
          />
        ))}
        {tasks.map((task) => (
          <TaskBlock
            key={task.id}
            task={task}
            windowStartMin={windowStartMin}
            windowEndMin={windowEndMin}
          />
        ))}
        {tasks.length === 0 && (
          <div className="absolute inset-0 flex items-center justify-center text-xs opacity-60">
            {t("schedule.empty")}
          </div>
        )}
      </div>

      {/* Habit 轨(Plan 3 之前留空占位) */}
      <HabitBlock />
    </div>
  );
}