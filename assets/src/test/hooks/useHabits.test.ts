import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { renderHook } from "@testing-library/preact";
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

describe("useHabits Hook", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  describe("API 客户端调用", () => {
    it("应该在初始化时调用 getHabitSets 和 getHabits", () => {
      renderHook(() => useHabits());

      expect(mockApiClient.getHabitSets).toHaveBeenCalled();
      expect(mockApiClient.getHabits).toHaveBeenCalled();
    });
  });

  describe("refresh", () => {
    it("应该调用 API 刷新数据", async () => {
      const { result } = renderHook(() => useHabits());

      await result.current.refresh();

      expect(mockApiClient.getHabitSets).toHaveBeenCalled();
      expect(mockApiClient.getHabits).toHaveBeenCalled();
    });
  });

  describe("createSet", () => {
    it("应该调用 createHabitSet API", async () => {
      const { result } = renderHook(() => useHabits());

      const newSet = await result.current.createSet("测试", "描述", "#6366f1");

      expect(mockApiClient.createHabitSet).toHaveBeenCalledWith("测试", "描述", "#6366f1");
      expect(newSet).not.toBeNull();
    });
  });

  describe("updateSet", () => {
    it("应该调用 updateHabitSet API", async () => {
      const { result } = renderHook(() => useHabits());

      await result.current.updateSet(1, "测试", "描述", "#6366f1");

      expect(mockApiClient.updateHabitSet).toHaveBeenCalledWith(1, "测试", "描述", "#6366f1");
    });
  });

  describe("deleteSet", () => {
    it("应该调用 deleteHabitSet API", async () => {
      const { result } = renderHook(() => useHabits());

      await result.current.deleteSet(1);

      expect(mockApiClient.deleteHabitSet).toHaveBeenCalledWith(1);
    });
  });

  describe("createHabit", () => {
    it("应该调用 createHabit API", async () => {
      const { result } = renderHook(() => useHabits());

      const newHabit = await result.current.createHabit(1, "测试", 1500, "#6366f1");

      expect(mockApiClient.createHabit).toHaveBeenCalledWith(1, "测试", 1500, "#6366f1");
      expect(newHabit).not.toBeNull();
    });
  });

  describe("updateHabit", () => {
    it("应该调用 updateHabit API", async () => {
      const { result } = renderHook(() => useHabits());

      await result.current.updateHabit(1, "测试", 1500, "#6366f1");

      expect(mockApiClient.updateHabit).toHaveBeenCalledWith(1, "测试", 1500, "#6366f1");
    });
  });

  describe("deleteHabit", () => {
    it("应该调用 deleteHabit API", async () => {
      const { result } = renderHook(() => useHabits());

      await result.current.deleteHabit(1);

      expect(mockApiClient.deleteHabit).toHaveBeenCalledWith(1);
    });
  });

  describe("getHabitDetail", () => {
    it("应该调用 getHabitDetail API", async () => {
      const { result } = renderHook(() => useHabits());

      await result.current.getHabitDetail(1);

      expect(mockApiClient.getHabitDetail).toHaveBeenCalled();
    });
  });
});
