import { t } from "../../utils/i18n";

/** 任务 4 维调度评分（blocked_by 是第 5 维依赖，不在本面板编辑）。 */
export interface ScoreValues {
  priority_score: number;
  urgency_score: number;
  energy_required: number;
  context_tag: string;
}

export interface ScoreFieldsProps {
  value: ScoreValues;
  onChange: (next: ScoreValues) => void;
  /** 容器 testid（TaskEditModal 沿用 score-edit-panel，E2E 依赖）。 */
  testId?: string;
}

const CONTEXT_OPTIONS = ["写作", "编码", "沟通", "学习", "杂事", "general"];

/**
 * ScoreFields —— 4 维评分的纯受控字段组。
 *
 * 从 ScoreEditPanel 抽出，供两处复用：
 *  - ScoreEditPanel：独立保存（计划页侧栏），自带冲突横幅；
 *  - TaskEditModal：与标题/备注一起走底部单一「保存」。
 * 因此本组件不发请求、不 toast、不渲染保存按钮。
 */
export function ScoreFields({ value, onChange, testId = "score-fields" }: ScoreFieldsProps) {
  const set = <K extends keyof ScoreValues>(key: K, v: ScoreValues[K]) =>
    onChange({ ...value, [key]: v });

  return (
    <div className="space-y-3" data-testid={testId}>
      <div className="space-y-1.5">
        <div className="flex items-baseline justify-between gap-3">
          <span className="text-sm font-medium text-[var(--my-on-surface)]">{t("score.priority")}</span>
          <span
            className="text-sm tabular-nums text-[var(--my-on-surface-variant)]"
            data-testid="priority-value"
          >
            {value.priority_score}
          </span>
        </div>
        <input
          type="range"
          min={1}
          max={5}
          step={1}
          className="range range-primary range-sm w-full"
          value={value.priority_score}
          data-testid="priority-input"
          aria-label={t("score.priority")}
          onInput={(e) => set("priority_score", Number((e.currentTarget as HTMLInputElement).value))}
        />
      </div>

      <div className="space-y-1.5">
        <div className="flex items-baseline justify-between gap-3">
          <span className="text-sm font-medium text-[var(--my-on-surface)]">{t("score.urgency")}</span>
          <span
            className="text-sm tabular-nums text-[var(--my-on-surface-variant)]"
            data-testid="urgency-value"
          >
            {value.urgency_score}
          </span>
        </div>
        <input
          type="range"
          min={1}
          max={10}
          step={1}
          className="range range-primary range-sm w-full"
          value={value.urgency_score}
          data-testid="urgency-input"
          aria-label={t("score.urgency")}
          onInput={(e) => set("urgency_score", Number((e.currentTarget as HTMLInputElement).value))}
        />
      </div>

      <div className="space-y-1.5">
        <span className="text-sm font-medium text-[var(--my-on-surface)]">{t("score.energy")}</span>
        <div className="flex gap-2" role="radiogroup" aria-label={t("score.energy")} data-testid="energy-radio">
          {[1, 2, 3].map((n) => (
            <label
              key={n}
              className={`my-filter-btn h-11 w-11 justify-center text-sm ${
                value.energy_required === n ? "my-filter-btn-active" : ""
              }`}
            >
              <input
                type="radio"
                name="energy"
                value={n}
                className="sr-only"
                checked={value.energy_required === n}
                data-testid={`energy-${n}`}
                onChange={() => set("energy_required", n)}
              />
              <span>{n}</span>
            </label>
          ))}
        </div>
      </div>

      <div className="space-y-1.5">
        <label className="block text-sm font-medium text-[var(--my-on-surface)]">
          {t("score.context")}
        </label>
        <select
          className="my-select w-full"
          value={value.context_tag}
          data-testid="context-select"
          onChange={(e) => set("context_tag", (e.currentTarget as HTMLSelectElement).value)}
        >
          {CONTEXT_OPTIONS.map((opt) => (
            <option key={opt} value={opt}>{opt}</option>
          ))}
        </select>
      </div>
    </div>
  );
}
