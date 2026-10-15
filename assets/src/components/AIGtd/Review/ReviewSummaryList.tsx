import type { DailySweepProposal } from "../../../types/aigtd";
import { t } from "../../../utils/i18n";

interface ReviewSummaryListProps {
  proposals: DailySweepProposal[];
  onOpen: (proposal: DailySweepProposal) => void;
}

/**
 * ReviewSummaryList —— review 概览。
 *
 * 按 task_id 分组,每组:
 *   - 任务标题(从 proposal[0].proposal 拿不到,直接显示 task_id 兜底)
 *   - 该 task 的 1+ 条 proposal 摘要(简短描述 action + reason)
 *
 * 卡片可点击 → 调 onOpen(proposal) → 父组件展开 ReviewProposalPanel(见 Task 9)。
 */
export function ReviewSummaryList({ proposals, onOpen }: ReviewSummaryListProps) {
  if (proposals.length === 0) return null;

  // 按 task_id 分组
  const byTask = new Map<number, DailySweepProposal[]>();
  for (const p of proposals) {
    const arr = byTask.get(p.proposal.task_id) ?? [];
    arr.push(p);
    byTask.set(p.proposal.task_id, arr);
  }

  return (
    <div className="space-y-3" role="list" aria-label={t("review.list.label")}>
      {[...byTask.entries()].map(([taskID, group]) => (
        <div
          key={taskID}
          role="listitem"
          className="rounded-lg border border-base-300 bg-base-100 p-3 space-y-2"
        >
          <div className="text-xs text-base-content/60">task #{taskID}</div>
          <ul className="space-y-1">
            {group.map((p) => (
              <li key={p.proposal.change_id}>
                <button
                  className="w-full text-left rounded p-2 hover:bg-base-200 transition"
                  onClick={() => onOpen(p)}
                  data-testid={`review-proposal-${p.proposal.change_id}`}
                >
                  <div className="flex items-center gap-2">
                    <span className="badge badge-sm badge-outline">{p.proposal.action}</span>
                    <span className="font-mono text-xs text-base-content/50">
                      {p.proposal.change_id}
                    </span>
                  </div>
                  {p.proposal.reason && (
                    <p className="mt-1 text-sm text-base-content/80 line-clamp-2">
                      {p.proposal.reason}
                    </p>
                  )}
                </button>
              </li>
            ))}
          </ul>
        </div>
      ))}
    </div>
  );
}
