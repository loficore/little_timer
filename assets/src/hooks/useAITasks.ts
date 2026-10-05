/**
 * useAITasks —— AI GTD 页面状态管理。
 *
 * - capture(text):提交一条随手记,返回 task_id(任务以 pending 占位卡 prepend)
 * - refresh:重拉顶层任务列表
 * - archive(id):删除任务
 * - toggleSubtask(taskId, subId, done):乐观更新子任务完成状态
 * - reparse(id):触发 AI 重新解析
 *
 * 轮询:每 pollIntervalMs 检查所有 ai_status ∈ {pending, processing} 的任务,
 * 拉详情后合并进列表。
 */

import { useCallback, useEffect, useRef, useState } from "preact/hooks";
import { getAPIClient } from "../utils/apiClientSingleton";
import { logError } from "../utils/logger";
import type { TaskDTO } from "../types/aigtd";

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
      subtasks: [],
      created_at: new Date().toISOString(),
      updated_at: new Date().toISOString(),
    };
    setTasks((prev) => [placeholder, ...prev.filter((t) => t.id !== task_id)]);
    return task_id;
  }, []);

  const archive = useCallback(async (id: number) => {
    const prev = tasksRef.current;
    setTasks((cur) => cur.filter((t) => t.id !== id));
    try {
      await getAPIClient().deleteTask(id);
    } catch (err) {
      const msg = err instanceof Error ? err.message : String(err);
      setError(msg);
      logError("useAITasks.archive failed", new Error(msg));
      setTasks(prev);
    }
  }, []);

  const toggleSubtask = useCallback(
    async (taskId: number, subId: number, done: boolean) => {
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
        await getAPIClient().toggleSubtask(taskId, subId, done ? "done" : "active");
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
    setTasks((cur) =>
      cur.map((t) => (t.id === id ? { ...t, ai_status: "pending", ai_error: undefined } : t)),
    );
    try {
      await getAPIClient().reparseAIGtd(id);
    } catch (err) {
      const msg = err instanceof Error ? err.message : String(err);
      setError(msg);
      logError("useAITasks.reparse failed", new Error(msg));
    }
  }, []);

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

  return { tasks, topLevel: tasks, loading, error, refresh, capture, archive, toggleSubtask, reparse };
}
