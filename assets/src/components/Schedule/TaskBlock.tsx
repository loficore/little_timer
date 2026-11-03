/**
 * TaskBlock —— Timeline 中的单个任务块。
 *
 * 视觉:
 *  - 优先级 1-5 映射到 `my-priority-N`(单一主色相的明度阶梯,见 surfaces.css)。
 *  - context_tag 决定左边 tint(`context-writing` / `context-coding` / …)。
 *  - 内部渲染 title + 优先级徽章 + 时长。
 *
 * 定位由父组件 Timeline 计算(像素百分比 style),TaskBlock 只负责样式映射
 * 与内容渲染。这样 TaskBlock 可以在不依赖 Timeline 内部 window 算法的情况下
 * 独立测试。
 */

import type { TimelineTask } from "./types";
import { t } from "../../utils/i18n";

interface TaskBlockProps {
  task: TimelineTask;
  /** Timeline 的窗口起始分钟(相对当天 00:00,local)。 */
  windowStartMin: number;
  /** Timeline 的窗口结束分钟(相对当天 00:00,local)。 */
  windowEndMin: number;
  /** 传入时时间块可点击（打开编辑）；不传则只读。 */
  onSelect?: (task: TimelineTask) => void;
}

const PRIORITY_STYLE: Record<number, string> = {
  1: "my-priority-1",
  2: "my-priority-2",
  3: "my-priority-3",
  4: "my-priority-4",
  5: "my-priority-5",
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

/** 把 unix 秒换算成「本地时区当天第几分钟」(0..1439)。 */
export function localMinuteOfDay(sec: number): number {
  const d = new Date(sec * 1000);
  return d.getHours() * 60 + d.getMinutes();
}

function positionStyle(
  startSec: number,
  endSec: number,
  windowStartMin: number,
  windowEndMin: number,
): { left: string; width: string } {
  const span = windowEndMin - windowStartMin;
  if (span <= 0) return { left: "0%", width: "2%" };

  // scheduled_start/end 是 unix 秒(绝对时刻),轨道坐标是「当天第几分钟」。
  // 不做这一步换算的话,epoch 分钟永远大于窗口右界,所有任务都会塌到最左边
  // 变成 2% 的细条(线上真实出现过)。
  const startMin = localMinuteOfDay(startSec);
  let endMin = localMinuteOfDay(endSec);
  if (endMin <= startMin) {
    // 跨午夜,或上游给了零长度:按真实时长补足,避免宽度塌成 0。
    endMin = startMin + Math.max(1, Math.round((endSec - startSec) / 60));
  }

  if (endMin <= windowStartMin || startMin >= windowEndMin) {
    // 任务完全落在窗口外:贴最近的一侧画一条最小宽度,提示"排到窗口外了"。
    return startMin >= windowEndMin
      ? { left: "98.5%", width: "1.5%" }
      : { left: "0%", width: "1.5%" };
  }

  const start = Math.max(startMin, windowStartMin);
  const end = Math.min(endMin, windowEndMin);
  // 最小宽度用于保证短任务可见,但它可能把右边界顶出轨道:
  // 回夹 left,确保 left + width 不超过 100%。
  const widthPct = Math.max(1.5, ((end - start) / span) * 100);
  const leftPct = Math.min(((start - windowStartMin) / span) * 100, 100 - widthPct);
  return {
    left: `${leftPct.toFixed(3)}%`,
    width: `${widthPct.toFixed(3)}%`,
  };
}

export function TaskBlock({ task, windowStartMin, windowEndMin, onSelect }: TaskBlockProps) {
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

  const interactive = typeof onSelect === "function";

  return (
    <div
      className={`timeline-task-block priority-${priority} ${tint} ${priorityClasses} absolute top-1 bottom-1 rounded-lg px-2 py-1 border text-xs overflow-hidden flex items-center gap-1${
        interactive ? " cursor-pointer hover:brightness-110 focus-visible:outline focus-visible:outline-2 focus-visible:outline-[var(--accent-color)]" : ""
      }`}
      data-task-id={task.id}
      data-testid={`task-block-${task.id}`}
      data-context={task.context_tag ?? ""}
      style={{ left: pos.left, width: pos.width }}
      role={interactive ? "button" : undefined}
      tabIndex={interactive ? 0 : undefined}
      aria-label={interactive ? t("schedule.edit_block", { title: task.title }) : undefined}
      onClick={interactive ? () => onSelect(task) : undefined}
      onKeyDown={
        interactive
          ? (e) => {
              if (e.key === "Enter" || e.key === " ") {
                e.preventDefault();
                onSelect(task);
              }
            }
          : undefined
      }
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