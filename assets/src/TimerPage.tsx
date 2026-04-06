import { useState, useEffect, useRef } from "preact/hooks";
import type { FunctionalComponent } from "preact";
import { getAPIClient } from "./utils/apiClientSingleton";
import { logSuccess, logError } from "./utils/logger";
import { formatDuration } from "./utils/formatters";
import { t } from "./utils/i18n";
import type { Habit, HabitSet, HabitDetail } from "./types/habit";
import {
    audioEngine,
    loadAudioPreferences,
    type AudioPreferences,
} from "./utils/audio";
import { SevenSegmentDisplay } from "./components/SevenSegmentDisplay";
import { DropdownSelect } from "./components/DropdownSelect";
import { TimerConfig } from "./components/TimerConfig";
import { StarIconComponent } from "./utils/icons";
import { useSSE } from "./hooks/useSSE";

interface TimerPageProps {
    onHabitsClick?: () => void;
}

type TimerMode = "stopwatch" | "countdown";

interface TimerConfig {
    mode: TimerMode;
    workDuration: number;
    restDuration: number;
    loopCount: number;
}

type LayoutDensity = "compact" | "normal" | "spacious";
type TimeDisplayStyle = "classic" | "seven_segment";

export const TimerPage: FunctionalComponent<TimerPageProps> = ({
    onHabitsClick,
}) => {
    const [habitSets, setHabitSets] = useState<HabitSet[]>([]);
    const [habits, setHabits] = useState<Habit[]>([]);
    const [selectedHabitId, setSelectedHabitId] = useState<number | null>(null);
    const [selectedHabit, setSelectedHabit] = useState<Habit | null>(null);
    const [habitDetail, setHabitDetail] = useState<HabitDetail | null>(null);
    const [showHabitPicker, setShowHabitPicker] = useState(false);

    const [timerConfig, setTimerConfig] = useState<TimerConfig>({
        mode: "stopwatch",
        workDuration: 25 * 60,
        restDuration: 5 * 60,
        loopCount: 0,
    });

    const [isRunning, setIsRunning] = useState(false);
    const [isPaused, setIsPaused] = useState(false);
    const [elapsedSeconds, setElapsedSeconds] = useState(0);
    const [remainingSeconds, setRemainingSeconds] = useState(0);
    const [isFinished, setIsFinished] = useState(false);
    const [isResting, setIsResting] = useState(false);
    const [currentRound, setCurrentRound] = useState(0);
    const [audioPreferences, setAudioPreferences] = useState<AudioPreferences>(() => loadAudioPreferences());
    const [layoutDensity, setLayoutDensity] = useState<LayoutDensity>(() => {
        const saved = localStorage.getItem("layout_density");
        return (saved as LayoutDensity) || "normal";
    });
    const [timeDisplayStyle, setTimeDisplayStyle] = useState<TimeDisplayStyle>(() => {
        const saved = localStorage.getItem("time_display_style");
        return (saved as TimeDisplayStyle) || "classic";
    });

    const sessionRecordedRef = useRef(false);
    const apiClientRef = useRef(getAPIClient());
    const timerIntervalRef = useRef<number | null>(null);
    const previousFinishedRef = useRef(false);
    // 标记初始数据加载是否成功
    const initialDataLoadedRef = useRef(false);

    // 使用 SSE hook 来监听连接状态
    const { isConnected: sseConnected } = useSSE();

    useEffect(() => {
        const handleStorageChange = (e: StorageEvent) => {
            if (e.key === "layout_density" && e.newValue) {
                setLayoutDensity(e.newValue as LayoutDensity);
            }

            if (e.key === "time_display_style" && e.newValue) {
                setTimeDisplayStyle(e.newValue as TimeDisplayStyle);
            }
        };

        window.addEventListener("storage", handleStorageChange);
        return () => window.removeEventListener("storage", handleStorageChange);
    }, []);

    useEffect(() => {
        void loadData();
        void restoreTimerProgress();
    }, []);

    // 当 SSE 连接成功且初始数据还未加载时，重新尝试加载数据
    useEffect(() => {
        if (sseConnected && !initialDataLoadedRef.current) {
            console.log('SSE 连接成功，重新加载数据...');
            void loadData();
        }
    }, [sseConnected]);

    // 恢复计时进度（页面刷新后）
    const restoreTimerProgress = async () => {
        try {
            const progress = await apiClientRef.current.getTimerProgress();
            
            if (progress.session_id && !progress.is_finished) {
                // 有活跃的计时会话，恢复状态
                setSelectedHabitId(progress.habit_id);
                setTimerConfig(prev => ({
                    ...prev,
                    mode: progress.mode as TimerMode,
                }));
                
                if (progress.mode === 'stopwatch') {
                    setElapsedSeconds(progress.elapsed_seconds);
                } else {
                    setRemainingSeconds(progress.remaining_seconds);
                    setIsResting(progress.in_rest);
                }
                
                setIsRunning(progress.is_running);
                setIsPaused(progress.is_paused);
                
                // 如果正在运行，需要加载 habit 详情
                if (progress.habit_id) {
                    void loadHabitDetail(progress.habit_id);
                }
                
                logSuccess('✓ 已恢复计时进度');
            }
        } catch (e: any) {
            // 没有保存的进度是正常的，不用报错
            console.debug('恢复计时进度失败:', e);
        }
    };

    useEffect(() => {
        const latest = loadAudioPreferences();
        setAudioPreferences(latest);
        audioEngine.setPreferences(latest);
    }, []);

    useEffect(() => {
        audioEngine.setPreferences(audioPreferences);
    }, [audioPreferences]);

    useEffect(() => {
        if (selectedHabitId) {
            void loadHabitDetail(selectedHabitId);
        }
    }, [selectedHabitId]);

    useEffect(() => {
        if (isRunning && !isPaused && !isFinished) {
            timerIntervalRef.current = window.setInterval(() => {
                if (timerConfig.mode === "stopwatch") {
                    setElapsedSeconds(prev => prev + 1);
                } else {
                    if (isResting) {
                        setRemainingSeconds(prev => {
                            const newVal = prev - 1;
                            if (newVal <= 0) {
                                setIsResting(false);
                                setRemainingSeconds(timerConfig.workDuration);
                                setCurrentRound(prev => prev + 1);
                                return 0;
                            }
                            return newVal;
                        });
                    } else {
                        setRemainingSeconds(prev => {
                            const newVal = prev - 1;
                            if (newVal <= 0) {
                                if (timerConfig.loopCount > 0 && currentRound >= timerConfig.loopCount) {
                                    setIsFinished(true);
                                    setIsRunning(false);
                                    void recordSession();
                                    return 0;
                                } else if (timerConfig.restDuration > 0) {
                                    setIsResting(true);
                                    setRemainingSeconds(timerConfig.restDuration);
                                    return timerConfig.restDuration;
                                } else {
                                    setCurrentRound(prev => prev + 1);
                                    setRemainingSeconds(timerConfig.workDuration);
                                    return timerConfig.workDuration;
                                }
                            }
                            return newVal;
                        });
                    }
                }
            }, 1000);
        }

        return () => {
            if (timerIntervalRef.current) {
                clearInterval(timerIntervalRef.current);
                timerIntervalRef.current = null;
            }
        };
    }, [isRunning, isPaused, isFinished, timerConfig, isResting, currentRound]);

    useEffect(() => {
        if (isFinished && !previousFinishedRef.current) {
            audioEngine.playFinish();
        }
        previousFinishedRef.current = isFinished;
    }, [isFinished]);

    useEffect(() => {
        if (timerConfig.mode === "stopwatch" && habitDetail && elapsedSeconds > 0 && !sessionRecordedRef.current) {
            const totalTodaySeconds = habitDetail.today_seconds + elapsedSeconds;
            if (totalTodaySeconds >= habitDetail.goal_seconds) {
                setIsFinished(true);
                setIsRunning(false);
                void recordSession();
            }
        }
    }, [elapsedSeconds, habitDetail, timerConfig.mode]);

    const loadData = async () => {
        const maxRetries = 3;
        let retryCount = 0;

        const doLoadData = async (): Promise<boolean> => {
            try {
                const client = apiClientRef.current;
                const sets = await client.getHabitSets();
                setHabitSets(Array.isArray(sets) ? sets : []);
                const allHabits = await client.getHabits();
                setHabits(Array.isArray(allHabits) ? allHabits : []);
                initialDataLoadedRef.current = true;
                return true;
            } catch (e: any) {
                retryCount++;
                if (retryCount < maxRetries) {
                    // 指数退避重试：1000ms, 2000ms, 4000ms
                    const delay = 1000 * Math.pow(2, retryCount - 1);
                    console.log(`数据加载失败，${delay}ms 后进行第 ${retryCount} 次重试...`);
                    await new Promise(resolve => setTimeout(resolve, delay));
                    return doLoadData();
                } else {
                    logError(`加载数据失败，已重试 ${maxRetries} 次: ${e}`);
                    return false;
                }
            }
        };

        await doLoadData();
    };

    const loadHabitDetail = async (habitId: number) => {
        try {
            const today = new Date().toISOString().split("T")[0];
            const detail = await apiClientRef.current.getHabitDetail(habitId, today);
            setHabitDetail(detail);
            const habit = habits.find(h => h.id === habitId);
            setSelectedHabit(habit || null);
        } catch (e: any) {
            logError(`加载习惯详情失败: ${e}`);
        }
    };

    const recordSession = async () => {
        if (!selectedHabit || sessionRecordedRef.current) return;
        
        sessionRecordedRef.current = true;
        const habitId = selectedHabit.id;
        const today = new Date().toISOString().split("T")[0];
        const totalSeconds = timerConfig.mode === "stopwatch" ? elapsedSeconds : (timerConfig.workDuration * currentRound);

        try {
            await apiClientRef.current.createSession(habitId, totalSeconds, 1, today);
            logSuccess("✓ Session 已自动记录");
            void loadHabitDetail(habitId);
        } catch (e: any) {
            logError(`记录 session 失败: ${e}`);
        }

        if ("Notification" in window) {
            if (Notification.permission === "granted") {
                new Notification(t("notification.habit_completed"), {
                    body: t("notification.habit_completed_body", { name: selectedHabit.name })
                });
            } else if (Notification.permission !== "denied") {
                Notification.requestPermission().then((p) => {
                    if (p === "granted") {
                        new Notification(t("notification.habit_completed"), {
                            body: t("notification.habit_completed_body", { name: selectedHabit.name })
                        });
                    }
                }).catch(() => {
                    // 忽略通知权限错误
                });
            }
        }
    };

    const startTimer = async () => {
        void audioEngine.unlock();

        if (!selectedHabitId) {
            setShowHabitPicker(true);
            return;
        }

        if (isFinished) {
            void resetTimer();
            return;
        }

        sessionRecordedRef.current = false;
        setIsRunning(true);
        setIsPaused(false);

        if (timerConfig.mode === "countdown") {
            setRemainingSeconds(timerConfig.workDuration);
            setCurrentRound(1);
            setIsResting(false);
        } else {
            setElapsedSeconds(0);
        }

        try {
            await apiClientRef.current.startTimer(selectedHabitId ?? undefined, {
                mode: timerConfig.mode,
                workDuration: timerConfig.workDuration,
                restDuration: timerConfig.restDuration,
                loopCount: timerConfig.loopCount,
            });
            audioEngine.playTick();
        } catch (e: any) {
            logError(`启动计时失败: ${e}`);
        }
    };

    const pauseTimer = async () => {
        audioEngine.stopTick();
        setIsPaused(true);
        try {
            await apiClientRef.current.pauseTimer();
        } catch (e: any) {
            logError(`暂停计时失败: ${e}`);
        }
    };

    const resumeTimer = async () => {
        void audioEngine.unlock();
        setIsPaused(false);
        try {
            await apiClientRef.current.resumeTimer(selectedHabitId ?? undefined);
            audioEngine.playTick();
        } catch (e: any) {
            logError(`恢复计时失败: ${e}`);
        }
    };

    const finishTimer = async () => {
        audioEngine.stopTick();
        try {
            const result = await apiClientRef.current.finishTimer();
            logSuccess(`✓ 已计入今日统计: ${formatDuration(result.elapsed_seconds)}`);
            
            // 刷新习惯详情
            if (selectedHabitId) {
                void loadHabitDetail(selectedHabitId);
            }
        } catch (e: any) {
            logError(`结束计时失败: ${e}`);
            return;
        }

        // 重置前端状态
        setIsRunning(false);
        setIsPaused(false);
        setIsFinished(false);
        setElapsedSeconds(0);
        setRemainingSeconds(timerConfig.workDuration);
        setCurrentRound(0);
        sessionRecordedRef.current = false;
    };

    const resetTimer = async () => {
        audioEngine.stopTick();
        setIsRunning(false);
        setIsPaused(false);
        setIsFinished(false);
        setIsResting(false);
        setElapsedSeconds(0);
        setRemainingSeconds(timerConfig.workDuration);
        setCurrentRound(0);
        sessionRecordedRef.current = false;

        try {
            await apiClientRef.current.resetTimer();
        } catch (e: any) {
            logError(`重置计时失败: ${e}`);
        }
    };

    const skipToNext = () => {
        if (timerConfig.mode === "countdown" && isRunning) {
            if (isResting) {
                setIsResting(false);
                setRemainingSeconds(timerConfig.workDuration);
                setCurrentRound(prev => prev + 1);
            } else {
                if (timerConfig.loopCount > 0 && currentRound >= timerConfig.loopCount) {
                    setIsFinished(true);
                    setIsRunning(false);
                    void recordSession();
                } else if (timerConfig.restDuration > 0) {
                    setIsResting(true);
                    setRemainingSeconds(timerConfig.restDuration);
                } else {
                    setCurrentRound(prev => prev + 1);
                    setRemainingSeconds(timerConfig.workDuration);
                }
            }
        }
    };

    const handleHabitSelect = async (habitId: number) => {
        audioEngine.stopTick();
        setIsRunning(false);
        setIsPaused(false);
        setIsFinished(false);
        setIsResting(false);
        setElapsedSeconds(0);
        setCurrentRound(0);
        sessionRecordedRef.current = false;

        setSelectedHabitId(habitId);
        setShowHabitPicker(false);

        try {
            const today = new Date().toISOString().split("T")[0];
            const detail = await apiClientRef.current.getHabitDetail(habitId, today);
            setHabitDetail(detail);
            const habit = habits.find(h => h.id === habitId);
            setSelectedHabit(habit || null);
        } catch (e: any) {
            logError(`加载习惯详情失败: ${e}`);
        }
    };

    const displayTime = timerConfig.mode === "stopwatch" ? elapsedSeconds : remainingSeconds;
    const timeDisplay = formatDuration(displayTime);
    const timeStateClass = isFinished
        ? "time-state-finished"
        : isResting
            ? "time-state-rest"
            : "time-state-active";

    const todayProgress = habitDetail ? habitDetail.today_seconds + elapsedSeconds : elapsedSeconds;
    const progressPercent = habitDetail && habitDetail.goal_seconds > 0
        ? Math.min(Math.floor((todayProgress * 100) / habitDetail.goal_seconds), 100)
        : 0;

    return (
        <div className="flex flex-col flex-1 bg-transparent overflow-hidden">
            <header className="navbar my-topbar shrink-0 px-2 sm:px-3">
                <div className="flex-1">
                    <span className="my-topbar-title text-xl font-bold">{t("timer.title")}</span>
                </div>
                <div className="flex-none">
                    <button className="my-icon-btn" onClick={onHabitsClick}>
                        <svg xmlns="http://www.w3.org/2000/svg" className="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" />
                        </svg>
                    </button>
                </div>
            </header>

            <div className={`flex-1 w-full layout-${layoutDensity}`} style={{
                paddingLeft: 'var(--layout-px)',
                paddingRight: 'var(--layout-px)',
                paddingTop: 'var(--layout-py)',
                paddingBottom: 'var(--layout-py)',
            }}>
                <div className="mx-auto flex h-full w-full max-w-6xl flex-col justify-between">
                <div className="flex flex-wrap items-center justify-center gap-3 sm:gap-4" style={{
                    marginBottom: 'var(--layout-control-mb)',
                    gap: 'var(--layout-control-gap)',
                }}>
                    <button
                        className="my-surface-card flex items-center gap-2 sm:gap-3 min-w-[170px] justify-between px-4 py-3 rounded-xl cursor-pointer"
                        onClick={() => setShowHabitPicker(true)}
                    >
                        {selectedHabit ? (
                            <>
                                <span className="w-3 h-3 rounded-full" style={{ backgroundColor: selectedHabit.color }} />
                                {selectedHabit.name}
                            </>
                        ) : (
                            <>
                                <svg xmlns="http://www.w3.org/2000/svg" className="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 6v6m0 0v6m0-6h6m-6 0H6" />
                                </svg>
                                {t("timer.select_habit")}
                            </>
                        )}
                        <svg xmlns="http://www.w3.org/2000/svg" className="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
                        </svg>
                    </button>

<DropdownSelect
                        value={timerConfig.mode}
                        options={[
                            { value: "stopwatch", label: t("timer.stopwatch") },
                            { value: "countdown", label: t("timer.countdown") }
                        ]}
                        onChange={(value) => setTimerConfig({...timerConfig, mode: value as TimerMode})}
                        disabled={isRunning}
                        minWidth="130px"
                    />
                </div>

                <TimerConfig
                    config={timerConfig}
                    isRunning={isRunning}
                    isCountdownMode={timerConfig.mode === "countdown"}
                    onChange={(config) => setTimerConfig({...timerConfig, ...config})}
                />

                <div className="flex flex-1 flex-col items-center justify-center">
                <div className="my-clock-glass mb-3 sm:mb-4">
                  <div className={`text-[clamp(3.2rem,13vw,8rem)] leading-none font-mono font-semibold time-transition ${timeDisplayStyle === "seven_segment" ? "time-style-seven-segment" : "time-style-classic"} ${timeDisplayStyle === "classic" ? (isFinished ? "text-success" : (isResting ? "text-warning" : "text-primary")) : ""} ${timeStateClass} ${isRunning && timeDisplayStyle === "seven_segment" ? "time-running-segment" : ""}`}>
                      {timeDisplayStyle === "seven_segment" ? (
                          <SevenSegmentDisplay value={timeDisplay} />
                      ) : (
                          <span className="time-value-swap">
                              {timeDisplay}
                          </span>
                      )}
                  </div>
                </div>

                {timerConfig.mode === "countdown" && isRunning && (
                    <div className="text-base sm:text-lg text-base-content/60 mb-6 sm:mb-7">
                        {isResting ? t("timer.resting") : t("timer.round", { current: currentRound })}
                        {timerConfig.loopCount > 0 && t("timer.of_total", { total: timerConfig.loopCount })}
                    </div>
                )}

                {habitDetail && timerConfig.mode === "stopwatch" && (
                    <div className="w-full max-w-2xl mb-6 sm:mb-8">
                        <div className="flex justify-between text-sm text-base-content/70 mb-1">
                            <span>{t("timer.today_progress")} {formatDuration(todayProgress)}</span>
                            <span>{t("timer.goal")} {formatDuration(habitDetail.goal_seconds)}</span>
                        </div>
                        <progress
                            className={`progress w-full ${isFinished ? "progress-success" : "progress-primary"}`}
                            value={Math.min(todayProgress, habitDetail.goal_seconds)}
                            max={habitDetail.goal_seconds}
                        />
                        <div className="flex justify-between items-center mt-2 text-sm">
                            <span className="text-base-content/60">
                                {t("timer.progress")} {progressPercent}%
                            </span>
                            {habitDetail.streak > 0 && (
                                <span className="text-warning inline-flex items-center gap-1">
                                    <StarIconComponent />
                                    {habitDetail.streak} {t("timer.streak")}
                                </span>
                            )}
                        </div>
                    </div>
                )}

                </div>

                <div className="flex flex-wrap items-center justify-center gap-3 sm:gap-4 mt-2">
                    {isRunning && !isPaused ? (
                        <button
                            className="btn btn-primary btn-lg min-w-[130px]"
                            onClick={() => void pauseTimer()}
                        >
                            <svg xmlns="http://www.w3.org/2000/svg" className="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M10 9v6m4-6v6m7-3a9 9 0 11-18 0 9 9 0 0118 0z" />
                            </svg>
                            {t("timer.pause")}
                        </button>
                    ) : isPaused ? (
                        <button
                            className="btn btn-primary btn-lg min-w-[130px]"
                            onClick={() => void resumeTimer()}
                        >
                            <svg xmlns="http://www.w3.org/2000/svg" className="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M14.752 11.168l-3.197-2.132A1 1 0 0010 9.87v4.263a1 1 0 001.555.832l3.197-2.132a1 1 0 000-1.664z" />
                            </svg>
                            {t("timer.resume")}
                        </button>
                    ) : (
                        <button
                            className="btn btn-primary btn-lg min-w-[130px]"
                            onClick={() => void startTimer()}
                        >
                            <svg xmlns="http://www.w3.org/2000/svg" className="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M14.752 11.168l-3.197-2.132A1 1 0 0010 9.87v4.263a1 1 0 001.555.832l3.197-2.132a1 1 0 000-1.664z" />
                            </svg>
                            {isFinished ? t("timer.restart") : (selectedHabitId ? t("timer.start") : t("timer.select_habit"))}
                        </button>
                    )}
                    
                    {timerConfig.mode === "countdown" && isRunning && (
                        <button
                            className="btn btn-ghost btn-lg min-w-[110px]"
                            onClick={() => void skipToNext()}
                        >
                            <svg xmlns="http://www.w3.org/2000/svg" className="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13 5l7 7-7 7M5 5l7 7-7 7" />
                            </svg>
                            {t("timer.skip")}
                        </button>
                    )}

                    {isRunning && (
                        <button
                            className="btn btn-success btn-lg min-w-[110px]"
                            onClick={() => void finishTimer()}
                        >
                            <svg xmlns="http://www.w3.org/2000/svg" className="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 13l4 4L19 7" />
                            </svg>
                            {t("timer.finish")}
                        </button>
                    )}
                    
                    <button
                        className="btn btn-secondary btn-lg min-w-[110px]"
                        onClick={() => void resetTimer()}
                    >
                        <svg xmlns="http://www.w3.org/2000/svg" className="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
                        </svg>
                        {t("timer.reset")}
                    </button>
                </div>
                </div>
            </div>

            {showHabitPicker && (
                <div className="fixed inset-0 z-50 flex items-center justify-center" style={{ background: 'rgba(0,0,0,0.2)', backdropFilter: 'blur(4px)' }}>
                    <div className="relative my-surface-card rounded-xl w-full max-w-md mx-4 max-h-[70vh] overflow-hidden flex flex-col">
                        <div className="p-4 border-b border-[var(--my-outline)] flex justify-between items-center">
                            <h3 className="text-lg font-bold">{t("timer.select_habit")}</h3>
                            <button className="btn btn-ghost btn-sm btn-circle" onClick={() => setShowHabitPicker(false)}>
                                <svg xmlns="http://www.w3.org/2000/svg" className="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
                                </svg>
                            </button>
                        </div>
                        <div className="flex-1 overflow-y-auto p-2">
                            {habitSets.length === 0 ? (
                                <div className="text-center py-8 text-[var(--my-on-surface-variant)]">
                                    <p>{t("habit.no_habits")}</p>
                                </div>
                            ) : (
                                <div className="space-y-2">
                                    {habitSets.map(set => (
                                        <div key={set.id}>
                                            <div className="px-2 py-1 text-sm font-semibold text-[var(--my-on-surface-variant)]">
                                                {set.name}
                                            </div>
                                            {habits.filter(h => h.set_id === set.id).map(habit => (
                                                <button
                                                    key={habit.id}
                                                    className="w-full p-3 flex items-center gap-3 rounded-lg hover:bg-[var(--my-surface-strong)] transition-colors"
                                                    onClick={() => void handleHabitSelect(habit.id)}
                                                >
                                                    <span className="w-3 h-3 rounded-full" style={{ backgroundColor: habit.color }} />
                                                    <div className="flex-1 text-left">
                                                        <div className="font-medium">{habit.name}</div>
                                                        <div className="text-xs text-[var(--my-on-surface-variant)]">
                                                            {t("timer.goal")}: {formatDuration(habit.goal_seconds)}
                                                        </div>
                                                    </div>
                                                </button>
                                            ))}
                                        </div>
                                    ))}
                                </div>
                            )}
                        </div>
                    </div>
                </div>
            )}
        </div>
    );
};
