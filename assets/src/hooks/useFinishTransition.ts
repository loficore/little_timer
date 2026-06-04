import { useRef, useState, useCallback } from "preact/hooks";
import type { TimerState } from "../utils/apiClientSingleton";
import { formatDuration } from "../utils/formatters";

const TRANSITION_DURATION_MS = 1500;

interface TransitionState {
  active: boolean;
  startTime: number;
  localTime: number;
  authoritativeTime: number;
}

const easeOutCubic = (t: number, start: number, end: number): number => {
  const duration = TRANSITION_DURATION_MS;
  const progress = Math.min(t / duration, 1);
  const eased = 1 - Math.pow(1 - progress, 3);
  return start + (end - start) * eased;
};

interface UseFinishTransitionReturn {
  displayValue: string;
  isTransitioning: boolean;
  startTransition: (localTime: number, authoritativeTime: number) => void;
}

export const useFinishTransition = (
  _lastState: TimerState | null
): UseFinishTransitionReturn => {
  const rafRef = useRef<number | null>(null);
  const transitionRef = useRef<TransitionState>({
    active: false,
    startTime: 0,
    localTime: 0,
    authoritativeTime: 0,
  });

  const [isTransitioning, setIsTransitioning] = useState(false);
  const [displayValue, setDisplayValue] = useState("");

    const tick = useCallback(() => {
    const { active, startTime, localTime, authoritativeTime } = transitionRef.current;
    if (!active) return;

    const elapsed = Date.now() - startTime;
    if (elapsed >= TRANSITION_DURATION_MS) {
      transitionRef.current.active = false;
      setIsTransitioning(false);
      setDisplayValue(formatDuration(authoritativeTime));
      return;
    }

    const value = easeOutCubic(elapsed, localTime, authoritativeTime);
    setDisplayValue(formatDuration(Math.round(value)));
    rafRef.current = requestAnimationFrame(tick);
  }, []);

  const startTransition = useCallback(
    (localTime: number, authoritativeTime: number) => {
      if (rafRef.current) {
        cancelAnimationFrame(rafRef.current);
        rafRef.current = null;
      }

      transitionRef.current = {
        active: true,
        startTime: Date.now(),
        localTime,
        authoritativeTime,
      };
      setIsTransitioning(true);
      setDisplayValue(formatDuration(localTime));

      rafRef.current = requestAnimationFrame(tick);
    },
    [tick]
  );

  return {
    displayValue,
    isTransitioning,
    startTransition,
  };
};