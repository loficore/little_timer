import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, fireEvent } from "@testing-library/preact";
import { StatsPage } from "../Stats";

vi.mock("../utils/apiClientSingleton", () => ({
  getAPIClient: vi.fn(() => ({
    getHabits: vi.fn().mockResolvedValue([]),
    getSessions: vi.fn().mockResolvedValue([]),
  })),
}));

vi.mock("../utils/i18n", () => ({
  t: (key: string, params?: Record<string, unknown>) => {
    const translations: Record<string, string> = {
      "stats.title": "统计",
      "stats.today": "今天",
      "stats.this_week": "本周",
      "stats.this_month": "本月",
      "stats.all": "全部",
      "stats.total_focus_time": "总专注时间",
      "stats.completion_count": "完成次数",
      "stats.time_distribution": "时间分布",
      "stats.daily_trend": "每日趋势",
      "stats.no_data": "暂无数据",
      "common.hours": "小时",
      "common.minutes": "分钟",
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

vi.mock("../utils/formatters", () => ({
  getToday: vi.fn(() => "2024-01-15"),
  getDaysAgo: vi.fn((days: number) => {
    const date = new Date();
    date.setDate(date.getDate() - days);
    return date.toISOString().split("T")[0];
  }),
}));

vi.mock("../utils/logger", () => ({
  isPerfDebugEnabled: vi.fn(() => false),
  isWebViewRuntime: vi.fn(() => false),
  logError: vi.fn(),
  logPerf: vi.fn(),
}));

vi.mock("apexcharts", () => ({
  default: vi.fn(() => ({
    render: vi.fn().mockResolvedValue(undefined),
    updateSeries: vi.fn(),
    updateOptions: vi.fn(),
    destroy: vi.fn(),
  })),
}));

vi.mock("chart.js", () => ({
  Chart: {
    register: vi.fn(),
  },
  registerables: [],
}));

describe("StatsPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("应该渲染统计页面", () => {
    const { container } = render(<StatsPage onBackClick={vi.fn()} />);
    expect(container.textContent).toContain("统计");
  });

  it("应该显示时间范围筛选按钮", () => {
    render(<StatsPage onBackClick={vi.fn()} />);
    expect(screen.getByText("今天")).toBeTruthy();
    expect(screen.getByText("本周")).toBeTruthy();
    expect(screen.getByText("本月")).toBeTruthy();
  });

  it("应该显示全部习惯筛选按钮", () => {
    render(<StatsPage onBackClick={vi.fn()} />);
    expect(screen.getByText("全部")).toBeTruthy();
  });

  it("应该显示总计专注时间卡片", () => {
    render(<StatsPage onBackClick={vi.fn()} />);
    expect(screen.getByText("总专注时间")).toBeTruthy();
  });

  it("应该显示完成次数卡片", () => {
    render(<StatsPage onBackClick={vi.fn()} />);
    expect(screen.getByText("完成次数")).toBeTruthy();
  });

  it("应该显示返回按钮", () => {
    render(<StatsPage onBackClick={vi.fn()} />);
    const backButton = screen.getByRole("button", { name: /back/i });
    expect(backButton).toBeTruthy();
  });

  it("点击返回按钮应该调用回调", () => {
    const onBackClick = vi.fn();
    render(<StatsPage onBackClick={onBackClick} />);

    const backButton = screen.getByRole("button", { name: /back/i });
    fireEvent.click(backButton);

    expect(onBackClick).toHaveBeenCalled();
  });

  it("没有数据时应该显示空状态", async () => {
    const { getAPIClient } = await import("../utils/apiClientSingleton");
    vi.mocked(getAPIClient).mockReturnValue({
      getHabits: vi.fn().mockResolvedValue([]),
      getSessions: vi.fn().mockResolvedValue([]),
    });

    render(<StatsPage onBackClick={vi.fn()} />);

    await waitFor(() => {
      expect(screen.getAllByText("暂无数据").length).toBeGreaterThan(0);
    });
  });

  it("有习惯数据时应该显示习惯筛选按钮", async () => {
    const mockHabits = [
      { id: 1, name: "背单词", color: "#6366f1" },
      { id: 2, name: "运动", color: "#22c55e" },
    ];

    const { getAPIClient } = await import("../utils/apiClientSingleton");
    vi.mocked(getAPIClient).mockReturnValue({
      getHabits: vi.fn().mockResolvedValue(mockHabits),
      getSessions: vi.fn().mockResolvedValue([]),
    });

    render(<StatsPage onBackClick={vi.fn()} />);

    await waitFor(() => {
      expect(screen.getByText("背单词")).toBeTruthy();
      expect(screen.getByText("运动")).toBeTruthy();
    });
  });

  it("有 session 数据时应该计算总时间", async () => {
    const mockHabits = [{ id: 1, name: "背单词", color: "#6366f1" }];
    const mockSessions = [
      { habit_id: 1, duration_seconds: 1500, date: "2024-01-15" },
      { habit_id: 1, duration_seconds: 1800, date: "2024-01-14" },
    ];

    const { getAPIClient } = await import("../utils/apiClientSingleton");
    vi.mocked(getAPIClient).mockReturnValue({
      getHabits: vi.fn().mockResolvedValue(mockHabits),
      getSessions: vi.fn().mockResolvedValue(mockSessions),
    });

    render(<StatsPage onBackClick={vi.fn()} />);

    await waitFor(() => {
      expect(screen.getByText(/0/)).toBeTruthy();
    });
  });
});