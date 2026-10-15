import { useState } from "preact/hooks";
import type { DailySweepProposal, ReviewProposal } from "../../../types/aigtd";
import { t } from "../../../utils/i18n";
import { useReviewRefine } from "../../../hooks/useReviewRefine";
import { ReviewChip, type ChipKind } from "./ReviewChip";

interface ReviewProposalPanelProps {
  proposal: DailySweepProposal;
  onAdopt: (newProposal: ReviewProposal) => void;
  onReject: () => void;
  onClose: () => void;
}

/**
 * ReviewProposalPanel —— 展开态:5 chip + 自然语言输入框 + 精化状态。
 *
 * 状态机:
 *   idle       → 等待用户操作
 *   refining   → refine 模式(调 LLM),chip 行 spinner
 *   reparsing  → rewrite 模式(不调 LLM,内部 reparse),spinner
 *   done       → 精化完成(显示新 proposal,等用户采纳/拒绝)
 *   error      → 显示错误,chip 仍可用
 */
export function ReviewProposalPanel({
  proposal,
  onAdopt,
  onReject,
  onClose,
}: ReviewProposalPanelProps) {
  const p = proposal.proposal;
  const [userMessage, setUserMessage] = useState("");
  const [lastChip, setLastChip] = useState<ChipKind | null>(null);
  const [refinedProposal, setRefinedProposal] = useState<ReviewProposal | null>(null);
  const { status, error, refine, lastMode, reset } = useReviewRefine();

  const busy = status === "submitting" || status === "waiting";

  const handleChip = async (kind: ChipKind) => {
    setLastChip(kind);
    const chipHint = t(`review.chip.${kind}`);
    const result = await refine(proposal.ai_job_id, p.change_id, chipHint, "", p.task_id);
    if (result.status === "done" && result.proposal) {
      setRefinedProposal(result.proposal);
    }
  };

  const handleNL = async () => {
    if (!userMessage.trim()) return;
    setLastChip(null);
    const result = await refine(proposal.ai_job_id, p.change_id, "", userMessage, p.task_id);
    if (result.status === "done" && result.proposal) {
      setRefinedProposal(result.proposal);
    }
  };

  const currentProposal = refinedProposal ?? p;
  const modeHint = lastMode === "rewrite" ? t("review.refine.reparsing") : t("review.refine.refining");

  return (
    <div
      className="rounded-lg border border-primary/40 bg-base-100 p-4 space-y-3"
      data-testid="review-panel"
    >
      <div className="flex items-center justify-between">
        <span className="badge badge-outline">{currentProposal.action}</span>
        <button className="btn btn-ghost btn-xs" onClick={onClose} aria-label="close">
          ✕
        </button>
      </div>
      <p className="text-sm">{currentProposal.reason}</p>
      {refinedProposal && (
        <p className="text-xs text-success" data-testid="refined-marker">
          {t("review.refine.refinedFrom", { from: p.change_id })}
        </p>
      )}

      <div className="flex flex-wrap gap-2" data-testid="review-chips">
        {(["reschedule", "split", "rewrite", "mark_done", "drop"] as ChipKind[]).map((k) => (
          <ReviewChip
            key={k}
            kind={k}
            active={lastChip === k && busy}
            disabled={busy}
            onClick={() => void handleChip(k)}
          />
        ))}
      </div>

      <div className="flex gap-2">
        <input
          className="input input-bordered input-sm flex-1"
          placeholder={t("review.refine.placeholder")}
          value={userMessage}
          onInput={(e) => setUserMessage((e.target as HTMLInputElement).value)}
          disabled={busy}
          data-testid="review-nl-input"
        />
        <button
          className="btn btn-sm btn-secondary"
          disabled={busy || !userMessage.trim()}
          onClick={() => void handleNL()}
          data-testid="review-nl-send"
        >
          {t("review.refine.send")}
        </button>
      </div>

      {busy && (
        <p className="text-xs opacity-60" data-testid="review-status">
          {modeHint}…
        </p>
      )}
      {status === "error" && error && (
        <p className="text-xs text-error" data-testid="review-error">
          {error}
        </p>
      )}

      <div className="card-actions justify-end">
        <button className="btn btn-sm btn-ghost" onClick={onReject} disabled={busy}>
          {t("dailySweep.reject")}
        </button>
        <button
          className="btn btn-sm btn-primary"
          disabled={busy || !refinedProposal}
          onClick={() => refinedProposal && onAdopt(refinedProposal)}
          data-testid="review-adopt"
        >
          {t("dailySweep.adopt")}
        </button>
      </div>

      {/* 调试用:完成后允许重置 */}
      {status === "done" && (
        <button className="btn btn-ghost btn-xs" onClick={() => { reset(); setRefinedProposal(null); setUserMessage(""); }}>
          reset
        </button>
      )}
    </div>
  );
}
