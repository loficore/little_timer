/**
 * SSE 连接管理 Hook
 * 统一管理 SSE 连接、自动重连和事件处理
 */

import { useState, useEffect, useRef, useCallback } from "preact/hooks";
import { SSEClient } from "../utils/sseClient";
import { getAPIClient, type TimerState } from "../utils/apiClientSingleton";
import { logInfo, logError } from "../utils/logger";

export interface UseSSEReturn {
  isConnected: boolean;
  connect: () => void;
  disconnect: () => void;
  lastState: TimerState | null;
}

export const useSSE = (
  onMessage?: (data: TimerState) => void,
  onError?: (error: unknown) => void
): UseSSEReturn => {
  const sseClientRef = useRef<SSEClient | null>(null);
  const apiClientRef = useRef(getAPIClient());
  const [isConnected, setIsConnected] = useState(false);
  const [lastState, setLastState] = useState<TimerState | null>(null);

  const disconnect = useCallback(() => {
    if (sseClientRef.current) {
      sseClientRef.current.close();
      sseClientRef.current = null;
    }
    setIsConnected(false);
  }, []);

  const connect = useCallback(() => {
    if (sseClientRef.current?.isConnected()) {
      return;
    }

    const baseUrl = apiClientRef.current.baseUrl;
    sseClientRef.current = new SSEClient(baseUrl);

    sseClientRef.current.connect(
      (timerState) => {
        setLastState(timerState);
        onMessage?.(timerState);
      },
      (error) => {
        setIsConnected(false);
        let errorMsg = "未知错误";
        if (error instanceof Error) {
          errorMsg = error.message;
        } else if (typeof error === "string") {
          errorMsg = error;
        }
        logError(`SSE 连接错误: ${errorMsg}`);
        onError?.(error);
      },
      // 连接成功时的回调
      () => {
        setIsConnected(true);
        logInfo("SSE 连接已建立");
      }
    );
  }, [onMessage, onError]);

  useEffect(() => {
    return () => {
      disconnect();
    };
  }, [disconnect]);

  return {
    isConnected,
    connect,
    disconnect,
    lastState,
  };
};
