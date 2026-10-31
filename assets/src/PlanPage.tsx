/**
 * PlanPage —— 「计划」页：调度计划 + 每日回顾合并后的单页双 tab 容器。
 *
 * 顶栏只放页名(R5)，两段内容用既有 .my-tabs 控件族切换，
 * 各面板的标题由 tab 标签承担，不再各自渲染 h1。
 */
import { useState } from "preact/hooks";
import { SchedulePage } from "./SchedulePage";
import { DailySweepPage } from "./DailySweepPage";
import { t } from "./utils/i18n";

type PlanTab = "schedule" | "review";

export function PlanPage() {
  const [tab, setTab] = useState<PlanTab>("schedule");

  return (
    <div className="flex flex-col flex-1 bg-transparent overflow-hidden">
      <header className="navbar my-topbar shrink-0 px-2 sm:px-3">
        <div className="flex-1 flex justify-center">
          <span className="my-topbar-title text-xl font-bold">{t("plan.title")}</span>
        </div>
      </header>

      <div className="flex-1 overflow-y-auto px-4 sm:px-6 py-4">
        <div className="container mx-auto max-w-6xl">
          <div className="my-tabs mb-4">
            <button
              type="button"
              data-testid="plan-tab-schedule"
              className={`my-tab ${tab === "schedule" ? "my-tab-active" : ""}`}
              onClick={() => setTab("schedule")}
            >
              {t("plan.tab_schedule")}
            </button>
            <button
              type="button"
              data-testid="plan-tab-review"
              className={`my-tab ${tab === "review" ? "my-tab-active" : ""}`}
              onClick={() => setTab("review")}
            >
              {t("plan.tab_review")}
            </button>
          </div>

          {tab === "schedule" ? <SchedulePage /> : <DailySweepPage />}
        </div>
      </div>
    </div>
  );
}
