import { useEffect } from "preact/hooks";
import { useDailySweep } from "./hooks/useDailySweep";
import { t } from "./utils/i18n";
import { ProposalCard } from "./components/DailySweep/ProposalCard";
import { ArrowPathIcon } from "./utils/icons";

/**
 * DailySweepPage —— GTD Daily Sweep / Review 提议页面。
 *
 * 顶部 "立即扫一遍" 按钮(POST /daily-sweep/start → refresh);
 * 下方按时间倒序列出当日(近 1 天)成功的 review job proposals,
 * 每条 ProposalCard 支持 adopt / reject。
 */
export function DailySweepPage() {
  const { sweptAt, source, proposals, loading, error, start, adopt, reject, refresh } =
    useDailySweep();

  useEffect(() => {
    // 页面挂载时拉一次(hook 本身不自动刷新,集中在这里做,避免多源)。
    void refresh();
  }, []);

  return (
    <div className="container mx-auto max-w-3xl space-y-3 p-4">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold">{t("dailySweep.title")}</h1>
        <div className="flex gap-2">
          <button
            className="btn btn-sm btn-ghost"
            data-testid="refresh-btn"
            onClick={() => void refresh()}
            disabled={loading}
          >
            <ArrowPathIcon className="h-4 w-4" />
          </button>
          <button
            className="btn btn-primary btn-sm"
            data-testid="start-btn"
            onClick={() => void start()}
            disabled={loading}
          >
            {t("dailySweep.start")}
          </button>
        </div>
      </div>

      {sweptAt && (
        <p className="text-xs opacity-60" data-testid="swept-meta">
          {t("dailySweep.sweptAt", { at: new Date(sweptAt).toLocaleString() })}
          {source ? ` (${source})` : ""}
        </p>
      )}

      {error && (
        <div className="alert alert-warning" data-testid="error-alert">
          <span>{error}</span>
        </div>
      )}

      {loading && proposals.length === 0 && (
        <p className="opacity-60">…</p>
      )}

      {proposals.length === 0 && !loading ? (
        <div className="text-center opacity-60 py-8" data-testid="empty">
          {t("dailySweep.empty")}
        </div>
      ) : (
        proposals.map((p) => (
          <ProposalCard
            key={`${p.ai_job_id}:${p.proposal.change_id}`}
            proposal={p}
            onAdopt={() => void adopt(p)}
            onReject={() => void reject(p)}
          />
        ))
      )}
    </div>
  );
}
