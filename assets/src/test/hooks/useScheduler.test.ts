import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, act, waitFor } from "@testing-library/preact";
import { useScheduler } from "../../hooks/useScheduler";
import { VersionConflictError } from "../../utils/apiClient";
import type { SchedulerPlan, SchedulerPlacement, TaskDTO } from "../../types/aigtd";

const mocks = vi.hoisted(() => ({
  runSchedulePreview: vi.fn(),
  applySchedulePlacements: vi.fn(),
  listTasks: vi.fn(),
}));

vi.mock("../../utils/apiClientSingleton", () => ({
  getAPIClient: () => mocks,
}));

vi.mock("../../utils/i18n", () => ({
  t: (key: string, params?: Record<string, string | number>) => {
    let out = key;
    if (params) {
      for (const [k, v] of Object.entries(params)) out = out.replace(`{${k}}`, String(v));
    }
    return out;
  },
}));

function makeTask(overrides: Partial<TaskDTO> = {}): TaskDTO {
  return {
    id: 1,
    title: "t",
    status: "active",
    ai_status: "done",
    pinned: false,
    estimated_minutes: 30,
    order_index: 0,
    source: "manual",
    version: 5,
    priority_score: 3,
    urgency_score: 5,
    energy_required: 2,
    context_tag: "general",
    created_at: "2026-10-05T00:00:00Z",
    updated_at: "2026-10-05T00:00:00Z",
    subtasks: [],
    ...overrides,
  };
}

const emptyPlan: SchedulerPlan = {
  Date: "2026-10-05",
  Placements: [],
  Unscheduled: [],
  Warnings: [],
};

const placement: SchedulerPlacement = {
  TaskID: 1,
  ScheduledStart: 1735689600,
  ScheduledEnd: 1735691400,
  Reason: "test",
};

describe("useScheduler", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mocks.listTasks.mockResolvedValue([makeTask()]);
  });

  it("runPreview() calls the preview API and stores the returned plan", async () => {
    mocks.runSchedulePreview.mockResolvedValue(emptyPlan);
    const { result } = renderHook(() => useScheduler());
    await waitFor(() => expect(mocks.listTasks).toHaveBeenCalled());

    await act(async () => {
      await result.current.runPreview("2026-10-05");
    });

    expect(mocks.runSchedulePreview).toHaveBeenCalledWith("2026-10-05");
    expect(result.current.plan).toEqual(emptyPlan);
    expect(result.current.error).toBeNull();
  });

  it("runPreview() surfaces API errors without throwing", async () => {
    mocks.runSchedulePreview.mockRejectedValue(new Error("boom"));
    const { result } = renderHook(() => useScheduler());
    await waitFor(() => expect(mocks.listTasks).toHaveBeenCalled());

    await act(async () => {
      await result.current.runPreview("2026-10-05");
    });

    expect(result.current.error).toContain("boom");
    expect(result.current.plan).toBeNull();
  });

  it("apply() maps placements to the snake_case request with the task version", async () => {
    mocks.listTasks.mockResolvedValue([makeTask({ id: 1, version: 7 })]);
    mocks.runSchedulePreview.mockResolvedValue({ ...emptyPlan, Placements: [placement] });
    mocks.applySchedulePlacements.mockResolvedValue({ applied: 1 });

    const { result } = renderHook(() => useScheduler());
    await waitFor(() => expect(mocks.listTasks).toHaveBeenCalled());

    await act(async () => {
      await result.current.runPreview("2026-10-05");
      await result.current.apply([placement]);
    });

    expect(mocks.applySchedulePlacements).toHaveBeenCalledWith([
      {
        task_id: 1,
        scheduled_start: 1735689600,
        scheduled_end: 1735691400,
        version: 7,
      },
    ]);
    expect(result.current.conflictMessage).toBeNull();
  });

  it("apply() on 409 sets conflictMessage and refetches the preview", async () => {
    mocks.runSchedulePreview.mockResolvedValue(emptyPlan);
    mocks.applySchedulePlacements.mockRejectedValue(new VersionConflictError());

    const { result } = renderHook(() => useScheduler());
    await waitFor(() => expect(mocks.listTasks).toHaveBeenCalled());

    await act(async () => {
      await result.current.runPreview("2026-10-05");
    });
    expect(mocks.runSchedulePreview).toHaveBeenCalledTimes(1);

    await act(async () => {
      await result.current.apply([placement]);
    });

    expect(result.current.conflictMessage).toBeTruthy();
    // 409 → re-run preview to refresh stale data
    expect(mocks.runSchedulePreview).toHaveBeenCalledTimes(2);
  });

  it("apply() reports an error when a placement has no known version", async () => {
    mocks.listTasks.mockResolvedValue([]); // no tasks -> no version for TaskID 1
    mocks.runSchedulePreview.mockResolvedValue(emptyPlan);

    const { result } = renderHook(() => useScheduler());
    await waitFor(() => expect(mocks.listTasks).toHaveBeenCalled());

    await act(async () => {
      await result.current.apply([placement]);
    });

    expect(mocks.applySchedulePlacements).not.toHaveBeenCalled();
    expect(result.current.error).toBeTruthy();
  });
});