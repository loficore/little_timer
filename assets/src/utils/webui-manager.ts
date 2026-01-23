//! WebUI 连接管理器 - 处理与后端的连接、重试和错误恢复

export const ConnectionState = {
  CONNECTED: 'connected' as const,
  DISCONNECTED: 'disconnected' as const,
  RECONNECTING: 'reconnecting' as const,
  ERROR: 'error' as const,
} as const;

export type ConnectionState = typeof ConnectionState[keyof typeof ConnectionState];

export interface WebuiManager {
  state: ConnectionState;
  isConnected: boolean;
  lastError?: Error;
  retryCount: number;
  call: (functionName: string, ...args: unknown[]) => Promise<void>;
}

// 全局连接状态
let connectionState: ConnectionState = ConnectionState.DISCONNECTED;
let retryCount = 0;
const MAX_RETRIES = 5;
const RETRY_DELAY_MS = 1000;
let lastError: Error | undefined;

// 事件监听器
type EventCallback = (state: ConnectionState, error?: Error) => void;
const listeners: Map<string, Set<EventCallback>> = new Map();

/**
 * 注册连接状态变化监听器
 * @param eventName - 事件名称（'connected', 'disconnected', 'reconnecting', 'error'）
 * @param callback - 回调函数
 */
export function onConnectionStateChange(
  eventName: ConnectionState,
  callback: (state: ConnectionState, error?: Error) => void
): () => void {
  if (!listeners.has(eventName)) {
    listeners.set(eventName, new Set());
  }
  listeners.get(eventName)!.add(callback);

  // 返回取消注册函数
  return () => {
    listeners.get(eventName)?.delete(callback);
  };
}

/**
 * 触发连接状态变化事件
 */
function emitStateChange(state: ConnectionState, error?: Error): void {
  connectionState = state;
  if (error) {
    lastError = error;
  }

  const callbacks = listeners.get(state);
  if (callbacks) {
    callbacks.forEach((callback) => {
      try {
        callback(state, error);
      } catch (e) {
        console.error('Event listener error:', e);
      }
    });
  }
}

/**
 * 检查 WebUI 是否可用
 */
function isWebuiAvailable(): boolean {
  return typeof window.webui !== 'undefined' && window.webui !== null;
}

/**
 * 包装 WebUI 调用，添加错误处理和重试机制
 */
export const createWebuiCall = (
  originalCall: (functionName: string, ...args: unknown[]) => void
) => {
  return async (functionName: string, ...args: unknown[]): Promise<void> => {
    // 如果没有连接，尝试重新连接
    if (!isWebuiAvailable()) {
      if (connectionState !== ConnectionState.RECONNECTING) {
        emitStateChange(ConnectionState.DISCONNECTED);
        attemptReconnect();
      }
      throw new Error('WebUI 后端未连接，正在尝试重新连接...');
    }

    try {
      // 尝试调用
      originalCall(functionName, ...args);
      
      // 成功，更新状态
      if (connectionState !== ConnectionState.CONNECTED) {
        retryCount = 0;
        emitStateChange(ConnectionState.CONNECTED);
      }
    } catch (error) {
      const err = error instanceof Error ? error : new Error(String(error));
      console.error(`❌ WebUI 调用失败 [${functionName}]:`, err);
      
      emitStateChange(ConnectionState.ERROR, err);
      
      // 尝试重新连接
      if (isWebuiAvailable()) {
        attemptReconnect();
      }
      
      throw err;
    }
  };
};

/**
 * 尝试重新连接
 */
function attemptReconnect(): void {
  if (retryCount >= MAX_RETRIES) {
    emitStateChange(
      ConnectionState.ERROR,
      new Error(`无法连接到 WebUI 后端（已重试 ${MAX_RETRIES} 次）`)
    );
    return;
  }

  retryCount++;
  emitStateChange(ConnectionState.RECONNECTING);

  console.info(`🔄 尝试重新连接... (${retryCount}/${MAX_RETRIES})`);

  setTimeout(() => {
    if (isWebuiAvailable()) {
      console.info('✅ WebUI 后端已恢复连接');
      retryCount = 0;
      emitStateChange(ConnectionState.CONNECTED);
    } else {
      attemptReconnect();
    }
  }, RETRY_DELAY_MS * retryCount); // 指数退避
}

/**
 * 获取当前连接状态
 */
export function getConnectionState(): ConnectionState {
  return connectionState;
}

/**
 * 检查是否已连接
 */
export function isConnected(): boolean {
  return connectionState === ConnectionState.CONNECTED && isWebuiAvailable();
}

/**
 * 获取最后的错误
 */
export function getLastError(): Error | undefined {
  return lastError;
}

/**
 * 获取重试次数
 */
export function getRetryCount(): number {
  return retryCount;
}

/**
 * 获取 WebUI 管理器实例
 */
export function getWebuiManager(): WebuiManager {
  return {
    state: connectionState,
    isConnected: isConnected(),
    lastError,
    retryCount,
    call: window.webui ? createWebuiCall(window.webui.call) : async () => {
      throw new Error('WebUI 未初始化');
    },
  };
}

/**
 * 初始化 WebUI 管理器
 */
export function initWebuiManager(): void {
  if (isWebuiAvailable()) {
    console.info('✅ WebUI 连接已建立');
    emitStateChange(ConnectionState.CONNECTED);
  } else {
    console.warn('⚠️ WebUI 连接未建立，等待重试...');
    emitStateChange(ConnectionState.DISCONNECTED);
    attemptReconnect();
  }
}
