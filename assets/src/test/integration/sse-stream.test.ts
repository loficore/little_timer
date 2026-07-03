import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { renderHook, act } from "@testing-library/preact";
import { useSSE } from "../../hooks/useSSE";
import type { TimerState } from "../../types/api";

const { mockSSEClientInstance, SSEClientMock, callbackSlots } = vi.hoisted(() => {
  const slots: {
    onEvent: ((state: TimerState) => void) | null;
    onError: ((error: Event) => void) | null;
    onConnect: (() => void) | null;
  } = { onEvent: null, onError: null, onConnect: null };

  const instance = {
    connect: vi.fn(
      (
        onEvent: (state: TimerState) => void,
        onError: (error: Event) => void,
        onConnect: () => void,
      ) => {
        slots.onEvent = onEvent;
        slots.onError = onError;
        slots.onConnect = onConnect;
      },
    ),
    close: vi.fn(),
    isConnected: vi.fn().mockReturnValue(false),
  };

  const SSEClient = vi.fn(() => instance);
  return { mockSSEClientInstance: instance, SSEClientMock: SSEClient, callbackSlots: slots };
});

vi.mock("../../utils/sseClient", () => ({
  SSEClient: SSEClientMock,
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

const sampleState: TimerState = {
  status: "running",
  elapsed_seconds: 42,
  target_seconds: 1500,
};

const makeErrorEvent = () => new Event("error");

describe("SSE Stream Integration Lifecycle", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    callbackSlots.onEvent = null;
    callbackSlots.onError = null;
    callbackSlots.onConnect = null;
    mockSSEClientInstance.isConnected.mockReturnValue(false);
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
    vi.restoreAllMocks();
  });

  describe("初始状态", () => {
    it("hook 初始化时 isConnected=false、lastState=null", () => {
      const { result } = renderHook(() => useSSE());

      expect(result.current.isConnected).toBe(false);
      expect(result.current.lastState).toBeNull();
    });

    it("初始化时不应构造 SSEClient 实例", () => {
      renderHook(() => useSSE());

      expect(SSEClientMock).not.toHaveBeenCalled();
      expect(mockSSEClientInstance.connect).not.toHaveBeenCalled();
    });
  });

  describe("connect", () => {
    it("调用 connect 后应使用 baseUrl 构造 SSEClient", () => {
      const { result } = renderHook(() => useSSE());

      act(() => {
        result.current.connect();
      });

      expect(SSEClientMock).toHaveBeenCalledTimes(1);
      expect(SSEClientMock).toHaveBeenCalledWith("http://localhost:8080");
    });

    it("connect 应将 onEvent/onError/onConnect 三个回调注册到 SSEClient", () => {
      const { result } = renderHook(() => useSSE());

      act(() => {
        result.current.connect();
      });

      expect(mockSSEClientInstance.connect).toHaveBeenCalledTimes(1);
      expect(typeof callbackSlots.onEvent).toBe("function");
      expect(typeof callbackSlots.onError).toBe("function");
      expect(typeof callbackSlots.onConnect).toBe("function");
    });

    it("onConnect 回调触发后 isConnected 应变为 true", () => {
      const { result } = renderHook(() => useSSE());

      act(() => {
        result.current.connect();
      });
      expect(result.current.isConnected).toBe(false);

      act(() => {
        callbackSlots.onConnect?.();
      });

      expect(result.current.isConnected).toBe(true);
    });

    it("已连接时再次调用 connect 应为 no-op,不再构造新实例", () => {
      mockSSEClientInstance.isConnected.mockReturnValue(true);
      const { result } = renderHook(() => useSSE());

      act(() => {
        result.current.connect();
      });
      const callsAfterFirst = SSEClientMock.mock.calls.length;

      act(() => {
        result.current.connect();
      });

      expect(SSEClientMock.mock.calls.length).toBe(callsAfterFirst);
    });
  });

  describe("消息接收", () => {
    it("onEvent 回调触发后 lastState 应更新为最新 TimerState", () => {
      const { result } = renderHook(() => useSSE());

      act(() => {
        result.current.connect();
      });

      act(() => {
        callbackSlots.onEvent?.(sampleState);
      });

      expect(result.current.lastState).toEqual(sampleState);
    });

    it("接收消息时应调用 useSSE 注入的 onMessage 回调", () => {
      const onMessage = vi.fn();
      const { result } = renderHook(() => useSSE(onMessage));

      act(() => {
        result.current.connect();
      });

      act(() => {
        callbackSlots.onEvent?.(sampleState);
      });

      expect(onMessage).toHaveBeenCalledTimes(1);
      expect(onMessage).toHaveBeenCalledWith(sampleState);
    });

    it("连续接收多条消息应持续更新 lastState 至最新值", () => {
      const { result } = renderHook(() => useSSE());

      act(() => {
        result.current.connect();
      });

      const states: TimerState[] = [
        { status: "running", elapsed_seconds: 1, target_seconds: 1500 },
        { status: "paused", elapsed_seconds: 2, target_seconds: 1500 },
        { status: "running", elapsed_seconds: 3, target_seconds: 1500 },
      ];

      states.forEach((state) => {
        act(() => {
          callbackSlots.onEvent?.(state);
        });
      });

      expect(result.current.lastState).toEqual(states[states.length - 1]);
    });
  });

  describe("disconnect/reconnect", () => {
    it("onError 触发后 isConnected 应变为 false", () => {
      const { result } = renderHook(() => useSSE());

      act(() => {
        result.current.connect();
      });
      act(() => {
        callbackSlots.onConnect?.();
      });
      expect(result.current.isConnected).toBe(true);

      act(() => {
        callbackSlots.onError?.(makeErrorEvent());
      });

      expect(result.current.isConnected).toBe(false);
    });

    it("onError 应调用 useSSE 注入的 onError 回调并传入原始错误", () => {
      const onError = vi.fn();
      const { result } = renderHook(() => useSSE(undefined, onError));

      act(() => {
        result.current.connect();
      });

      const err = makeErrorEvent();
      act(() => {
        callbackSlots.onError?.(err);
      });

      expect(onError).toHaveBeenCalledTimes(1);
      expect(onError).toHaveBeenCalledWith(err);
    });

    it("disconnect 应调用 SSEClient.close 并将 isConnected 重置为 false", () => {
      const { result } = renderHook(() => useSSE());

      act(() => {
        result.current.connect();
      });
      act(() => {
        callbackSlots.onConnect?.();
      });
      expect(result.current.isConnected).toBe(true);

      act(() => {
        result.current.disconnect();
      });

      expect(mockSSEClientInstance.close).toHaveBeenCalledTimes(1);
      expect(result.current.isConnected).toBe(false);
    });

    it("disconnect 后再 connect 应重建 SSEClient 实例", () => {
      const { result } = renderHook(() => useSSE());

      act(() => {
        result.current.connect();
      });
      expect(SSEClientMock).toHaveBeenCalledTimes(1);

      act(() => {
        result.current.disconnect();
      });

      act(() => {
        result.current.connect();
      });

      expect(SSEClientMock).toHaveBeenCalledTimes(2);
      expect(mockSSEClientInstance.connect).toHaveBeenCalledTimes(2);
    });

    it("错误后调用 connect 重连应恢复正常,新实例 onConnect 后 isConnected=true", () => {
      const { result } = renderHook(() => useSSE());

      act(() => {
        result.current.connect();
      });
      act(() => {
        callbackSlots.onConnect?.();
      });

      act(() => {
        callbackSlots.onError?.(makeErrorEvent());
      });
      expect(result.current.isConnected).toBe(false);

      act(() => {
        vi.advanceTimersByTime(2000);
      });

      act(() => {
        result.current.connect();
      });
      act(() => {
        callbackSlots.onConnect?.();
      });

      expect(result.current.isConnected).toBe(true);
      expect(SSEClientMock).toHaveBeenCalledTimes(2);
    });

    it("错误事件在 reconnect 窗口内外只触发一次 onError 回调", () => {
      const onError = vi.fn();
      const { result } = renderHook(() => useSSE(undefined, onError));

      act(() => {
        result.current.connect();
      });

      act(() => {
        callbackSlots.onError?.(makeErrorEvent());
      });
      expect(onError).toHaveBeenCalledTimes(1);

      act(() => {
        vi.advanceTimersByTime(10000);
      });

      expect(onError).toHaveBeenCalledTimes(1);
    });
  });
});