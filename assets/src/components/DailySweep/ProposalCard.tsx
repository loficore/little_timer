import type { FunctionalComponent } from "preact";
import type { DailySweepProposal } from "../../types/aigtd";
import { t } from "../../utils/i18n";
import { CheckIconComponent, CloseIcon } from "../../utils/icons";

interface ProposalCardProps {
  proposal: DailySweepProposal;
  onAdopt: () => void;
  onReject: () => void;
}

const ACTION_BADGES: Record<string, string> = {
  reschedule: "badge-info",
  drop: "badge-error",
  modify_subtasks: "badge-warning",
  no_op: "badge-ghost",
};

/**
 * ProposalCard —— 单条 AI review 提议卡片。
 * 显示 action 标签 + reason + 采纳/拒绝按钮。
 */
export const ProposalCard: FunctionalComponent<ProposalCardProps> = ({
  proposal,
  onAdopt,
  onReject,
}) => {
  const p = proposal.proposal;
  const cls = ACTION_BADGES[p.action] ?? "badge-ghost";
  const subChanges = p.suggested_subtask_changes ?? [];

  return (
    <div className="card bg-base-100 shadow-sm my-2" data-testid="proposal-card">
      <div className="card-body p-4">
        <div className="flex items-center justify-between">
          <span className={`badge ${cls}`} data-testid="action-badge">
            {p.action}
          </span>
          {proposal.is_adopted && (
            <span className="badge badge-success badge-outline">{t("dailySweep.adopted")}</span>
          )}
        </div>
        <p className="mt-2 text-sm">{p.reason}</p>
        {subChanges.length > 0 && (
          <ul className="mt-1 text-xs opacity-70 list-disc pl-5">
            {subChanges.map((sc, i) => (
              <li key={i}>
                {sc.op}
                {sc.subtask_id != null ? ` #${sc.subtask_id}` : ""}
                {sc.new_title ? ` → ${sc.new_title}` : ""}
              </li>
            ))}
          </ul>
        )}
        {p.action === "reschedule" && p.suggested_scheduled_start != null && (
          <p className="text-xs opacity-60">
            → {new Date(p.suggested_scheduled_start * 1000).toLocaleString()}
            {p.suggested_scheduled_end != null
              ? ` – ${new Date(p.suggested_scheduled_end * 1000).toLocaleString()}`
              : ""}
          </p>
        )}
        <div className="card-actions justify-end mt-2">
          <button
            className="btn btn-sm btn-ghost"
            data-testid="reject-btn"
            onClick={onReject}
          >
            <CloseIcon className="h-4 w-4" />
            {t("dailySweep.reject")}
          </button>
          <button
            className="btn btn-sm btn-primary"
            data-testid="adopt-btn"
            onClick={onAdopt}
          >
            <CheckIconComponent className="h-4 w-4" />
            {t("dailySweep.adopt")}
          </button>
        </div>
      </div>
    </div>
  );
};
