import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent } from "@testing-library/preact";
import { Sidebar } from "../../components/Sidebar";

vi.mock("../../utils/i18n", () => ({
  t: (key: string) => {
    const translations: Record<string, string> = {
      "nav.timer": "专注",
      "nav.habits": "习惯",
      "nav.stats": "统计",
      "nav.settings": "设置",
    };
    return translations[key] || key;
  },
}));

describe("Sidebar 组件", () => {
  const mockOnNavigate = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("应该渲染导航项", () => {
    render(<Sidebar currentPage="timer" onNavigate={mockOnNavigate} />);

    expect(screen.getByText("专注")).toBeTruthy();
    expect(screen.getByText("习惯")).toBeTruthy();
    expect(screen.getByText("统计")).toBeTruthy();
    expect(screen.getByText("设置")).toBeTruthy();
  });

  it("应该高亮当前页面", () => {
    render(<Sidebar currentPage="stats" onNavigate={mockOnNavigate} />);

    const statsButton = screen.getByText("统计").closest("button");
    expect(statsButton?.className).toContain("is-active");
  });

  it("点击导航项应该调用 onNavigate", () => {
    render(<Sidebar currentPage="timer" onNavigate={mockOnNavigate} />);

    fireEvent.click(screen.getByText("设置"));

    expect(mockOnNavigate).toHaveBeenCalledWith("settings");
  });

  it("应该渲染侧边栏", () => {
    const { container } = render(
      <Sidebar currentPage="timer" onNavigate={mockOnNavigate} />
    );

    const nav = container.querySelector("nav");
    expect(nav).toBeTruthy();
  });
});
