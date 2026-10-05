/**
 * useAITasks —— AI GTD 页面状态管理。
 *
 * - capture(text):提交一条随手记,返回 task_id(任务以 pending 占位卡 prepend)
 * - refresh:重拉顶层任务列表
 * - archive(id):删除任务(乐观移除 + 409 回滚)
 * - toggleSubtask(taskId, subId, done):乐观更新子任务完成状态
 * - reparse(id):触发 AI 重新解析
 * - patchScores(taskId, fields, version):乐观锁重试写 5 维评分(409 拉新版本重发 1 次)
 *
 * 轮询:每 pollIntervalMs 检查所有 ai_status ∈ {pending, processing} 的任务,
 * 拉详情后合并进列表。
 */

import { useCallback, useEffect, useRef, useState } from "preact/hooks";
import { getAPIClient } from "../utils/apiClientSingleton";
import { VersionConflictError } from "../utils/apiClient";
import { logError } from "../utils/logger";
import type { TaskDTO } from "../types/aigtd";

/** patchScores 的可选字段集合(版本号由第三个参数单独传)。 */
export interface TaskScoreFields {
  priority_score?: number;
  urgency_score?: number;
  energy_required?: number;
  context_tag?: string;
}

export interface UseAITasksOptions {
  pollIntervalMs?: number;
}

export function useAITasks(opts: UseAITasksOptions = {}) {
  const pollIntervalMs = opts.pollIntervalMs ?? 2000;
  const [tasks, setTasks] = useState<TaskDTO[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const tasksRef = useRef<TaskDTO[]>([]);
  tasksRef.current = tasks;

  const refresh = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const list = await getAPIClient().listTasks();
      setTasks(list);
    } catch (err) {
      const msg = err instanceof Error ? err.message : String(err);
      setError(msg);
      logError("useAITasks.refresh failed", new Error(msg));
    } finally {
      setLoading(false);
    }
  }, []);

  const capture = useCallback(async (rawText: string): Promise<number> => {
    const trimmed = rawText.trim();
    if (!trimmed) throw new Error("empty");
    const { task_id } = await getAPIClient().captureAIGtd(trimmed);
    const placeholder: TaskDTO = {
      id: task_id,
      title: trimmed,
      raw_text: trimmed,
      status: "inbox",
      ai_status: "pending",
      pinned: false,
      estimated_minutes: 0,
      order_index: 0,
      source: "manual",
      version: 0,
      priority_score: 3,
      urgency_score: 5,
      energy_required: 2,
      context_tag: "general",
      subtasks: [],
      created_at: new Date().toISOString(),
      updated_at: new Date().toISOString(),
    };
    setTasks((prev) => [placeholder, ...prev.filter((t) => t.id !== task_id)]);
    return task_id;
  }, []);

  const archive = useCallback(async (id: number) => {
    const prev = tasksRef.current;
    const target = prev.find((t) => t.id === id);
    if (!target || typeof target.version !== "number") {
      // 没找到任务或无版本号,直接抛错而非静默发请求(避免中间件 400)
      throw new Error(`archive: task ${id} missing or has no version`);
    }
    setTasks((cur) => cur.filter((t) => t.id !== id));
    try {
      await getAPIClient().deleteTask(id, target.version);
    } catch (err) {
      const msg = err instanceof Error ? err.message : String(err);
      setError(msg);
      logError("useAITasks.archive failed", new Error(msg));
      setTasks(prev);
    }
  }, []);

  const toggleSubtask = useCallback(
    async (taskId: number, subId: number, done: boolean) => {
      const target = tasksRef.current.find((t) => t.id === taskId);
      if (!target || typeof target.version !== "number") {
        throw new Error(`toggleSubtask: task ${taskId} missing or has no version`);
      }
      setTasks((cur) =>
        cur.map((t) =>
          t.id !== taskId
            ? t
            : {
                ...t,
                subtasks: (t.subtasks ?? []).map((s) =>
                  s.id === subId ? { ...s, status: done ? "done" : "active" } : s,
                ),
              },
        ),
      );
      try {
        await getAPIClient().toggleSubtask(taskId, subId, done ? "done" : "active", target.version);
      } catch (err) {
        const msg = err instanceof Error ? err.message : String(err);
        setError(msg);
        logError("useAITasks.toggleSubtask failed", new Error(msg));
        // 回滚
        setTasks((cur) =>
          cur.map((t) =>
            t.id !== taskId
              ? t
              : {
                  ...t,
                  subtasks: (t.subtasks ?? []).map((s) =>
                    s.id === subId ? { ...s, status: done ? "active" : "done" } : s,
                  ),
                },
          ),
        );
      }
    },
    [],
  );

  const reparse = useCallback(async (id: number) => {
    const target = tasksRef.current.find((t) => t.id === id);
    if (!target || typeof target.version !== "number") {
      throw new Error(`reparse: task ${id} missing or has no version`);
    }
    setTasks((cur) =>
      cur.map((t) => (t.id === id ? { ...t, ai_status: "pending", ai_error: undefined } : t)),
    );
    try {
      await getAPIClient().reparseAIGtd(id, target.version);
    } catch (err) {
      const msg = err instanceof Error ? err.message : String(err);
      setError(msg);
      logError("useAITasks.reparse failed", new Error(msg));
    }
  }, []);

  /**
   * patchScores —— 5 维评分局部更新,带 409 有界重试。
   *
   * 流程:
   *  1) 以入参 version 调 patchTaskScores;
   *  2) 若返回 VersionConflictError: getTask(id) 拉最新版本,再调一次;
   *  3) 第二次仍 409(或其它错误)→ 抛出,不无限循环。
   * 成功时把 updated 合并进 tasks 列表。
   */
  const patchScores = useCallback(
    async (
      taskId: number,
      fields: TaskScoreFields,
      version: number,
    ): Promise<TaskDTO> => {
      const api = getAPIClient();
      let currentVersion = version;
      let updated: TaskDTO | null = null;
      for (let attempt = 0; attempt < 2; attempt++) {
        try {
          updated = await api.patchTaskScores(taskId, { version: currentVersion, ...fields });
          break;
        } catch (err) {
          if (err instanceof VersionConflictError && attempt === 0) {
            const fresh = await api.getTask(taskId);
            currentVersion = fresh.version;
            continue;
          }
          throw err;
        }
      }
      if (!updated) {
        throw new Error("patchScores: no result after retry");
      }
      const next = updated as TaskDTO;
      setTasks((cur) => cur.map((t) => (t.id === taskId ? next : t)));
      return next;
    },
    [],
  );

  // 首次加载
  useEffect(() => {
    void refresh();
  }, [refresh]);

  // 轮询 in-flight 任务
  useEffect(() => {
    const tick = async () => {
      const inFlight = tasksRef.current.filter(
        (t) => t.ai_status === "pending" || t.ai_status === "processing",
      );
      if (inFlight.length === 0) return;
      const api = getAPIClient();
      const updates = await Promise.all(
        inFlight.map(async (t) => {
          try {
            const job = await api.getAIGtdJob(t.id);
            const fresh = await api.getTask(t.id);
            return { id: t.id, fresh, ai_status: job.ai_status, ai_error: job.ai_error };
          } catch {
            return null;
          }
        }),
      );
      setTasks((cur) => {
        let next = cur;
        for (const u of updates) {
          if (!u) continue;
          next = next.map((t) =>
            t.id === u.id
              ? {
                  ...u.fresh,
                  ai_status: u.ai_status as TaskDTO["ai_status"],
                  ai_error: u.ai_error,
                }
              : t,
          );
        }
        return next;
      });
    };
    const h = setInterval(() => { void tick(); }, pollIntervalMs);
    return () => clearInterval(h);
  }, [pollIntervalMs]);

  return { tasks, topLevel: tasks, loading, error, refresh, capture, archive, toggleSubtask, reparse, patchScores };
}
