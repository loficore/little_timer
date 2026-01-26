// import { h } from "preact"; // preact 自动注入，无需显式导入
import {
  useEffect,
  useState,
  useRef,
  useCallback,
  useMemo,
} from "preact/hooks";
import { memo } from "preact/compat";
import { logInfo, logSuccess, logError } from "./utils/logger";
import { Mode } from "./utils/share";
import { t } from "./utils/i18n";

interface HomePageProps {
  onSettingsClick?: () => void;
}

// 声明全局 webui 类型
declare global {
  interface Window {
    webui?: {
      call: (functionName: string, ...args: unknown[]) => void;
    };
  }
}

// 倒计时/秒表用：格式化持续时间（秒）
const formatDuration = (totalSeconds: number): string => {
  const hours = Math.floor(totalSeconds / 3600)
    .toString()
    .padStart(2, "0");
  const minutes = Math.floor((totalSeconds % 3600) / 60)
    .toString()
    .padStart(2, "0");
  const seconds = Math.floor(totalSeconds % 60)
    .toString()
    .padStart(2, "0");
  return `${hours}:${minutes}:${seconds}`;
};

// 世界时钟用：格式化 Unix 秒为本地时区时间
const formatClockTime = (unixSeconds: number): string => {
  const d = new Date(unixSeconds * 1000);
  // 使用 UTC 视角读取，避免本地时区再次偏移（否则会双重偏移）
  const h = d.getUTCHours().toString().padStart(2, "0");
  const m = d.getUTCMinutes().toString().padStart(2, "0");
  const s = d.getUTCSeconds().toString().padStart(2, "0");
  return `${h}:${m}:${s}`;
};

interface HomeState {
  time: string;
  mode: Mode;
  isRunning: boolean;
  inRest: boolean;
  loopRemaining: number | null;
  loopTotal: number | null;
  restRemaining: number;
  isFinished: boolean;
  timezone: number; // 前端本地 world clock 时区（来自 settings），单位小时
}

const HomePage = memo(({ onSettingsClick }: HomePageProps) => {
  // 合并所有状态为一个对象，减少多次 setState
  const [state, setState] = useState<HomeState>(() => ({
    time: "25:00:00",
    mode: Mode.Countdown,
    isRunning: false,
    inRest: false,
    loopRemaining: null,
    loopTotal: null,
    restRemaining: 0,
    isFinished: false,
    timezone: 8,
  }));
  const prevFinishedRef = useRef(false);

  useEffect(() => {
    logSuccess("✅ React 应用已加载，准备就绪");
    if (typeof window.webui !== "undefined") {
      logSuccess("✅ webui 对象已加载");
    } else {
      logError("❌ webui 对象未加载！这可能是一个问题");
    }
    // 监听系统主题变化（自动模式）
    const mediaQuery = window.matchMedia("(prefers-color-scheme: light)");
    const handleThemeChange = (e: MediaQueryListEvent) => {
      const theme = e.matches ? "light" : "dark";
      applyTheme(theme);
    };
    mediaQuery.addEventListener("change", handleThemeChange);

    // 设置全局事件处理函数
    (window as any).webuiEvent = (event: any) => {
      logInfo("收到来自后端的事件: " + event.function);
      if (event.function === "update_time") {
        // 只在 time 变化时 setState
        const seconds = typeof event.data === "number" ? event.data : 0;
        setState((prev) => {
          // 世界时钟交给前端自驱动，不使用后端时间
          if (prev.mode === Mode.WorldClock) return prev;
          const formatted = formatDuration(seconds);
          if (prev.time === formatted) return prev;
          logInfo("⏱️ 时间已更新: " + seconds + "秒 -> " + formatted);
          return { ...prev, time: formatted };
        });
      } else if (event.function === "update_mode") {
        const newMode = event.data as Mode;
        logInfo(
          "🔄 收到模式更新事件，新模式值: " +
            newMode +
            " (类型: " +
            typeof newMode +
            ")",
        );
        setState((prev) => {
          if (prev.mode === newMode) {
            logInfo("🔄 模式相同，跳过更新");
            return prev;
          }
          logSuccess("🔄 模式已更新: " + prev.mode + " -> " + newMode);
          return { ...prev, mode: newMode };
        });
      } else if (event.function === "update_state") {
        const s = event.data as {
          isRunning: boolean;
          isFinished: boolean;
          inRest: boolean;
          loopRemaining?: number;
          loopTotal?: number;
          restRemaining?: number;
          timezone?: number;
        };
        setState((prev) => {
          // 只有有变化时才 setState，避免无谓重渲染
          if (
            prev.isRunning === s.isRunning &&
            prev.inRest === s.inRest &&
            prev.loopRemaining === (s.loopRemaining ?? null) &&
            prev.loopTotal === (s.loopTotal ?? null) &&
            prev.restRemaining === (s.restRemaining ?? 0) &&
            prev.isFinished === s.isFinished &&
            prev.timezone === (s.timezone ?? prev.timezone)
          ) {
            return prev;
          }
          return {
            ...prev,
            isRunning: s.isRunning,
            inRest: s.inRest,
            loopRemaining: s.loopRemaining ?? null,
            loopTotal: s.loopTotal ?? null,
            restRemaining: s.restRemaining ?? 0,
            isFinished: s.isFinished,
            timezone: s.timezone ?? prev.timezone,
          };
        });
        // 完成通知（仅在从未完成到完成时触发一次）
        if (s.isFinished && !prevFinishedRef.current) {
          try {
            const AudioCtx =
              (window as any).AudioContext ||
              (window as any).webkitAudioContext;
            if (AudioCtx) {
              const ctx = new AudioCtx();
              const o = ctx.createOscillator();
              const g = ctx.createGain();
              o.type = "sine";
              o.frequency.value = 880;
              g.gain.value = 0.06;
              o.connect(g);
              g.connect(ctx.destination);
              o.start();
              setTimeout(() => {
                o.stop();
                ctx.close();
              }, 300);
            }
          } catch {
            // 忽略音频相关错误
          }
          try {
            if ("Notification" in window) {
              if (Notification.permission === "granted") {
                new Notification(t("home.finished_notification"));
              } else if (Notification.permission !== "denied") {
                Notification.requestPermission().then((p) => {
                  if (p === "granted")
                    new Notification(t("home.finished_notification"));
                });
              }
            }
          } catch {
            // 忽略通知相关错误
          }
        }
        prevFinishedRef.current = s.isFinished;
        logInfo("📊 状态已更新: " + JSON.stringify(s));
      }
    };
    logInfo("初始化完成，等待用户交互...");
  }, []);

  // 世界时钟前端自驱 tick：根据 settings 时区每秒刷新
  useEffect(() => {
    if (state.mode !== Mode.WorldClock) return;

    const tick = () => {
      const now = Math.floor(Date.now() / 1000);
      const shifted = now + state.timezone * 3600;
      setState((prev) => ({ ...prev, time: formatClockTime(shifted) }));
    };

    tick();
    const timer = window.setInterval(tick, 1000);
    return () => window.clearInterval(timer);
  }, [state.mode, state.timezone]);

  // 应用主题
  const applyTheme = useCallback((theme: string) => {
    const html = document.documentElement;
    if (theme === "light") {
      html.classList.add("light-mode");
      document.body.classList.add("light-mode");
    } else {
      html.classList.remove("light-mode");
      document.body.classList.remove("light-mode");
    }
  }, []);

  // 控制按钮事件全部用 useCallback 包裹，避免不必要的重渲染
  const handleStart = useCallback(() => {
    logInfo('🚀 "开始"按钮被点击');
    try {
      if (typeof window.webui === "undefined") {
        logError("❌ webui 对象未定义！后端连接失败");
        return;
      }
      logInfo("✓ webui 对象存在，准备调用 start 函数");
      window.webui.call("start");
      // 只由后端事件驱动 isRunning 状态
      logSuccess('✓ webui.call("start") 调用成功');
    } catch (e) {
      const errorMsg = e instanceof Error ? e.message : String(e);
      logError("❌ 调用 start 时发生错误: " + errorMsg);
      console.error(e);
    }
  }, []);

  const handlePause = useCallback(() => {
    logInfo('⏸️ "暂停"按钮被点击');
    try {
      if (typeof window.webui === "undefined") {
        logError("❌ webui 对象未定义！后端连接失败");
        return;
      }
      logInfo("✓ webui 对象存在，准备调用 pause 函数");
      window.webui.call("pause");
      logSuccess('✓ webui.call("pause") 调用成功');
    } catch (e) {
      const errorMsg = e instanceof Error ? e.message : String(e);
      logError("❌ 调用 pause 时发生错误: " + errorMsg);
      console.error(e);
    }
  }, []);

  const handleReset = useCallback(() => {
    logInfo('🔄 "重置"按钮被点击');
    try {
      if (typeof window.webui === "undefined") {
        logError("❌ webui 对象未定义！");
        return;
      }
      logInfo("✓ webui 对象存在，准备调用 reset 函数");
      window.webui.call("reset");
      logSuccess('✓ webui.call("reset") 调用成功');
    } catch (e) {
      const errorMsg = e instanceof Error ? e.message : String(e);
      logError("❌ 调用 reset 时发生错误: " + errorMsg);
      console.error(e);
    }
  }, []);

  const handleModeChange = useCallback((newMode: Mode) => {
    try {
      if (typeof window.webui === "undefined") {
        logError("❌ webui 对象未定义！");
        return;
      }
      logInfo(`✓ webui 对象存在，准备调用 change_mode 函数，参数: ${newMode}`);
      window.webui.call("change_mode", newMode);
      logSuccess('✓ webui.call("change_mode") 调用成功');
    } catch (e) {
      const errorMsg = e instanceof Error ? e.message : String(e);
      logError("❌ 调用 change_mode 时发生错误: " + errorMsg);
      console.error(e);
    }
  }, []);

  // 计算 memo 化的状态，避免不必要的渲染
  const statusMemo = useMemo(() => {
    return {
      isRunning: state.isRunning,
      isFinished: state.isFinished,
      inRest: state.inRest,
      loopRemaining: state.loopRemaining,
      loopTotal: state.loopTotal,
      restRemaining: state.restRemaining,
      mode: state.mode,
      time: state.time,
    };
  }, [
    state.isRunning,
    state.isFinished,
    state.inRest,
    state.loopRemaining,
    state.loopTotal,
    state.restRemaining,
    state.mode,
    state.time,
  ]);

  return (
    <div className="flex flex-col w-screen h-screen bg-primary-dark dark:bg-primary-dark transition-colors duration-300 animate-fadeIn overflow-hidden">
      {/* 标题栏 */}
      <div className="flex justify-between items-center px-4 sm:px-6 md:px-8 py-3 sm:py-4 md:py-6 border-b border-border-dark flex-shrink-0">
        <h1 className="text-lg sm:text-xl md:text-2xl font-semibold text-text-primary-dark truncate pr-2">
          {t("common.app_name")}
        </h1>
        <button
          onClick={onSettingsClick}
          title={t("common.settings_title")}
          className="w-10 h-10 flex-shrink-0 flex items-center justify-center rounded-xl bg-transparent border-0 cursor-pointer transition-all duration-200 text-text-secondary-dark hover:bg-secondary-dark hover:text-text-primary-dark hover:scale-110 active:bg-tertiary-dark"
        >
          ⚙
        </button>
      </div>

      {/* 主内容区域 */}
      <div className="flex-1 flex flex-col items-center justify-center px-4 sm:px-6 md:px-8 py-4 sm:py-6 md:py-12 overflow-y-auto">
        {/* 状态指示器 */}
        <div
          className="flex gap-2 sm:gap-3 md:gap-4 justify-center mb-6 sm:mb-8 flex-wrap animate-slideUp w-full"
          style={{ animationDelay: "0.1s", animationFillMode: "both" }}
        >
          <span
            className={`px-3 sm:px-4 py-1.5 sm:py-2 rounded-full text-xs sm:text-sm font-medium transition-all duration-200 whitespace-nowrap ${
              statusMemo.isRunning
                ? "bg-accent-dark text-white border border-accent-dark animate-pulse"
                : prevFinishedRef.current
                  ? "bg-green-600 text-white border border-green-600"
                  : "bg-secondary-dark text-text-secondary-dark border border-border-dark"
            }`}
          >
            {statusMemo.isRunning
              ? t("home.status_running")
              : prevFinishedRef.current
                ? "✅ 已完成"
                : t("home.status_paused")}
          </span>
          <span className="px-3 sm:px-4 py-1.5 sm:py-2 rounded-full text-xs sm:text-sm font-medium bg-accent-dark text-white border border-accent-dark whitespace-nowrap">
            {(() => {
              if (statusMemo.mode === Mode.Countdown)
                return t("home.mode_countdown");
              if (statusMemo.mode === Mode.Stopwatch)
                return t("home.mode_stopwatch");
              return t("home.mode_world_clock");
            })()}
          </span>
        </div>

        {/* 时间显示 */}
        <div
          className={`text-4xl sm:text-6xl md:text-8xl font-light tracking-wider text-text-primary-dark font-mono my-4 sm:my-6 md:my-6 text-center transition-all duration-300 animate-slideUp break-all ${
            statusMemo.isRunning ? "text-accent-dark" : ""
          }`}
          style={{ animationDelay: "0.2s", animationFillMode: "both" }}
        >
          {statusMemo.time}
        </div>

        {/* 循环和休息状态提示 */}
        {(statusMemo.inRest ||
          (statusMemo.loopRemaining !== null &&
            statusMemo.loopTotal !== null &&
            statusMemo.loopTotal > 0)) && (
          <div
            className="text-center mb-3 sm:mb-4 text-xs sm:text-sm text-text-secondary-dark animate-slideUp w-full px-2"
            style={{ animationDelay: "0.25s", animationFillMode: "both" }}
          >
            {statusMemo.inRest && (
              <div className="text-accent-dark font-semibold">
                {t("home.rest_status", { seconds: statusMemo.restRemaining })}
              </div>
            )}
            {statusMemo.loopRemaining !== null &&
              statusMemo.loopTotal !== null &&
              statusMemo.loopTotal > 0 &&
              !statusMemo.inRest && (
                <div className="text-accent-dark font-semibold">
                  {t("home.loop_status", {
                    remaining: statusMemo.loopRemaining,
                    total: statusMemo.loopTotal,
                  })}
                </div>
              )}
          </div>
        )}

        {/* 主控制按钮 */}
        <div
          className="flex gap-2 sm:gap-3 md:gap-4 my-4 sm:my-6 md:my-8 justify-center flex-wrap animate-slideUp w-full"
          style={{ animationDelay: "0.3s", animationFillMode: "both" }}
        >
          {!statusMemo.isRunning ? (
            <button
              onClick={handleStart}
              className="btn-primary flex-1 sm:flex-none sm:w-32 md:w-40 flex items-center justify-center gap-1 sm:gap-2 text-sm sm:text-base"
            >
              <span className="text-lg sm:text-xl">▶</span>
              <span className="hidden sm:inline">{t("home.start")}</span>
            </button>
          ) : (
            <button
              onClick={handlePause}
              className="btn-primary flex-1 sm:flex-none sm:w-32 md:w-40 flex items-center justify-center gap-1 sm:gap-2 text-sm sm:text-base"
            >
              <span className="text-lg sm:text-xl">⏸</span>
              <span className="hidden sm:inline">{t("home.pause")}</span>
            </button>
          )}
          <button
            onClick={handleReset}
            className="btn-secondary flex-1 sm:flex-none sm:w-32 md:w-40 flex items-center justify-center gap-1 sm:gap-2 text-sm sm:text-base"
          >
            <span className="text-lg sm:text-xl">↻</span>
            <span className="hidden sm:inline">{t("home.reset")}</span>
          </button>
        </div>

        {/* 模式切换 */}
        <div
          className="mt-6 sm:mt-8 md:mt-12 pt-4 sm:pt-6 md:pt-8 border-t border-border-dark text-center animate-slideUp w-full"
          style={{ animationDelay: "0.4s", animationFillMode: "both" }}
        >
          <h3 className="text-xs font-semibold text-text-secondary-dark uppercase tracking-wider mb-3 sm:mb-4 md:mb-6 px-2">
            {t("home.switch_mode")}
          </h3>
          <div className="grid grid-cols-3 gap-2 sm:gap-3 md:gap-4 px-2">
            {[
              {
                key: Mode.Countdown,
                label: t("home.mode_countdown"),
                icon: "⏱",
              },
              {
                key: Mode.Stopwatch,
                label: t("home.mode_stopwatch"),
                icon: "⏲",
              },
              {
                key: Mode.WorldClock,
                label: t("home.mode_world_clock"),
                icon: "🌐",
              },
            ].map(({ key, label, icon }) => (
              <button
                key={key}
                onClick={() => handleModeChange(key)}
                className={`p-2 sm:p-3 md:p-4 rounded-xl flex flex-col items-center gap-1 sm:gap-2 text-xs sm:text-sm font-medium transition-all duration-200 hover:scale-105 active:scale-95 min-h-[60px] sm:min-h-[80px] ${
                  statusMemo.mode === key
                    ? "bg-accent-dark border-accent-dark text-white"
                    : "bg-transparent border border-border-dark text-text-secondary-dark hover:bg-secondary-dark hover:border-accent-dark hover:text-text-primary-dark"
                }`}
              >
                <span className="text-xl sm:text-2xl">{icon}</span>
                <span className="text-center line-clamp-2">{label}</span>
              </button>
            ))}
          </div>
        </div>
      </div>

      {/* 页脚 */}
      <div
        className="px-4 sm:px-6 py-3 sm:py-4 md:py-6 text-center text-text-secondary-dark text-xs border-t border-border-dark bg-primary-dark animate-slideUp flex-shrink-0"
        style={{ animationDelay: "0.5s", animationFillMode: "both" }}
      >
        <p>{t("common.version")}</p>
      </div>
    </div>
  );
});

export { HomePage };
