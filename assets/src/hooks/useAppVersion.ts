import { useEffect, useState } from "preact/hooks";
import { getAPIClient } from "../utils/apiClientSingleton";

export interface AppVersion {
  version: string;
  gitCommit: string;
}

/**
 * useAppVersion —— 拉一次进程级构建信息（GET /api/version）。
 *
 * 版本号唯一来源是后端的链接期打戳（发布时 = git tag），前端不得写死。
 * 加载中返回 null（调用方不渲染，避免闪一下旧值）；接口失败回退 "dev"。
 */
export function useAppVersion(): AppVersion | null {
  const [info, setInfo] = useState<AppVersion | null>(null);

  useEffect(() => {
    let alive = true;
    // 防御部分 mock / Android Wails 代理：拿不到客户端或方法时按 "dev" 兜底，
    // 不让版本探测把整个页面搞崩。
    const client = typeof getAPIClient === "function" ? getAPIClient() : undefined;
    const request: Promise<{ version?: string; git_commit?: string } | undefined> =
      client && typeof client.getVersion === "function"
        ? client.getVersion()
        : Promise.resolve(undefined);

    request
      .then((v) => {
        if (!alive) return;
        setInfo({ version: v?.version || "dev", gitCommit: v?.git_commit || "unknown" });
      })
      .catch(() => {
        if (alive) setInfo({ version: "dev", gitCommit: "unknown" });
      });
    return () => {
      alive = false;
    };
  }, []);

  return info;
}
