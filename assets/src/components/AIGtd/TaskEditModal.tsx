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
  estimatedMinutes: number;
}

/** 编辑中的步骤：有 id 的是已有子任务，没有 id 的是本次新增。 */
interface DraftSubtask {
  id?: number;
  version?: number;
  title: string;
  minutes: number;
}

function fieldsFrom(task: TaskDTO): Fields {
  return {
    title: task.title,
    notes: task.notes ?? "",
    dueDate: task.due_date ?? "",
    estimatedMinutes: task.estimated_minutes ?? 0,
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

function subtasksFrom(task: TaskDTO): DraftSubtask[] {
  return (task.subtasks ?? []).map((s) => ({
    id: s.id,
    version: s.version,
    title: s.title,
    minutes: s.estimated_minutes,
  }));
}

function CloseIcon() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2} className="h-4 w-4" aria-hidden="true">
      <path strokeLinecap="round" strokeLinejoin="round" d="M6 18 18 6M6 6l12 12" />
    </svg>
  );
}

/**
 * TaskEditModal —— 编辑 AI 拆解结果本身：标题 / 备注 / 截止日期 / 预计时长、
 * 拆解步骤（增删改）以及 4 维评分。
 *
 * 底部只有一个 primary「保存」，按：
 *   删除步骤 → 修改步骤（子任务自己的 version）→ 新增步骤 →
 *   评分 patch（拿新 version）→ 任务字段 updateTask
 * 的顺序落库；409 时刷新最新任务并展示冲突横幅，不自动重发。
 */
export function TaskEditModal({ task, onClose, onSaved }: TaskEditModalProps) {
  const [current, setCurrent] = useState<TaskDTO>(task);
  const [fields, setFields] = useState<Fields>(fieldsFrom(task));
  const [baseline, setBaseline] = useState<Fields>(fieldsFrom(task));
  const [scores, setScores] = useState<ScoreValues>(scoresFrom(task));
  const [subtasks, setSubtasks] = useState<DraftSubtask[]>(subtasksFrom(task));
  const [subtaskBaseline, setSubtaskBaseline] = useState<DraftSubtask[]>(subtasksFrom(task));
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [conflict, setConflict] = useState<string | null>(null);

  const setField = <K extends keyof Fields>(key: K, value: Fields[K]) =>
    setFields((prev) => ({ ...prev, [key]: value }));

  const fieldsDirty =
    fields.title.trim() !== baseline.title ||
    fields.notes !== baseline.notes ||
    fields.dueDate !== baseline.dueDate ||
    fields.estimatedMinutes !== baseline.estimatedMinutes;

  const scoresDirty =
    scores.priority_score !== current.priority_score ||
    scores.urgency_score !== current.urgency_score ||
    scores.energy_required !== current.energy_required ||
    scores.context_tag !== current.context_tag;

  const baseById = new Map(subtaskBaseline.filter((s) => s.id !== undefined).map((s) => [s.id!, s]));
  const deletedSubtasks = subtaskBaseline.filter(
    (s) => s.id !== undefined && !subtasks.some((row) => row.id === s.id),
  );
  const changedSubtasks = subtasks.filter((row) => {
    if (row.id === undefined) return false;
    const base = baseById.get(row.id);
    return !!base && (base.title !== row.title.trim() || base.minutes !== row.minutes);
  });
  const createdSubtasks = subtasks.filter((row) => row.id === undefined && row.title.trim() !== "");
  const blankExisting = subtasks.some((row) => row.id !== undefined && row.title.trim() === "");


  const refreshAfterConflict = async () => {
    try {
      const fresh = await getAPIClient().getTask(current.id);
      setCurrent(fresh);
      setFields(fieldsFrom(fresh));
      setBaseline(fieldsFrom(fresh));
      setScores(scoresFrom(fresh));
      setSubtasks(subtasksFrom(fresh));
      setSubtaskBaseline(subtasksFrom(fresh));
      setConflict(t("score.conflict"));
    } catch (refreshErr) {
      const reason = refreshErr instanceof Error ? refreshErr.message : String(refreshErr);
      setConflict(t("score.conflict_refresh_failed", { error: reason }));
    }
  };

  const save = async () => {
    if (!fields.title.trim() || blankExisting) return;
    setSaving(true);
    setError(null);
    setConflict(null);
    try {
      const api = getAPIClient();
      let version = current.version;

      // 1) 删除被移除的步骤
      for (const sub of deletedSubtasks) {
        await api.deleteSubtask(current.id, sub.id!, sub.version ?? 0);
      }

      // 2) 修改已有步骤（子任务自己的 version）
      for (const row of changedSubtasks) {
        const base = baseById.get(row.id!)!;
        const patch: { title?: string; estimated_minutes?: number } = {};
        if (row.title.trim() !== base.title) patch.title = row.title.trim();
        if (row.minutes !== base.minutes) patch.estimated_minutes = row.minutes;
        await api.updateSubtask(current.id, row.id!, patch, row.version ?? 0);
        row.version = (row.version ?? 0) + 1;
      }

      // 3) 新增步骤（父任务 version 只用于通过 ConflictGuard）
      const existingCount = subtasks.filter((row) => row.id !== undefined).length;
      for (let i = 0; i < createdSubtasks.length; i++) {
        const row = createdSubtasks[i];
        const created = await api.createSubtask(
          current.id,
          {
            title: row.title.trim(),
            estimated_minutes: row.minutes,
            order_index: existingCount + i,
          },
          version,
        );
        row.id = created.id;
        row.version = created.version;
      }

      // 4) 评分（会 bump 父任务 version）
      if (scoresDirty) {
        const afterScores = await api.patchTaskScores(current.id, { version, ...scores });
        version = afterScores.version;
        setCurrent(afterScores);
      }

      // 5) 任务字段
      if (fieldsDirty) {
        await api.updateTask(current.id, {
          version,
          title: fields.title.trim(),
          notes: fields.notes,
          due_date: fields.dueDate || null,
          estimated_minutes: fields.estimatedMinutes,
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
            disabled={saving || !fields.title.trim() || blankExisting}
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

          <div className="grid grid-cols-2 gap-3">
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
            <label className={labelClass}>
              <span className={labelTextClass}>{t("aigtd.field_estimated_minutes")}</span>
              <input
                type="number"
                min={0}
                step={5}
                className="my-input w-full"
                data-testid="task-edit-estimated-minutes"
                value={fields.estimatedMinutes}
                onInput={(e) =>
                  setField(
                    "estimatedMinutes",
                    Math.max(0, Number((e.currentTarget as HTMLInputElement).value) || 0),
                  )
                }
              />
            </label>
          </div>
        </section>

        <section className="space-y-3">
          <h3 className="m-0 text-base font-semibold text-[var(--my-on-surface)]">
            {t("aigtd.section_subtasks")}
          </h3>

          {subtasks.length === 0 && (
            <p className="m-0 text-sm text-[var(--my-on-surface-variant)]" data-testid="subtasks-empty">
              {t("aigtd.subtasks_empty")}
            </p>
          )}

          <ul className="space-y-2">
            {subtasks.map((row, index) => {
              const newIndex = subtasks.slice(0, index).filter((s) => s.id === undefined).length;
              const key = row.id !== undefined ? String(row.id) : `new-${newIndex}`;
              return (
                <li key={key} className="flex items-center gap-2">
                  <input
                    className="my-input w-full flex-1"
                    aria-label={t("aigtd.subtask_title")}
                    data-testid={`subtask-title-${key}`}
                    value={row.title}
                    onInput={(e) => {
                      const value = (e.currentTarget as HTMLInputElement).value;
                      setSubtasks((cur) =>
                        cur.map((s) => (s === row ? { ...s, title: value } : s)),
                      );
                    }}
                  />
                  <input
                    type="number"
                    min={0}
                    step={5}
                    className="my-input w-20"
                    aria-label={t("aigtd.subtask_minutes")}
                    data-testid={`subtask-minutes-${key}`}
                    value={row.minutes}
                    onInput={(e) => {
                      const value = Math.max(0, Number((e.currentTarget as HTMLInputElement).value) || 0);
                      setSubtasks((cur) =>
                        cur.map((s) => (s === row ? { ...s, minutes: value } : s)),
                      );
                    }}
                  />
                  <button
                    type="button"
                    className="inline-flex h-11 w-11 shrink-0 items-center justify-center rounded-xl text-[var(--my-on-surface-variant)] hover:text-error"
                    aria-label={t("aigtd.subtask_delete")}
                    data-testid={`subtask-delete-${key}`}
                    onClick={() => {
                      setSubtasks((cur) => cur.filter((s) => s !== row));
                    }}
                  >
                    <CloseIcon />
                  </button>
                </li>
              );
            })}
          </ul>

          <button
            type="button"
            className="btn btn-sm btn-ghost"
            data-testid="subtask-add"
            onClick={() => {
              setSubtasks((cur) => [...cur, { title: "", minutes: 15 }]);
            }}
          >
            + {t("aigtd.subtask_add")}
          </button>

          {blankExisting && (
            <p className="m-0 text-sm text-error" data-testid="subtask-invalid" role="alert">
              {t("aigtd.subtask_invalid")}
            </p>
          )}
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
