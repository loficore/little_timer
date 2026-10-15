import { t } from "../../../utils/i18n";

export type ChipKind = "reschedule" | "split" | "rewrite" | "mark_done" | "drop";

interface ReviewChipProps {
  kind: ChipKind;
  active?: boolean;
  disabled?: boolean;
  onClick: () => void;
}

/** 5 chip(改期/拆分/重写/完成/放弃),i18n key `review.chip.<kind>`。 */
export function ReviewChip({ kind, active, disabled, onClick }: ReviewChipProps) {
  return (
    <button
      className={`btn btn-sm ${active ? "btn-primary" : "btn-outline"}`}
      data-testid={`review-chip-${kind}`}
      disabled={disabled}
      onClick={onClick}
    >
      {t(`review.chip.${kind}`)}
    </button>
  );
}
