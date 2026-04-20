import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { renderHook, act } from "@testing-library/preact";
import { useSSE } from "../../hooks/useSSE";

const mockSSEClient = {
  connect: vi.fn(),
  close: vi.fn(),
  isConnected: vi.fn().mockReturnValue(false),
};

vi.mock("../../utils/sseClient", () => ({
  SSEClient: vi.fn(() => mockSSEClient),
}));

vi.mock("../../utils/apiClientSingleton", () => ({
  getAPIClient: vi.fn(() => ({
    baseUrl: "http://localhost:8080",
  })),
}));

describe("useSSE Hook", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  describe("初始状态", () => {
    it("应该返回初始未连接状态", () => {
      const { result } = renderHook(() => useSSE());

      expect(result.current.isConnected).toBe(false);
      expect(result.current.lastState).toBeNull();
    });
  });

  describe("connect", () => {
    it("应该调用 SSE 客户端的 connect 方法", () => {
      const { result } = renderHook(() => useSSE());

      act(() => {
        result.current.connect();
      });

      expect(mockSSEClient.connect).toHaveBeenCalled();
    });
  });

  describe("disconnect", () => {
    it("应该关闭连接", () => {
      const { result } = renderHook(() => useSSE());

      act(() => {
        result.current.connect();
      });

      act(() => {
        result.current.disconnect();
      });

      expect(mockSSEClient.close).toHaveBeenCalled();
    });

    it("应该设置 isConnected 为 false", () => {
      const { result } = renderHook(() => useSSE());

      act(() => {
        result.current.disconnect();
      });

      expect(result.current.isConnected).toBe(false);
    });
  });

  describe("清理", () => {
    it("组件卸载时应该断开连接", () => {
      const { result, unmount } = renderHook(() => useSSE());

      act(() => {
        result.current.connect();
      });

      expect(() => unmount()).not.toThrow();
    });
  });
});
