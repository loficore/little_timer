/**
 * useScheduler —— SchedulePage 状态管理。
 *
 * - runPreview(date):POST /api/schedule/run (mode=preview) → setPlan
 * - apply(placements):POST /api/schedule/apply,把 plan 的 placements 显式落库。
 *   409(乐观锁冲突逐条回滚)→ refetch tasks + 重跑 preview,并设 conflictMessage。
 * - tasks:当前任务列表(含 version / 评分),供右侧分数面板与 apply 版本查找。
 *
 * 关键决策:后端 `/api/schedule/run` 返回 Go 默认 JSON 键(PascalCase:
 * `Placements` / `TaskID` / `ScheduledStart` ...),`/api/schedule/apply` 的请求
 * 体却是显式 snake_case json tag。本 hook 在边界处把 PascalCase 的 Placement
 * 映射成 snake_case 的 ScheduleApplyPlacement,并为每条补充当前 task version。
 */

import { useCallback, useEffect, useRef, useState } from "preact/hooks";
import { getAPIClient } from "../utils/apiClientSingleton";
import { VersionConflictError } from "../utils/apiClient";
import { logError } from "../utils/logger";
import { t } from "../utils/i18n";
import type {
  Plan,
  SchedulerPlacement,
  ScheduleApplyPlacement,
  TaskDTO,
} from "../types/aigtd";

export interface UseScheduler {
  /** 最近一次 preview 的计划。 */
  plan: Plan | null;
  /** 当前任务列表(含 version / 5 维评分),供右侧面板 + apply 查版本。 */
  tasks: TaskDTO[];
  loading: boolean;
  error: string | null;
  /** apply 遇 409 时置位;成功/重新预览后清空。 */
  conflictMessage: string | null;
  /** 预览某天的调度计划。 */
  runPreview: (date: string) => Promise<void>;
  /** 将 preview 的 placements 显式落库(带 task version)。 */
  apply: (placements: SchedulerPlacement[]) => Promise<void>;
  /** 重新拉取任务列表。 */
  refresh: () => Promise<void>;
}

export function useScheduler(): UseScheduler {
  const [plan, setPlan] = useState<Plan | null>(null);
  const [tasks, setTasks] = useState<TaskDTO[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [conflictMessage, setConflictMessage] = useState<string | null>(null);

  const tasksRef = useRef<TaskDTO[]>([]);
  tasksRef.current = tasks;
  const lastDateRef = useRef<string | null>(null);

  const refresh = useCallback(async () => {
    try {
      const list = await getAPIClient().listTasks();
      setTasks(list);
    } catch (err) {
      const msg = err instanceof Error ? err.message : String(err);
      setError(msg);
      logError("useScheduler.refresh failed", new Error(msg));
    }
  }, []);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  const runPreview = useCallback(async (date: string) => {
    lastDateRef.current = date;
    setLoading(true);
    setError(null);
    try {
      const next = await getAPIClient().runSchedulePreview(date);
      setPlan(next);
      setConflictMessage(null);
    } catch (err) {
      const msg = err instanceof Error ? err.message : String(err);
      setError(msg);
      logError("useScheduler.runPreview failed", new Error(msg));
    } finally {
      setLoading(false);
    }
  }, []);

  const apply = useCallback(async (placements: SchedulerPlacement[]) => {
    // 1) 从当前 task 列表查 version(apply 请求必须带乐观锁版本)。
    const versionById = new Map<number, number>();
    for (const task of tasksRef.current) {
      if (typeof task.version === "number") versionById.set(task.id, task.version);
    }

    const payload: ScheduleApplyPlacement[] = [];
    for (const p of placements) {
      const version = versionById.get(p.TaskID);
      if (typeof version !== "number") {
        setError(t("schedule.no_version", { id: p.TaskID }));
        return;
      }
      payload.push({
        task_id: p.TaskID,
        scheduled_start: p.ScheduledStart,
        scheduled_end: p.ScheduledEnd,
        version,
      });
    }
    if (payload.length === 0) {
      setError(t("schedule.empty"));
      return;
    }

    setLoading(true);
    setError(null);
    try {
      await getAPIClient().applySchedulePlacements(payload);
      setConflictMessage(null);
    } catch (err) {
      if (err instanceof VersionConflictError) {
        // 409:事务已整体回滚,本地数据 stale → 刷新任务 + 重跑 preview。
        setConflictMessage(t("schedule.conflict"));
        await refresh();
        const date = lastDateRef.current;
        if (date) {
          try {
            const next = await getAPIClient().runSchedulePreview(date);
            setPlan(next);
          } catch (refreshErr) {
            const reason =
              refreshErr instanceof Error ? refreshErr.message : String(refreshErr);
            logError("useScheduler.apply refresh preview failed", new Error(reason));
          }
        }
      } else {
        const msg = err instanceof Error ? err.message : String(err);
        setError(msg);
        logError("useScheduler.apply failed", new Error(msg));
      }
    } finally {
      setLoading(false);
    }
  }, [refresh]);

  return {
    plan,
    tasks,
    loading,
    error,
    conflictMessage,
    runPreview,
    apply,
    refresh,
  };
}