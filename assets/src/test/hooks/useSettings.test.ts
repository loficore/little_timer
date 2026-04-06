import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { renderHook, act, waitFor } from "@testing-library/preact";
import { useSettings, type BasicSettings, type ClockDefaults } from "../../hooks/useSettings";

vi.mock("../../utils/apiClientSingleton", () => ({
  getAPIClient: vi.fn(() => ({
    getSettings: vi.fn().mockResolvedValue({
      basic: {
        timezone: 8,
        language: "ZH",
        default_mode: "countdown",
        theme_mode: "dark",
        wallpaper: "",
        sound_enabled: true,
        sound_tick: true,
        sound_finish: true,
        sound_volume: 0.5,
      },
      countdown: { duration_seconds: 1500, loop: false, loop_count: 0, loop_interval_seconds: 0 },
      stopwatch: { max_seconds: 86400 },
    }),
    updateSettings: vi.fn().mockResolvedValue({ status: "ok" }),
  })),
}));

vi.mock("../../utils/i18n", () => ({
  t: vi.fn((key: string) => key),
}));

vi.mock("../../utils/logger", () => ({
  logError: vi.fn(),
}));

vi.mock("../../utils/constants", () => ({
  STORAGE_KEYS: {
    LAYOUT_DENSITY: "layout_density",
    TIME_DISPLAY_STYLE: "time_display_style",
  },
}));

describe("useSettings Hook", () => {
  beforeEach(() => {
    localStorage.clear();
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.restoreAllMocks();
    vi.useRealTimers();
    localStorage.clear();
  });

  describe("初始状态", () => {
    it("应该返回默认设置", () => {
      const { result } = renderHook(() => useSettings());

      expect(result.current.settings.timezone).toBe(8);
      expect(result.current.settings.language).toBe("ZH");
      expect(result.current.settings.default_mode).toBe("countdown");
      expect(result.current.settings.theme_mode).toBe("dark");
    });

    it("应该返回默认时钟配置", () => {
      const { result } = renderHook(() => useSettings());

      expect(result.current.clockDefaults.countdown.duration_seconds).toBe(1500);
      expect(result.current.clockDefaults.countdown.loop).toBe(false);
      expect(result.current.clockDefaults.stopwatch.max_seconds).toBe(86400);
    });

    it("初始状态 isSaving 应为 false", () => {
      const { result } = renderHook(() => useSettings());

      expect(result.current.isSaving).toBe(false);
    });
  });

  describe("updateSettings", () => {
    it("应该更新部分设置", async () => {
      const { result } = renderHook(() => useSettings());

      await waitFor(() => expect(result.current.settings.timezone).toBe(8));

      act(() => {
        result.current.updateSettings({ timezone: 12 });
      });

      expect(result.current.settings.timezone).toBe(12);
    });

    it("应该保留未更新的设置", async () => {
      const { result } = renderHook(() => useSettings());

      await waitFor(() => expect(result.current.settings.language).toBe("ZH"));

      act(() => {
        result.current.updateSettings({ timezone: 12 });
      });

      expect(result.current.settings.language).toBe("ZH");
    });
  });

  describe("updateClockDefaults", () => {
    it("应该更新倒计时默认配置", async () => {
      const { result } = renderHook(() => useSettings());

      await waitFor(() => expect(result.current.clockDefaults.countdown.duration_seconds).toBe(1500));

      act(() => {
        result.current.updateClockDefaults({ countdown: { duration_seconds: 1800 } });
      });

      expect(result.current.clockDefaults.countdown.duration_seconds).toBe(1800);
    });

    it("应该更新正计时默认配置", async () => {
      const { result } = renderHook(() => useSettings());

      await waitFor(() => expect(result.current.clockDefaults.stopwatch.max_seconds).toBe(86400));

      act(() => {
        result.current.updateClockDefaults({ stopwatch: { max_seconds: 3600 } });
      });

      expect(result.current.clockDefaults.stopwatch.max_seconds).toBe(3600);
    });
  });

  describe("save", () => {
    it("保存时应该设置 isSaving 状态", async () => {
      const { result } = renderHook(() => useSettings());

      await waitFor(() => expect(result.current.settings.timezone).toBe(8));

      act(() => {
        result.current.save();
      });

      expect(result.current.isSaving).toBe(true);
    });

    it("保存成功应该显示成功消息", async () => {
      const { result } = renderHook(() => useSettings());

      await waitFor(() => expect(result.current.settings.timezone).toBe(8));

      await act(async () => {
        await result.current.save();
      });

      expect(result.current.saveMessage).toBe("common.save_success");
    });
  });

  describe("reset", () => {
    it("应该重置设置为默认值", async () => {
      const { result } = renderHook(() => useSettings());

      await waitFor(() => expect(result.current.settings.timezone).toBe(8));

      act(() => {
        result.current.updateSettings({ timezone: 12 });
      });

      act(() => {
        result.current.reset();
      });

      expect(result.current.settings.timezone).toBe(8);
    });

    it("应该重置时钟配置", async () => {
      const { result } = renderHook(() => useSettings());

      await waitFor(() => expect(result.current.clockDefaults.countdown.duration_seconds).toBe(1500));

      act(() => {
        result.current.updateClockDefaults({ countdown: { duration_seconds: 1800 } });
      });

      act(() => {
        result.current.reset();
      });

      expect(result.current.clockDefaults.countdown.duration_seconds).toBe(1500);
    });
  });

  describe("主题应用", () => {
    it("应该在加载时应用主题", async () => {
      const { result } = renderHook(() => useSettings());

      await waitFor(() => expect(result.current.settings.theme_mode).toBe("dark"));

      expect(document.documentElement.classList.contains("light-mode")).toBe(false);
    });
  });
});
