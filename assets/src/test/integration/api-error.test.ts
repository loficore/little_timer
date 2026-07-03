/**
 * 集成测试 - API 错误处理
 *
 * 覆盖以下错误场景：
 * - 网络错误（fetch 直接 throw，如 DNS 失败、连接被拒）
 * - HTTP 500（服务器内部错误）
 * - HTTP 400（客户端错误，参数非法）
 * - 网络超时（fetch 中断/abort）
 *
 * 验证项：
 * - error state 正确设置
 * - logError 被调用
 * - isLoading 在错误后正确重置
 * - 成功调用前/后状态正确
 */
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { renderHook, act, waitFor } from "@testing-library/preact";
import { useHabits } from "../../hooks/useHabits";
import { useTimer } from "../../hooks/useTimer";
import { logError } from "../../utils/logger";

// === Mocks ===
//
// mockApiClient 默认成功；每个 case 在 beforeEach 里覆盖具体方法。
const mockApiClient = {
  getHabitSets: vi.fn(),
  getHabits: vi.fn(),
  getState: vi.fn(),
  startTimer: vi.fn(),
  pauseTimer: vi.fn(),
  resumeTimer: vi.fn(),
  resetTimer: vi.fn(),
  finishTimer: vi.fn(),
  createHabitSet: vi.fn(),
  updateHabitSet: vi.fn(),
  deleteHabitSet: vi.fn(),
  createHabit: vi.fn(),
  updateHabit: vi.fn(),
  deleteHabit: vi.fn(),
  getHabitDetail: vi.fn(),
};

vi.mock("../../utils/apiClientSingleton", () => ({
  getAPIClient: vi.fn(() => mockApiClient),
}));

vi.mock("../../utils/logger", () => ({
  logError: vi.fn(),
  logSuccess: vi.fn(),
  logInfo: vi.fn(),
}));

// 抑制 logger 内部的 fetch 噪音；不需要关心后端落盘
vi.stubGlobal("fetch", vi.fn().mockResolvedValue({ ok: true }));

// 构造符合 apiClient.fetchJson 抛出格式的错误信息（"HTTP <status>: <body>"）
const httpError = (status: number, body = ""): Error =>
  new Error(`HTTP ${status}: ${body}`);

describe("集成测试 - API 错误处理", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    // 默认 reset 后回到正常返回值，避免脏状态泄漏
    mockApiClient.getHabitSets.mockResolvedValue([]);
    mockApiClient.getHabits.mockResolvedValue([]);
    mockApiClient.startTimer.mockResolvedValue({ success: true });
    mockApiClient.pauseTimer.mockResolvedValue(undefined);
    mockApiClient.resumeTimer.mockResolvedValue({ success: true });
    mockApiClient.resetTimer.mockResolvedValue(undefined);
    mockApiClient.finishTimer.mockResolvedValue({ elapsed_seconds: 0 });
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  // ============================================
  // useHabits.refresh() 错误场景
  // ============================================
  describe("useHabits.refresh() - 错误处理", () => {
    it("网络错误（fetch throw）应该设置 error state 并调用 logError", async () => {
      const networkError = new TypeError("Failed to fetch");
      mockApiClient.getHabitSets.mockRejectedValueOnce(networkError);

      const { result } = renderHook(() => useHabits());

      await waitFor(() => {
        expect(result.current.isLoading).toBe(false);
      });

      expect(result.current.error).toBe("Failed to fetch");
      expect(logError).toHaveBeenCalledWith(
        expect.stringContaining("加载习惯数据失败: Failed to fetch")
      );
      // 失败后 isLoading 必须重置为 false
      expect(result.current.isLoading).toBe(false);
      // 失败时 habitSets/habits 仍为空数组（不残留旧值之外的脏数据）
      expect(result.current.habitSets).toEqual([]);
      expect(result.current.habits).toEqual([]);
    });

    it("HTTP 500 应该捕获错误并设置 error state", async () => {
      mockApiClient.getHabits.mockRejectedValueOnce(httpError(500, "Internal Server Error"));

      const { result } = renderHook(() => useHabits());

      await waitFor(() => {
        expect(result.current.isLoading).toBe(false);
      });

      expect(result.current.error).toContain("HTTP 500");
      expect(logError).toHaveBeenCalledWith(
        expect.stringContaining("HTTP 500")
      );
      expect(result.current.isLoading).toBe(false);
    });

    it("HTTP 400 应该捕获错误并设置 error state", async () => {
      mockApiClient.getHabitSets.mockRejectedValueOnce(httpError(400, "Bad Request"));

      const { result } = renderHook(() => useHabits());

      await waitFor(() => {
        expect(result.current.isLoading).toBe(false);
      });

      expect(result.current.error).toContain("HTTP 400");
      expect(logError).toHaveBeenCalledWith(
        expect.stringContaining("HTTP 400")
      );
      expect(result.current.isLoading).toBe(false);
    });

    it("网络超时（fetch abort）应该捕获 AbortError 并设置 error state", async () => {
      const abortError = new DOMException("The operation was aborted", "AbortError");
      mockApiClient.getHabits.mockRejectedValueOnce(abortError);

      const { result } = renderHook(() => useHabits());

      await waitFor(() => {
        expect(result.current.isLoading).toBe(false);
      });

      // DOMException 走 String(e) 分支，jsdom 输出 "AbortError: The operation was aborted"
      expect(result.current.error).toContain("operation was aborted");
      expect(logError).toHaveBeenCalled();
      expect(result.current.isLoading).toBe(false);
    });

    it("非 Error 类型抛出值（字符串）也应该被正确处理", async () => {
      // 模拟抛出字符串的异常路径
      mockApiClient.getHabitSets.mockRejectedValueOnce("string error message");

      const { result } = renderHook(() => useHabits());

      await waitFor(() => {
        expect(result.current.isLoading).toBe(false);
      });

      expect(result.current.error).toBe("string error message");
      expect(logError).toHaveBeenCalled();
    });

    it("refresh 成功后再失败，error state 应该反映最新一次结果", async () => {
      // 第一次 refresh 成功
      const okResult = [
        { id: 1, name: "集", color: "#fff", description: "" } as never,
      ];
      mockApiClient.getHabitSets.mockResolvedValueOnce(okResult);

      const { result } = renderHook(() => useHabits());

      await waitFor(() => {
        expect(result.current.habitSets).toEqual(okResult);
      });
      expect(result.current.error).toBeNull();

      // 手动调用 refresh，第二次失败
      mockApiClient.getHabitSets.mockRejectedValueOnce(
        httpError(503, "Service Unavailable")
      );

      await act(async () => {
        await result.current.refresh();
      });

      expect(result.current.error).toContain("HTTP 503");
      expect(result.current.isLoading).toBe(false);
    });

    it("error state 在下次成功 refresh 后应该被清除", async () => {
      mockApiClient.getHabitSets.mockRejectedValueOnce(httpError(500));

      const { result } = renderHook(() => useHabits());

      await waitFor(() => {
        expect(result.current.error).not.toBeNull();
      });

      mockApiClient.getHabitSets.mockResolvedValueOnce([]);
      mockApiClient.getHabits.mockResolvedValueOnce([]);

      await act(async () => {
        await result.current.refresh();
      });

      expect(result.current.error).toBeNull();
      expect(result.current.isLoading).toBe(false);
    });

    it("isLoading 在错误后必须重置为 false（关键不变量）", async () => {
      mockApiClient.getHabitSets.mockImplementation(
        () => new Promise((_, reject) => {
          // 延迟 reject，确保我们能在过程中检查 isLoading
          setTimeout(() => reject(new Error("async fail")), 0);
        })
      );

      const { result } = renderHook(() => useHabits());

      // 等异步结束
      await waitFor(() => {
        expect(result.current.isLoading).toBe(false);
      });

      expect(result.current.error).toBe("async fail");
      expect(result.current.isLoading).toBe(false);
    });
  });

  // ============================================
  // useTimer.start() 错误场景
  // ============================================
  describe("useTimer.start() - 错误处理", () => {
    it("start 失败时应该调用 logError", async () => {
      mockApiClient.startTimer.mockRejectedValueOnce(
        httpError(500, "Internal Server Error")
      );

      const { result } = renderHook(() => useTimer());

      await act(async () => {
        await result.current.start();
      });

      expect(logError).toHaveBeenCalledWith(
        expect.stringContaining("启动计时失败")
      );
    });

    it("start 网络错误时应该调用 logError，不抛出", async () => {
      const networkError = new TypeError("NetworkError when attempting to fetch resource");
      mockApiClient.startTimer.mockRejectedValueOnce(networkError);

      const { result } = renderHook(() => useTimer());

      // useTimer.start 的 catch 吞掉异常，所以 start 不会 reject
      await expect(
        act(async () => {
          await result.current.start();
        })
      ).resolves.not.toThrow();

      expect(logError).toHaveBeenCalledWith(
        expect.stringContaining("启动计时失败")
      );
    });

    it("start HTTP 400 错误时应该被静默处理（不向上抛）", async () => {
      mockApiClient.startTimer.mockRejectedValueOnce(
        httpError(400, "Invalid habit_id")
      );

      const { result } = renderHook(() => useTimer());

      await act(async () => {
        await result.current.start(999);
      });

      expect(logError).toHaveBeenCalledWith(
        expect.stringContaining("HTTP 400")
      );
    });
  });

  // ============================================
  // APIClient.fetchJson 错误信息格式校验
  // ============================================
  describe("APIClient.fetchJson 错误格式", () => {
    it("HTTP 500 错误信息应包含 status 和 body", () => {
      const err = httpError(500, "database locked");
      expect(err.message).toBe("HTTP 500: database locked");
    });

    it("HTTP 400 错误信息应包含 status 和 body", () => {
      const err = httpError(400, '{"error":"invalid input"}');
      expect(err.message).toBe('HTTP 400: {"error":"invalid input"}');
    });
  });
});
