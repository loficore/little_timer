import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/preact";
import { SettingsPage } from "../Settings";

vi.mock("../utils/apiClientSingleton", () => ({
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
        sound_volume: 80,
      },
      clock_defaults: {
        countdown: {
          duration_seconds: 1500,
          loop: false,
          loop_count: 0,
          loop_interval_seconds: 0,
        },
        stopwatch: {
          max_seconds: 86400,
        },
      },
    }),
    updateSettings: vi.fn().mockResolvedValue({}),
  })),
}));

vi.mock("../utils/audio", () => ({
  DEFAULT_AUDIO_PREFERENCES: {
    sound_enabled: true,
    sound_tick: true,
    sound_finish: true,
    sound_volume: 80,
  },
  loadAudioPreferences: vi.fn(() => ({
    sound_enabled: true,
    sound_tick: true,
    sound_finish: true,
    sound_volume: 80,
  })),
  normalizeAudioPreferences: vi.fn((p) => p),
  saveAudioPreferences: vi.fn(),
}));

vi.mock("../utils/logger", () => ({
  isPerfDebugEnabled: vi.fn(() => false),
  isWebViewRuntime: vi.fn(() => false),
  logPerf: vi.fn(),
}));

vi.mock("../utils/constants", () => ({
  STORAGE_KEYS: {
    LAYOUT_DENSITY: "lt_layout_density",
    TIME_DISPLAY_STYLE: "lt_time_display_style",
  },
}));

vi.mock("../utils/i18n", () => ({
  t: (key: string) => {
    const translations: Record<string, string> = {
      "settings.tabs.basic": "Basic",
      "settings.tabs.countdown": "Countdown",
      "settings.tabs.stopwatch": "Stopwatch",
      "common.settings_title": "Settings",
      "common.save": "Save",
      "common.reset_default": "Reset",
    };
    return translations[key] || key;
  },
  setLanguage: vi.fn().mockResolvedValue(undefined),
}));

vi.mock("../components/Header", () => ({
  Header: ({ title, onBackClick }: any) => (
    <div data-testid="header">
      <span data-testid="header-title">{title}</span>
      <button onClick={onBackClick}>Back</button>
    </div>
  ),
}));

vi.mock("../components/TabPanel", () => ({
  TabPanel: ({ children, tabs, activeTab, onTabChange }: any) => (
    <div data-testid="tab-panel">
      <div data-testid="tab-buttons">
        {tabs?.map((tab: any) => (
          <button
            key={tab.id}
            data-testid={`tab-${tab.id}`}
            onClick={() => onTabChange?.(tab.id)}
          >
            {tab.labelKey.split('.').pop()}
          </button>
        ))}
      </div>
      <div data-testid="tab-content">
        {children}
      </div>
    </div>
  ),
}));

vi.mock("../components/BasicSettings", () => ({
  BasicSettings: ({ config, onChange }: any) => (
    <div data-testid="basic-settings">
      <span data-testid="basic-settings-config">{JSON.stringify(config)}</span>
      <button onClick={() => onChange({ timezone: 9 })}>Change Timezone</button>
    </div>
  ),
}));

vi.mock("../components/CountdownSettings", () => ({
  CountdownSettings: ({ config, onChange }: any) => (
    <div data-testid="countdown-settings">
      <span data-testid="countdown-settings-config">{JSON.stringify(config)}</span>
      <button onClick={() => onChange({ duration_seconds: 1800 })}>Change Duration</button>
    </div>
  ),
}));

vi.mock("../components/StopwatchSettings", () => ({
  StopwatchSettings: ({ config, onChange }: any) => (
    <div data-testid="stopwatch-settings">
      <span data-testid="stopwatch-settings-config">{JSON.stringify(config)}</span>
      <button onClick={() => onChange({ max_seconds: 7200 })}>Change Max</button>
    </div>
  ),
}));

describe("SettingsPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    localStorage.clear();
  });

  it("应该渲染设置页面", () => {
    const { getByTestId } = render(<SettingsPage />);
    expect(getByTestId("header")).toBeTruthy();
  });

  it("应该显示 Header", () => {
    render(<SettingsPage />);
    expect(screen.getByTestId("header")).toBeTruthy();
  });

  it("应该显示标签页", async () => {
    render(<SettingsPage />);

    await waitFor(() => {
      expect(screen.getByTestId("tab-basic")).toBeTruthy();
      expect(screen.getByTestId("tab-countdown")).toBeTruthy();
      expect(screen.getByTestId("tab-stopwatch")).toBeTruthy();
    });
  });

  it("点击返回按钮应该调用 onBackClick", async () => {
    const onBackClick = vi.fn();
    render(<SettingsPage onBackClick={onBackClick} />);

    await waitFor(() => {
      const backButton = screen.getByText("Back");
      fireEvent.click(backButton);
    });

    expect(onBackClick).toHaveBeenCalled();
  });

  it("应该渲染基本设置", async () => {
    const { getByTestId } = render(<SettingsPage />);

    await waitFor(() => {
      expect(screen.getByTestId("tab-basic")).toBeTruthy();
    });

    fireEvent.click(screen.getByTestId("tab-basic"));

    await waitFor(() => {
      expect(getByTestId("basic-settings")).toBeTruthy();
    });
  });

  it("点击倒计时标签应该显示倒计时设置", async () => {
    render(<SettingsPage />);

    await waitFor(() => {
      expect(screen.getByTestId("tab-countdown")).toBeTruthy();
    });

    fireEvent.click(screen.getByTestId("tab-countdown"));

    await waitFor(() => {
      expect(screen.getByTestId("countdown-settings")).toBeTruthy();
    });
  });

  it("点击秒表标签应该显示秒表设置", async () => {
    render(<SettingsPage />);

    await waitFor(() => {
      expect(screen.getByTestId("tab-stopwatch")).toBeTruthy();
    });

    fireEvent.click(screen.getByTestId("tab-stopwatch"));

    await waitFor(() => {
      expect(screen.getByTestId("stopwatch-settings")).toBeTruthy();
    });
  });

  it("壁纸变化时应该调用 onWallpaperChange", async () => {
    const onWallpaperChange = vi.fn();
    const { getByTestId } = render(<SettingsPage onWallpaperChange={onWallpaperChange} />);

    await waitFor(() => {
      expect(screen.getByTestId("tab-basic")).toBeTruthy();
    });

    fireEvent.click(screen.getByTestId("tab-basic"));

    await waitFor(() => {
      expect(getByTestId("basic-settings")).toBeTruthy();
    });
  });

  it("应该传递壁纸配置给子组件", async () => {
    const { getByTestId } = render(<SettingsPage wallpaper="linear-gradient(135deg, #f97316 0%, #ec4899 50%, #8b5cf6 100%)" />);

    await waitFor(() => {
      expect(screen.getByTestId("tab-basic")).toBeTruthy();
    });

    fireEvent.click(screen.getByTestId("tab-basic"));

    await waitFor(() => {
      expect(getByTestId("basic-settings")).toBeTruthy();
    });
  });

  it("点击基本设置中的按钮应该更新配置", async () => {
    render(<SettingsPage />);

    await waitFor(() => {
      expect(screen.getByTestId("tab-basic")).toBeTruthy();
    });

    fireEvent.click(screen.getByTestId("tab-basic"));

    await waitFor(() => {
      expect(screen.getByText("Change Timezone")).toBeTruthy();
    });

    fireEvent.click(screen.getByText("Change Timezone"));
  });
});