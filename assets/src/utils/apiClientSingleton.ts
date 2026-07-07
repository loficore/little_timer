/**
 * API 客户端单例
 * 统一管理 API 实例，避免重复创建
 *
 * Android detection: Wails v3 sets window.wails on Android.
 * When detected, we return WailsAPIClient which calls Go bindings
 * directly via JNI — no HTTP server needed.
 */

import { APIClient } from "./apiClient";
import { WailsAPIClient } from "./wailsApiClient";
import { DEFAULT_API_URL } from "../utils/constants";
import type { TimerState, Settings } from "../types/api";

// Detect Android: Wails v3 sets window.wails on Android
const isAndroid = typeof window !== "undefined" && !!(window as any).wails;

// eslint-disable-next-line @typescript-eslint/no-explicit-any
type APIClientInterface = any;

let apiClientInstance: APIClientInterface = null;

/**
 * 获取 API 客户端单例
 * Android: uses WailsAPIClient (Go bindings via JNI)
 * Desktop: uses APIClient (HTTP fetch)
 */
export const getAPIClient = (): APIClientInterface => {
  if (!apiClientInstance) {
    if (isAndroid) {
      apiClientInstance = new WailsAPIClient();
    } else {
      const baseUrl = typeof window !== "undefined" ? window.location.origin : DEFAULT_API_URL;
      apiClientInstance = new APIClient(baseUrl);
    }
  }
  return apiClientInstance;
};

/**
 * 重置 API 客户端实例
 * 主要用于测试
 */
export const resetAPIClient = (): void => {
  apiClientInstance = null;
};

export { APIClient };
export type { TimerState, Settings };
