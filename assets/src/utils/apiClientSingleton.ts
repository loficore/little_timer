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
 *
 * 能力表：`capabilities` 由 `platformApi.ts` 在模块加载时算好，
 * 这里原样转出。必须保持为模块级常量，不能做成 getAPIClient() 实例属性，
 * 因为 Wails 客户端 Proxy 会把任意属性读取包装成恒 truthy 的函数。
 * `supportsDailySweep` 是 `capabilities.dailySweep` 的兼容别名，
 * 保留给既有调用点（Settings / PlanPage），新增代码请直接用 capabilities。
 */

import { APIClient } from "./apiClient";
import { DEFAULT_API_URL } from "../utils/constants";
import { capabilities } from "./platformApi";
import type { PlatformApi } from "./platformApi";
import type { TimerState, Settings } from "../types/api";

const isAndroid = typeof window !== "undefined" && !!(window as any).wails;

/** 每日梳理能力（GitHub issue #41 的绑定补齐），兼容既有调用点。 */
export const supportsDailySweep = capabilities.dailySweep;

export { capabilities };

let apiClientInstance: PlatformApi | null = null;
let _wailsClientPromise: Promise<any> | null = null;
let _wailsClientInstance: PlatformApi | null = null;

async function loadWailsClient() {
  if (!_wailsClientPromise) {
    _wailsClientPromise = import("./wailsApiClient").then((mod) => mod.WailsAPIClient);
  }
  return _wailsClientPromise;
}

/**
 * Wails 首调代理：绑定尚未加载完成时把调用排队，加载后转发到真实实例。
 *
 * 返回的方法一律是 async，因此首次调用不会因为实例还未就绪而返回 undefined。
 */
function _wailsClientProxy(): PlatformApi {
  return new Proxy({} as PlatformApi, {
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

/**
 * 取当前平台的 API 客户端。
 *
 * Android 上首次调用返回排队代理，后续调用返回真实 WailsAPIClient 实例；
 * 桌面 / Web 返回 HTTP APIClient。两者都满足 `PlatformApi`。
 */
export const getAPIClient = (): PlatformApi => {
  if (!apiClientInstance) {
    if (isAndroid) {
      void (async () => {
        const Clz = await loadWailsClient();
        _wailsClientInstance = new Clz() as PlatformApi;
        apiClientInstance = _wailsClientInstance;
      })();
      return _wailsClientProxy();
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
