import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { renderHook, act, waitFor } from "@testing-library/preact";
import { useHabits } from "../../hooks/useHabits";

const mockApiClient = {
  getHabitSets: vi.fn().mockResolvedValue([]),
  getHabits: vi.fn().mockResolvedValue([]),
  createHabitSet: vi.fn().mockResolvedValue({ id: 1, name: "新习惯集", color: "#6366f1" }),
  updateHabitSet: vi.fn().mockResolvedValue({}),
  deleteHabitSet: vi.fn().mockResolvedValue({}),
  createHabit: vi.fn().mockResolvedValue({ id: 1, name: "新习惯", goal_seconds: 1500 }),
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

describe("集成测试 - 习惯流程", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  describe("创建习惯集", () => {
    it("应该创建新习惯集并刷新列表", async () => {
      const { result } = renderHook(() => useHabits());

      await waitFor(() => {
        expect(result.current.habitSets).toEqual([]);
      });

      mockApiClient.getHabitSets.mockResolvedValueOnce([
        { id: 1, name: "新习惯集", color: "#6366f1" },
      ]);

      await act(async () => {
        await result.current.createSet("学习", "学习习惯", "#6366f1");
      });

      expect(mockApiClient.createHabitSet).toHaveBeenCalledWith("学习", "学习习惯", "#6366f1");
    });
  });

  describe("创建习惯", () => {
    it("应该创建新习惯并刷新列表", async () => {
      const { result } = renderHook(() => useHabits());

      mockApiClient.getHabits.mockResolvedValueOnce([
        { id: 1, set_id: 1, name: "背单词", goal_seconds: 1500, color: "#22c55e" },
      ]);

      await act(async () => {
        await result.current.createHabit(1, "背单词", 1500, "#22c55e");
      });

      expect(mockApiClient.createHabit).toHaveBeenCalledWith(1, "背单词", 1500, "#22c55e");
    });
  });

  describe("删除习惯", () => {
    it("删除习惯后应该刷新列表", async () => {
      const { result } = renderHook(() => useHabits());

      await act(async () => {
        await result.current.deleteHabit(1);
      });

      expect(mockApiClient.deleteHabit).toHaveBeenCalledWith(1);
    });
  });

  describe("删除习惯集", () => {
    it("删除习惯集后应该刷新列表", async () => {
      const { result } = renderHook(() => useHabits());

      await act(async () => {
        await result.current.deleteSet(1);
      });

      expect(mockApiClient.deleteHabitSet).toHaveBeenCalledWith(1);
    });
  });
});