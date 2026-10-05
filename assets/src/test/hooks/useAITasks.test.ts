import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, act, waitFor } from "@testing-library/preact";
import { useAITasks } from "../../hooks/useAITasks";
import { VersionConflictError } from "../../utils/apiClient";
import type { TaskDTO } from "../../types/aigtd";

const listTasks = vi.fn();
const captureAIGtd = vi.fn();
const getAIGtdJob = vi.fn();
const getTask = vi.fn();
const deleteTask = vi.fn();
const toggleSubtask = vi.fn();
const reparseAIGtd = vi.fn();
const patchTaskScores = vi.fn();

vi.mock("../../utils/apiClientSingleton", () => ({
  getAPIClient: () => ({
    listTasks,
    captureAIGtd,
    getAIGtdJob,
    getTask,
    deleteTask,
    toggleSubtask,
    reparseAIGtd,
    patchTaskScores,
  }),
}));

function makeTask(overrides: Partial<TaskDTO> = {}): TaskDTO {
  return {
    id: 7,
    title: "x",
    status: "active",
    ai_status: "done",
    pinned: false,
    estimated_minutes: 0,
    order_index: 0,
    source: "manual",
    version: 0,
    priority_score: 3,
    urgency_score: 5,
    energy_required: 2,
    context_tag: "general",
    created_at: "2026-10-04T00:00:00Z",
    updated_at: "2026-10-04T00:00:00Z",
    subtasks: [],
    ...overrides,
  };
}

describe("useAITasks", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    listTasks.mockResolvedValue([]);
    captureAIGtd.mockResolvedValue({ task_id: 42 });
    deleteTask.mockResolvedValue(undefined);
    toggleSubtask.mockResolvedValue(undefined);
    reparseAIGtd.mockResolvedValue({ job_id: 1 });
  });

  it("capture() 返回 task_id 并 prepend 一张 pending 占位卡", async () => {
    const { result } = renderHook(() => useAITasks({ pollIntervalMs: 60000 }));
    await waitFor(() => expect(listTasks).toHaveBeenCalled());

    let id = 0;
    await act(async () => {
      id = await result.current.capture("准备下周汇报");
    });

    expect(id).toBe(42);
    expect(captureAIGtd).toHaveBeenCalledWith("准备下周汇报");
    expect(result.current.tasks[0].id).toBe(42);
    expect(result.current.tasks[0].ai_status).toBe("pending");
  });

  it("archive() 乐观移除任务", async () => {
    listTasks.mockResolvedValue([
      {
        id: 7,
        title: "x",
        status: "active",
        ai_status: "done",
        pinned: false,
        estimated_minutes: 0,
        order_index: 0,
        source: "manual",
        version: 0,
        priority_score: 3,
        urgency_score: 5,
        energy_required: 2,
        context_tag: "general",
        created_at: "2026-10-04T00:00:00Z",
        updated_at: "2026-10-04T00:00:00Z",
        subtasks: [],
      },
    ]);
    const { result } = renderHook(() => useAITasks({ pollIntervalMs: 60000 }));
    await waitFor(() => expect(result.current.tasks.length).toBe(1));

    await act(async () => {
      await result.current.archive(7);
    });
    expect(deleteTask).toHaveBeenCalledWith(7, 0);
    expect(result.current.tasks.length).toBe(0);
  });

  it("toggleSubtask() 乐观更新子任务状态", async () => {
    listTasks.mockResolvedValue([
      {
        id: 7,
        title: "x",
        status: "active",
        ai_status: "done",
        pinned: false,
        estimated_minutes: 0,
        order_index: 0,
        source: "manual",
        version: 0,
        priority_score: 3,
        urgency_score: 5,
        energy_required: 2,
        context_tag: "general",
        created_at: "2026-10-04T00:00:00Z",
        updated_at: "2026-10-04T00:00:00Z",
        subtasks: [{ id: 1, task_id: 7, title: "a", status: "active", estimated_minutes: 20, order_index: 0 }],
      },
    ]);
    const { result } = renderHook(() => useAITasks({ pollIntervalMs: 60000 }));
    await waitFor(() => expect(result.current.tasks.length).toBe(1));

    await act(async () => {
      await result.current.toggleSubtask(7, 1, true);
    });
    expect(toggleSubtask).toHaveBeenCalledWith(7, 1, "done", 0);
    expect(result.current.tasks[0].subtasks?.[0].status).toBe("done");
  });

  describe("patchScores() 409 有界重试", () => {
    it("成功路径: 直接调 patchTaskScores 并合并结果, 不拉 getTask", async () => {
      const updated = makeTask({ version: 4, priority_score: 5 });
      patchTaskScores.mockResolvedValue(updated);
      const { result } = renderHook(() => useAITasks({ pollIntervalMs: 60000 }));
      await waitFor(() => expect(listTasks).toHaveBeenCalled());

      let out: TaskDTO | undefined;
      await act(async () => {
        out = await result.current.patchScores(7, { priority_score: 5 }, 3);
      });

      expect(patchTaskScores).toHaveBeenCalledTimes(1);
      expect(patchTaskScores).toHaveBeenCalledWith(7, { version: 3, priority_score: 5 });
      expect(getTask).not.toHaveBeenCalled();
      expect(out).toEqual(updated);
    });

    it("409 → 拉最新 version → 重试成功", async () => {
      const updated = makeTask({ version: 9, priority_score: 5 });
      patchTaskScores
        .mockRejectedValueOnce(new VersionConflictError())
        .mockResolvedValueOnce(updated);
      getTask.mockResolvedValue(makeTask({ version: 8 }));
      const { result } = renderHook(() => useAITasks({ pollIntervalMs: 60000 }));
      await waitFor(() => expect(listTasks).toHaveBeenCalled());

      let out: TaskDTO | undefined;
      await act(async () => {
        out = await result.current.patchScores(7, { priority_score: 5 }, 3);
      });

      expect(getTask).toHaveBeenCalledWith(7);
      expect(patchTaskScores).toHaveBeenCalledTimes(2);
      expect(patchTaskScores).toHaveBeenNthCalledWith(1, 7, { version: 3, priority_score: 5 });
      expect(patchTaskScores).toHaveBeenNthCalledWith(2, 7, { version: 8, priority_score: 5 });
      expect(out).toEqual(updated);
    });

    it("409 → 拉最新 version → 重试仍 409 → 抛出 VersionConflictError", async () => {
      patchTaskScores.mockRejectedValue(new VersionConflictError());
      getTask.mockResolvedValue(makeTask({ version: 8 }));
      const { result } = renderHook(() => useAITasks({ pollIntervalMs: 60000 }));
      await waitFor(() => expect(listTasks).toHaveBeenCalled());

      await act(async () => {
        await expect(result.current.patchScores(7, { priority_score: 5 }, 3)).rejects.toBeInstanceOf(
          VersionConflictError,
        );
      });

      expect(getTask).toHaveBeenCalledTimes(1);
      // 有界:仅重试 1 次,绝不无限循环
      expect(patchTaskScores).toHaveBeenCalledTimes(2);
    });
  });
});
