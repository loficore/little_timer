/**
 * API 客户端单例
 * 统一管理 API 实例，避免重复创建
 *
 * Android 检测：Wails v3 在 Android 上会设置 window.wails。
 * 检测到时返回 WailsAPIClient，它通过 JNI 直接调用 Go 绑定，
 * 无需 HTTP 服务器。
 *
 * Rollup 动态导入：仅在 isAndroid === true 时才导入 WailsAPIClient，
 * 桌面构建永远不会解析 ../bindings/... 路径。
 */

import { APIClient } from "./apiClient";
import { DEFAULT_API_URL } from "../utils/constants";
import type { TimerState, Settings } from "../types/api";

const isAndroid = typeof window !== "undefined" && !!(window as any).wails;

type APIClientInterface = any;

let apiClientInstance: APIClientInterface = null;
let _wailsClientPromise: Promise<any> | null = null;
let _wailsClientInstance: APIClientInterface | null = null;

async function loadWailsClient() {
  if (!_wailsClientPromise) {
    _wailsClientPromise = import("./wailsApiClient").then((mod) => mod.WailsAPIClient);
  }
  return _wailsClientPromise;
}

function _wailsClientProxy(): APIClientInterface {
  return new Proxy({}, {
    get(_target, prop: string) {
      return async (...args: any[]) => {
        if (!_wailsClientInstance) {
          await _wailsClientPromise;
        }
        const clz = _wailsClientInstance as any;
        const fn = clz[prop];
        if (typeof fn === "function") {
          return fn.apply(clz, args);
        }
        return clz[prop];
      };
    }
  });
}

export const getAPIClient = (): APIClientInterface => {
  if (!apiClientInstance) {
    if (isAndroid) {
      void (async () => {
        const Clz = await loadWailsClient();
        _wailsClientInstance = new Clz();
        apiClientInstance = _wailsClientInstance;
      })();
      return _wailsClientProxy() as APIClientInterface;
    }
    const baseUrl = typeof window !== "undefined" ? window.location.origin : DEFAULT_API_URL;
    apiClientInstance = new APIClient(baseUrl);
  }
  return apiClientInstance;
};

export const resetAPIClient = (): void => {
  apiClientInstance = null;
};

export { APIClient };
export type { TimerState, Settings };
