import { useEffect, useState } from "preact/hooks";
import { useDailySweep } from "./hooks/useDailySweep";
import { t } from "./utils/i18n";
import { ProposalCard } from "./components/DailySweep/ProposalCard";
import { ResetIcon } from "./utils/icons";
import { ReviewProposalPanel } from "./components/AIGtd/Review";
import type { DailySweepProposal, ReviewProposal } from "./types/aigtd";

/**
 * DailySweepPage —— GTD Daily Sweep / Review 提议页面。
 *
 * 顶部 "立即扫一遍" 按钮(POST /daily-sweep/start → refresh);
 * 下方按时间倒序列出当日(近 1 天)成功的 review job proposals,
 * 每条 ProposalCard 支持 adopt / reject / 精化(refine)。
 * 点 "精化" → 下方就地展开 ReviewProposalPanel(issue #32 task 9)。
 */
export function DailySweepPage() {
  const { sweptAt, source, proposals, loading, error, start, adopt, reject, refresh } =
    useDailySweep();
  const [refining, setRefining] = useState<DailySweepProposal | null>(null);

  useEffect(() => {
    // 页面挂载时拉一次(hook 本身不自动刷新,集中在这里做,避免多源)。
    void refresh();
  }, []);

  // 已采纳的提议是历史,不再堆在待办列表里(Review Fix I-2):server 保留
  // is_adopted 供审计,前端只展示"还需决策"的。否则每次进页面已采纳项都会
  // 回来,empty 状态永远出不来。
  const actionable = proposals.filter((p) => !p.is_adopted);

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
            <ResetIcon className="h-4 w-4" />
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

      {loading && actionable.length === 0 && (
        <p className="opacity-60">…</p>
      )}

      {actionable.length === 0 && !loading ? (
        <div className="text-center opacity-60 py-8" data-testid="empty">
          {t("dailySweep.empty")}
        </div>
      ) : (
        actionable.map((p) => (
          <div key={`${p.ai_job_id}:${p.proposal.change_id}`} className="space-y-2">
            <ProposalCard
              proposal={p}
              onAdopt={() => void adopt(p)}
              onReject={() => void reject(p)}
              onRefine={() => setRefining(p)}
            />
            {refining &&
              refining.ai_job_id === p.ai_job_id &&
              refining.proposal.change_id === p.proposal.change_id && (
                <ReviewProposalPanel
                  proposal={p}
                  onAdopt={(newProp: ReviewProposal) => {
                    void adopt({ ...p, proposal: newProp }).then(() => setRefining(null));
                  }}
                  onReject={() => {
                    void reject(p);
                    setRefining(null);
                  }}
                  onClose={() => setRefining(null)}
                />
              )}
          </div>
        ))
      )}
    </div>
  );
}
