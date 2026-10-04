import { useEffect, useState } from "preact/hooks";
import type { TaskDTO } from "../../types/aigtd";
import { getAPIClient } from "../../utils/apiClientSingleton";

interface TaskEditModalProps {
  task: TaskDTO;
  onClose: () => void;
}

/**
 * TaskEditModal —— 编辑 title / notes / due_date。
 */
export function TaskEditModal({ task, onClose }: TaskEditModalProps) {
  const [title, setTitle] = useState(task.title);
  const [notes, setNotes] = useState(task.notes ?? "");
  const [dueDate, setDueDate] = useState(task.due_date ?? "");
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
      await getAPIClient().updateTask(task.id, {
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
        <h3 className="font-bold">编辑 #{task.id}</h3>
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
