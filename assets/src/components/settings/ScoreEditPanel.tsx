/**
 * ScoreEditPanel —— 编辑任务的 4 维调度评分(priority/urgency/energy/context_tag)。
 *
 * 与 useAITasks.patchScores 的区别:
 *  - 本组件是 UI 入口:在 409 时拉一次最新任务,刷新本地图形状态,并展示冲突横幅
 *    让用户决定是否再点保存(不自动重发,避免覆盖用户未看到的远端改动)。
 *  - useAITasks.patchScores 是程序化 API:同样的 409 时有界自动重发 1 次,适用于
 *    非交互场景(scheduler apply 等)。
 *
 * 数据流:输入 current/version;保存成功 → onUpdated(updatedTask);409 → 拉取
 * 最新任务 → 回填本地图形状态 + onUpdated(fresh) + 显示冲突横幅。
 */

import { useState } from "preact/hooks";
import type { TaskDTO } from "../../types/aigtd";
import { getAPIClient } from "../../utils/apiClientSingleton";
import { VersionConflictError } from "../../utils/apiClient";
import { showToast } from "../common/Toast";
import { t } from "../../utils/i18n";

export interface ScoreEditPanelProps {
  taskId: number;
  current: {
    priority_score: number;
    urgency_score: number;
    energy_required: number;
    context_tag: string;
  };
  version: number;
  onUpdated: (next: TaskDTO) => void;
}

const CONTEXT_OPTIONS = ["写作", "编码", "沟通", "学习", "杂事", "general"];

export function ScoreEditPanel(props: ScoreEditPanelProps) {
  const [priority, setPriority] = useState(props.current.priority_score);
  const [urgency, setUrgency] = useState(props.current.urgency_score);
  const [energy, setEnergy] = useState(props.current.energy_required);
  const [context, setContext] = useState(props.current.context_tag);
  const [version, setVersion] = useState(props.version);
  const [saving, setSaving] = useState(false);
  const [conflict, setConflict] = useState<string | null>(null);

  const save = async () => {
    setSaving(true);
    setConflict(null);
    try {
      const api = getAPIClient();
      const updated = await api.patchTaskScores(props.taskId, {
        version,
        priority_score: priority,
        urgency_score: urgency,
        energy_required: energy,
        context_tag: context,
      });
      setVersion(updated.version);
      props.onUpdated(updated);
      showToast(t("score.save_success"), "success");
    } catch (err) {
      if (err instanceof VersionConflictError) {
        // 409:服务端版本已变,拉取最新任务并展示冲突,让用户重新编辑后保存
        try {
          const fresh = await getAPIClient().getTask(props.taskId);
          setVersion(fresh.version);
          setPriority(fresh.priority_score);
          setUrgency(fresh.urgency_score);
          setEnergy(fresh.energy_required);
          setContext(fresh.context_tag);
          setConflict(t("score.conflict"));
          props.onUpdated(fresh);
        } catch {
          setConflict(t("score.conflict"));
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
    <div className="score-edit-panel space-y-3" data-testid="score-edit-panel">
      {conflict && (
        <div className="alert alert-warning" data-testid="conflict-banner" role="alert">
          <span>{conflict}</span>
        </div>
      )}

      <div className="form-control">
        <label className="label">
          <span className="label-text">{t("score.priority")}</span>
          <span className="label-text-alt" data-testid="priority-value">{priority}</span>
        </label>
        <input
          type="range"
          min={1}
          max={5}
          step={1}
          className="range range-primary range-sm"
          value={priority}
          data-testid="priority-input"
          onInput={(e) => setPriority(Number((e.currentTarget as HTMLInputElement).value))}
        />
      </div>

      <div className="form-control">
        <label className="label">
          <span className="label-text">{t("score.urgency")}</span>
          <span className="label-text-alt" data-testid="urgency-value">{urgency}</span>
        </label>
        <input
          type="range"
          min={1}
          max={10}
          step={1}
          className="range range-primary range-sm"
          value={urgency}
          data-testid="urgency-input"
          onInput={(e) => setUrgency(Number((e.currentTarget as HTMLInputElement).value))}
        />
      </div>

      <div className="form-control">
        <label className="label">
          <span className="label-text">{t("score.energy")}</span>
        </label>
        <div className="flex gap-4" data-testid="energy-radio">
          {[1, 2, 3].map((n) => (
            <label key={n} className="flex cursor-pointer items-center gap-2">
              <input
                type="radio"
                name="energy"
                value={n}
                className="radio radio-sm"
                checked={energy === n}
                data-testid={`energy-${n}`}
                onChange={() => setEnergy(n)}
              />
              <span>{n}</span>
            </label>
          ))}
        </div>
      </div>

      <div className="form-control">
        <label className="label">
          <span className="label-text">{t("score.context")}</span>
        </label>
        <select
          className="select select-bordered select-sm"
          value={context}
          data-testid="context-select"
          onChange={(e) => setContext((e.currentTarget as HTMLSelectElement).value)}
        >
          {CONTEXT_OPTIONS.map((opt) => (
            <option key={opt} value={opt}>{opt}</option>
          ))}
        </select>
      </div>

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
