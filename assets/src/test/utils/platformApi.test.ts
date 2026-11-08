import { describe, it, expect } from "vitest";
import { capabilities, UnsupportedOnPlatformError } from "../../utils/platformApi";
import { APIClient } from "../../utils/apiClient";
import type { PlatformApi } from "../../utils/platformApi";
import type {
  Habit,
  HabitSet,
  Session,
  HabitDetail,
} from "../../types/habit";
import type {
  Habit as ApiHabit,
  HabitSet as ApiHabitSet,
  Session as ApiSession,
  HabitDetail as ApiHabitDetail,
} from "../../types/api";

describe("platformApi", () => {
  it("桌面环境启用全部能力", () => {
    expect(capabilities.timer).toBe(true);
    expect(capabilities.habits).toBe(true);
    expect(capabilities.tasks).toBe(true);
    expect(capabilities.schedule).toBe(true);
  });

  it("UnsupportedOnPlatformError 暴露稳定 name", () => {
    const err = new UnsupportedOnPlatformError("AIGTD");
    expect(err.name).toBe("UnsupportedOnPlatformError");
    expect(err.message).toContain("AIGTD");
  });

  it("APIClient 满足 PlatformApi", () => {
    const client: PlatformApi = new APIClient("http://localhost");
    expect(client).toBeInstanceOf(APIClient);
  });

  // types/habit.ts 只做 re-export，两类名指向同一份声明，必须可互换使用。
  it("habit 类型与 api 类型可互换", () => {
    // 双向赋值成立，说明消费方从任一模块导入都不会制造 DTO 漂移。
    const habit: Habit = {} as ApiHabit;
    const apiHabit: ApiHabit = habit;
    const habitSet: HabitSet = {} as ApiHabitSet;
    const apiHabitSet: ApiHabitSet = habitSet;
    const session: Session = {} as ApiSession;
    const apiSession: ApiSession = session;
    const habitDetail: HabitDetail = {} as ApiHabitDetail;
    const apiHabitDetail: ApiHabitDetail = habitDetail;

    expect(apiHabit).toBe(habit);
    expect(apiHabitSet).toBe(habitSet);
    expect(apiSession).toBe(session);
    expect(apiHabitDetail).toBe(habitDetail);
  });
});
