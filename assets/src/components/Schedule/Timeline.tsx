/**
 * Timeline —— 一日调度计划视图。
 *
 * 结构:
 *  - 上方轴:dayStartHour..dayEndHour(默认 6..23)的整点 tick 标签
 *  - 中间轨:绝对定位的 TaskBlock;左右/宽度百分比按窗口换算
 *
 * 注意:
 *  - 入参 `scheduled_start`/`scheduled_end` 是 unix 秒,需用本地时区解析到「当日
 *    几点几分」;窗口本身也是分钟(0-1440)。这种"日期 + 当天 hour window"
 *    的混合由 `localMinuteOfDay` 处理,和后端 scheduler 的 `time.Local` 对齐。
 *  - 落在窗口外的任务会被夹到最近一侧的最小宽度占位;窗口内的短任务同样保留
 *    最小宽度,但会回夹 left,避免右边界溢出轨道。
 */

import { useEffect, useState } from "preact/hooks";
import { TaskBlock } from "./TaskBlock";
import type { TimelineTask } from "./types";
import { t } from "../../utils/i18n";

export interface TimelineProps {
  tasks: TimelineTask[];
  dayStartHour?: number; // 默认 6
  dayEndHour?: number; // 默认 23
  /** 传入时任务块可点击（交给上层打开编辑弹窗）。 */
  onSelectTask?: (task: TimelineTask) => void;
}

function buildHourTicks(startHour: number, endHour: number): number[] {
  const out: number[] = [];
  for (let h = startHour; h <= endHour; h++) out.push(h);
  return out;
}

/**
 * 按视口宽度决定显示哪些整点刻度:窄屏抽稀避免 390px 下标签重叠(R6)。
 */
export function pickTickHours(
  viewportWidth: number,
  startHour = 6,
  endHour = 23,
): number[] {
  const hours = buildHourTicks(startHour, endHour);
  if (viewportWidth < 640) return hours.filter((h) => h % 6 === 0); // [6,12,18]
  if (viewportWidth < 1024) return hours.filter((h) => h % 3 === 0); // [6,9,12,15,18,21]
  return hours;
}

function pctOfWindow(min: number, startMin: number, endMin: number): number {
  const span = endMin - startMin;
  if (span <= 0) return 0;
  return ((min - startMin) / span) * 100;
}

export function Timeline({ tasks, dayStartHour = 6, dayEndHour = 23, onSelectTask }: TimelineProps) {
  const windowStartMin = dayStartHour * 60;
  const windowEndMin = dayEndHour * 60;

  const [viewportWidth, setViewportWidth] = useState(
    typeof window === "undefined" ? 1280 : window.innerWidth,
  );

  useEffect(() => {
    const onResize = () => setViewportWidth(window.innerWidth);
    window.addEventListener("resize", onResize);
    return () => window.removeEventListener("resize", onResize);
  }, []);

  const ticks = pickTickHours(viewportWidth, dayStartHour, dayEndHour);

  return (
    <div
      className="schedule-timeline relative w-full rounded-lg border border-base-300 bg-base-100 px-3 py-3"
      data-testid="timeline"
    >
      {/* 小时轴 */}
      <div className="timeline-axis relative mb-2 h-6 border-b border-base-300">
        {ticks.map((h) => {
          const pct = pctOfWindow(h * 60, windowStartMin, windowEndMin);
          // 首尾刻度贴着轨道两端:0% 左对齐、100% 右对齐,避免半个标签溢出卡片。
          const shift = pct <= 0 ? "0" : pct >= 100 ? "-100%" : "-50%";
          return (
            <span
              key={h}
              className="timeline-tick absolute top-0 text-[10px] opacity-60"
              style={{ left: `${pct}%`, transform: `translateX(${shift})` }}
            >
              {viewportWidth < 640
                ? String(h).padStart(2, "0")
                : String(h).padStart(2, "0") + ":00"}
            </span>
          );
        })}
      </div>

      {/* Task 轨 */}
      <div
        className="timeline-track relative h-16 rounded-lg bg-base-200/40"
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
            onSelect={onSelectTask}
          />
        ))}
        {tasks.length === 0 && (
          <div className="absolute inset-0 flex items-center justify-center text-xs opacity-60">
            {t("schedule.empty")}
          </div>
        )}
      </div>
    </div>
  );
}