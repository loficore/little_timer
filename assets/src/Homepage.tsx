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
import { Header } from "./components/Header";
import { TimeDisplay } from "./components/TimeDisplay";
import { StatusBadge } from "./components/StatusBadge";
import { ControlPanel } from "./components/ControlPanel";
import { ModeSelector } from "./components/ModeSelector";
import { APIClient, type TimerState } from "./utils/apiClient";
import { SSEClient } from "./utils/sseClient";

interface HomePageProps {
  onSettingsClick?: () => void;
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
  timezone: number;
}

const HomePage = memo(({ onSettingsClick }: HomePageProps) => {
  const apiClientRef = useRef<APIClient | null>(null);
  const sseClientRef = useRef<SSEClient | null>(null);

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
  const isConnectedRef = useRef(false);

  const modeMap: Record<string, Mode> = {
    countdown: Mode.Countdown,
    stopwatch: Mode.Stopwatch,
    world_clock: Mode.WorldClock,
  };

  const modeToString = (mode: Mode): string => {
    switch (mode) {
      case Mode.Countdown:
        return "countdown";
      case Mode.Stopwatch:
        return "stopwatch";
      case Mode.WorldClock:
        return "world_clock";
    }
  };

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

  const updateStateFromTimerState = useCallback((timerState: TimerState) => {
    const newMode = modeMap[timerState.mode] || Mode.Countdown;

    setState((prev) => {
      let newTime = prev.time;
      if (newMode !== Mode.WorldClock) {
        newTime = formatDuration(timerState.time);
      }

      if (
        prev.time === newTime &&
        prev.mode === newMode &&
        prev.isRunning === timerState.is_running &&
        prev.inRest === timerState.in_rest &&
        prev.loopRemaining === (timerState.loop_remaining ?? null) &&
        prev.loopTotal === (timerState.loop_total ?? null) &&
        prev.restRemaining === (timerState.rest_remaining ?? 0) &&
        prev.isFinished === timerState.is_finished &&
        prev.timezone === timerState.timezone
      ) {
        return prev;
      }

      return {
        ...prev,
        time: newTime,
        mode: newMode,
        isRunning: timerState.is_running,
        inRest: timerState.in_rest,
        loopRemaining: timerState.loop_remaining ?? null,
        loopTotal: timerState.loop_total ?? null,
        restRemaining: timerState.rest_remaining ?? 0,
        isFinished: timerState.is_finished,
        timezone: timerState.timezone,
      };
    });

    if (timerState.is_finished && !prevFinishedRef.current) {
      try {
        const AudioCtxClass = window.AudioContext || (window as any).webkitAudioContext;
        if (AudioCtxClass) {
          const ctx = new AudioCtxClass();
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
            void ctx.close();
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
            }).catch(() => {
              // ignore notification errors
            });
          }
        }
      } catch {
        // 忽略通知相关错误
      }
    }
    prevFinishedRef.current = timerState.is_finished;
  }, []);

  useEffect(() => {
    const baseUrl = window.location.origin;
    apiClientRef.current = new APIClient(baseUrl);
    sseClientRef.current = new SSEClient(baseUrl);

    const initApp = async () => {
      logSuccess("✅ React 应用已加载，准备就绪");

      try {
        const initialState = await apiClientRef.current!.getState();
        updateStateFromTimerState(initialState);
        logSuccess("✅ 初始状态已获取");
      } catch (e) {
        const errorMsg = e instanceof Error ? e.message : String(e);
        logError(`❌ 获取初始状态失败: ${errorMsg}`);
      }

      sseClientRef.current!.connect(
        (timerState) => {
          isConnectedRef.current = true;
          updateStateFromTimerState(timerState);
        },
        (error: unknown) => {
          isConnectedRef.current = false;
          const errorMsg = error instanceof Error ? error.message : String(error);
          logError(`❌ SSE 连接错误: ${errorMsg}`);
        }
      );
      logInfo("📡 SSE 连接已建立");
    };

    void initApp();

    const mediaQuery = window.matchMedia("(prefers-color-scheme: light)");
    const handleThemeChange = (e: MediaQueryListEvent) => {
      const theme = e.matches ? "light" : "dark";
      applyTheme(theme);
    };
    mediaQuery.addEventListener("change", handleThemeChange);

    return () => {
      mediaQuery.removeEventListener("change", handleThemeChange);
      if (sseClientRef.current) {
        sseClientRef.current.close();
      }
    };
  }, [applyTheme, updateStateFromTimerState]);

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

  const handleStart = useCallback(() => {
    logInfo('🚀 "开始"按钮被点击');
    if (!apiClientRef.current) {
      logError("❌ API 客户端未初始化");
      return;
    }
    void apiClientRef.current.startTimer().then(() => {
      logSuccess('✓ startTimer() 调用成功');
    }).catch((e) => {
      const errorMsg = e instanceof Error ? e.message : String(e);
      logError(`❌ 调用 startTimer 时发生错误: ${errorMsg}`);
    });
  }, []);

  const handlePause = useCallback(() => {
    logInfo('⏸️ "暂停"按钮被点击');
    if (!apiClientRef.current) {
      logError("❌ API 客户端未初始化");
      return;
    }
    void apiClientRef.current.pauseTimer().then(() => {
      logSuccess('✓ pauseTimer() 调用成功');
    }).catch((e) => {
      const errorMsg = e instanceof Error ? e.message : String(e);
      logError(`❌ 调用 pauseTimer 时发生错误: ${errorMsg}`);
    });
  }, []);

  const handleReset = useCallback(() => {
    logInfo('🔄 "重置"按钮被点击');
    if (!apiClientRef.current) {
      logError("❌ API 客户端未初始化");
      return;
    }
    void apiClientRef.current.resetTimer().then(() => {
      logSuccess('✓ resetTimer() 调用成功');
    }).catch((e) => {
      const errorMsg = e instanceof Error ? e.message : String(e);
      logError(`❌ 调用 resetTimer 时发生错误: ${errorMsg}`);
    });
  }, []);

  const handleModeChange = useCallback((newMode: Mode) => {
    if (!apiClientRef.current) {
      logError("❌ API 客户端未初始化");
      return;
    }
    const modeStr = modeToString(newMode);
    logInfo(`准备切换模式: ${modeStr}`);
    void apiClientRef.current.changeMode(newMode).then(() => {
      logSuccess('✓ changeMode() 调用成功');
    }).catch((e) => {
      const errorMsg = e instanceof Error ? e.message : String(e);
      logError(`❌ 调用 changeMode 时发生错误: ${errorMsg}`);
    });
  }, []);

  const statusMemo = useMemo(
    () => state,
    [
      state.isRunning,
      state.isFinished,
      state.inRest,
      state.loopRemaining,
      state.loopTotal,
      state.restRemaining,
      state.mode,
      state.time,
    ]
  );

  return (
    <div className="flex flex-col w-screen h-screen bg-primary-dark dark:bg-primary-dark transition-colors duration-300 animate-fadeIn overflow-hidden">
      <Header
        title={t("common.app_name")}
        showSettings={true}
        onSettingsClick={onSettingsClick}
      />

      <div className="flex-1 flex flex-col items-center justify-center px-4 sm:px-6 md:px-8 py-4 sm:py-6 md:py-12 overflow-y-auto">
        <div
          className="flex gap-2 sm:gap-3 md:gap-4 justify-center mb-6 sm:mb-8 flex-wrap w-full"
          style={{ animationDelay: "0.1s", animationFillMode: "both" }}
        >
          <StatusBadge
            status={
              statusMemo.isRunning
                ? "running"
                : prevFinishedRef.current
                  ? "finished"
                  : "paused"
            }
            label={
              statusMemo.isRunning
                ? t("home.status_running")
                : prevFinishedRef.current
                  ? t("home.status_finished")
                  : t("home.status_paused")
            }
            animationDelay="0.1s"
          />
          <span
            className="px-3 sm:px-4 py-1.5 sm:py-2 rounded-full text-xs sm:text-sm font-medium bg-accent-dark text-white border border-accent-dark whitespace-nowrap animate-slideUp"
            style={{ animationDelay: "0.12s", animationFillMode: "both" }}
          >
            {(() => {
              if (statusMemo.mode === Mode.Countdown)
                return t("home.mode_countdown");
              if (statusMemo.mode === Mode.Stopwatch)
                return t("home.mode_stopwatch");
              return t("home.mode_world_clock");
            })()}
          </span>
        </div>

        <TimeDisplay
          time={statusMemo.time}
          isRunning={statusMemo.isRunning}
        />

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

        <ControlPanel
          isRunning={statusMemo.isRunning}
          onStart={handleStart}
          onPause={handlePause}
          onReset={handleReset}
          animationDelay="0.3s"
        />

        <ModeSelector
          modes={[
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
          ]}
          activeMode={statusMemo.mode}
          onModeChange={handleModeChange}
          animationDelay="0.4s"
        />
      </div>

      <div
        className="px-4 sm:px-6 py-3 sm:py-4 md:py-6 text-center text-text-secondary-dark text-xs border-t border-border-dark bg-primary-dark animate-slideUp shrink-0"
        style={{ animationDelay: "0.5s", animationFillMode: "both" }}
      >
        <p>{t("common.version")}</p>
      </div>
    </div>
  );
});

export { HomePage };
