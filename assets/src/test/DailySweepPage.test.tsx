import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/preact";
import { DailySweepPage } from "../DailySweepPage";
import type { DailySweepProposal } from "../types/aigtd";

const mocks = vi.hoisted(() => ({
  getDailySweepToday: vi.fn(),
  startDailySweep: vi.fn(),
  adoptDailySweep: vi.fn(),
  rejectDailySweep: vi.fn(),
}));

vi.mock("../utils/apiClientSingleton", () => ({
  getAPIClient: () => mocks,
}));

vi.mock("../utils/logger", () => ({
  logError: vi.fn(),
}));

vi.mock("../utils/i18n", () => ({
  t: (key: string) => key,
}));

function makeP(overrides: Partial<DailySweepProposal["proposal"]> = {}, isAdopted = false): DailySweepProposal {
  return {
    ai_job_id: 1,
    version: 0,
    is_adopted: isAdopted,
    proposal: {
      change_id: overrides.change_id ?? "c",
      task_id: 1,
      action: "reschedule",
      reason: "r",
      ...overrides,
    },
  };
}

describe("DailySweepPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("隐藏已采纳的提议,只显示待决策的(Review Fix I-2)", async () => {
    mocks.getDailySweepToday.mockResolvedValue({
      proposals: [
        makeP({ change_id: "adopted-1", reason: "已采纳项" }, true),
        makeP({ change_id: "pending-1", reason: "待处理" }, false),
      ],
    });

    render(<DailySweepPage />);

    // 等拉取完成,出现"待处理"卡片
    await waitFor(() => {
      expect(screen.getByText("待处理")).toBeTruthy();
    });
    // "已采纳项" 卡片不应在 DOM 里(actionable 过滤)
    expect(screen.queryByText("已采纳项")).toBeNull();
    // 空状态文案不应出现(还有待处理的)
    expect(screen.queryByTestId("empty")).toBeNull();
  });

  it("全部已采纳 → 显示空状态", async () => {
    mocks.getDailySweepToday.mockResolvedValue({
      proposals: [
        makeP({ change_id: "adopted-1", reason: "A" }, true),
        makeP({ change_id: "adopted-2", reason: "B" }, true),
      ],
    });
    render(<DailySweepPage />);
    await waitFor(() => {
      expect(screen.getByTestId("empty")).toBeTruthy();
    });
    // 没有 proposal-card
    expect(screen.queryByTestId("proposal-card")).toBeNull();
  });
});
