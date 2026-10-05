/**
 * TaskBlock —— Timeline 中的单个任务块。
 *
 * 视觉:
 *  - 优先级 1-5 映射到 5 个明显不同的背景色(daisyUI/Tailwind utility)。
 *  - context_tag 决定左边 tint(`context-writing` / `context-coding` / …)。
 *  - 内部渲染 title + 优先级徽章 + 时长。
 *
 * 定位由父组件 Timeline 计算(像素百分比 style),TaskBlock 只负责样式映射
 * 与内容渲染。这样 TaskBlock 可以在不依赖 Timeline 内部 window 算法的情况下
 * 独立测试。
 */

import type { TimelineTask } from "./types";

interface TaskBlockProps {
  task: TimelineTask;
  /** Timeline 的窗口起始分钟(相对当天 00:00,local)。 */
  windowStartMin: number;
  /** Timeline 的窗口结束分钟(相对当天 00:00,local)。 */
  windowEndMin: number;
}

const PRIORITY_STYLE: Record<number, string> = {
  1: "bg-blue-200 text-blue-950 border-blue-300",
  2: "bg-cyan-300 text-cyan-950 border-cyan-400",
  3: "bg-amber-300 text-amber-950 border-amber-400",
  4: "bg-orange-400 text-white border-orange-500",
  5: "bg-red-600 text-white border-red-700",
};

const CONTEXT_TINT: Record<string, string> = {
  写作: "context-writing",
  编码: "context-coding",
  沟通: "context-communication",
  学习: "context-study",
  杂事: "context-chores",
  general: "context-general",
};

function clampPriority(p: number): number {
  if (!Number.isFinite(p)) return 3;
  if (p < 1) return 1;
  if (p > 5) return 5;
  return Math.round(p);
}

function durationMinutes(startSec: number, endSec: number): number {
  const min = Math.round((endSec - startSec) / 60);
  return min > 0 ? min : 0;
}

function positionStyle(
  startSec: number,
  endSec: number,
  windowStartMin: number,
  windowEndMin: number,
): { left: string; width: string } {
  const taskStartMin = (startSec / 60);
  const taskEndMin = (endSec / 60);
  const start = Math.max(taskStartMin, windowStartMin);
  const end = Math.min(taskEndMin, windowEndMin);
  const span = windowEndMin - windowStartMin;
  if (span <= 0 || end <= start) {
    // 任务完全落在窗口外:贴左画最小宽度,确保可被定位到,可见。
    return { left: "0%", width: "2%" };
  }
  const leftPct = ((start - windowStartMin) / span) * 100;
  const widthPct = ((end - start) / span) * 100;
  return {
    left: `${leftPct.toFixed(3)}%`,
    width: `${Math.max(1.5, widthPct).toFixed(3)}%`,
  };
}

export function TaskBlock({ task, windowStartMin, windowEndMin }: TaskBlockProps) {
  const priority = clampPriority(task.priority_score);
  const priorityClasses = PRIORITY_STYLE[priority] ?? PRIORITY_STYLE[3];
  const tint = CONTEXT_TINT[task.context_tag ?? ""] ?? "context-default";
  const duration = durationMinutes(task.scheduled_start, task.scheduled_end);
  const pos = positionStyle(
    task.scheduled_start,
    task.scheduled_end,
    windowStartMin,
    windowEndMin,
  );

  return (
    <div
      className={`timeline-task-block priority-${priority} ${tint} ${priorityClasses} absolute top-1 bottom-1 rounded px-2 py-1 border text-xs overflow-hidden flex items-center gap-1`}
      data-task-id={task.id}
      data-testid={`task-block-${task.id}`}
      data-context={task.context_tag ?? ""}
      style={{ left: pos.left, width: pos.width }}
    >
      <span className="truncate font-medium">{task.title}</span>
      <span
        className="ml-auto shrink-0 rounded bg-black/10 px-1 text-[10px] font-semibold"
        data-testid="priority-badge"
      >
        P{priority}
      </span>
      <span className="sr-only">{`duration ${duration} minutes`}</span>
      <span
        className="shrink-0 rounded bg-black/10 px-1 text-[10px]"
        data-testid="duration-badge"
      >
        {duration}m
      </span>
    </div>
  );
}