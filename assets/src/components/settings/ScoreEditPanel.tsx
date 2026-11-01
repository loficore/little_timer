/**
 * ScoreEditPanel —— 独立保存的 4 维调度评分面板（计划页侧栏使用）。
 *
 * 字段本身由 ScoreFields 渲染；本组件负责「保存 + 409 处理 + toast」：
 *  - 409：拉一次最新任务，刷新本地状态并展示冲突横幅（不自动重发，避免覆盖
 *    用户没看到的远端改动）；
 *  - 刷新也失败：保留用户编辑，只展示带原因的冲突提示。
 *
 * TaskEditModal 不经过本组件，它直接用 ScoreFields + 底部统一保存。
 */

import { useState } from "preact/hooks";
import type { TaskDTO } from "../../types/aigtd";
import { getAPIClient } from "../../utils/apiClientSingleton";
import { VersionConflictError } from "../../utils/apiClient";
import { showToast } from "../common/Toast";
import { t } from "../../utils/i18n";
import { ScoreFields, type ScoreValues } from "./ScoreFields";

export interface ScoreEditPanelProps {
  taskId: number;
  current: ScoreValues;
  version: number;
  onUpdated: (next: TaskDTO) => void;
}

export function ScoreEditPanel(props: ScoreEditPanelProps) {
  const [values, setValues] = useState<ScoreValues>({ ...props.current });
  const [version, setVersion] = useState(props.version);
  const [saving, setSaving] = useState(false);
  const [conflict, setConflict] = useState<string | null>(null);

  const save = async () => {
    setSaving(true);
    setConflict(null);
    try {
      const api = getAPIClient();
      const updated = await api.patchTaskScores(props.taskId, { version, ...values });
      setVersion(updated.version);
      props.onUpdated(updated);
      showToast(t("score.save_success"), "success");
    } catch (err) {
      if (err instanceof VersionConflictError) {
        try {
          const fresh = await getAPIClient().getTask(props.taskId);
          setVersion(fresh.version);
          setValues({
            priority_score: fresh.priority_score,
            urgency_score: fresh.urgency_score,
            energy_required: fresh.energy_required,
            context_tag: fresh.context_tag,
          });
          setConflict(t("score.conflict"));
          props.onUpdated(fresh);
        } catch (refreshErr) {
          const reason = refreshErr instanceof Error ? refreshErr.message : String(refreshErr);
          setConflict(t("score.conflict_refresh_failed", { error: reason }));
        }
      } else {
        const msg = err instanceof Error ? err.message : String(err);
        showToast(t("score.save_fail") + (msg ? `: ${msg}` : ""), "error");
      }
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="space-y-4" data-testid="score-edit-panel">
      {conflict && (
        <div className="alert alert-warning" data-testid="conflict-banner" role="alert">
          <span>{conflict}</span>
        </div>
      )}

      <ScoreFields value={values} onChange={setValues} testId="score-fields" />

      <div className="flex justify-end">
        <button
          type="button"
          className="btn btn-primary btn-sm"
          data-testid="save-scores"
          disabled={saving}
          onClick={() => void save()}
        >
          {saving ? t("button.saving") : t("score.save")}
        </button>
      </div>
    </div>
  );
}
