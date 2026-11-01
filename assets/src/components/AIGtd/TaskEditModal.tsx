import { useState } from "preact/hooks";
import type { TaskDTO } from "../../types/aigtd";
import { getAPIClient } from "../../utils/apiClientSingleton";
import { VersionConflictError } from "../../utils/apiClient";
import { t } from "../../utils/i18n";
import { Modal } from "../common/Modal";
import { ScoreFields, type ScoreValues } from "../settings/ScoreFields";

interface TaskEditModalProps {
  task: TaskDTO;
  onClose: () => void;
  /** 保存成功后的回调（列表刷新）；关闭由本组件负责。 */
  onSaved?: () => void;
}

interface Fields {
  title: string;
  notes: string;
  dueDate: string;
}

function fieldsFrom(task: TaskDTO): Fields {
  return {
    title: task.title,
    notes: task.notes ?? "",
    dueDate: task.due_date ?? "",
  };
}

function scoresFrom(task: TaskDTO): ScoreValues {
  return {
    priority_score: task.priority_score,
    urgency_score: task.urgency_score,
    energy_required: task.energy_required,
    context_tag: task.context_tag,
  };
}

/**
 * TaskEditModal —— 编辑标题 / 备注 / 截止日期 + 4 维评分。
 *
 * 壳走共用 Modal（R7）；底部只有一个 primary「保存」：
 *   1. 评分有改动 → patchTaskScores（拿回新 version）；
 *   2. 字段有改动 → 用最新 version 调 updateTask；
 *   3. 都没改 → 直接关闭。
 * 409 时刷新最新任务并展示冲突横幅，不自动重发。
 */
export function TaskEditModal({ task, onClose, onSaved }: TaskEditModalProps) {
  const [current, setCurrent] = useState<TaskDTO>(task);
  const [fields, setFields] = useState<Fields>(fieldsFrom(task));
  const [baseline, setBaseline] = useState<Fields>(fieldsFrom(task));
  const [scores, setScores] = useState<ScoreValues>(scoresFrom(task));
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [conflict, setConflict] = useState<string | null>(null);

  const setField = <K extends keyof Fields>(key: K, value: Fields[K]) =>
    setFields((prev) => ({ ...prev, [key]: value }));

  const fieldsDirty =
    fields.title.trim() !== baseline.title ||
    fields.notes !== baseline.notes ||
    fields.dueDate !== baseline.dueDate;

  const scoresDirty =
    scores.priority_score !== current.priority_score ||
    scores.urgency_score !== current.urgency_score ||
    scores.energy_required !== current.energy_required ||
    scores.context_tag !== current.context_tag;

  const refreshAfterConflict = async () => {
    try {
      const fresh = await getAPIClient().getTask(current.id);
      setCurrent(fresh);
      setFields(fieldsFrom(fresh));
      setBaseline(fieldsFrom(fresh));
      setScores(scoresFrom(fresh));
      setConflict(t("score.conflict"));
    } catch (refreshErr) {
      const reason = refreshErr instanceof Error ? refreshErr.message : String(refreshErr);
      setConflict(t("score.conflict_refresh_failed", { error: reason }));
    }
  };

  const save = async () => {
    if (!fields.title.trim()) return;
    setSaving(true);
    setError(null);
    setConflict(null);
    try {
      const api = getAPIClient();
      let version = current.version;

      if (scoresDirty) {
        const afterScores = await api.patchTaskScores(current.id, { version, ...scores });
        version = afterScores.version;
        setCurrent(afterScores);
      }

      if (fieldsDirty) {
        await api.updateTask(current.id, {
          version,
          title: fields.title.trim(),
          notes: fields.notes,
          due_date: fields.dueDate || null,
        });
      }

      onSaved?.();
      onClose();
    } catch (err) {
      const msg = err instanceof Error ? err.message : String(err);
      if (err instanceof VersionConflictError || msg.includes("HTTP 409")) {
        await refreshAfterConflict();
      } else {
        setError(msg);
      }
    } finally {
      setSaving(false);
    }
  };

  const labelClass = "block space-y-1.5";
  const labelTextClass = "text-sm font-medium text-[var(--my-on-surface)]";

  return (
    <Modal
      title={t("aigtd.edit_title", { id: current.id })}
      onClose={onClose}
      testId="task-edit-modal"
      footer={
        <>
          <button
            type="button"
            className="btn btn-ghost"
            data-testid="task-edit-cancel"
            disabled={saving}
            onClick={onClose}
          >
            {t("button.cancel")}
          </button>
          <button
            type="button"
            className="btn btn-primary"
            data-testid="task-edit-save"
            disabled={saving || !fields.title.trim()}
            onClick={() => void save()}
          >
            {saving ? t("button.saving") : t("button.save")}
          </button>
        </>
      }
    >
      <form
        className="space-y-3"
        onSubmit={(e) => {
          e.preventDefault();
          void save();
        }}
      >
        <section className="space-y-3">
          <h3 className="m-0 text-base font-semibold text-[var(--my-on-surface)]">
            {t("aigtd.section_basic")}
          </h3>

          <label className={labelClass}>
            <span className={labelTextClass}>{t("aigtd.field_title")}</span>
            <input
              className="my-input w-full"
              data-testid="task-edit-title"
              value={fields.title}
              onInput={(e) => setField("title", (e.currentTarget as HTMLInputElement).value)}
            />
          </label>

          <label className={labelClass}>
            <span className={labelTextClass}>{t("aigtd.field_notes")}</span>
            <textarea
              className="my-input min-h-[76px] w-full resize-y"
              rows={3}
              data-testid="task-edit-notes"
              value={fields.notes}
              onInput={(e) => setField("notes", (e.currentTarget as HTMLTextAreaElement).value)}
            />
          </label>

          <label className={labelClass}>
            <span className={labelTextClass}>{t("aigtd.field_due_date")}</span>
            <input
              type="date"
              className="my-input w-full"
              data-testid="task-edit-due-date"
              value={fields.dueDate}
              onInput={(e) => setField("dueDate", (e.currentTarget as HTMLInputElement).value)}
            />
          </label>
        </section>

        <section className="space-y-3">
          <h3 className="m-0 text-base font-semibold text-[var(--my-on-surface)]">
            {t("aigtd.section_scores")}
          </h3>
          <ScoreFields value={scores} onChange={setScores} testId="score-edit-panel" />
        </section>

        {conflict && (
          <div className="alert alert-warning" data-testid="conflict-banner" role="alert">
            <span>{conflict}</span>
          </div>
        )}
        {error && (
          <div className="alert alert-error" data-testid="task-edit-error" role="alert">
            <span>{error}</span>
          </div>
        )}
      </form>
    </Modal>
  );
}
