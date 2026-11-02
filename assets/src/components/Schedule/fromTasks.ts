import type { TaskDTO } from "../../types/aigtd";
import type { TimelineTask } from "./types";

/** 把 `YYYY-MM-DD` + 本地时区解析成当天 00:00 的 unix 秒。 */
function localDayStartUnix(date: string): number | null {
  const [y, m, d] = date.split("-").map((s) => Number(s));
  if (!y || !m || !d) return null;
  return Math.floor(new Date(y, m - 1, d, 0, 0, 0, 0).getTime() / 1000);
}

/**
 * 取「某一天（本地时区）」已落库的排程任务，转成 Timeline 的展示形状。
 *
 * 用于没有 AI 预览草稿时的默认视图：刷新页面后仍然能看到当天已应用的
 * 时间块（否则计划页只有点过「运行调度」才有内容）。
 */
export function tasksScheduledOnDate(tasks: TaskDTO[], date: string): TimelineTask[] {
  const dayStart = localDayStartUnix(date);
  if (dayStart === null) return [];
  const [y, m, d] = date.split("-").map((s) => Number(s));
  const dayEnd = Math.floor(new Date(y, m - 1, d + 1, 0, 0, 0, 0).getTime() / 1000);

  return tasks
    .filter(
      (t) =>
        typeof t.scheduled_start === "number" &&
        typeof t.scheduled_end === "number" &&
        t.scheduled_start >= dayStart &&
        t.scheduled_start < dayEnd,
    )
    .map((t) => ({
      id: t.id,
      title: t.title || t.raw_text || `#${t.id}`,
      priority_score: t.priority_score,
      scheduled_start: t.scheduled_start as number,
      scheduled_end: t.scheduled_end as number,
      context_tag: t.context_tag,
    }))
    .sort((a, b) => a.scheduled_start - b.scheduled_start);
}
