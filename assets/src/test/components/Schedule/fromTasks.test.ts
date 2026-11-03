import { describe, it, expect } from "vitest";
import { tasksScheduledOnDate } from "../../../components/Schedule/fromTasks";
import type { TaskDTO } from "../../../types/aigtd";

function at(day: number, hour: number, minute = 0): number {
  return Math.floor(new Date(2026, 10, day, hour, minute).getTime() / 1000);
}

function makeTask(overrides: Partial<TaskDTO> = {}): TaskDTO {
  return {
    id: 1,
    title: "任务",
    status: "active",
    pinned: false,
    estimated_minutes: 30,
    order_index: 0,
    source: "manual",
    ai_status: "done",
    version: 1,
    priority_score: 3,
    urgency_score: 5,
    energy_required: 2,
    context_tag: "general",
    scheduled_start: at(3, 9),
    scheduled_end: at(3, 10),
    created_at: "2026-11-03T00:00:00Z",
    updated_at: "2026-11-03T00:00:00Z",
    subtasks: [],
    ...overrides,
  };
}

describe("tasksScheduledOnDate", () => {
  it("只取所选日期（本地时区）已落库的排程", () => {
    const tasks = [
      makeTask({ id: 1 }),
      makeTask({ id: 2, scheduled_start: at(4, 9), scheduled_end: at(4, 10) }),
      makeTask({ id: 3, scheduled_start: undefined, scheduled_end: undefined }),
    ];

    const got = tasksScheduledOnDate(tasks, "2026-11-03");
    expect(got.map((t) => t.id)).toEqual([1]);
    expect(got[0].scheduled_start).toBe(at(3, 9));
  });

  it("同一天多个块按开始时间排序", () => {
    const tasks = [
      makeTask({ id: 1, scheduled_start: at(3, 15), scheduled_end: at(3, 16) }),
      makeTask({ id: 2, scheduled_start: at(3, 8), scheduled_end: at(3, 9) }),
    ];
    expect(tasksScheduledOnDate(tasks, "2026-11-03").map((t) => t.id)).toEqual([2, 1]);
  });

  it("跨日边界：当天 23:59 属于当天，次日 00:00 不属于", () => {
    const tasks = [
      makeTask({ id: 1, scheduled_start: at(3, 23, 59), scheduled_end: at(4, 0, 30) }),
      makeTask({ id: 2, scheduled_start: at(4, 0), scheduled_end: at(4, 1) }),
    ];
    expect(tasksScheduledOnDate(tasks, "2026-11-03").map((t) => t.id)).toEqual([1]);
  });
});
