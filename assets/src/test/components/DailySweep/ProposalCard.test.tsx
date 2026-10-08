import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/preact";
import { ProposalCard } from "../../../components/DailySweep/ProposalCard";
import type { DailySweepProposal } from "../../../types/aigtd";

vi.mock("../../../utils/i18n", () => ({
  t: (key: string, params?: Record<string, string | number>) => {
    let out = key;
    if (params) {
      for (const [k, v] of Object.entries(params)) out = out.replace(`{${k}}`, String(v));
    }
    return out;
  },
}));

function makeP(overrides: Partial<DailySweepProposal["proposal"]> = {}): DailySweepProposal {
  return {
    ai_job_id: 1,
    version: 0,
    proposal: {
      change_id: "c",
      task_id: 1,
      action: "reschedule",
      reason: "deadline tomorrow",
      ...overrides,
    },
  };
}

describe("ProposalCard", () => {
  it("显示 action 标签 + reason", () => {
    render(
      <ProposalCard
        proposal={makeP()}
        onAdopt={vi.fn()}
        onReject={vi.fn()}
      />,
    );
    // getByText 在元素缺失时抛错 — 这就是断言。
    expect(screen.getByText(/reschedule/)).toBeTruthy();
    expect(screen.getByText(/deadline tomorrow/)).toBeTruthy();
  });

  it("adopt 按钮触发 onAdopt", () => {
    const onAdopt = vi.fn();
    render(
      <ProposalCard proposal={makeP()} onAdopt={onAdopt} onReject={vi.fn()} />,
    );
    fireEvent.click(screen.getByTestId("adopt-btn"));
    expect(onAdopt).toHaveBeenCalledOnce();
  });

  it("reject 按钮触发 onReject", () => {
    const onReject = vi.fn();
    render(
      <ProposalCard proposal={makeP()} onAdopt={vi.fn()} onReject={onReject} />,
    );
    fireEvent.click(screen.getByTestId("reject-btn"));
    expect(onReject).toHaveBeenCalledOnce();
  });
});
