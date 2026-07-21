/**
 * 计时器状态管理 Hook
 * 统一管理计时器状态和操作
 */

import { useState, useEffect, useRef, useCallback } from "preact/hooks";
import { getAPIClient } from "../utils/apiClientSingleton";
import { logSuccess, logError } from "../utils/logger";
import { formatDuration } from "../utils/formatters";

export type TimerMode = "stopwatch" | "countdown";

export interface TimerConfig {
  mode: TimerMode;
  workDuration: number;
  restDuration: number;
  loopCount: number;
}

export interface TimerState {
  isRunning: boolean;
  isPaused: boolean;
  isFinished: boolean;
  isResting: boolean;
  currentRound: number;
  elapsedSeconds: number;
  remainingSeconds: number;
}

export interface UseTimerReturn {
  // 状态
  timerConfig: TimerConfig;
  isRunning: boolean;
  isPaused: boolean;
  isFinished: boolean;
  isResting: boolean;
  currentRound: number;
  elapsedSeconds: number;
  remainingSeconds: number;
  displayTime: string;

  // 操作
  setTimerConfig: (config: Partial<TimerConfig>) => void;
  start: (habitId?: number) => Promise<void>;
  pause: () => Promise<void>;
  resume: (habitId?: number) => Promise<void>;
  reset: () => Promise<void>;
  skipToNext: () => void;
  finish: () => Promise<{ elapsed_seconds: number }>;
}

export const useTimer = (): UseTimerReturn => {
  const apiClientRef = useRef(getAPIClient());
  const rafRef = useRef<number | null>(null);
  const startTimeRef = useRef<number>(0);
  const totalElapsedRef = useRef<number>(0);
  const sessionRecordedRef = useRef(false);

  const [timerConfig, setTimerConfigState] = useState<TimerConfig>({
    mode: "stopwatch",
    workDuration: 25 * 60,
    restDuration: 5 * 60,
    loopCount: 0,
  });

  const [timerState, setTimerState] = useState<TimerState>({
    isRunning: false,
    isPaused: false,
    isFinished: false,
    isResting: false,
    currentRound: 0,
    elapsedSeconds: 0,
    remainingSeconds: 0,
  });

  const displayTime = timerConfig.mode === "stopwatch" ? timerState.elapsedSeconds : timerState.remainingSeconds;
  const displayTimeStr = formatDuration(displayTime);

  const setTimerConfig = useCallback((config: Partial<TimerConfig>) => {
    setTimerConfigState((prev) => ({ ...prev, ...config }));
  }, []);

  const start = useCallback(async (habitId?: number) => {
    if (timerState.isFinished) {
      await reset();
      return;
    }

    sessionRecordedRef.current = false;
    startTimeRef.current = Date.now();

    setTimerState((prev) => ({
      ...prev,
      isRunning: true,
      isPaused: false,
      isResting: false,
      ...(timerConfig.mode === "countdown"
        ? { remainingSeconds: timerConfig.workDuration, currentRound: 1, elapsedSeconds: 0 }
        : { elapsedSeconds: 0 }),
    }));

    try {
      await apiClientRef.current.startTimer(habitId, {
        mode: timerConfig.mode,
        workDuration: timerConfig.workDuration,
        restDuration: timerConfig.restDuration,
        loopCount: timerConfig.loopCount,
      });
    } catch (e) {
      logError(`启动计时失败: ${e}`);
    }
  }, [timerConfig, timerState.isFinished]);

  const pause = useCallback(async () => {
    setTimerState((prev) => ({ ...prev, isPaused: true }));
    try {
      await apiClientRef.current.pauseTimer();
    } catch (e) {
      logError(`暂停计时失败: ${e}`);
      throw e;
    }
  }, []);

  const resume = useCallback(async (habitId?: number) => {
    if (rafRef.current) {
      cancelAnimationFrame(rafRef.current);
      rafRef.current = null;
    }
    startTimeRef.current = Date.now() - totalElapsedRef.current * 1000;
    setTimerState((prev) => ({ ...prev, isPaused: false }));
    try {
      await apiClientRef.current.resumeTimer(habitId);
    } catch (e) {
      logError(`恢复计时失败: ${e}`);
      throw e;
    }
  }, []);

  const reset = useCallback(async () => {
    setTimerState({
      isRunning: false,
      isPaused: false,
      isFinished: false,
      isResting: false,
      currentRound: 0,
      elapsedSeconds: 0,
      remainingSeconds: timerConfig.workDuration,
    });
    sessionRecordedRef.current = false;
    startTimeRef.current = 0;
    totalElapsedRef.current = 0;

    try {
      await apiClientRef.current.resetTimer();
    } catch (e) {
      logError(`重置计时失败: ${e}`);
      throw e;
    }
  }, [timerConfig.workDuration]);

  const finish = useCallback(async (): Promise<{ elapsed_seconds: number }> => {
    try {
      const result = await apiClientRef.current.finishTimer();
      logSuccess(`✓ 已计入今日统计: ${formatDuration(result.elapsed_seconds)}`);

      setTimerState({
        isRunning: false,
        isPaused: false,
        isFinished: false,
        currentRound: 0,
        elapsedSeconds: 0,
        remainingSeconds: timerConfig.workDuration,
        isResting: false,
      });
      sessionRecordedRef.current = false;

      return result;
    } catch (e) {
      logError(`结束计时失败: ${e}`);
      throw e;
    }
  }, [timerConfig.workDuration]);

  const skipToNext = useCallback(() => {
    if (timerConfig.mode === "countdown" && timerState.isRunning) {
      if (timerState.isResting) {
        setTimerState((prev) => ({
          ...prev,
          isResting: false,
          remainingSeconds: timerConfig.workDuration,
          currentRound: prev.currentRound + 1,
        }));
      } else {
        if (timerConfig.loopCount > 0 && timerState.currentRound >= timerConfig.loopCount) {
          setTimerState((prev) => ({ ...prev, isFinished: true, isRunning: false }));
        } else if (timerConfig.restDuration > 0) {
          setTimerState((prev) => ({
            ...prev,
            isResting: true,
            remainingSeconds: timerConfig.restDuration,
          }));
        } else {
          setTimerState((prev) => ({
            ...prev,
            currentRound: prev.currentRound + 1,
            remainingSeconds: timerConfig.workDuration,
          }));
        }
      }
    }
  }, [timerConfig, timerState.isRunning, timerState.isResting, timerState.currentRound]);

  useEffect(() => {
    if (!timerState.isRunning || timerState.isPaused || timerState.isFinished) {
      if (rafRef.current) {
        cancelAnimationFrame(rafRef.current);
        rafRef.current = null;
      }
      return;
    }

    let cancelled = false;

    const tick = () => {
      if (cancelled) return;

      const wallElapsed = Math.floor((Date.now() - startTimeRef.current) / 1000);

      if (timerConfig.mode === "stopwatch") {
        totalElapsedRef.current = wallElapsed;
        setTimerState((prev) => ({ ...prev, elapsedSeconds: wallElapsed }));
      } else {
        if (timerState.isResting) {
          const restElapsed = wallElapsed - timerConfig.workDuration;
          const newVal = timerConfig.restDuration - restElapsed;
          if (newVal <= 0) {
            setTimerState((prev) => ({
              ...prev,
              isResting: false,
              remainingSeconds: timerConfig.workDuration,
              currentRound: prev.currentRound + 1,
            }));
          } else {
            setTimerState((prev) => ({ ...prev, remainingSeconds: newVal }));
          }
        } else {
          const newVal = timerConfig.workDuration - wallElapsed;
          if (newVal <= 0) {
            if (timerConfig.loopCount > 0 && timerState.currentRound >= timerConfig.loopCount) {
              setTimerState((prev) => ({ ...prev, isFinished: true, isRunning: false }));
              return;
            } else if (timerConfig.restDuration > 0) {
              setTimerState((prev) => ({
                ...prev,
                isResting: true,
                remainingSeconds: timerConfig.restDuration,
              }));
            } else {
              setTimerState((prev) => ({
                ...prev,
                currentRound: prev.currentRound + 1,
                remainingSeconds: timerConfig.workDuration,
              }));
            }
          } else {
            setTimerState((prev) => ({ ...prev, remainingSeconds: newVal }));
          }
        }
      }

      if (!cancelled) {
        rafRef.current = requestAnimationFrame(tick);
      }
    };

    rafRef.current = requestAnimationFrame(tick);

    return () => {
      cancelled = true;
      if (rafRef.current) {
        cancelAnimationFrame(rafRef.current);
        rafRef.current = null;
      }
    };
  }, [timerState.isRunning, timerState.isPaused, timerState.isFinished, timerConfig, timerState.isResting, timerState.currentRound]);

  return {
    timerConfig,
    isRunning: timerState.isRunning,
    isPaused: timerState.isPaused,
    isFinished: timerState.isFinished,
    isResting: timerState.isResting,
    currentRound: timerState.currentRound,
    elapsedSeconds: timerState.elapsedSeconds,
    remainingSeconds: timerState.remainingSeconds,
    displayTime: displayTimeStr,
    setTimerConfig,
    start,
    pause,
    resume,
    reset,
    skipToNext,
    finish,
  };
};
