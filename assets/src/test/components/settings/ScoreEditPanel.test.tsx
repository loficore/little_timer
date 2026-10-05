import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/preact";

const mocks = vi.hoisted(() => ({
  showToastMock: vi.fn(),
  patchTaskScores: vi.fn(),
  getTask: vi.fn(),
}));

vi.mock("../../../utils/apiClientSingleton", () => ({
  getAPIClient: () => ({
    patchTaskScores: mocks.patchTaskScores,
    getTask: mocks.getTask,
  }),
}));

vi.mock("../../../components/common/Toast", () => ({
  showToast: mocks.showToastMock,
}));

vi.mock("../../../utils/i18n", () => ({
  t: (key: string) => {
    const translations: Record<string, string> = {
      "score.priority": "优先级",
      "score.urgency": "紧急度",
      "score.energy": "精力",
      "score.context": "情境",
      "score.save": "保存",
      "score.save_success": "评分已保存",
      "score.save_fail": "保存失败",
      "score.conflict": "版本冲突，已刷新最新数据，请重试",
      "button.saving": "保存中...",
    };
    return translations[key] ?? key;
  },
}));

import { ScoreEditPanel } from "../../../components/settings/ScoreEditPanel";
import { VersionConflictError } from "../../../utils/apiClient";
import type { TaskDTO } from "../../../types/aigtd";

function makeTask(overrides: Partial<TaskDTO> = {}): TaskDTO {
  return {
    id: 7,
    title: "写周报",
    status: "active",
    pinned: false,
    estimated_minutes: 0,
    order_index: 0,
    source: "manual",
    ai_status: "done",
    version: 3,
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

describe("ScoreEditPanel", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("把 priority_score 改为 5 并点击保存时, 以 version 调用 patchTaskScores", async () => {
    const updated = makeTask({ version: 4, priority_score: 5 });
    mocks.patchTaskScores.mockResolvedValue(updated);
    const onUpdated = vi.fn();

    render(
      <ScoreEditPanel
        taskId={7}
        current={{
          priority_score: 3,
          urgency_score: 5,
          energy_required: 2,
          context_tag: "general",
        }}
        version={3}
        onUpdated={onUpdated}
      />,
    );

    const slider = screen.getByTestId("priority-input") as HTMLInputElement;
    fireEvent.input(slider, { target: { value: "5" } });
    expect(screen.getByTestId("priority-value").textContent).toBe("5");

    fireEvent.click(screen.getByTestId("save-scores"));

    await waitFor(() => {
      expect(mocks.patchTaskScores).toHaveBeenCalledWith(
        7,
        expect.objectContaining({
          version: 3,
          priority_score: 5,
          urgency_score: 5,
          energy_required: 2,
          context_tag: "general",
        }),
      );
    });

    await waitFor(() => {
      expect(onUpdated).toHaveBeenCalledWith(updated);
    });
    expect(mocks.showToastMock).toHaveBeenCalledWith("评分已保存", "success");
  });

  it("API 返回 409 时重新拉取最新任务并显示冲突提示", async () => {
    mocks.patchTaskScores.mockRejectedValue(new VersionConflictError());
    const fresh = makeTask({
      version: 9,
      priority_score: 4,
      urgency_score: 8,
      energy_required: 1,
      context_tag: "编码",
    });
    mocks.getTask.mockResolvedValue(fresh);
    const onUpdated = vi.fn();

    render(
      <ScoreEditPanel
        taskId={7}
        current={{
          priority_score: 3,
          urgency_score: 5,
          energy_required: 2,
          context_tag: "general",
        }}
        version={3}
        onUpdated={onUpdated}
      />,
    );

    fireEvent.click(screen.getByTestId("save-scores"));

    await waitFor(() => {
      expect(mocks.getTask).toHaveBeenCalledWith(7);
    });

    const banner = await screen.findByTestId("conflict-banner");
    expect(banner.textContent).toContain("冲突");

    await waitFor(() => {
      expect(onUpdated).toHaveBeenCalledWith(fresh);
    });
  });
});
