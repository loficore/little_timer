import { t } from "../../utils/i18n";
import type { TaskDTO } from "../../types/aigtd";

interface TaskCardProps {
  task: TaskDTO;
  onArchive: (id: number) => void;
  onReparse: (id: number) => void;
  onToggleSubtask: (taskId: number, subId: number, done: boolean) => void;
  onEdit: (task: TaskDTO) => void;
}

function statusBadge(task: TaskDTO) {
  if (task.ai_status === "pending" || task.ai_status === "processing") {
    return (
      <span className="badge badge-info badge-sm" data-testid="ai-spinner">
        {t("aigtd.status_parsing")}
      </span>
    );
  }
  if (task.ai_status === "error") {
    return <span className="badge badge-error badge-sm">{t("aigtd.status_error")}</span>;
  }
  if (task.status === "inbox") {
    return <span className="badge badge-ghost badge-sm">{t("aigtd.status_inbox")}</span>;
  }
  return <span className="badge badge-success badge-sm">{t("aigtd.status_parsed")}</span>;
}

/**
 * TaskCard —— 单条任务卡片(pending/processing/done/error 四态)。
 */
export function TaskCard({ task, onArchive, onReparse, onToggleSubtask, onEdit }: TaskCardProps) {
  const subtasks = task.subtasks ?? [];

  return (
    <div className="card card-border bg-base-100" data-testid={`task-card-${task.id}`}>
      <div className="card-body gap-2 p-3">
        <div className="flex items-start justify-between gap-2">
          <div className="min-w-0 flex-1">
            <h3 className="truncate font-semibold">{task.title || task.raw_text}</h3>
            {task.raw_text && task.raw_text !== task.title && (
              <p className="truncate text-xs opacity-60">{task.raw_text}</p>
            )}
          </div>
          {statusBadge(task)}
        </div>

        {subtasks.length > 0 && (
          <ul className="space-y-1">
            {subtasks.map((s) => (
              <li key={s.id} className="flex items-center gap-2">
                <input
                  type="checkbox"
                  className="checkbox checkbox-sm"
                  checked={s.status === "done"}
                  onChange={(e) =>
                    onToggleSubtask(task.id, s.id, (e.currentTarget as HTMLInputElement).checked)
                  }
                />
                <span className={s.status === "done" ? "line-through opacity-50" : ""}>
                  {s.title}
                </span>
                {s.estimated_minutes > 0 && (
                  <span className="ml-auto text-xs opacity-50">{s.estimated_minutes}m</span>
                )}
              </li>
            ))}
          </ul>
        )}

        {task.ai_status === "error" && (
          <div className="rounded bg-error/10 p-2 text-sm text-error">
            <span>{t("aigtd.parse_failed_title")}</span>
            {task.ai_error && <span className="ml-1 opacity-70">({task.ai_error})</span>}
          </div>
        )}

        <div className="card-actions justify-end">
          {task.ai_status === "error" && (
            <button
              className="btn btn-xs btn-warning"
              onClick={() => onReparse(task.id)}
            >
              {t("aigtd.action_reparse")}
            </button>
          )}
          <button className="btn btn-xs btn-ghost" onClick={() => onEdit(task)}>
            {t("aigtd.action_edit")}
          </button>
          <button className="btn btn-xs btn-ghost" onClick={() => onArchive(task.id)}>
            {t("aigtd.action_archive")}
          </button>
        </div>
      </div>
    </div>
  );
}
