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
  start: () => Promise<void>;
  pause: () => Promise<void>;
  resume: () => Promise<void>;
  reset: () => Promise<void>;
  skipToNext: () => void;
  finish: () => Promise<void>;
}

export const useTimer = (): UseTimerReturn => {
  const apiClientRef = useRef(getAPIClient());
  const timerIntervalRef = useRef<number | null>(null);
  const sessionRecordedRef = useRef(false);

  const [timerConfig, setTimerConfigState] = useState<TimerConfig>({
    mode: "stopwatch",
    workDuration: 25 * 60,
    restDuration: 5 * 60,
    loopCount: 0,
  });

  const [isRunning, setIsRunning] = useState(false);
  const [isPaused, setIsPaused] = useState(false);
  const [isFinished, setIsFinished] = useState(false);
  const [isResting, setIsResting] = useState(false);
  const [currentRound, setCurrentRound] = useState(0);
  const [elapsedSeconds, setElapsedSeconds] = useState(0);
  const [remainingSeconds, setRemainingSeconds] = useState(0);

  const displayTime = timerConfig.mode === "stopwatch" ? elapsedSeconds : remainingSeconds;
  const displayTimeStr = formatDuration(displayTime);

  const setTimerConfig = useCallback((config: Partial<TimerConfig>) => {
    setTimerConfigState((prev) => ({ ...prev, ...config }));
  }, []);

  const start = useCallback(async () => {
    if (isFinished) {
      await reset();
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
      await apiClientRef.current.startTimer(undefined, {
        mode: timerConfig.mode,
        workDuration: timerConfig.workDuration,
        restDuration: timerConfig.restDuration,
        loopCount: timerConfig.loopCount,
      });
    } catch (e) {
      logError(`启动计时失败: ${e}`);
    }
  }, [timerConfig, isFinished]);

  const pause = useCallback(async () => {
    setIsPaused(true);
    try {
      await apiClientRef.current.pauseTimer();
    } catch (e) {
      logError(`暂停计时失败: ${e}`);
    }
  }, []);

  const resume = useCallback(async () => {
    setIsPaused(false);
    try {
      await apiClientRef.current.resumeTimer();
    } catch (e) {
      logError(`恢复计时失败: ${e}`);
    }
  }, []);

  const reset = useCallback(async () => {
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
    } catch (e) {
      logError(`重置计时失败: ${e}`);
    }
  }, [timerConfig.workDuration]);

  const finish = useCallback(async () => {
    try {
      const result = await apiClientRef.current.finishTimer();
      logSuccess(`✓ 已计入今日统计: ${formatDuration(result.elapsed_seconds)}`);
    } catch (e) {
      logError(`结束计时失败: ${e}`);
      return;
    }

    setIsRunning(false);
    setIsPaused(false);
    setIsFinished(false);
    setElapsedSeconds(0);
    setRemainingSeconds(timerConfig.workDuration);
    setCurrentRound(0);
    sessionRecordedRef.current = false;
  }, [timerConfig.workDuration]);

  const skipToNext = useCallback(() => {
    if (timerConfig.mode === "countdown" && isRunning) {
      if (isResting) {
        setIsResting(false);
        setRemainingSeconds(timerConfig.workDuration);
        setCurrentRound((prev) => prev + 1);
      } else {
        if (timerConfig.loopCount > 0 && currentRound >= timerConfig.loopCount) {
          setIsFinished(true);
          setIsRunning(false);
        } else if (timerConfig.restDuration > 0) {
          setIsResting(true);
          setRemainingSeconds(timerConfig.restDuration);
        } else {
          setCurrentRound((prev) => prev + 1);
          setRemainingSeconds(timerConfig.workDuration);
        }
      }
    }
  }, [timerConfig, isRunning, isResting, currentRound]);

  useEffect(() => {
    if (isRunning && !isPaused && !isFinished) {
      timerIntervalRef.current = window.setInterval(() => {
        if (timerConfig.mode === "stopwatch") {
          setElapsedSeconds((prev) => prev + 1);
        } else {
          if (isResting) {
            setRemainingSeconds((prev) => {
              const newVal = prev - 1;
              if (newVal <= 0) {
                setIsResting(false);
                setRemainingSeconds(timerConfig.workDuration);
                setCurrentRound((prev) => prev + 1);
                return 0;
              }
              return newVal;
            });
          } else {
            setRemainingSeconds((prev) => {
              const newVal = prev - 1;
              if (newVal <= 0) {
                if (timerConfig.loopCount > 0 && currentRound >= timerConfig.loopCount) {
                  setIsFinished(true);
                  setIsRunning(false);
                  return 0;
                } else if (timerConfig.restDuration > 0) {
                  setIsResting(true);
                  setRemainingSeconds(timerConfig.restDuration);
                  return timerConfig.restDuration;
                } else {
                  setCurrentRound((prev) => prev + 1);
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

  return {
    timerConfig,
    isRunning,
    isPaused,
    isFinished,
    isResting,
    currentRound,
    elapsedSeconds,
    remainingSeconds,
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
