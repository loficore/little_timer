/**
 * SchedulePage —— 一天的调度计划预览 + 落库页面。
 *
 * 结构:
 *  - 顶部 toolbar:日期选择器 + 左右日切换 + Run Scheduler + Apply All
 *  - 左侧主区:用 plan.Placements + 当前 tasks 拼成 TimelineTask[],交给 Timeline
 *  - 右侧 aside:任务列表 + 5 维评分摘要;点开后内嵌 ScoreEditPanel 做评分编辑
 *
 * 错误/冲突反馈:
 *  - useScheduler.error → 红色 alert(API 异常)
 *  - useScheduler.conflictMessage → 黄色 alert(乐观锁冲突 → 已自动 refetch + re-preview)
 *
 * 注意:页面本身不做 fetch;所有 IO 集中在 useScheduler(),便于复用 + 单测。
 */

import { useMemo, useState } from "preact/hooks";
import { Timeline } from "./components/Schedule/Timeline";
import type { TimelineTask } from "./components/Schedule/types";
import { ScoreEditPanel } from "./components/settings/ScoreEditPanel";
import { useScheduler } from "./hooks/useScheduler";
import { t } from "./utils/i18n";

/** 返回本地时区下的 YYYY-MM-DD。 */
function todayIso(): string {
  const d = new Date();
  const y = d.getFullYear();
  const m = String(d.getMonth() + 1).padStart(2, "0");
  const day = String(d.getDate()).padStart(2, "0");
  return `${y}-${m}-${day}`;
}

/** 在本地时区上把日期移动 days 天,返回 YYYY-MM-DD。 */
function shiftDate(iso: string, days: number): string {
  const [y, m, d] = iso.split("-").map((s) => Number(s));
  if (!y || !m || !d) return iso;
  const date = new Date(y, m - 1, d);
  date.setDate(date.getDate() + days);
  const yy = date.getFullYear();
  const mm = String(date.getMonth() + 1).padStart(2, "0");
  const dd = String(date.getDate()).padStart(2, "0");
  return `${yy}-${mm}-${dd}`;
}

export function SchedulePage() {
  const { plan, tasks, loading, error, conflictMessage, runPreview, apply, refresh: refreshTasks } =
    useScheduler();
  const [date, setDate] = useState<string>(todayIso());
  const [editingId, setEditingId] = useState<number | null>(null);

  const timelineTasks: TimelineTask[] = useMemo(() => {
    if (!plan) return [];
    const byId = new Map(tasks.map((t) => [t.id, t]));
    return plan.Placements.map((p) => {
      const task = byId.get(p.TaskID);
      return {
        id: p.TaskID,
        title: task?.title ?? `#${p.TaskID}`,
        priority_score: task?.priority_score ?? 3,
        scheduled_start: p.ScheduledStart,
        scheduled_end: p.ScheduledEnd,
        context_tag: task?.context_tag,
      };
    });
  }, [plan, tasks]);

  const placementCount = plan?.Placements.length ?? 0;
  const unscheduledCount = plan?.Unscheduled.length ?? 0;

  return (
    <div className="container mx-auto max-w-5xl space-y-3 p-4" data-testid="schedule-page">
      <h1 className="text-2xl font-bold">{t("schedule.title")}</h1>

      <div className="flex flex-wrap items-center gap-2" data-testid="schedule-toolbar">
        <button
          type="button"
          className="btn btn-sm btn-ghost"
          data-testid="prev-day"
          aria-label={t("schedule.prev_day")}
          onClick={() => setDate((d) => shiftDate(d, -1))}
        >
          ‹
        </button>
        <input
          type="date"
          className="input input-bordered input-sm"
          data-testid="schedule-date"
          value={date}
          onInput={(e) => setDate((e.currentTarget as HTMLInputElement).value)}
        />
        <button
          type="button"
          className="btn btn-sm btn-ghost"
          data-testid="next-day"
          aria-label={t("schedule.next_day")}
          onClick={() => setDate((d) => shiftDate(d, 1))}
        >
          ›
        </button>
        <button
          type="button"
          className="btn btn-sm btn-primary"
          data-testid="run-scheduler"
          disabled={loading}
          onClick={() => void runPreview(date)}
        >
          {loading ? t("button.saving") : t("schedule.run")}
        </button>
        {plan && (
          <button
            type="button"
            className="btn btn-sm btn-success"
            data-testid="apply-all"
            disabled={loading || placementCount === 0}
            onClick={() => void apply(plan.Placements)}
          >
            {t("schedule.applyAll")}
          </button>
        )}
        <span className="ml-auto text-xs opacity-70" data-testid="schedule-summary">
          {plan
            ? t("schedule.summary", {
                placed: placementCount,
                unscheduled: unscheduledCount,
              })
            : ""}
        </span>
      </div>

      {error && (
        <div className="alert alert-error" role="alert" data-testid="schedule-error">
          {error}
        </div>
      )}
      {conflictMessage && (
        <div className="alert alert-warning" role="alert" data-testid="schedule-conflict">
          {conflictMessage}
        </div>
      )}
      {plan && plan.Warnings.length > 0 && (
        <div className="alert alert-info" role="alert" data-testid="schedule-warnings">
          <ul className="list-disc pl-5">
            {plan.Warnings.map((w, i) => (
              <li key={i}>{w}</li>
            ))}
          </ul>
        </div>
      )}

      <div className="grid gap-3 lg:grid-cols-[1fr_18rem]">
        <Timeline tasks={timelineTasks} />

        <section
          className="rounded border border-base-300 bg-base-100 p-3"
          data-testid="schedule-tasks"
        >
          <h2 className="mb-2 font-semibold">{t("schedule.tasks")}</h2>
          {tasks.length === 0 && (
            <p className="text-xs opacity-60">{t("schedule.empty")}</p>
          )}
          <ul className="space-y-2">
            {tasks.map((task) => {
              const isEditing = editingId === task.id;
              return (
                <li
                  key={task.id}
                  className="rounded border border-base-200 p-2 text-sm"
                  data-testid={`schedule-task-${task.id}`}
                >
                  <div className="flex items-center justify-between gap-2">
                    <span className="truncate" title={task.title}>{task.title}</span>
                    <span className="badge badge-sm">P{task.priority_score}</span>
                  </div>
                  <div className="mt-1 text-xs opacity-70">
                    {t("score.urgency")} {task.urgency_score} · {t("score.energy")}{" "}
                    {task.energy_required} · {task.context_tag}
                  </div>
                  <button
                    type="button"
                    className="btn btn-xs btn-ghost mt-1"
                    data-testid={`edit-task-${task.id}`}
                    onClick={() => setEditingId(isEditing ? null : task.id)}
                  >
                    {isEditing ? t("button.cancel") : t("aigtd.action_edit")}
                  </button>
                  {isEditing && (
                    <div className="mt-2">
                      <ScoreEditPanel
                        taskId={task.id}
                        current={{
                          priority_score: task.priority_score,
                          urgency_score: task.urgency_score,
                          energy_required: task.energy_required,
                          context_tag: task.context_tag,
                        }}
                        version={task.version}
                        onUpdated={() => {
                          void refreshTasks();
                        }}
                      />
                    </div>
                  )}
                </li>
              );
            })}
          </ul>
        </section>
      </div>
    </div>
  );
}