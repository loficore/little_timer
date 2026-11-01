import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/preact";
import { PlanPage } from "../PlanPage";

vi.mock("../utils/i18n", () => ({ t: (key: string) => key }));
vi.mock("../utils/logger", () => ({ logError: vi.fn(), logPerf: vi.fn(), logLifecycle: vi.fn() }));
// Android 等 supportsDailySweep=false 的平台：每日梳理后端绑定未落地(issue #41)，
// 合并页里「回顾」tab 必须和原来的 dailySweep 入口一样隐藏。
vi.mock("../utils/apiClientSingleton", () => ({
  supportsDailySweep: false,
  getAPIClient: () => ({
    listTasks: vi.fn().mockResolvedValue([]),
    getDailySweepToday: vi.fn().mockResolvedValue({ proposals: [] }),
    runSchedulePreview: vi.fn().mockResolvedValue(null),
    applySchedulePlacements: vi.fn().mockResolvedValue({}),
  }),
}));

describe("PlanPage (无每日梳理能力的平台)", () => {
  it("只渲染调度 tab，不渲染回顾 tab", () => {
    render(<PlanPage />);
    expect(screen.getByTestId("plan-tab-schedule")).toBeTruthy();
    expect(screen.queryByTestId("plan-tab-review")).toBeNull();
  });

  it("默认仍展示调度面板", () => {
    render(<PlanPage />);
    expect(screen.getByTestId("plan-panel-schedule")).toBeTruthy();
    expect(screen.queryByTestId("plan-panel-review")).toBeNull();
  });
});
