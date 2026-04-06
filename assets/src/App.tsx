import { useState, useEffect } from "preact/hooks";
import { Sidebar } from "./components/Sidebar";
import { TimerPage } from "./TimerPage";
import { HabitsPage } from "./HabitsPage";
import { SettingsPage } from "./Settings.tsx";
import { StatsPage } from "./Stats.tsx";
import { ErrorNotification } from "./components/ErrorNotification.tsx";
import { getAPIClient } from "./utils/apiClientSingleton";
import { WALLPAPER_FALLBACK_GRADIENT, STORAGE_KEYS } from "./utils/constants";

type Page = "timer" | "habits" | "stats" | "settings";
const WALLPAPER_STORAGE_KEY = STORAGE_KEYS.WALLPAPER;
const WALLPAPER_DEBUG_STORAGE_KEY = STORAGE_KEYS.WALLPAPER_DEBUG;

const normalizeWallpaper = (value: unknown): string => {
  return typeof value === "string" ? value.trim() : "";
};

const isWallpaperDebugEnabled = (): boolean => {
  try {
    if (typeof window === "undefined") return false;

    const search = new URLSearchParams(window.location.search);
    if (search.has("debugWallpaper")) return true;

    return localStorage.getItem(WALLPAPER_DEBUG_STORAGE_KEY) === "1";
  } catch {
    return false;
  }
};

const logWallpaperDebug = (event: string, payload?: Record<string, unknown>) => {
  if (!isWallpaperDebugEnabled()) return;

  const time = new Date().toISOString();
  // eslint-disable-next-line no-console
  console.info("[wallpaper-debug]", time, event, payload || {});
};

const readCachedWallpaper = (): string => {
  try {
    return normalizeWallpaper(localStorage.getItem(WALLPAPER_STORAGE_KEY));
  } catch {
    return "";
  }
};

export const App = () => {
  const [page, setPage] = useState<Page>("timer");
  const [globalWallpaper, setGlobalWallpaper] = useState<string | null>(() => {
    const cached = readCachedWallpaper();
    return cached || null;
  });

  const navigateTo = (newPage: Page) => {
    setPage(newPage);
  };

  const updateGlobalWallpaper = (value: string, source = "unknown") => {
    const next = normalizeWallpaper(value);

    logWallpaperDebug("updateGlobalWallpaper", {
      source,
      incoming: value,
      normalized: next,
      prev: globalWallpaper,
    });

    setGlobalWallpaper(next);

    try {
      if (next) {
        localStorage.setItem(WALLPAPER_STORAGE_KEY, next);
      } else {
        localStorage.removeItem(WALLPAPER_STORAGE_KEY);
      }
    } catch {
      // 忽略 localStorage 不可用场景
    }
  };

  useEffect(() => {
    const client = getAPIClient();
    client.getSettings().then(settings => {
      const serverWallpaper = normalizeWallpaper(settings.basic?.wallpaper);
      const cachedWallpaper = readCachedWallpaper();

      logWallpaperDebug("serverSettingsLoaded", {
        serverWallpaper,
        cachedWallpaper,
      });

      // 服务端空值时保留本地最近一次有效值，避免刷新后背景闪回黑色
      updateGlobalWallpaper(serverWallpaper || cachedWallpaper, "server-settings");
    }).catch(() => {
      // 忽略设置获取错误
      const cachedWallpaper = readCachedWallpaper();
      logWallpaperDebug("serverSettingsFailed", { cachedWallpaper });
      updateGlobalWallpaper(cachedWallpaper, "server-fallback-cache");
    });
  }, []);

  const getWallpaperStyle = () => {
    const wp = normalizeWallpaper(globalWallpaper);
    if (!wp) return null;

    if (wp.startsWith("linear")) {
      return { type: "gradient" as const, value: wp };
    }

    if (wp.startsWith("#")) {
      return { type: "color" as const, value: wp };
    }

    // 兜底：除渐变和纯色外，统一按图片 URL/路径处理（支持 https/data/blob/相对路径）
    return { type: "image" as const, value: wp };
  };

  useEffect(() => {
    // 首次加载设置前不改动背景，避免刷新时出现黑屏闪回
    if (globalWallpaper === null) {
      logWallpaperDebug("skipApplyWallpaper", { reason: "pending-initial-load" });
      return;
    }

    const html = document.documentElement;
    const wallpaperInfo = getWallpaperStyle();

    logWallpaperDebug("applyWallpaper:start", {
      globalWallpaper,
      wallpaperType: wallpaperInfo?.type || "none",
      wallpaperValue: wallpaperInfo?.value || "",
    });

    // 清除 html 的所有背景样式和动画
    html.style.background = "";
    html.style.backgroundImage = "";
    html.style.backgroundColor = "";
    html.style.backgroundSize = "";
    html.style.backgroundPosition = "";
    html.style.backgroundRepeat = "";
    html.style.backgroundAttachment = "";
    html.style.animation = "none";

    // 根据壁纸类型设置 html 元素背景
    if (wallpaperInfo) {
      if (wallpaperInfo.type === "gradient") {
        html.style.background = wallpaperInfo.value;
      } else if (wallpaperInfo.type === "image") {
        // 图片层失败时仍显示兜底渐变，避免退化成纯黑背景
        html.style.backgroundColor = "#0d0d0d";
        html.style.backgroundImage = `url("${wallpaperInfo.value.replace(/"/g, '\\"')}"), ${WALLPAPER_FALLBACK_GRADIENT}`;
        html.style.backgroundSize = "cover, 140% 140%";
        html.style.backgroundPosition = "center center, center center";
        html.style.backgroundRepeat = "no-repeat, no-repeat";
      } else if (wallpaperInfo.type === "color") {
        html.style.background = wallpaperInfo.value;
      }
      html.style.backgroundAttachment = "fixed";
    }

    logWallpaperDebug("applyWallpaper:end", {
      htmlBackground: html.style.background,
      htmlBackgroundImage: html.style.backgroundImage,
      htmlAnimation: html.style.animation,
    });
  }, [globalWallpaper]);

  return (
    <>
      <ErrorNotification visible={true} />

      <div className="flex h-screen bg-transparent">
        {/* 侧边栏 - 桌面端 */}
        <div className="hidden lg:block lg:flex shrink-0">
          <Sidebar currentPage={page} onNavigate={navigateTo} />
        </div>

        {/* 主内容区 */}
        <main className="flex-1 flex flex-col overflow-hidden pb-20 lg:pb-0">
          <div className={page === "timer" ? "flex-1" : "hidden"}>
            <TimerPage
              onHabitsClick={() => navigateTo("habits")}
            />
          </div>
          {page === "habits" && (
            <HabitsPage
              onStatsClick={() => navigateTo("stats")}
              onSettingsClick={() => navigateTo("settings")}
            />
          )}
          {page === "stats" && (
            <StatsPage onBackClick={() => navigateTo("timer")} />
          )}
          {page === "settings" && (
            <SettingsPage 
              onBackClick={() => navigateTo("timer")} 
              wallpaper={globalWallpaper || ""}
              onWallpaperChange={(wallpaper) => updateGlobalWallpaper(wallpaper, "settings-prop")}
            />
          )}
        </main>
      </div>

      {/* 底部导航 - 移动端 */}
      <nav
        className="my-bottom-nav lg:hidden fixed inset-x-0 bottom-0 w-full z-50"
        data-testid="bottom-nav"
      >
        <button
          type="button"
          data-testid="nav-timer"
          className={`my-bottom-nav-item ${page === "timer" ? "active" : ""}`}
          onClick={() => navigateTo("timer")}
        >
          <svg xmlns="http://www.w3.org/2000/svg" className="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
          </svg>
          <span className="btm-nav-label">计时</span>
        </button>
        <button
          type="button"
          data-testid="nav-habits"
          className={`my-bottom-nav-item ${page === "habits" ? "active" : ""}`}
          onClick={() => navigateTo("habits")}
        >
          <svg xmlns="http://www.w3.org/2000/svg" className="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2m-6 9l2 2 4-4" />
          </svg>
          <span className="btm-nav-label">习惯</span>
        </button>
        <button
          type="button"
          data-testid="nav-stats"
          className={`my-bottom-nav-item ${page === "stats" ? "active" : ""}`}
          onClick={() => navigateTo("stats")}
        >
          <svg xmlns="http://www.w3.org/2000/svg" className="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z" />
          </svg>
          <span className="btm-nav-label">统计</span>
        </button>
        <button
          type="button"
          data-testid="nav-settings"
          className={`my-bottom-nav-item ${page === "settings" ? "active" : ""}`}
          onClick={() => navigateTo("settings")}
        >
          <svg xmlns="http://www.w3.org/2000/svg" className="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" />
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
          </svg>
          <span className="btm-nav-label">设置</span>
        </button>
      </nav>
    </>
  );
};
