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

describe("useTimer 扩展测试", () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.restoreAllMocks();
    vi.useRealTimers();
  });

  describe("resume", () => {
    it("应该恢复暂停的计时", async () => {
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

      expect(result.current.isPaused).toBe(true);
      expect(result.current.elapsedSeconds).toBe(2);

      await act(async () => {
        await result.current.resume();
      });

      expect(result.current.isPaused).toBe(false);
    });
  });

  describe("skipToNext", () => {
    it("倒计时模式下应该跳到休息", async () => {
      const { result } = renderHook(() => useTimer());

      act(() => {
        result.current.setTimerConfig({
          mode: "countdown",
          workDuration: 10,
          restDuration: 5,
        });
      });

      await act(async () => {
        await result.current.start();
      });

      act(() => {
        vi.advanceTimersByTime(3000);
      });

      act(() => {
        result.current.skipToNext();
      });

      expect(result.current.isResting).toBe(true);
      expect(result.current.remainingSeconds).toBe(5);
    });

    it("倒计时模式下休息时应该跳到下一轮", async () => {
      const { result } = renderHook(() => useTimer());

      act(() => {
        result.current.setTimerConfig({
          mode: "countdown",
          workDuration: 10,
          restDuration: 5,
          loopCount: 3,
        });
      });

      await act(async () => {
        await result.current.start();
      });

      act(() => {
        vi.advanceTimersByTime(3000);
      });

      act(() => {
        result.current.skipToNext();
      });

      expect(result.current.isResting).toBe(true);

      act(() => {
        result.current.skipToNext();
      });

      expect(result.current.isResting).toBe(false);
      expect(result.current.currentRound).toBe(2);
    });

    it("无 restDuration 时应该直接进入下一轮", async () => {
      const { result } = renderHook(() => useTimer());

      act(() => {
        result.current.setTimerConfig({
          mode: "countdown",
          workDuration: 10,
          restDuration: 0,
        });
      });

      await act(async () => {
        await result.current.start();
      });

      act(() => {
        vi.advanceTimersByTime(3000);
      });

      act(() => {
        result.current.skipToNext();
      });

      expect(result.current.isResting).toBe(false);
      expect(result.current.currentRound).toBe(2);
      expect(result.current.remainingSeconds).toBe(10);
    });
  });

  describe("暂停后恢复精度", () => {
    it("暂停期间不应该推进时间", async () => {
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
        vi.advanceTimersByTime(5000);
      });

      expect(result.current.elapsedSeconds).toBe(2);
    });
  });

  describe("finish 行为", () => {
    it("finish 后应该重置到初始状态", async () => {
      const { result } = renderHook(() => useTimer());

      act(() => {
        result.current.setTimerConfig({
          mode: "countdown",
          workDuration: 60,
        });
      });

      await act(async () => {
        await result.current.start();
      });

      act(() => {
        vi.advanceTimersByTime(10000);
      });

      await act(async () => {
        await result.current.finish();
      });

      expect(result.current.isRunning).toBe(false);
      expect(result.current.isPaused).toBe(false);
      expect(result.current.isFinished).toBe(false);
      expect(result.current.elapsedSeconds).toBe(0);
      expect(result.current.remainingSeconds).toBe(60);
    });
  });
});
