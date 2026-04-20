import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { SSEClient, type TimerState } from "../../utils/sseClient";

describe("SSEClient", () => {
  let client: SSEClient;
  let mockEventSource: {
    close: ReturnType<typeof vi.fn>;
    onopen: ((event: Event) => void) | null;
    onmessage: ((event: MessageEvent) => void) | null;
    onerror: ((event: Event) => void) | null;
    addEventListener: ReturnType<typeof vi.fn>;
    readyState: number;
  };

  beforeEach(() => {
    mockEventSource = {
      close: vi.fn(),
      onopen: null,
      onmessage: null,
      onerror: null,
      addEventListener: vi.fn((event: string, handler: Function) => {
        if (event === "ping") {
          // noop
        }
      }),
      readyState: 1,
    };

    const MockEventSource = vi.fn(() => mockEventSource);
    (MockEventSource as any).OPEN = 1;

    vi.stubGlobal("EventSource", MockEventSource);
    client = new SSEClient("http://localhost:8080");
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  describe("构造函数", () => {
    it("应该设置 baseUrl", () => {
      const testClient = new SSEClient("http://example.com");
      expect(testClient).toBeDefined();
    });
  });

  describe("connect", () => {
    it("应该创建 EventSource 连接", () => {
      const onEvent = vi.fn();
      client.connect(onEvent);

      expect(EventSource).toHaveBeenCalledWith("http://localhost:8080/api/events");
    });

    it("连接成功时应该调用 onConnect 回调", () => {
      const onEvent = vi.fn();
      const onConnect = vi.fn();
      client.connect(onEvent, undefined, onConnect);

      mockEventSource.onopen?.({} as Event);

      expect(onConnect).toHaveBeenCalled();
    });

    it("接收到消息时应该解析并调用 onEvent", () => {
      const onEvent = vi.fn();
      const mockState: TimerState = { time: 100, mode: "stopwatch", is_running: true };

      client.connect(onEvent);

      mockEventSource.onmessage?.({
        data: JSON.stringify(mockState),
      } as MessageEvent);

      expect(onEvent).toHaveBeenCalledWith(mockState);
    });

    it("忽略 SSE 心跳消息", () => {
      const onEvent = vi.fn();

      client.connect(onEvent);

      mockEventSource.onmessage?.({
        data: ": heartbeat",
      } as MessageEvent);

      expect(onEvent).not.toHaveBeenCalled();
    });

    it("无效 JSON 时应该静默处理", () => {
      const onEvent = vi.fn();

      client.connect(onEvent);

      mockEventSource.onmessage?.({
        data: "invalid json",
      } as MessageEvent);

      expect(onEvent).not.toHaveBeenCalled();
    });

    it("空消息时应该静默处理", () => {
      const onEvent = vi.fn();

      client.connect(onEvent);

      mockEventSource.onmessage?.({
        data: "",
      } as MessageEvent);

      expect(onEvent).not.toHaveBeenCalled();
    });
  });

  describe("close", () => {
    it("应该关闭 EventSource", () => {
      const onEvent = vi.fn();
      client.connect(onEvent);

      client.close();

      expect(mockEventSource.close).toHaveBeenCalled();
    });

    it("应该重置重连计数", () => {
      const onEvent = vi.fn();
      client.connect(onEvent);

      client.close();

      expect(client.isConnected()).toBe(false);
    });
  });

  describe("isConnected", () => {
    it("EventSource 打开时应该返回 true", () => {
      const onEvent = vi.fn();
      client.connect(onEvent);

      expect(client.isConnected()).toBe(true);
    });

    it("EventSource 关���时应该返回 false", () => {
      const onEvent = vi.fn();
      mockEventSource.readyState = 2; // CLOSED
      client.connect(onEvent);

      expect(client.isConnected()).toBe(false);
    });
  });

  describe("错误处理", () => {
    it("连接错误时应该调用 onError", () => {
      const onEvent = vi.fn();
      const onError = vi.fn();
      client.connect(onEvent, onError);

      mockEventSource.onerror?.({} as Event);

      expect(onError).toHaveBeenCalled();
    });

    it("连接错误时应该尝试重连", () => {
      vi.useFakeTimers();

      const onEvent = vi.fn();
      const onError = vi.fn();
      client.connect(onEvent, onError);

      mockEventSource.onerror?.({} as Event);

      vi.advanceTimersByTime(2000);

      expect(EventSource).toHaveBeenCalledTimes(2);

      vi.useRealTimers();
    });

    it("超过最大重连次数后应该停止", () => {
      vi.useFakeTimers();

      const onEvent = vi.fn();
      const onError = vi.fn();
      client.connect(onEvent, onError);

      for (let i = 0; i < 6; i++) {
        mockEventSource.onerror?.({} as Event);
        vi.advanceTimersByTime(2000 * Math.pow(2, i));
      }

      vi.advanceTimersByTime(100000);

      expect(EventSource).toHaveBeenCalledTimes(6);

      vi.useRealTimers();
    });
  });
});