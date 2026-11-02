import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/preact";

const mocks = vi.hoisted(() => ({
  updateTask: vi.fn(),
  patchTaskScores: vi.fn(),
  getTask: vi.fn(),
}));

vi.mock("../../../utils/apiClientSingleton", () => ({
  getAPIClient: () => ({
    updateTask: mocks.updateTask,
    patchTaskScores: mocks.patchTaskScores,
    getTask: mocks.getTask,
  }),
}));

vi.mock("../../../utils/i18n", () => ({
  t: (key: string, params?: Record<string, string | number>) => {
    const translations: Record<string, string> = {
      "aigtd.edit_title": "编辑任务 #{id}",
      "aigtd.field_title": "标题",
      "aigtd.field_notes": "备注",
      "aigtd.field_due_date": "截止日期",
      "aigtd.section_basic": "基本信息",
      "aigtd.section_scores": "评分",
      "score.priority": "优先级",
      "score.urgency": "紧急度",
      "score.energy": "精力",
      "score.context": "情境",
      "score.conflict": "版本冲突，已刷新最新数据，请重试",
      "score.conflict_refresh_failed": "版本冲突，刷新失败：{error}",
      "button.cancel": "取消",
      "button.save": "保存",
      "button.saving": "保存中...",
    };
    let out = translations[key] ?? key;
    if (params) {
      for (const [k, v] of Object.entries(params)) out = out.replace(`{${k}}`, String(v));
    }
    return out;
  },
}));

import { TaskEditModal } from "../../../components/AIGtd/TaskEditModal";
import { VersionConflictError } from "../../../utils/apiClient";
import type { TaskDTO } from "../../../types/aigtd";

function makeTask(overrides: Partial<TaskDTO> = {}): TaskDTO {
  return {
    id: 7,
    title: "写周报",
    notes: "本周进度",
    due_date: "2026-11-10",
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

describe("TaskEditModal", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("字段标签走 i18n，不再是 title/notes/due_date 裸标识符", () => {
    render(<TaskEditModal task={makeTask()} onClose={() => {}} />);
    expect(screen.getByText("标题")).toBeTruthy();
    expect(screen.getByText("备注")).toBeTruthy();
    expect(screen.getByText("截止日期")).toBeTruthy();
    expect(screen.queryByText("title")).toBeNull();
    expect(screen.queryByText("due_date")).toBeNull();
  });

  it("底部只有一个保存主按钮（评分不再有独立保存）", () => {
    render(<TaskEditModal task={makeTask()} onClose={() => {}} />);
    expect(screen.getAllByTestId("task-edit-save")).toHaveLength(1);
    expect(screen.queryByTestId("save-scores")).toBeNull();
  });

  it("只改标题时，保存只调用 updateTask，并带上 version", async () => {
    const updated = makeTask({ version: 4, title: "写月报" });
    mocks.updateTask.mockResolvedValue(updated);
    const onClose = vi.fn();

    render(<TaskEditModal task={makeTask()} onClose={onClose} />);
    fireEvent.input(screen.getByTestId("task-edit-title"), { target: { value: "写月报" } });
    fireEvent.click(screen.getByTestId("task-edit-save"));

    await waitFor(() => {
      expect(mocks.updateTask).toHaveBeenCalledWith(
        7,
        expect.objectContaining({ version: 3, title: "写月报", notes: "本周进度", due_date: "2026-11-10" }),
      );
    });
    expect(mocks.patchTaskScores).not.toHaveBeenCalled();
    await waitFor(() => expect(onClose).toHaveBeenCalled());
  });

  it("标题和评分都改时，先 patch 评分，再用新 version 保存字段", async () => {
    const afterScores = makeTask({ version: 4, priority_score: 5 });
    const afterFields = makeTask({ version: 5, priority_score: 5, title: "写月报" });
    mocks.patchTaskScores.mockResolvedValue(afterScores);
    mocks.updateTask.mockResolvedValue(afterFields);

    render(<TaskEditModal task={makeTask()} onClose={() => {}} />);
    fireEvent.input(screen.getByTestId("priority-input"), { target: { value: "5" } });
    fireEvent.input(screen.getByTestId("task-edit-title"), { target: { value: "写月报" } });
    fireEvent.click(screen.getByTestId("task-edit-save"));

    await waitFor(() => {
      expect(mocks.patchTaskScores).toHaveBeenCalledWith(
        7,
        expect.objectContaining({ version: 3, priority_score: 5 }),
      );
    });
    await waitFor(() => {
      expect(mocks.updateTask).toHaveBeenCalledWith(
        7,
        expect.objectContaining({ version: 4, title: "写月报" }),
      );
    });
  });

  it("什么都没改时，保存直接关闭，不产生请求", async () => {
    const onClose = vi.fn();
    render(<TaskEditModal task={makeTask()} onClose={onClose} />);
    fireEvent.click(screen.getByTestId("task-edit-save"));
    await waitFor(() => expect(onClose).toHaveBeenCalled());
    expect(mocks.updateTask).not.toHaveBeenCalled();
    expect(mocks.patchTaskScores).not.toHaveBeenCalled();
  });

  it("评分保存 409 时刷新最新任务并显示冲突提示，不自动重试", async () => {
    mocks.patchTaskScores.mockRejectedValue(new VersionConflictError());
    const fresh = makeTask({ version: 9, priority_score: 4, urgency_score: 8 });
    mocks.getTask.mockResolvedValue(fresh);

    render(<TaskEditModal task={makeTask()} onClose={() => {}} />);
    fireEvent.input(screen.getByTestId("priority-input"), { target: { value: "5" } });
    fireEvent.click(screen.getByTestId("task-edit-save"));

    const banner = await screen.findByTestId("conflict-banner");
    expect(banner.textContent).toContain("冲突");
    expect(mocks.getTask).toHaveBeenCalledWith(7);
    expect(mocks.patchTaskScores).toHaveBeenCalledTimes(1);
    // 刷新后表单回到服务端值
    await waitFor(() => expect(screen.getByTestId("priority-value").textContent).toBe("4"));
  });
});
