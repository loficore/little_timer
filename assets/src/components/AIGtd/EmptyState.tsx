import { t } from "../../utils/i18n";

export function EmptyState() {
  return (
    <div className="rounded-lg border border-dashed border-base-300 p-8 text-center opacity-60">
      <p>{t("aigtd.empty")}</p>
    </div>
  );
}
