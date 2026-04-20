import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { renderHook, act } from "@testing-library/preact";
import { useHabits } from "../../hooks/useHabits";

const mockApiClient = {
  getHabitSets: vi.fn().mockResolvedValue([]),
  getHabits: vi.fn().mockResolvedValue([]),
  createHabitSet: vi.fn().mockResolvedValue({ id: 1, name: "新习惯集" }),
  updateHabitSet: vi.fn().mockResolvedValue({}),
  deleteHabitSet: vi.fn().mockResolvedValue({}),
  createHabit: vi.fn().mockResolvedValue({ id: 1, name: "新习惯" }),
  updateHabit: vi.fn().mockResolvedValue({}),
  deleteHabit: vi.fn().mockResolvedValue({}),
  getHabitDetail: vi.fn().mockResolvedValue(null),
};

vi.mock("../../utils/apiClientSingleton", () => ({
  getAPIClient: vi.fn(() => mockApiClient),
}));

vi.mock("../../utils/logger", () => ({
  logError: vi.fn(),
}));

describe("useHabits 扩展测试", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockApiClient.getHabitSets.mockResolvedValue([]);
    mockApiClient.getHabits.mockResolvedValue([]);
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  describe("createHabit", () => {
    it("应该正常创建习惯", async () => {
      const { result } = renderHook(() => useHabits());

      let newHabit: any;
      await act(async () => {
        newHabit = await result.current.createHabit(1, "背单词", 1500, "#6366f1");
      });

      expect(mockApiClient.createHabit).toHaveBeenCalledWith(1, "背单词", 1500, "#6366f1");
      expect(newHabit).not.toBeUndefined();
    });

    it("创建习惯时 API 抛出错误应该返回 null", async () => {
      mockApiClient.createHabit.mockRejectedValueOnce(new Error("创建失败"));

      const { result } = renderHook(() => useHabits());

      let newHabit: any;
      await act(async () => {
        newHabit = await result.current.createHabit(1, "背单词", 1500, "#6366f1");
      });

      expect(newHabit).toBeNull();
    });
  });

  describe("updateHabit", () => {
    it("应该正常更新习惯", async () => {
      const { result } = renderHook(() => useHabits());

      await act(async () => {
        await result.current.updateHabit(1, "更新名称", 1800, "#8b5cf6");
      });

      expect(mockApiClient.updateHabit).toHaveBeenCalledWith(1, "更新名称", 1800, "#8b5cf6");
    });

    it("更新习惯时 API 抛出错误应该不抛出异常", async () => {
      mockApiClient.updateHabit.mockRejectedValueOnce(new Error("更新失败"));

      const { result } = renderHook(() => useHabits());

      await act(async () => {
        await result.current.updateHabit(999, "名称", 1500, "#6366f1");
      });

      expect(mockApiClient.updateHabit).toHaveBeenCalled();
    });
  });

  describe("deleteHabit", () => {
    it("应该正常删除习惯", async () => {
      const { result } = renderHook(() => useHabits());

      await act(async () => {
        await result.current.deleteHabit(1);
      });

      expect(mockApiClient.deleteHabit).toHaveBeenCalledWith(1);
    });

    it("删除习惯时 API 抛出错误应该不抛出异常", async () => {
      mockApiClient.deleteHabit.mockRejectedValueOnce(new Error("删除失败"));

      const { result } = renderHook(() => useHabits());

      await act(async () => {
        await result.current.deleteHabit(999);
      });

      expect(mockApiClient.deleteHabit).toHaveBeenCalled();
    });
  });

  describe("空数据状态", () => {
    it("getHabitSets 返回空数组时应该显示空状态", async () => {
      mockApiClient.getHabitSets.mockResolvedValueOnce([]);

      const { result } = renderHook(() => useHabits());

      await act(async () => {
        await result.current.refresh();
      });

      expect(result.current.habitSets).toEqual([]);
    });

    it("getHabits 抛出错误时应该设置 error 状态", async () => {
      mockApiClient.getHabits.mockRejectedValueOnce(new Error("网络错误"));

      const { result } = renderHook(() => useHabits());

      await act(async () => {
        await result.current.refresh();
      });

      expect(result.current.error).toBe("网络错误");
    });

    it("refresh 成功后应该清除 error 状态", async () => {
      mockApiClient.getHabits
        .mockRejectedValueOnce(new Error("之前的错误"))
        .mockResolvedValueOnce([]);

      const { result } = renderHook(() => useHabits());

      await act(async () => {
        await result.current.refresh();
      });
      expect(result.current.error).toBe("之前的错误");

      await act(async () => {
        await result.current.refresh();
      });
      expect(result.current.error).toBeNull();
    });
  });

  describe("getHabitsBySet", () => {
    it("无习惯数据时应该返回空数组", () => {
      const { result } = renderHook(() => useHabits());

      const set1Habits = result.current.getHabitsBySet(1);

      expect(set1Habits).toHaveLength(0);
    });
  });
});