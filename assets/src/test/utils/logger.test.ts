import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { logInfo, logError, logSuccess, logPerf, logLifecycle, logOperation, logNetwork } from "../../utils/logger";

describe("logger utils", () => {
  let mockFetch: ReturnType<typeof vi.fn>;
  let mockConsoleLog: ReturnType<typeof vi.fn>;
  let mockConsoleError: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    vi.clearAllMocks();
    mockFetch = vi.fn().mockResolvedValue({ ok: true });
    mockConsoleLog = vi.fn();
    mockConsoleError = vi.fn();
    vi.stubGlobal("fetch", mockFetch);
    vi.stubGlobal("console", {
      ...console,
      log: mockConsoleLog,
      error: mockConsoleError,
    });
    vi.stubGlobal("window", {
      webui: undefined,
      location: { search: "" },
    });
  });

  afterEach(() => {
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
  });

  describe("logInfo", () => {
    it("应该记录日志到 console", () => {
      logInfo("test message");
      expect(mockConsoleLog).toHaveBeenCalled();
    });

    it("应该发送到后端", () => {
      logInfo("test message");

      expect(mockFetch).toHaveBeenCalledWith("/api/log", expect.objectContaining({
        method: "POST",
        headers: { "Content-Type": "application/json", "If-Match": '"0"' },
        body: expect.stringContaining("test message"),
      }));
    });
  });

  describe("logError", () => {
    it("应该记录错误消息和堆栈", () => {
      const error = new Error("test error");
      logError("Operation failed", error);
      expect(mockConsoleError).toHaveBeenCalled();
    });

    it("应该处理无错误对象的场景", () => {
      logError("Simple error message");
      expect(mockConsoleError).toHaveBeenCalled();
    });
  });

  describe("logToBackend routing", () => {
    it("with wails bridge: routes via wails.log, skips fetch", () => {
      const wailsLogMock = vi.fn();
      vi.stubGlobal("window", {
        wails: { log: wailsLogMock },
        webui: undefined,
        location: { search: "" },
      });

      logError("wails test error");

      expect(wailsLogMock).toHaveBeenCalledWith("error", "wails test error");
      expect(mockFetch).not.toHaveBeenCalled();
    });

    it("without wails bridge: falls back to fetch /api/log", () => {
      // beforeEach 桩的 window 没有 wails —— 走桌面回退路径
      logInfo("desktop test message");

      expect(mockFetch).toHaveBeenCalledWith(
        "/api/log",
        expect.objectContaining({
          method: "POST",
          headers: { "Content-Type": "application/json", "If-Match": '"0"' },
          body: expect.stringContaining("desktop test message"),
        })
      );
    });

    // Regression /api/log 挂在 api 路由组,RequireVersionForMutation
    // 中间件强制 POST 带 `If-Match: "<version>"`。日志条目本身无版本
    // 语义,固定 `"0"`。缺 header 会被 ConflictGuard 400 拒收。
    it("POST /api/log 必须携带 If-Match: '\"0\"' 否则被 ConflictGuard 400", () => {
      logError("regression: header 必须带 If-Match");

      expect(mockFetch).toHaveBeenCalledTimes(1);
      const [, init] = mockFetch.mock.calls[0] as [string, RequestInit];
      expect(init.method).toBe("POST");
      expect((init.headers as Record<string, string>)["If-Match"]).toBe('"0"');
    });
  });

  describe("logSuccess", () => {
    it("应该记录成功日志", () => {
      logSuccess("Operation completed");
      expect(mockConsoleLog).toHaveBeenCalled();
    });
  });

  describe("logPerf", () => {
    it("WebView 环境应该记录 perf", () => {
      vi.stubGlobal("window", {
        webui: { call: vi.fn() },
        location: { search: "" },
      });

      logPerf("test scope", { duration: 100 });
      expect(mockConsoleLog).toHaveBeenCalled();
    });
  });

  describe("便捷函数", () => {
    it("logLifecycle 应该使用 lifecycle 类别", () => {
      logLifecycle("app started");
      expect(mockConsoleLog).toHaveBeenCalled();
    });

    it("logOperation 应该使用 operation 类别", () => {
      logOperation("user clicked");
      expect(mockConsoleLog).toHaveBeenCalled();
    });

    it("logNetwork 应该使用 network 类别", () => {
      logNetwork("api called");
      expect(mockConsoleLog).toHaveBeenCalled();
    });
  });
});
