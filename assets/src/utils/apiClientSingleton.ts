/**
 * API 客户端单例
 * 统一管理 API 实例，避免重复创建
 */

import { APIClient } from "./apiClient";

let apiClientInstance: APIClient | null = null;

/**
 * 获取 API 客户端单例
 * 使用浏览器的 origin 作为基础 URL
 */
export const getAPIClient = (): APIClient => {
  if (!apiClientInstance) {
    const baseUrl = typeof window !== "undefined" ? window.location.origin : "http://localhost:8080";
    apiClientInstance = new APIClient(baseUrl);
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
export type { TimerState, Settings } from "./apiClient";
