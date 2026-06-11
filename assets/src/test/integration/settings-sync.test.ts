import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { renderHook, act, waitFor } from "@testing-library/preact";
import { useSettings } from "../../hooks/useSettings";

const mockSettingsResponse = {
  basic: {
    timezone: 8,
    language: "ZH",
    default_mode: "countdown",
    theme_mode: "dark",
    wallpaper: "",
    sound_enabled: true,
    sound_tick: true,
    sound_finish: true,
    sound_volume: 80,
  },
  countdown: { duration_seconds: 1500, loop: false, loop_count: 0, loop_interval_seconds: 0 },
  stopwatch: { max_seconds: 86400 },
};

vi.mock("../../utils/apiClientSingleton", () => ({
  getAPIClient: vi.fn(() => ({
    getSettings: vi.fn().mockResolvedValue(mockSettingsResponse),
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

describe("集成测试 - 设置同步流程", () => {
  beforeEach(() => {
    localStorage.clear();
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.restoreAllMocks();
    vi.useRealTimers();
    localStorage.clear();
  });

  describe("设置加载流程", () => {
    it("应该从 API 加载设置并初始化", async () => {
      const { result } = renderHook(() => useSettings());

      await waitFor(() => {
        expect(result.current.settings.timezone).toBe(8);
      });

      expect(result.current.settings.default_mode).toBe("countdown");
      expect(result.current.settings.theme_mode).toBe("dark");
    });

    it("加载完成应该显示 saveMessage 为空", async () => {
      const { result } = renderHook(() => useSettings());

      await waitFor(() => {
        expect(result.current.saveMessage).toBe("");
      });
    });
  });

  describe("设置修改流程", () => {
    it("修改 timezone 应该立即更新本地状态", async () => {
      const { result } = renderHook(() => useSettings());

      await waitFor(() => {
        expect(result.current.settings.timezone).toBe(8);
      });

      act(() => {
        result.current.updateSettings({ timezone: 12 });
      });

      expect(result.current.settings.timezone).toBe(12);
    });

    it("修改 theme_mode 应该立即更新", async () => {
      const { result } = renderHook(() => useSettings());

      await waitFor(() => {
        expect(result.current.settings.theme_mode).toBe("dark");
      });

      act(() => {
        result.current.updateSettings({ theme_mode: "light" });
      });

      expect(result.current.settings.theme_mode).toBe("light");
    });

    it("修改 default_mode 应该立即更新", async () => {
      const { result } = renderHook(() => useSettings());

      await waitFor(() => {
        expect(result.current.settings.default_mode).toBe("countdown");
      });

      act(() => {
        result.current.updateSettings({ default_mode: "stopwatch" });
      });

      expect(result.current.settings.default_mode).toBe("stopwatch");
    });

    it("批量修改多个设置应该同时更新", async () => {
      const { result } = renderHook(() => useSettings());

      await waitFor(() => {
        expect(result.current.settings.timezone).toBe(8);
      });

      act(() => {
        result.current.updateSettings({
          timezone: 9,
          language: "EN",
          theme_mode: "light",
        });
      });

      expect(result.current.settings.timezone).toBe(9);
      expect(result.current.settings.language).toBe("EN");
      expect(result.current.settings.theme_mode).toBe("light");
    });
  });

  describe("设置保存流程", () => {
    it("保存成功应该显示成功消息", async () => {
      const { result } = renderHook(() => useSettings());

      await waitFor(() => {
        expect(result.current.settings.timezone).toBe(8);
      });

      act(() => {
        result.current.updateSettings({ timezone: 12 });
      });

      await act(async () => {
        await result.current.save();
      });

      expect(result.current.saveMessage).toBeTruthy();
    });
  });

  describe("设置重置流程", () => {
    it("重置应该恢复默认设置", async () => {
      const { result } = renderHook(() => useSettings());

      await waitFor(() => {
        expect(result.current.settings.timezone).toBe(8);
      });

      act(() => {
        result.current.updateSettings({ timezone: 12, language: "EN" });
      });

      expect(result.current.settings.timezone).toBe(12);
      expect(result.current.settings.language).toBe("EN");

      act(() => {
        result.current.reset();
      });

      expect(result.current.settings.timezone).toBe(8);
      expect(result.current.settings.language).toBe("ZH");
    });
  });

  describe("设置时钟默认值", () => {
    it("应该加载默认倒计时时长", async () => {
      const { result } = renderHook(() => useSettings());

      await waitFor(() => {
        expect(result.current.clockDefaults.countdown.duration_seconds).toBe(1500);
      });
    });

    it("修改后应该更新 clockDefaults", async () => {
      const { result } = renderHook(() => useSettings());

      await waitFor(() => {
        expect(result.current.clockDefaults.countdown.duration_seconds).toBe(1500);
      });

      act(() => {
        result.current.updateClockDefaults({
          countdown: { duration_seconds: 1800 },
        });
      });

      expect(result.current.clockDefaults.countdown.duration_seconds).toBe(1800);
    });
  });
});