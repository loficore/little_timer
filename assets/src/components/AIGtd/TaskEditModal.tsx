import { useEffect, useState } from "preact/hooks";
import type { TaskDTO } from "../../types/aigtd";
import { getAPIClient } from "../../utils/apiClientSingleton";
import { ScoreEditPanel } from "../settings/ScoreEditPanel";

interface TaskEditModalProps {
  task: TaskDTO;
  onClose: () => void;
}

/**
 * TaskEditModal —— 编辑 title / notes / due_date,并嵌入 ScoreEditPanel
 * 提供 4 维评分编辑。title/notes/due_date 走 updateTask(task.version, ...);
 * 评分独立走 patchTaskScores 与乐观锁;两路都在保存时把 version 带回去。
 *
 * 任务的实时版本号用 localCurrent.version 持有;onScoresUpdated 把 patch 后
 * 的最新任务回流,保证 score 路径的 version 与 next save 的 version 一致。
 */
export function TaskEditModal({ task, onClose }: TaskEditModalProps) {
  const [title, setTitle] = useState(task.title);
  const [notes, setNotes] = useState(task.notes ?? "");
  const [dueDate, setDueDate] = useState(task.due_date ?? "");
  const [current, setCurrent] = useState<TaskDTO>(task);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [onClose]);

  const save = async () => {
    setSaving(true);
    try {
      await getAPIClient().updateTask(current.id, {
        version: current.version,
        title,
        notes,
        due_date: dueDate || null,
      });
      onClose();
    } finally {
      setSaving(false);
    }
  };

  return (
    <dialog className="modal modal-open">
      <div className="modal-box">
        <h3 className="font-bold">编辑 #{current.id}</h3>
        <div className="form-control mt-3">
          <label className="label">
            <span className="label-text">title</span>
          </label>
          <input className="input input-bordered" value={title} onInput={(e) => setTitle((e.currentTarget as HTMLInputElement).value)} />
        </div>
        <div className="form-control mt-2">
          <label className="label">
            <span className="label-text">notes</span>
          </label>
          <textarea className="textarea textarea-bordered" value={notes} onInput={(e) => setNotes((e.currentTarget as HTMLTextAreaElement).value)} rows={3} />
        </div>
        <div className="form-control mt-2">
          <label className="label">
            <span className="label-text">due_date</span>
          </label>
          <input type="date" className="input input-bordered" value={dueDate} onInput={(e) => setDueDate((e.currentTarget as HTMLInputElement).value)} />
        </div>
        <div className="divider mt-4">5 维评分</div>
        <ScoreEditPanel
          taskId={current.id}
          current={{
            priority_score: current.priority_score,
            urgency_score: current.urgency_score,
            energy_required: current.energy_required,
            context_tag: current.context_tag,
          }}
          version={current.version}
          onUpdated={(next) => setCurrent(next)}
        />
        <div className="modal-action">
          <button className="btn btn-ghost" onClick={onClose} disabled={saving}>取消</button>
          <button className="btn btn-primary" onClick={() => void save()} disabled={saving || !title.trim()}>
            {saving ? "保存中..." : "保存"}
          </button>
        </div>
      </div>
      <div className="modal-backdrop" onClick={onClose} />
    </dialog>
  );
}
