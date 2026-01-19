import type { FunctionalComponent } from "preact";
import { useEffect, useState, useRef } from "preact/hooks";
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

const formatTime = (totalSeconds: number): string => {
  const hours = Math.floor(totalSeconds / 3600)
    .toString()
    .padStart(2, "0");
  const minutes = Math.floor((totalSeconds % 3600) / 60)
    .toString()
    .padStart(2, "0");
  const seconds = (totalSeconds % 60).toString().padStart(2, "0");
  return `${hours}:${minutes}:${seconds}`;
};

export const HomePage: FunctionalComponent<HomePageProps> = ({
  onSettingsClick,
}) => {
  const [time, setTime] = useState("25:00:00");
  const [mode, setMode] = useState<Mode>(Mode.Countdown);
  const [isRunning, setIsRunning] = useState(false);
  const [inRest, setInRest] = useState(false);
  const [loopRemaining, setLoopRemaining] = useState<number | null>(null);
  const [loopTotal, setLoopTotal] = useState<number | null>(null);
  const [restRemaining, setRestRemaining] = useState<number>(0);
  const prevFinishedRef = useRef(false);

  useEffect(() => {
    logSuccess("✅ React 应用已加载，准备就绪");

    // 检查 webui 对象是否存在
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
    (
      window as Window & {
        webuiEvent?: (event: {
          function: string;
          data:
            | number
            | string
            | {
                isRunning: boolean;
                isFinished: boolean;
                inRest: boolean;
                loopRemaining?: number;
                loopTotal?: number;
                restRemaining?: number;
              };
        }) => void;
        updateSettingsDisplay?: (settingsJson: string) => void;
      }
    ).webuiEvent = (event) => {
      logInfo("收到来自后端的事件: " + event.function);

      if (event.function === "update_time") {
        // 后端发送秒数（数字），前端格式化为 HH:MM:SS
        const seconds = typeof event.data === "number" ? event.data : 0;
        const formatted = formatTime(seconds);
        setTime(formatted);
        logInfo("⏱️ 时间已更新: " + seconds + "秒 -> " + formatted);
      } else if (event.function === "update_mode") {
        setMode(event.data as Mode);
        logInfo("🔄 模式已更新: " + event.data);
      } else if (event.function === "update_state") {
        // 处理状态更新（运行中、已完成、休息中等）
        const state = event.data as {
          isRunning: boolean;
          isFinished: boolean;
          inRest: boolean;
          loopRemaining?: number;
          loopTotal?: number;
          restRemaining?: number;
        };

        setIsRunning(state.isRunning);
        setInRest(state.inRest);
        setLoopRemaining(state.loopRemaining ?? null);
        setLoopTotal(state.loopTotal ?? null);
        setRestRemaining(state.restRemaining ?? 0);

        // 完成通知（仅在从未完成到完成时触发一次）
        if (state.isFinished && !prevFinishedRef.current) {
          // 播放提示音
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
          } catch (e) {
            // ignore audio errors
          }

          // 浏览器通知
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
          } catch (e) {}
        }

        prevFinishedRef.current = state.isFinished;

        logInfo("📊 状态已更新: " + JSON.stringify(state));
      }
    };

    logInfo("初始化完成，等待用户交互...");
  }, []);

  // 应用主题
  const applyTheme = (theme: string) => {
    const html = document.documentElement;

    if (theme === "light") {
      html.classList.add("light-mode");
      document.body.classList.add("light-mode");
    } else {
      html.classList.remove("light-mode");
      document.body.classList.remove("light-mode");
    }
  };

  const handleStart = () => {
    logInfo('🚀 "开始"按钮被点击');
    try {
      if (typeof window.webui === "undefined") {
        logError("❌ webui 对象未定义！");
        return;
      }
      logInfo("✓ webui 对象存在，准备调用 start 函数");
      window.webui.call("start");
      setIsRunning(true);
      logSuccess('✓ webui.call("start") 调用成功');
    } catch (e) {
      logError("❌ 调用 start 时发生错误: " + (e as Error).message);
      console.error(e);
    }
  };

  const handlePause = () => {
    logInfo('⏸️ "暂停"按钮被点击');
    try {
      if (typeof window.webui === "undefined") {
        logError("❌ webui 对象未定义！");
        return;
      }
      logInfo("✓ webui 对象存在，准备调用 pause 函数");
      window.webui.call("pause");
      setIsRunning(false);
      logSuccess('✓ webui.call("pause") 调用成功');
    } catch (e) {
      logError("❌ 调用 pause 时发生错误: " + (e as Error).message);
      console.error(e);
    }
  };

  const handleReset = () => {
    logInfo('🔄 "重置"按钮被点击');
    try {
      if (typeof window.webui === "undefined") {
        logError("❌ webui 对象未定义！");
        return;
      }
      logInfo("✓ webui 对象存在，准备调用 reset 函数");
      window.webui.call("reset");
      setIsRunning(false);
      logSuccess('✓ webui.call("reset") 调用成功');
    } catch (e) {
      logError("❌ 调用 reset 时发生错误: " + (e as Error).message);
      console.error(e);
    }
  };

  //切换模式
  const handleModeChange = (newMode: Mode) => {
    try {
      if (typeof window.webui === "undefined") {
        logError("❌ webui 对象未定义！");
        return;
      }
      logInfo(`✓ webui 对象存在，准备调用 change_mode 函数，参数: ${newMode}`);
      window.webui.call("change_mode", newMode);
      logSuccess('✓ webui.call("change_mode") 调用成功');
    } catch (e) {
      logError("❌ 调用 change_mode 时发生错误: " + (e as Error).message);
      console.error(e);
    }
  };

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
              isRunning
                ? "bg-accent-dark text-white border border-accent-dark animate-pulse"
                : prevFinishedRef.current
                  ? "bg-green-600 text-white border border-green-600"
                  : "bg-secondary-dark text-text-secondary-dark border border-border-dark"
            }`}
          >
            {isRunning
              ? t("home.status_running")
              : prevFinishedRef.current
                ? "✅ 已完成"
                : t("home.status_paused")}
          </span>
          <span className="px-3 sm:px-4 py-1.5 sm:py-2 rounded-full text-xs sm:text-sm font-medium bg-accent-dark text-white border border-accent-dark whitespace-nowrap">
            {(() => {
              if (mode === Mode.Countdown) return t("home.mode_countdown");
              if (mode === Mode.Stopwatch) return t("home.mode_stopwatch");
              return t("home.mode_world_clock");
            })()}
          </span>
        </div>

        {/* 时间显示 */}
        <div
          className={`text-4xl sm:text-6xl md:text-8xl font-light tracking-wider text-text-primary-dark font-mono my-4 sm:my-6 md:my-6 text-center transition-all duration-300 animate-slideUp break-all ${
            isRunning ? "text-accent-dark" : ""
          }`}
          style={{ animationDelay: "0.2s", animationFillMode: "both" }}
        >
          {time}
        </div>

        {/* 循环和休息状态提示 */}
        {(inRest ||
          (loopRemaining !== null && loopTotal !== null && loopTotal > 0)) && (
          <div
            className="text-center mb-3 sm:mb-4 text-xs sm:text-sm text-text-secondary-dark animate-slideUp w-full px-2"
            style={{ animationDelay: "0.25s", animationFillMode: "both" }}
          >
            {inRest && (
              <div className="text-accent-dark font-semibold">
                {t("home.rest_status", { seconds: restRemaining })}
              </div>
            )}
            {loopRemaining !== null &&
              loopTotal !== null &&
              loopTotal > 0 &&
              !inRest && (
                <div className="text-accent-dark font-semibold">
                  {t("home.loop_status", {
                    remaining: loopRemaining,
                    total: loopTotal,
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
          {!isRunning ? (
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
                onClick={() => {
                  handleModeChange(key);
                  setMode(key);
                }}
                className={`p-2 sm:p-3 md:p-4 rounded-xl flex flex-col items-center gap-1 sm:gap-2 text-xs sm:text-sm font-medium transition-all duration-200 hover:scale-105 active:scale-95 min-h-[60px] sm:min-h-[80px] ${
                  mode === key
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
};
