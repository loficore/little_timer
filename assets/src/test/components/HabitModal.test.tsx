import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/preact";
import { HabitModal } from "../../components/HabitModal";

vi.mock("../../utils/apiClient", () => ({
  APIClient: vi.fn(),
}));

vi.mock("../../utils/i18n", () => ({
  t: (key: string, params?: Record<string, unknown>) => {
    const translations: Record<string, string> = {
      "modal.edit_set": "编辑习惯集",
      "modal.create_set": "创建习惯集",
      "modal.edit_habit": "编辑习惯",
      "habit.add_habit": "添加习惯",
      "modal.name": "名称",
      "modal.name_placeholder_set": "如：学习习惯",
      "modal.name_placeholder_habit": "如：背单词",
      "modal.description": "描述（可选）",
      "modal.description_placeholder": "简单描述这个习惯集...",
      "modal.color": "颜色",
      "modal.wallpaper": "壁纸",
      "modal.goal_duration": "目标时长",
      "modal.hours": "小时",
      "modal.minutes": "分钟",
      "button.cancel": "取消",
      "button.save": "保存",
      "button.create": "创建",
      "button.update": "更新",
      "button.saving": "保存中...",
      "modal.goal_error": "请设置目标时长",
      "modal.goal_summary": "目标: {hours}h {minutes}m = {total} 分钟",
      "modal.select_color": "选择颜色",
      "modal.color_invalid": "请输入有效颜色",
    };
    let result = translations[key] || key;
    if (params) {
      Object.entries(params).forEach(([k, v]) => {
        result = result.replace(`{${k}}`, String(v));
      });
    }
    return result;
  },
}));

vi.mock("../../components/WallpaperSelector", () => ({
  WallpaperSelector: ({ value, onChange }: any) => (
    <div data-testid="wallpaper-selector">
      <button onClick={() => onChange("new_wallpaper")}>Change</button>
    </div>
  ),
}));

vi.mock("../../components/PickerNumberInput", () => ({
  PickerNumberInput: ({ value, onChange }: any) => (
    <div data-testid="number-input">
      <button onClick={() => onChange(value + 1)}>Inc</button>
    </div>
  ),
}));

describe("HabitModal", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("isOpen 为 false 时不应该渲染", () => {
    render(
      <HabitModal
        isOpen={false}
        mode="set"
        onClose={vi.fn()}
        onSuccess={vi.fn()}
      />
    );

    expect(screen.queryByTestId("habit-modal")).toBeNull();
  });

  it("isOpen 为 true 时应该渲染", () => {
    render(
      <HabitModal
        isOpen={true}
        mode="set"
        onClose={vi.fn()}
        onSuccess={vi.fn()}
      />
    );

    expect(screen.getByText("创建习惯集")).toBeTruthy();
  });

  it("mode 为 set 时应该显示创建习惯集", () => {
    render(
      <HabitModal
        isOpen={true}
        mode="set"
        onClose={vi.fn()}
        onSuccess={vi.fn()}
      />
    );

    expect(screen.getByText("创建习惯集")).toBeTruthy();
  });

  it("mode 为 habit 时应该显示创建习惯", () => {
    render(
      <HabitModal
        isOpen={true}
        mode="habit"
        onClose={vi.fn()}
        onSuccess={vi.fn()}
      />
    );

    expect(screen.getByText("添加习惯")).toBeTruthy();
  });

  it("编辑模式时应该显示编辑标题", () => {
    render(
      <HabitModal
        isOpen={true}
        mode="set"
        editData={{ id: 1, name: "Test Set" }}
        onClose={vi.fn()}
        onSuccess={vi.fn()}
      />
    );

    expect(screen.getByText("编辑习惯集")).toBeTruthy();
  });

  it("点击取消按钮应该调用 onClose", () => {
    const onClose = vi.fn();
    render(
      <HabitModal
        isOpen={true}
        mode="set"
        onClose={onClose}
        onSuccess={vi.fn()}
      />
    );

    fireEvent.click(screen.getByText("取消"));
    expect(onClose).toHaveBeenCalled();
  });

  it("应该显示名称输入框", () => {
    render(
      <HabitModal
        isOpen={true}
        mode="set"
        onClose={vi.fn()}
        onSuccess={vi.fn()}
      />
    );

    expect(screen.getByText("名称")).toBeTruthy();
  });

  it("应该显示颜色选择器", () => {
    render(
      <HabitModal
        isOpen={true}
        mode="set"
        onClose={vi.fn()}
        onSuccess={vi.fn()}
      />
    );

    expect(screen.getByText("颜色")).toBeTruthy();
  });

  it("habit 模式应该显示目标时间", () => {
    render(
      <HabitModal
        isOpen={true}
        mode="habit"
        onClose={vi.fn()}
        onSuccess={vi.fn()}
      />
    );

    expect(screen.getByText("目标时长")).toBeTruthy();
  });

  it("应该渲染壁纸选择器", () => {
    render(
      <HabitModal
        isOpen={true}
        mode="set"
        onClose={vi.fn()}
        onSuccess={vi.fn()}
      />
    );

    expect(screen.getByTestId("wallpaper-selector")).toBeTruthy();
  });

  it("应该显示保存按钮", () => {
    render(
      <HabitModal
        isOpen={true}
        mode="set"
        editData={{ id: 1, name: "Test" }}
        onClose={vi.fn()}
        onSuccess={vi.fn()}
      />
    );

    expect(screen.getByText("保存")).toBeTruthy();
  });

  it("创建模式应该显示创建按钮", () => {
    render(
      <HabitModal
        isOpen={true}
        mode="set"
        onClose={vi.fn()}
        onSuccess={vi.fn()}
      />
    );

    expect(screen.getByText("创建")).toBeTruthy();
  });

  it("编辑模式应该显示保存按钮", () => {
    render(
      <HabitModal
        isOpen={true}
        mode="set"
        editData={{ id: 1, name: "Test" }}
        onClose={vi.fn()}
        onSuccess={vi.fn()}
      />
    );

    expect(screen.getByText("保存")).toBeTruthy();
  });
});
