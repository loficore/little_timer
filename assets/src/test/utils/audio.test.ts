import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import {
  loadAudioPreferences,
  saveAudioPreferences,
  normalizeAudioPreferences,
  DEFAULT_AUDIO_PREFERENCES,
  type AudioPreferences,
} from "../../utils/audio";

describe("audio", () => {
  beforeEach(() => {
    vi.stubGlobal("localStorage", {
      getItem: vi.fn(),
      setItem: vi.fn(),
      removeItem: vi.fn(),
      clear: vi.fn(),
    });
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  describe("DEFAULT_AUDIO_PREFERENCES", () => {
    it("应该有正确的默认值", () => {
      expect(DEFAULT_AUDIO_PREFERENCES.sound_enabled).toBe(true);
      expect(DEFAULT_AUDIO_PREFERENCES.sound_tick).toBe(false);
      expect(DEFAULT_AUDIO_PREFERENCES.sound_finish).toBe(true);
      expect(DEFAULT_AUDIO_PREFERENCES.sound_volume).toBe(35);
    });
  });

  describe("normalizeAudioPreferences", () => {
    it("空对象时应该返回默认值", () => {
      const result = normalizeAudioPreferences({});
      expect(result).toEqual(DEFAULT_AUDIO_PREFERENCES);
    });

    it("null 时应该返回默认值", () => {
      const result = normalizeAudioPreferences(null);
      expect(result).toEqual(DEFAULT_AUDIO_PREFERENCES);
    });

    it("undefined 时应该返回默认值", () => {
      const result = normalizeAudioPreferences(undefined);
      expect(result).toEqual(DEFAULT_AUDIO_PREFERENCES);
    });

    it("部分属性时应该合并默认值", () => {
      const result = normalizeAudioPreferences({ sound_enabled: false });
      expect(result.sound_enabled).toBe(false);
      expect(result.sound_tick).toBe(DEFAULT_AUDIO_PREFERENCES.sound_tick);
      expect(result.sound_finish).toBe(DEFAULT_AUDIO_PREFERENCES.sound_finish);
      expect(result.sound_volume).toBe(DEFAULT_AUDIO_PREFERENCES.sound_volume);
    });

    it("音量超过 100 时应该限制为 100", () => {
      const result = normalizeAudioPreferences({ sound_volume: 150 });
      expect(result.sound_volume).toBe(100);
    });

    it("音量小于 0 时应该限制为 0", () => {
      const result = normalizeAudioPreferences({ sound_volume: -10 });
      expect(result.sound_volume).toBe(0);
    });

    it("小数音量应该四舍五入", () => {
      const result = normalizeAudioPreferences({ sound_volume: 35.6 });
      expect(result.sound_volume).toBe(36);
    });
  });

  describe("loadAudioPreferences", () => {
    it("localStorage 为空时返回默认值", () => {
      vi.mocked(localStorage.getItem).mockReturnValue(null);

      const result = loadAudioPreferences();
      expect(result).toEqual(DEFAULT_AUDIO_PREFERENCES);
    });

    it("localStorage 有数据时解析并规范化", () => {
      const prefs = { sound_enabled: false, sound_tick: true, sound_finish: false, sound_volume: 50 };
      vi.mocked(localStorage.getItem).mockReturnValue(JSON.stringify(prefs));

      const result = loadAudioPreferences();
      expect(result.sound_enabled).toBe(false);
      expect(result.sound_tick).toBe(true);
      expect(result.sound_finish).toBe(false);
      expect(result.sound_volume).toBe(50);
    });

    it("localStorage 数据无效 JSON 时返回默认值", () => {
      vi.mocked(localStorage.getItem).mockReturnValue("invalid json");

      const result = loadAudioPreferences();
      expect(result).toEqual(DEFAULT_AUDIO_PREFERENCES);
    });

    it("localStorage 解析失败时返回默认值", () => {
      vi.mocked(localStorage.getItem).mockImplementation(() => {
        throw new Error("Storage error");
      });

      const result = loadAudioPreferences();
      expect(result).toEqual(DEFAULT_AUDIO_PREFERENCES);
    });
  });

  describe("saveAudioPreferences", () => {
    it("应该保存规范化后的数据", () => {
      saveAudioPreferences({ sound_enabled: true, sound_tick: false, sound_finish: true, sound_volume: 75 });

      expect(localStorage.setItem).toHaveBeenCalledWith(
        "little_timer_audio_preferences",
        JSON.stringify({ sound_enabled: true, sound_tick: false, sound_finish: true, sound_volume: 75 })
      );
    });

    it("应该规范化超出范围的值", () => {
      saveAudioPreferences({ sound_enabled: true, sound_tick: false, sound_finish: true, sound_volume: 200 });

      expect(localStorage.setItem).toHaveBeenCalledWith(
        "little_timer_audio_preferences",
        JSON.stringify({ sound_enabled: true, sound_tick: false, sound_finish: true, sound_volume: 100 })
      );
    });

    it("localStorage 写入失败时应该静默处理", () => {
      vi.mocked(localStorage.setItem).mockImplementation(() => {
        throw new Error("Quota exceeded");
      });

      expect(() => saveAudioPreferences(DEFAULT_AUDIO_PREFERENCES)).not.toThrow();
    });
  });
});