import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { renderHook, act, waitFor } from "@testing-library/preact";
import { useTimer } from "../../hooks/useTimer";
import { useSettings } from "../../hooks/useSettings";

vi.mock("../../utils/apiClientSingleton", () => ({
  getAPIClient: vi.fn(() => ({
    startTimer: vi.fn().mockResolvedValue({ status: "started" }),
    pauseTimer: vi.fn().mockResolvedValue({ status: "paused" }),
    resumeTimer: vi.fn().mockResolvedValue({ status: "started" }),
    resetTimer: vi.fn().mockResolvedValue({ status: "reset" }),
    finishTimer: vi.fn().mockResolvedValue({ elapsed_seconds: 1500 }),
    getSettings: vi.fn().mockResolvedValue({
      basic: {
        timezone: 8,
        language: "ZH",
        default_mode: "countdown",
        theme_mode: "dark",
        sound_enabled: true,
        sound_tick: false,
        sound_finish: true,
        sound_volume: 35,
      },
    }),
    updateSettings: vi.fn().mockResolvedValue({ status: "ok" }),
  })),
}));

vi.mock("../../utils/i18n", () => ({
  t: vi.fn((key: string) => key),
}));

vi.mock("../../utils/logger", () => ({
  logSuccess: vi.fn(),
  logError: vi.fn(),
}));

vi.mock("../../utils/constants", () => ({
  STORAGE_KEYS: {
    LAYOUT_DENSITY: "layout_density",
    TIME_DISPLAY_STYLE: "time_display_style",
  },
}));

describe("集成测试 - 计时器流程", () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.restoreAllMocks();
    vi.useRealTimers();
  });

  describe("stopwatch 完整流程", () => {
    it("start → tick → pause → resume → reset 状态流转", async () => {
      const { result } = renderHook(() => useTimer());

      expect(result.current.isRunning).toBe(false);
      expect(result.current.elapsedSeconds).toBe(0);

      await act(async () => {
        await result.current.start();
      });

      expect(result.current.isRunning).toBe(true);
      expect(result.current.isPaused).toBe(false);
      expect(result.current.elapsedSeconds).toBe(0);

      act(() => {
        vi.advanceTimersByTime(5000);
      });

      expect(result.current.elapsedSeconds).toBe(5);

      await act(async () => {
        await result.current.pause();
      });

      expect(result.current.isPaused).toBe(true);

      act(() => {
        vi.advanceTimersByTime(3000);
      });

      expect(result.current.elapsedSeconds).toBe(5);

      await act(async () => {
        await result.current.resume();
      });

      expect(result.current.isPaused).toBe(false);

      act(() => {
        vi.advanceTimersByTime(2000);
      });

      expect(result.current.elapsedSeconds).toBe(7);

      await act(async () => {
        await result.current.reset();
      });

      expect(result.current.isRunning).toBe(false);
      expect(result.current.elapsedSeconds).toBe(0);
    });
  });

  describe("countdown 完整流程", () => {
    it("start 后应该正确初始化", async () => {
      const { result } = renderHook(() => useTimer());

      act(() => {
        result.current.setTimerConfig({ mode: "countdown", workDuration: 10 });
      });

      await act(async () => {
        await result.current.start();
      });

      expect(result.current.isRunning).toBe(true);
      expect(result.current.remainingSeconds).toBe(10);
      expect(result.current.currentRound).toBe(1);
      expect(result.current.isResting).toBe(false);
    });
  });

  describe("countdown + loop 流程", () => {
    it("work → rest → work → finish", async () => {
      const { result } = renderHook(() => useTimer());

      act(() => {
        result.current.setTimerConfig({
          mode: "countdown",
          workDuration: 5,
          restDuration: 3,
          loopCount: 2,
        });
      });

      await act(async () => {
        await result.current.start();
      });

      expect(result.current.isRunning).toBe(true);
      expect(result.current.currentRound).toBe(1);
      expect(result.current.isResting).toBe(false);
      expect(result.current.remainingSeconds).toBe(5);

      act(() => {
        vi.advanceTimersByTime(5000);
      });

      expect(result.current.isResting).toBe(true);
      expect(result.current.remainingSeconds).toBe(3);

      act(() => {
        vi.advanceTimersByTime(3000);
      });

      expect(result.current.currentRound).toBe(2);
      expect(result.current.isResting).toBe(false);

      act(() => {
        vi.advanceTimersByTime(5000);
      });

      expect(result.current.isFinished).toBe(true);
      expect(result.current.isRunning).toBe(false);
    });
  });
});

describe("集成测试 - 设置持久化", () => {
  beforeEach(() => {
    localStorage.clear();
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.restoreAllMocks();
    vi.useRealTimers();
    localStorage.clear();
  });

  it("更新后保存到 localStorage", async () => {
    const { result } = renderHook(() => useSettings());

    await waitFor(() => expect(result.current.settings.timezone).toBe(8));

    act(() => {
      result.current.updateSettings({ timezone: 12 });
    });

    expect(result.current.settings.timezone).toBe(12);

    await act(async () => {
      await result.current.save();
    });

    expect(result.current.saveMessage).toBe("common.save_success");
  });

  it("重置后恢复默认值", async () => {
    const { result } = renderHook(() => useSettings());

    await waitFor(() => expect(result.current.settings.timezone).toBe(8));

    act(() => {
      result.current.updateSettings({ timezone: 12 });
    });

    act(() => {
      result.current.reset();
    });

    expect(result.current.settings.timezone).toBe(8);
  });
});