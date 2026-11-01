import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/preact";
import { PlanPage } from "../PlanPage";

vi.mock("../utils/i18n", () => ({ t: (key: string) => key }));
vi.mock("../utils/logger", () => ({ logError: vi.fn(), logPerf: vi.fn(), logLifecycle: vi.fn() }));
// 两个面板各自用 useScheduler / useDailySweep，都会打 API —— 这里只测 tab 切换，全部打桩。
vi.mock("../utils/apiClientSingleton", () => ({
  supportsDailySweep: true,
  getAPIClient: () => ({
    listTasks: vi.fn().mockResolvedValue([]),
    getDailySweepToday: vi.fn().mockResolvedValue({ proposals: [] }),
    runSchedulePreview: vi.fn().mockResolvedValue(null),
    applySchedulePlacements: vi.fn().mockResolvedValue({}),
    startDailySweep: vi.fn().mockResolvedValue({}),
    adoptDailySweep: vi.fn().mockResolvedValue({}),
    rejectDailySweep: vi.fn().mockResolvedValue({}),
  }),
}));

describe("PlanPage", () => {
  it("渲染两个页内 tab", () => {
    render(<PlanPage />);
    expect(screen.getByTestId("plan-tab-schedule")).toBeTruthy();
    expect(screen.getByTestId("plan-tab-review")).toBeTruthy();
  });

  it("默认展示调度面板", () => {
    render(<PlanPage />);
    expect(screen.getByTestId("plan-panel-schedule")).toBeTruthy();
    expect(screen.queryByTestId("plan-panel-review")).toBeNull();
  });

  it("点击回顾 tab 切到回顾面板", () => {
    render(<PlanPage />);
    fireEvent.click(screen.getByTestId("plan-tab-review"));
    expect(screen.getByTestId("plan-panel-review")).toBeTruthy();
    expect(screen.queryByTestId("plan-panel-schedule")).toBeNull();
  });
});
