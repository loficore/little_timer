import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/preact";
import { TimerPage } from "../TimerPage";

vi.mock("../utils/apiClientSingleton", () => ({
  getAPIClient: vi.fn(() => ({
    getHabitSets: vi.fn().mockResolvedValue([]),
    getHabits: vi.fn().mockResolvedValue([]),
    getHabitDetail: vi.fn().mockResolvedValue(null),
    startTimer: vi.fn().mockResolvedValue({}),
    pauseTimer: vi.fn().mockResolvedValue({}),
    resetTimer: vi.fn().mockResolvedValue({}),
    finishTimer: vi.fn().mockResolvedValue({ elapsed_seconds: 0 }),
    getTimerProgress: vi.fn().mockResolvedValue({
      session_id: null,
      is_finished: true,
      is_running: false,
      is_paused: false,
      habit_id: null,
      mode: "stopwatch",
      elapsed_seconds: 0,
      remaining_seconds: 1500,
      in_rest: false,
    }),
  })),
}));

vi.mock("../utils/audio", () => ({
  audioEngine: {
    setPreferences: vi.fn(),
    unlock: vi.fn(),
    playTick: vi.fn(),
    stopTick: vi.fn(),
    playFinish: vi.fn(),
  },
  loadAudioPreferences: vi.fn(() => ({
    sound_enabled: true,
    sound_tick: true,
    sound_finish: true,
    sound_volume: 80,
  })),
}));

vi.mock("../hooks/useSSE", () => ({
  useSSE: vi.fn(() => ({
    isConnected: false,
    lastMessage: null,
  })),
}));

vi.mock("../utils/logger", () => ({
  logSuccess: vi.fn(),
  logError: vi.fn(),
}));

vi.mock("../utils/formatters", () => ({
  formatDuration: vi.fn((seconds: number) => {
    const mins = Math.floor(seconds / 60);
    const secs = seconds % 60;
    return `${mins.toString().padStart(2, "0")}:${secs.toString().padStart(2, "0")}`;
  }),
}));

vi.mock("../utils/i18n", () => ({
  t: (key: string, params?: Record<string, unknown>) => {
    const translations: Record<string, string> = {
      "timer.title": "专注",
      "timer.select_habit": "选择习惯",
      "timer.stopwatch": "秒表",
      "timer.countdown": "倒计时",
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

describe("TimerPage 布局测试", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("习惯选择按钮和模式选择按钮宽度应相等（误差 ≤2px）", () => {
    render(<TimerPage />);

    const buttons = screen.getAllByRole("button");
    const habitBtn = buttons.find((btn) =>
      btn.textContent.includes("选择习惯") || btn.textContent.includes("Select Habit")
    );
    const modeBtn = buttons.find((btn) =>
      btn.textContent.includes("秒表") ||
      btn.textContent.includes("Stopwatch") ||
      btn.textContent.includes("倒计时") ||
      btn.textContent.includes("Countdown")
    );

    expect(habitBtn).toBeTruthy();
    expect(modeBtn).toBeTruthy();

    const habitWidth = habitBtn!.offsetWidth;
    const modeWidth = modeBtn!.offsetWidth;

    expect(Math.abs(habitWidth - modeWidth)).toBeLessThanOrEqual(2);
  });

  it("标题应在导航栏内水平居中", () => {
    render(<TimerPage />);

    const title = screen.getByText("专注");
    const navbar = title.closest("header");

    expect(navbar).toBeTruthy();

    const navbarWidth = navbar!.offsetWidth;
    const titleLeft = title.offsetLeft;
    const titleWidth = title.offsetWidth;

    const titleCenterX = titleLeft + titleWidth / 2;
    const navbarCenterX = navbarWidth / 2;

    expect(Math.abs(titleCenterX - navbarCenterX)).toBeLessThanOrEqual(2);
  });

  it("时钟区域内容应垂直居中于 my-clock-glass 内", () => {
    render(<TimerPage />);

    const glass = document.querySelector(".my-clock-glass");
    expect(glass).toBeTruthy();

    const innerDiv = glass!.querySelector(":scope > div") as HTMLElement;
    expect(innerDiv).toBeTruthy();

    expect(innerDiv.className).toContain("flex");
    expect(innerDiv.className).toContain("items-center");
    expect(innerDiv.className).toContain("justify-center");
  });
});