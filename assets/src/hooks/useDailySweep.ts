/**
 * useDailySweep —— GTD Daily Sweep / Review 提议读写 hook。
 *
 * - refresh():GET /api/daily-sweep/today
 * - start():POST /api/daily-sweep/start → 之后 refresh
 * - adopt():POST /api/daily-sweep/adopt;409 → 设冲突 error + refresh
 *   (reason: AI 提议基于旧 version,用户需重拉最新提议)
 * - reject():POST /api/daily-sweep/reject → 从列表移除
 *
 * 不自动挂载时 refresh:页面(或调用方)显式 refresh,避免每次渲染多打一次
 * 请求 —— 单元测试与 UI 都更可控。
 */

import { useCallback, useState } from "preact/hooks";
import { getAPIClient } from "../utils/apiClientSingleton";
import { logError } from "../utils/logger";
import type { DailySweepProposal } from "../types/aigtd";

export interface UseDailySweep {
  sweptAt: string | null;
  source: "cron" | "manual" | null;
  proposals: DailySweepProposal[];
  loading: boolean;
  error: string | null;
  start: () => Promise<void>;
  adopt: (p: DailySweepProposal) => Promise<void>;
  reject: (p: DailySweepProposal) => Promise<void>;
  refresh: () => Promise<void>;
}

/** 从 proposals 列表里去掉指定 (ai_job_id, change_id) 的条目。 */
function dropProposal(list: DailySweepProposal[], p: DailySweepProposal): DailySweepProposal[] {
  return list.filter(
    (x) => !(x.ai_job_id === p.ai_job_id && x.proposal.change_id === p.proposal.change_id),
  );
}

export function useDailySweep(): UseDailySweep {
  const [sweptAt, setSweptAt] = useState<string | null>(null);
  const [source, setSource] = useState<"cron" | "manual" | null>(null);
  const [proposals, setProposals] = useState<DailySweepProposal[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const refresh = useCallback(async () => {
    setLoading(true);
    try {
      const data = await getAPIClient().getDailySweepToday();
      setSweptAt(data?.swept_at ?? null);
      setSource(data?.source ?? null);
      setProposals(data?.proposals ?? []);
      setError(null);
    } catch (err) {
      const msg = err instanceof Error ? err.message : String(err);
      setError(msg);
      logError("useDailySweep.refresh failed", new Error(msg));
    } finally {
      setLoading(false);
    }
  }, []);

  const start = useCallback(async () => {
    setLoading(true);
    try {
      await getAPIClient().startDailySweep();
      await refresh();
    } catch (err) {
      const msg = err instanceof Error ? err.message : String(err);
      setError(msg);
      logError("useDailySweep.start failed", new Error(msg));
    } finally {
      setLoading(false);
    }
  }, [refresh]);

  const adopt = useCallback(
    async (p: DailySweepProposal) => {
      try {
        await getAPIClient().adoptDailySweep({
          change_id: p.proposal.change_id,
          ai_job_id: p.ai_job_id,
          version: p.version,
          proposal: p.proposal,
        });
        setProposals((prev) => dropProposal(prev, p));
        setError(null);
      } catch (err) {
        const msg = err instanceof Error ? err.message : String(err);
        logError("useDailySweep.adopt failed", new Error(msg));
        // 先重拉最新提议(refresh 内部会清 error),再设冲突提示,避免被清空。
        await refresh();
        setError(msg);
      }
    },
    [refresh],
  );

  const reject = useCallback(async (p: DailySweepProposal) => {
    try {
      await getAPIClient().rejectDailySweep({ change_id: p.proposal.change_id });
      setProposals((prev) => dropProposal(prev, p));
      setError(null);
    } catch (err) {
      const msg = err instanceof Error ? err.message : String(err);
      setError(msg);
      logError("useDailySweep.reject failed", new Error(msg));
    }
  }, []);

  return { sweptAt, source, proposals, loading, error, start, adopt, reject, refresh };
}
