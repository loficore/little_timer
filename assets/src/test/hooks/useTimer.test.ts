import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { renderHook, act } from "@testing-library/preact";
import { useTimer } from "../../hooks/useTimer";

vi.mock("../../utils/apiClientSingleton", () => ({
  getAPIClient: vi.fn(() => ({
    startTimer: vi.fn().mockResolvedValue({ status: "started" }),
    pauseTimer: vi.fn().mockResolvedValue({ status: "paused" }),
    resumeTimer: vi.fn().mockResolvedValue({ status: "started" }),
    resetTimer: vi.fn().mockResolvedValue({ status: "reset" }),
    finishTimer: vi.fn().mockResolvedValue({ elapsed_seconds: 1500 }),
  })),
}));

vi.mock("../../utils/logger", () => ({
  logSuccess: vi.fn(),
  logError: vi.fn(),
}));

describe("useTimer Hook", () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.restoreAllMocks();
    vi.useRealTimers();
  });

  describe("初始状态", () => {
    it("应该返回默认配置", () => {
      const { result } = renderHook(() => useTimer());

      expect(result.current.timerConfig.mode).toBe("stopwatch");
      expect(result.current.timerConfig.workDuration).toBe(25 * 60);
      expect(result.current.timerConfig.restDuration).toBe(5 * 60);
      expect(result.current.timerConfig.loopCount).toBe(0);
    });

    it("应该返回初始停止状态", () => {
      const { result } = renderHook(() => useTimer());

      expect(result.current.isRunning).toBe(false);
      expect(result.current.isPaused).toBe(false);
      expect(result.current.isFinished).toBe(false);
      expect(result.current.isResting).toBe(false);
    });
  });

  describe("setTimerConfig", () => {
    it("应该更新配置", () => {
      const { result } = renderHook(() => useTimer());

      act(() => {
        result.current.setTimerConfig({ mode: "countdown", workDuration: 600 });
      });

      expect(result.current.timerConfig.mode).toBe("countdown");
      expect(result.current.timerConfig.workDuration).toBe(600);
    });

    it("应该支持部分更新", () => {
      const { result } = renderHook(() => useTimer());

      act(() => {
        result.current.setTimerConfig({ restDuration: 300 });
      });

      expect(result.current.timerConfig.mode).toBe("stopwatch");
      expect(result.current.timerConfig.restDuration).toBe(300);
    });
  });

  describe("start", () => {
    it("应该在 stopwatch 模式下启动计时", async () => {
      const { result } = renderHook(() => useTimer());

      await act(async () => {
        await result.current.start();
      });

      expect(result.current.isRunning).toBe(true);
      expect(result.current.isPaused).toBe(false);
      expect(result.current.elapsedSeconds).toBe(0);
    });

    it("应该在 countdown 模式下设置剩余时间", async () => {
      const { result } = renderHook(() => useTimer());

      act(() => {
        result.current.setTimerConfig({ mode: "countdown", workDuration: 600 });
      });

      await act(async () => {
        await result.current.start();
      });

      expect(result.current.isRunning).toBe(true);
      expect(result.current.remainingSeconds).toBe(600);
      expect(result.current.currentRound).toBe(1);
    });
  });

  describe("pause", () => {
    it("应该暂停计时", async () => {
      const { result } = renderHook(() => useTimer());

      await act(async () => {
        await result.current.start();
      });

      await act(async () => {
        await result.current.pause();
      });

      expect(result.current.isPaused).toBe(true);
    });
  });

  describe("reset", () => {
    it("应该重置所有状态", async () => {
      const { result } = renderHook(() => useTimer());

      await act(async () => {
        await result.current.start();
      });

      act(() => {
        vi.advanceTimersByTime(5000);
      });

      await act(async () => {
        await result.current.reset();
      });

      expect(result.current.isRunning).toBe(false);
      expect(result.current.isPaused).toBe(false);
      expect(result.current.isFinished).toBe(false);
      expect(result.current.elapsedSeconds).toBe(0);
    });
  });

  describe("正计时模式 tick", () => {
    it("应该每秒递增 elapsedSeconds", async () => {
      const { result } = renderHook(() => useTimer());

      await act(async () => {
        await result.current.start();
      });

      act(() => {
        vi.advanceTimersByTime(3000);
      });

      expect(result.current.elapsedSeconds).toBe(3);
    });

    it("暂停时不应该递增", async () => {
      const { result } = renderHook(() => useTimer());

      await act(async () => {
        await result.current.start();
      });

      act(() => {
        vi.advanceTimersByTime(2000);
      });

      await act(async () => {
        await result.current.pause();
      });

      act(() => {
        vi.advanceTimersByTime(3000);
      });

      expect(result.current.elapsedSeconds).toBe(2);
    });
  });

  describe("倒计时模式 tick", () => {
    it("应该每秒递减 remainingSeconds", async () => {
      const { result } = renderHook(() => useTimer());

      act(() => {
        result.current.setTimerConfig({ mode: "countdown", workDuration: 10 });
      });

      await act(async () => {
        await result.current.start();
      });

      act(() => {
        vi.advanceTimersByTime(3000);
      });

      expect(result.current.remainingSeconds).toBe(7);
    });
  });

  describe("finish", () => {
    it("应该重置状态", async () => {
      const { result } = renderHook(() => useTimer());

      await act(async () => {
        await result.current.start();
      });

      act(() => {
        vi.advanceTimersByTime(1500);
      });

      await act(async () => {
        await result.current.finish();
      });

      expect(result.current.elapsedSeconds).toBe(0);
      expect(result.current.isRunning).toBe(false);
    });
  });

  describe("错误处理", () => {
    it("start API 失败时应该捕获错误不抛出", async () => {
      const { result } = renderHook(() => useTimer());

      await expect(
        act(async () => {
          await result.current.start();
        })
      ).resolves.not.toThrow();
      expect(result.current.isRunning).toBe(true);
    });

    it("pause API 失败时应该捕获错误不抛出", async () => {
      const { result } = renderHook(() => useTimer());

      await act(async () => {
        await result.current.start();
      });

      await expect(
        act(async () => {
          await result.current.pause();
        })
      ).resolves.not.toThrow();
      expect(result.current.isPaused).toBe(true);
    });

    it("reset API 失败时应该捕获错误不抛出", async () => {
      const { result } = renderHook(() => useTimer());

      await act(async () => {
        await result.current.start();
      });

      await expect(
        act(async () => {
          await result.current.reset();
        })
      ).resolves.not.toThrow();
      expect(result.current.isRunning).toBe(false);
    });

    it("finish API 失败时应该捕获错误不抛出", async () => {
      const { result } = renderHook(() => useTimer());

      await act(async () => {
        await result.current.start();
      });

      await expect(
        act(async () => {
          await result.current.finish();
        })
      ).resolves.not.toThrow();
    });
  });
});
