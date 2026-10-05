import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, act, waitFor } from "@testing-library/preact";
import { useAITasks } from "../../hooks/useAITasks";

const listTasks = vi.fn();
const captureAIGtd = vi.fn();
const getAIGtdJob = vi.fn();
const getTask = vi.fn();
const deleteTask = vi.fn();
const toggleSubtask = vi.fn();
const reparseAIGtd = vi.fn();

vi.mock("../../utils/apiClientSingleton", () => ({
  getAPIClient: () => ({
    listTasks,
    captureAIGtd,
    getAIGtdJob,
    getTask,
    deleteTask,
    toggleSubtask,
    reparseAIGtd,
  }),
}));

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
});
