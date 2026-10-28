import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen } from "@testing-library/preact";
import type { ComponentChildren } from "preact";
import { App } from "../App";

// Android 能力探测(supportsDailySweep=false)时底栏不应渲染每日梳理入口。
// 与 App.test.tsx 同套 mock 清单,仅 apiClientSingleton 工厂不同。
vi.mock("../utils/apiClientSingleton", () => ({
  supportsDailySweep: false,
  getAPIClient: vi.fn(() => ({
    getSettings: vi.fn().mockImplementation(() =>
      Promise.resolve({
        basic: {
          theme_mode: "dark",
          wallpaper: "",
        },
      })
    ),
  })),
}));

vi.mock("../utils/logger", () => ({
  getFrontendLogLevel: vi.fn(() => "info"),
  isPerfDebugEnabled: vi.fn(() => false),
  isWebViewRuntime: vi.fn(() => false),
  logError: vi.fn(),
  logLifecycle: vi.fn(),
  logPerf: vi.fn(),
}));

vi.mock("../utils/constants", () => ({
  WALLPAPER_FALLBACK_GRADIENT: "linear-gradient(135deg, #1a1a2e 0%, #16213e 100%)",
  STORAGE_KEYS: {
    WALLPAPER: "lt_wallpaper",
    WALLPAPER_DEBUG: "lt_wallpaper_debug",
  },
  resolveWallpaperUrl: vi.fn((value: string) => value),
}));

vi.mock("../components/Sidebar", () => ({
  Sidebar: () => <div data-testid="sidebar" />,
}));

vi.mock("../TimerPage", () => ({
  TimerPage: () => <div data-testid="timer-page" />,
}));

vi.mock("../HabitsPage", () => ({
  HabitsPage: () => <div data-testid="habits-page" />,
}));

vi.mock("../Stats", () => ({
  StatsPage: () => <div data-testid="stats-page" />,
}));

vi.mock("../Settings", () => ({
  SettingsPage: () => <div data-testid="settings-page" />,
}));

vi.mock("../components/ErrorBoundary", () => ({
  ErrorBoundary: ({ children }: { children: ComponentChildren }) => <div>{children}</div>,
}));

vi.mock("../components/common/Toast", () => ({
  ToastContainer: () => <div data-testid="toast-container" />,
  showToast: vi.fn(),
}));

describe("App 底部导航 Android 能力探测", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    localStorage.clear();
    document.documentElement.removeAttribute("style");
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("supportsDailySweep 为 false 时不渲染 nav-dailySweep,其余导航项照常", () => {
    render(<App />);

    expect(screen.queryByTestId("nav-dailySweep")).toBeNull();
    expect(screen.getByTestId("bottom-nav")).toBeTruthy();
    expect(screen.getByTestId("nav-timer")).toBeTruthy();
    expect(screen.getByTestId("nav-settings")).toBeTruthy();
  });
});
