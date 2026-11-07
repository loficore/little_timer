import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent } from "@testing-library/preact";
import type { PlatformCapabilities } from "../../utils/platformApi";

const mocks = vi.hoisted(() => ({
  capabilities: {
    timer: true,
    habits: true,
    backup: true,
    settings: true,
    tasks: false,
    schedule: false,
    llmSettings: false,
    credentials: false,
    wallpaper: false,
    dailySweep: false,
  } as PlatformCapabilities,
}));

vi.mock("../../utils/platformApi", () => ({
  get capabilities() {
    return mocks.capabilities;
  },
}));

vi.mock("../../utils/apiClientSingleton", () => ({
  get capabilities() {
    return mocks.capabilities;
  },
  supportsDailySweep: false,
  // Sidebar 底部的版本号来自 /api/version；导航测试不关心具体值。
  getAPIClient: () => ({
    getVersion: () =>
      Promise.resolve({ version: "test", git_commit: "test", build_time: "" }),
  }),
}));

vi.mock("../../utils/i18n", () => ({
  t: (key: string) => {
    const translations: Record<string, string> = {
      "nav.timer": "专注",
      "nav.habits": "习惯",
      "nav.stats": "统计",
      "nav.settings": "设置",
      "aigtd.nav_label": "AI 任务",
      "plan.nav_label": "计划",
    };
    return translations[key] || key;
  },
}));

import { Sidebar } from "../../components/Sidebar";

describe("Sidebar 组件", () => {
  const mockOnNavigate = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
    mocks.capabilities = {
      timer: true,
      habits: true,
      backup: true,
      settings: true,
      tasks: true,
      schedule: true,
      llmSettings: true,
      credentials: true,
      wallpaper: true,
      dailySweep: true,
    };
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

  it("设备不支持任务与调度时隐藏 AIGTD / Plan 入口", () => {
    mocks.capabilities = { ...mocks.capabilities, tasks: false, schedule: false };
    render(<Sidebar currentPage="timer" onNavigate={mockOnNavigate} />);

    expect(screen.queryByTestId("nav-aigtd")).toBeNull();
    expect(screen.queryByTestId("nav-plan")).toBeNull();
    // 其余入口照常。
    expect(screen.getByTestId("nav-timer")).toBeTruthy();
    expect(screen.getByTestId("nav-habits")).toBeTruthy();
    expect(screen.getByTestId("nav-stats")).toBeTruthy();
    expect(screen.getByTestId("nav-settings")).toBeTruthy();
  });

  it("支持任务与调度时保留 AIGTD / Plan 入口", () => {
    render(<Sidebar currentPage="timer" onNavigate={mockOnNavigate} />);

    expect(screen.getByTestId("nav-aigtd")).toBeTruthy();
    expect(screen.getByTestId("nav-plan")).toBeTruthy();
  });
});
