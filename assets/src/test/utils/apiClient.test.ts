import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { APIClient } from "../../utils/apiClient";

const mockFetch = vi.fn();

global.fetch = mockFetch;

describe("APIClient", () => {
  let client: APIClient;

  beforeEach(() => {
    vi.clearAllMocks();
    client = new APIClient("http://localhost:8080");
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  describe("getState", () => {
    it("应该返回计时器状态", async () => {
      const mockState = {
        time: 100,
        mode: "stopwatch",
        is_running: true,
        is_finished: false,
        in_rest: false,
        loop_remaining: 0,
        loop_total: 0,
        rest_remaining: 0,
        timezone: 8,
      };

      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: async () => mockState,
      });

      const result = await client.getState();

      expect(mockFetch).toHaveBeenCalledWith("http://localhost:8080/api/state");
      expect(result).toEqual(mockState);
    });

    it("请求失败时应该抛出错误", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: false,
        statusText: "Not Found",
      });

      await expect(client.getState()).rejects.toThrow("Error fetching state: Not Found");
    });
  });

  describe("startTimer", () => {
    it("应该发送带习惯ID的请求", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: async () => ({ status: "started", habit_id: 1 }),
      });

      const result = await client.startTimer(1);

      expect(mockFetch).toHaveBeenCalledWith(
        "http://localhost:8080/api/start",
        expect.objectContaining({
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ habit_id: 1 }),
        })
      );
      expect(result).toEqual({ status: "started", habit_id: 1 });
    });

    it("应该发送带选项的请求", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: async () => ({ status: "started" }),
      });

      await client.startTimer(undefined, {
        mode: "countdown",
        workDuration: 1500,
        restDuration: 300,
        loopCount: 4,
      });

      expect(mockFetch).toHaveBeenCalledWith(
        "http://localhost:8080/api/start",
        expect.objectContaining({
          method: "POST",
          body: JSON.stringify({
            mode: "countdown",
            work_duration: 1500,
            rest_duration: 300,
            loop_count: 4,
          }),
        })
      );
    });

    it("无参数时应该发送空请求体", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: async () => ({ status: "started" }),
      });

      await client.startTimer();

      expect(mockFetch).toHaveBeenCalledWith(
        "http://localhost:8080/api/start",
        expect.objectContaining({
          method: "POST",
        })
      );
    });
  });

  describe("pauseTimer", () => {
    it("应该发送暂停请求", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: async () => ({ status: "paused" }),
      });

      await client.pauseTimer();

      expect(mockFetch).toHaveBeenCalledWith(
        "http://localhost:8080/api/pause",
        expect.objectContaining({ method: "POST" })
      );
    });
  });

  describe("resetTimer", () => {
    it("应该发送重置请求", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
      });

      await client.resetTimer();

      expect(mockFetch).toHaveBeenCalledWith(
        "http://localhost:8080/api/reset",
        expect.objectContaining({ method: "POST" })
      );
    });
  });

  describe("getSettings", () => {
    it("应该返回设置", async () => {
      const mockSettings = {
        basic: { timezone: 8, language: "ZH", default_mode: "countdown" },
      };

      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: async () => mockSettings,
      });

      const result = await client.getSettings();

      expect(mockFetch).toHaveBeenCalledWith("http://localhost:8080/api/settings");
      expect(result).toEqual(mockSettings);
    });
  });

  describe("updateSettings", () => {
    it("应该发送设置更新请求", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
      });

      const newSettings = { basic: { timezone: 8 } };
      await client.updateSettings(newSettings);

      expect(mockFetch).toHaveBeenCalledWith(
        "http://localhost:8080/api/settings",
        expect.objectContaining({
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify(newSettings),
        })
      );
    });
  });

  describe("habit CRUD", () => {
    it("getHabits 应该返回习惯列表", async () => {
      const mockHabits = [
        { id: 1, name: "背单词", goal_seconds: 1500 },
      ];

      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: async () => mockHabits,
      });

      const result = await client.getHabits();

      expect(mockFetch).toHaveBeenCalledWith("http://localhost:8080/api/habits");
      expect(result).toEqual(mockHabits);
    });

    it("createHabit 应该创建习惯", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: async () => ({ id: 1, name: "新习惯", goal_seconds: 1500 }),
      });

      const result = await client.createHabit(1, "新习惯", 1500);

      expect(mockFetch).toHaveBeenCalledWith(
        "http://localhost:8080/api/habits",
        expect.objectContaining({
          method: "POST",
          body: JSON.stringify({ set_id: 1, name: "新习惯", goal_seconds: 1500 }),
        })
      );
      expect(result.id).toBe(1);
    });

    it("updateHabit 应该更新习惯", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: async () => ({ success: true }),
      });

      await client.updateHabit(1, "更新后", 1800);

      expect(mockFetch).toHaveBeenCalledWith(
        "http://localhost:8080/api/habits/1",
        expect.objectContaining({
          method: "PUT",
        })
      );
    });

    it("deleteHabit 应该删除习惯", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: async () => ({ success: true }),
      });

      await client.deleteHabit(1);

      expect(mockFetch).toHaveBeenCalledWith(
        "http://localhost:8080/api/habits/1",
        expect.objectContaining({ method: "DELETE" })
      );
    });
  });

  describe("habit set CRUD", () => {
    it("getHabitSets 应该返回习惯集列表", async () => {
      const mockSets = [{ id: 1, name: "学习", color: "#6366f1" }];

      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: async () => mockSets,
      });

      const result = await client.getHabitSets();

      expect(mockFetch).toHaveBeenCalledWith("http://localhost:8080/api/habit-sets");
      expect(result).toEqual(mockSets);
    });

    it("createHabitSet 应该创建习惯集", async () => {
      mockFetch.mockResolvedValueOnce({
        ok: true,
        json: async () => ({ id: 1, name: "新习惯集" }),
      });

      const result = await client.createHabitSet("新习惯集", "描述", "#ff0000");

      expect(mockFetch).toHaveBeenCalledWith(
        "http://localhost:8080/api/habit-sets",
        expect.objectContaining({
          method: "POST",
          body: JSON.stringify({ name: "新习惯集", description: "描述", color: "#ff0000" }),
        })
      );
      expect(result.id).toBe(1);
    });
  });

  describe("错误处理", () => {
    describe("网络错误", () => {
      it("网络断开时应该抛出网络错误", async () => {
        mockFetch.mockRejectedValueOnce(new TypeError("Failed to fetch"));

        await expect(client.getState()).rejects.toThrow("Failed to fetch");
      });

      it("请求超时时应该抛出超时错误", async () => {
        mockFetch.mockRejectedValueOnce(new DOMException("The operation was aborted.", "AbortError"));

        await expect(client.getState()).rejects.toThrow();
      });
    });

    describe("4xx 客户端错误", () => {
      it("400 错误时应该抛出详细错误信息", async () => {
        mockFetch.mockResolvedValueOnce({
          ok: false,
          status: 400,
          statusText: "Bad Request",
        });

        await expect(client.getState()).rejects.toThrow("Error fetching state: Bad Request");
      });

      it("401 错误时应该抛出认证错误", async () => {
        mockFetch.mockResolvedValueOnce({
          ok: false,
          status: 401,
          statusText: "Unauthorized",
        });

        await expect(client.getState()).rejects.toThrow("Error fetching state: Unauthorized");
      });

      it("403 错误时应该抛出禁止错误", async () => {
        mockFetch.mockResolvedValueOnce({
          ok: false,
          status: 403,
          statusText: "Forbidden",
        });

        await expect(client.getSettings()).rejects.toThrow("Error fetching settings: Forbidden");
      });
    });

    describe("5xx 服务器错误", () => {
      it("500 错误时应该抛出服务器错误", async () => {
        mockFetch.mockResolvedValueOnce({
          ok: false,
          status: 500,
          statusText: "Internal Server Error",
        });

        await expect(client.getSettings()).rejects.toThrow("Error fetching settings: Internal Server Error");
      });

      it("502 错误时应该抛出网关错误", async () => {
        mockFetch.mockResolvedValueOnce({
          ok: false,
          status: 502,
          statusText: "Bad Gateway",
        });

        await expect(client.getHabitSets()).rejects.toThrow("Error fetching habit sets: Bad Gateway");
      });

      it("503 错误时应该抛出服务不可用", async () => {
        mockFetch.mockResolvedValueOnce({
          ok: false,
          status: 503,
          statusText: "Service Unavailable",
        });

        await expect(client.getHabits()).rejects.toThrow("Error fetching habits: Service Unavailable");
      });
    });

    describe("JSON 解析错误", () => {
      it("响应不是有效 JSON 时应该抛出解析错误", async () => {
        mockFetch.mockResolvedValueOnce({
          ok: true,
          json: async () => {
            throw new SyntaxError("Unexpected token } in JSON at position 10");
          },
        });

        await expect(client.getState()).rejects.toThrow();
      });
    });

    describe("请求取消", () => {
      it("请求被取消时应该抛出错误", async () => {
        mockFetch.mockRejectedValueOnce(new DOMException("The operation was aborted.", "AbortError"));

        await expect(client.startTimer()).rejects.toThrow();
      });
    });
  });
});
