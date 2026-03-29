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
import { ControlPanel } from "./components/ControlPanel";
import { HabitModal } from "./components/HabitModal";
import { APIClient, type TimerState } from "./utils/apiClient";
import { SSEClient } from "./utils/sseClient";
import type { HabitSet, Habit, HabitWithProgress } from "./types/habit.ts";

interface HomePageProps {
  onStatsClick?: () => void;
  onBackClick?: () => void;
  selectedSetId?: number | null;
  selectedHabit?: Habit | null;
  onSetClick?: (setId: number) => void;
  onHabitClick?: (habit: Habit) => void;
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

const HomePage = memo((props: HomePageProps) => {
  const { onStatsClick, onBackClick, selectedSetId, selectedHabit, onSetClick, onHabitClick } = props;
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
  const [isConnected, setIsConnected] = useState(true);
  const [habitSets, setHabitSets] = useState<HabitSet[]>([]);
  const [habits, setHabits] = useState<HabitWithProgress[]>([]);
  const [modalState, setModalState] = useState<{ isOpen: boolean; mode: "set" | "habit"; setId?: number }>({
    isOpen: false,
    mode: "set",
  });
  const [isLoadingHabits, setIsLoadingHabits] = useState(false);
  const prevFinishedRef = useRef(false);
  const sessionRecordedRef = useRef(false);
  const isConnectedRef = useRef(false);
  const lastCalibratedTimeRef = useRef<number>(0);
  const lastCalibratedTimestampRef = useRef<number>(0);

  const modeMap: Record<string, Mode> = {
    countdown: Mode.Countdown,
    stopwatch: Mode.Stopwatch,
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

    lastCalibratedTimeRef.current = timerState.time;
    lastCalibratedTimestampRef.current = Date.now();

    setState((prev) => {
      const newTime = formatDuration(timerState.time);

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

      // 自动记录 session（仅在计时页且未记录过）
      if (selectedHabit && !sessionRecordedRef.current) {
        sessionRecordedRef.current = true;
        const durationSeconds = selectedHabit.goal_seconds;
        const today = new Date().toISOString().split("T")[0];
        const client = new APIClient(window.location.origin);
        void client.createSession(selectedHabit.id, durationSeconds, 1, today).then(() => {
          logSuccess("✓ Session 已自动记录");
        }).catch((e) => {
          logError(`❌ 记录 session 失败: ${e}`);
        });
      }
    }
    prevFinishedRef.current = timerState.is_finished;
  }, [selectedHabit]);

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
          setIsConnected(true);
          updateStateFromTimerState(timerState);
        },
        (error: unknown) => {
          isConnectedRef.current = false;
          setIsConnected(false);
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

  // 加载习惯集
  useEffect(() => {
    if (!apiClientRef.current) {
      const baseUrl = window.location.origin;
      apiClientRef.current = new APIClient(baseUrl);
    }
    
    const loadHabitSets = async () => {
      try {
        const sets = await apiClientRef.current!.getHabitSets();
        setHabitSets(Array.isArray(sets) ? sets : []);
      } catch (e) {
        logError(`❌ 获取习惯集失败: ${e}`);
      }
    };
    
    void loadHabitSets();
  }, []);

  // 加载习惯列表（当选择了习惯集时）
  useEffect(() => {
    if (!selectedSetId || !apiClientRef.current) {
      setHabits([]);
      return;
    }
    
    const loadHabits = async () => {
      setIsLoadingHabits(true);
      try {
        const allHabits = await apiClientRef.current!.getHabits();
        const filtered = (Array.isArray(allHabits) ? allHabits : [])
          .filter((h: any) => h.set_id === selectedSetId)
          .map((h: any) => ({
            ...h,
            today_seconds: 0,
            today_count: 0,
            progress: 0,
          }));
        setHabits(filtered);
      } catch (e) {
        logError(`❌ 获取习惯失败: ${e}`);
      } finally {
        setIsLoadingHabits(false);
      }
    };
    
    void loadHabits();
  }, [selectedSetId]);

  useEffect(() => {
    const tick = () => {
      const calibrated = lastCalibratedTimeRef.current;
      const lastTs = lastCalibratedTimestampRef.current;
      if (calibrated > 0 && lastTs > 0) {
        const elapsedSeconds = Math.floor((Date.now() - lastTs) / 1000);
        const newTime = state.mode === Mode.Countdown
          ? Math.max(0, calibrated - elapsedSeconds)
          : calibrated + elapsedSeconds;
        setState((prev) => ({ ...prev, time: formatDuration(newTime) }));
      }
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
    // 重置 session 记录标记
    sessionRecordedRef.current = false;
    // 乐观更新：立即更新本地状态
    setState(prev => ({ ...prev, isRunning: true, isFinished: false }));
    const habitId = selectedHabit?.id;
    void apiClientRef.current.startTimer(habitId).then(() => {
      logSuccess('✓ startTimer() 调用成功');
    }).catch((e) => {
      const errorMsg = e instanceof Error ? e.message : String(e);
      logError(`❌ 调用 startTimer 时发生错误: ${errorMsg}`);
      // 回滚状态
      setState(prev => ({ ...prev, isRunning: false }));
    });
  }, [selectedHabit]);

  const handlePause = useCallback(() => {
    logInfo('⏸️ "暂停"按钮被点击');
    if (!apiClientRef.current) {
      logError("❌ API 客户端未初始化");
      return;
    }
    // 乐观更新：立即更新本地状态
    setState(prev => ({ ...prev, isRunning: false }));
    void apiClientRef.current.pauseTimer().then(() => {
      logSuccess('✓ pauseTimer() 调用成功');
    }).catch((e) => {
      const errorMsg = e instanceof Error ? e.message : String(e);
      logError(`❌ 调用 pauseTimer 时发生错误: ${errorMsg}`);
      // 回滚状态
      setState(prev => ({ ...prev, isRunning: true }));
    });
  }, []);

  const handleReset = useCallback(() => {
    logInfo('🔄 "重置"按钮被点击');
    if (!apiClientRef.current) {
      logError("❌ API 客户端未初始化");
      return;
    }
    // 乐观更新：立即重置本地状态
    setState(prev => ({
      ...prev,
      isRunning: false,
      isFinished: false,
      inRest: false,
      restRemaining: 0,
      loopRemaining: null,
      time: "25:00:00",
    }));
    void apiClientRef.current.resetTimer().then(() => {
      logSuccess('✓ resetTimer() 调用成功');
    }).catch((e) => {
      const errorMsg = e instanceof Error ? e.message : String(e);
      logError(`❌ 调用 resetTimer 时发生错误: ${errorMsg}`);
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

  // 根据不同页面状态渲染内容
  const renderContent = () => {
    // 计时页面（选中了习惯）
    if (selectedHabit) {
      return (
        <div className="flex-1 flex flex-col items-center justify-center px-4 py-8">
          <div className="text-2xl font-bold mb-2" style={{ color: selectedHabit.color }}>
            {selectedHabit.name}
          </div>
          <div className="text-sm text-base-content/60 mb-8">
            目标: {Math.floor(selectedHabit.goal_seconds / 60)} 分钟
          </div>
          
          <TimeDisplay time={statusMemo.time} isRunning={statusMemo.isRunning} />
          
          <ControlPanel
            isRunning={statusMemo.isRunning}
            onStart={handleStart}
            onPause={handlePause}
            onReset={handleReset}
          />
          
          <div className="mt-6">
            <button
              className="btn btn-outline btn-sm"
              onClick={() => {
                if (apiClientRef.current) {
                  void apiClientRef.current.startRest();
                }
              }}
            >
              休息 5 分钟
            </button>
          </div>
        </div>
      );
    }
    
    // 习惯列表页面（选中了习惯集）
    if (selectedSetId) {
      return (
        <div className="flex-1 overflow-y-auto p-4 space-y-3">
          {isLoadingHabits ? (
            <div className="flex justify-center py-8">
              <span className="loading loading-spinner"></span>
            </div>
          ) : habits.length === 0 ? (
            <div className="text-center py-8 text-base-content/50">
              暂无习惯，点击下方添加
            </div>
          ) : (
            habits.map((habit) => (
              <div
                key={habit.id}
                className="card bg-base-200 cursor-pointer hover:scale-[1.02] transition-transform"
                onClick={() => onHabitClick?.(habit)}
              >
                <div className="card-body p-4 flex-row items-center justify-between">
                  <div className="flex items-center gap-3">
                    <div
                      className="w-3 h-3 rounded-full"
                      style={{ backgroundColor: habit.color }}
                    />
                    <div>
                      <div className="font-medium">{habit.name}</div>
                      <div className="text-xs text-base-content/60">
                        目标 {Math.floor(habit.goal_seconds / 60)} 分钟
                      </div>
                    </div>
                  </div>
                  <div className="text-base-content/40">▶</div>
                </div>
              </div>
            ))
          )}
          
          <button
            className="btn btn-outline btn-block mt-4"
            onClick={() => {
              if (selectedSetId) {
                setModalState({ isOpen: true, mode: "habit", setId: selectedSetId });
              }
            }}
          >
            + 添加习惯
          </button>
        </div>
      );
    }
    
    // 习惯集列表页面（首页）
    return (
      <div className="flex-1 overflow-y-auto p-4 space-y-3">
        {habitSets.length === 0 ? (
          <div className="text-center py-8 text-base-content/50">
            暂无习惯集，点击下方创建
          </div>
        ) : (
          habitSets.map((set) => (
            <div
              key={set.id}
              className="card bg-base-200 cursor-pointer hover:scale-[1.02] transition-transform"
              onClick={() => onSetClick?.(set.id)}
            >
              <div className="card-body p-4">
                <div className="flex items-center gap-3">
                  <div
                    className="w-4 h-4 rounded-full"
                    style={{ backgroundColor: set.color }}
                  />
                  <div className="font-semibold">{set.name}</div>
                </div>
                {set.description && (
                  <div className="text-sm text-base-content/60 mt-1">
                    {set.description}
                  </div>
                )}
              </div>
            </div>
          ))
        )}
        
        <button
          className="btn btn-primary btn-block mt-4"
          onClick={() => setModalState({ isOpen: true, mode: "set" })}
        >
          + 创建习惯集
        </button>
      </div>
    );
  };

  const getTitle = () => {
    if (selectedHabit) return selectedHabit.name;
    if (selectedSetId) {
      const set = habitSets.find(s => s.id === selectedSetId);
      return set?.name || "习惯列表";
    }
    return t("common.app_name");
  };

  return (
    <div className="flex flex-col w-screen h-screen bg-primary-dark dark:bg-primary-dark transition-colors duration-300 animate-fadeIn overflow-hidden pb-16 lg:pb-0">
      <Header
        title={getTitle()}
        showSettings={false}
        showBack={!!selectedSetId || !!selectedHabit}
        onBackClick={onBackClick}
        showStats={!selectedSetId && !selectedHabit}
        onStatsClick={onStatsClick}
      />

      {/* 连接状态指示器 */}
      {!isConnected && (
        <div className="bg-error text-white text-center py-1 text-sm font-medium animate-pulse">
          ⚠️ 连接中断 - 正在重连...
        </div>
      )}

      {renderContent()}

      <HabitModal
        isOpen={modalState.isOpen}
        mode={modalState.mode}
        setId={modalState.setId}
        onClose={() => setModalState({ isOpen: false, mode: "set" })}
        onSuccess={() => {
          setModalState({ isOpen: false, mode: "set" });
          // 刷新数据
          if (modalState.mode === "set") {
            const loadSets = async () => {
              const client = new APIClient(window.location.origin);
              const sets = await client.getHabitSets();
              setHabitSets(Array.isArray(sets) ? sets : []);
            };
            void loadSets();
          } else {
            const loadHabits = async () => {
              const client = new APIClient(window.location.origin);
              const all = await client.getHabits();
              const filtered = (Array.isArray(all) ? all : [])
                .filter((h: any) => h.set_id === selectedSetId)
                .map((h: any) => ({ ...h, today_seconds: 0, today_count: 0, progress: 0 }));
              setHabits(filtered);
            };
            void loadHabits();
          }
        }}
      />

      <div
        className="px-4 py-3 text-center text-text-secondary-dark text-xs border-t border-border-dark bg-primary-dark shrink-0 hidden lg:block"
      >
        <p>{t("common.version")}</p>
      </div>
    </div>
  );
});

export { HomePage };
