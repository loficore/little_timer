import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { renderHook, act } from "@testing-library/preact";
import { useSSE } from "../../hooks/useSSE";
import { SSEClient } from "../../utils/sseClient";

const mockSSEClientInstance = {
  connect: vi.fn(),
  close: vi.fn(),
  isConnected: vi.fn().mockReturnValue(false),
};

vi.mock("../../utils/sseClient", () => ({
  SSEClient: vi.fn(() => mockSSEClientInstance),
}));

vi.mock("../../utils/apiClientSingleton", () => ({
  getAPIClient: vi.fn(() => ({
    baseUrl: "http://localhost:8080",
  })),
}));

vi.mock("../../utils/logger", () => ({
  logInfo: vi.fn(),
  logError: vi.fn(),
}));

describe("useSSE 扩展测试", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  describe("重连机制", () => {
    it("已经在连接时不应该再次连接", () => {
      mockSSEClientInstance.isConnected.mockReturnValue(true);

      const { result } = renderHook(() => useSSE());

      act(() => {
        result.current.connect();
      });

      act(() => {
        result.current.connect();
      });

      expect(mockSSEClientInstance.connect).toHaveBeenCalledTimes(1);
    });

    it("断开后应该可以重新连接", () => {
      const { result } = renderHook(() => useSSE());

      act(() => {
        result.current.connect();
      });

      expect(mockSSEClientInstance.connect).toHaveBeenCalledTimes(1);

      act(() => {
        result.current.disconnect();
      });

      act(() => {
        result.current.connect();
      });

      expect(mockSSEClientInstance.connect).toHaveBeenCalledTimes(2);
    });
  });

  describe("消息解析", () => {
    it("应该正确处理 TimerState 消息", () => {
      const mockOnMessage = vi.fn();
      const mockTimerState = {
        mode: "stopwatch" as const,
        isRunning: true,
        isPaused: false,
        elapsed_seconds: 100,
      };

      const { result } = renderHook(() => useSSE(mockOnMessage));

      act(() => {
        result.current.connect();
      });

      const connectCallback = mockSSEClientInstance.connect.mock.calls[0][0];
      act(() => {
        connectCallback(mockTimerState);
      });

      expect(result.current.lastState).toEqual(mockTimerState);
      expect(mockOnMessage).toHaveBeenCalledWith(mockTimerState);
    });

    it("应该处理连接成功回调", () => {
      const { result } = renderHook(() => useSSE());

      act(() => {
        result.current.connect();
      });

      const successCallback = mockSSEClientInstance.connect.mock.calls[0][2];
      act(() => {
        successCallback();
      });

      expect(result.current.isConnected).toBe(true);
    });

    it("应该处理错误回调并设置 isConnected 为 false", () => {
      const mockOnError = vi.fn();

      const { result } = renderHook(() => useSSE(undefined, mockOnError));

      act(() => {
        result.current.connect();
      });

      const errorCallback = mockSSEClientInstance.connect.mock.calls[0][1];
      act(() => {
        errorCallback(new Error("Connection failed"));
      });

      expect(result.current.isConnected).toBe(false);
      expect(mockOnError).toHaveBeenCalled();
    });

    it("应该处理字符串错误", () => {
      const mockOnError = vi.fn();

      const { result } = renderHook(() => useSSE(undefined, mockOnError));

      act(() => {
        result.current.connect();
      });

      const errorCallback = mockSSEClientInstance.connect.mock.calls[0][1];
      act(() => {
        errorCallback("Connection failed");
      });

      expect(mockOnError).toHaveBeenCalledWith("Connection failed");
    });

    it("应该处理未知类型错误", () => {
      const mockOnError = vi.fn();

      const { result } = renderHook(() => useSSE(undefined, mockOnError));

      act(() => {
        result.current.connect();
      });

      const errorCallback = mockSSEClientInstance.connect.mock.calls[0][1];
      act(() => {
        errorCallback({ code: 500 });
      });

      expect(mockOnError).toHaveBeenCalled();
    });
  });

  describe("组件卸载清理", () => {
    it("卸载时应该断开 SSE 连接", () => {
      const { result, unmount } = renderHook(() => useSSE());

      act(() => {
        result.current.connect();
      });

      unmount();

      expect(mockSSEClientInstance.close).toHaveBeenCalled();
    });

    it("多次挂载卸载不应该出错", () => {
      for (let i = 0; i < 3; i++) {
        const { result, unmount } = renderHook(() => useSSE());

        act(() => {
          result.current.connect();
        });

        unmount();
      }

      expect(mockSSEClientInstance.close).toHaveBeenCalledTimes(3);
    });
  });

  describe("lastState 状态", () => {
    it("初始应该为 null", () => {
      const { result } = renderHook(() => useSSE());
      expect(result.current.lastState).toBeNull();
    });

    it("收到消息后应该更新 lastState", () => {
      const { result } = renderHook(() => useSSE());

      act(() => {
        result.current.connect();
      });

      const messageCallback = mockSSEClientInstance.connect.mock.calls[0][0];
      act(() => {
        messageCallback({
          mode: "countdown" as const,
          isRunning: true,
          remaining_seconds: 1500,
        });
      });

      expect(result.current.lastState).not.toBeNull();
      expect(result.current.lastState?.mode).toBe("countdown");
    });

    it("disconnect 后 lastState 应该保持不变", () => {
      const { result } = renderHook(() => useSSE());

      act(() => {
        result.current.connect();
      });

      const messageCallback = mockSSEClientInstance.connect.mock.calls[0][0];
      act(() => {
        messageCallback({
          mode: "stopwatch" as const,
          isRunning: true,
          elapsed_seconds: 100,
        });
      });

      act(() => {
        result.current.disconnect();
      });

      expect(result.current.lastState?.elapsed_seconds).toBe(100);
    });
  });
});
