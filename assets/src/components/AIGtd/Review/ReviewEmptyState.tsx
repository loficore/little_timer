import { t } from "../../../utils/i18n";

interface ReviewEmptyStateProps {
  /** 最近一次 review 触发的本地时间(YYYY-MM-DD HH:MM 或 ISO),undefined = 从未跑过 */
  lastSweptAt?: string;
  onRetrigger: () => void;
}

/**
 * ReviewEmptyState —— review 触发后没有积压时的空状态。
 *
 * 文案 + 「再触发一次」按钮(走 POST /api/daily-sweep/start,不限频)。
 */
export function ReviewEmptyState({ lastSweptAt, onRetrigger }: ReviewEmptyStateProps) {
  return (
    <div className="rounded-lg border border-dashed border-base-300 p-8 text-center space-y-3">
      <p className="text-base-content/80">{t("review.empty.title")}</p>
      {lastSweptAt && (
        <p className="text-sm text-base-content/60">
          {t("review.empty.last", { ts: lastSweptAt })}
        </p>
      )}
      <button className="btn btn-sm btn-primary" onClick={onRetrigger}>
        {t("review.empty.retrigger")}
      </button>
    </div>
  );
}
