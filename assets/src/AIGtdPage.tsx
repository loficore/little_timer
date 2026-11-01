import { useState } from "preact/hooks";
import { CaptureBox } from "./components/AIGtd/CaptureBox";
import { TaskList } from "./components/AIGtd/TaskList";
import { TaskEditModal } from "./components/AIGtd/TaskEditModal";
import { EmptyState } from "./components/AIGtd/EmptyState";
import { useAITasks } from "./hooks/useAITasks";
import { t } from "./utils/i18n";
import type { TaskDTO } from "./types/aigtd";

export function AIGtdPage() {
  const { tasks, refresh, capture, archive, toggleSubtask, reparse } = useAITasks();
  const [editing, setEditing] = useState<TaskDTO | null>(null);

  return (
    <div className="flex-1 overflow-y-auto">
      <div className="container mx-auto max-w-4xl space-y-4 p-4">
        <h1 className="text-2xl font-bold">{t("aigtd.title")}</h1>
        <CaptureBox onCapture={capture} />
        {tasks.length === 0 ? (
          <EmptyState />
        ) : (
          <TaskList
            tasks={tasks}
            onArchive={(id) => void archive(id)}
            onReparse={(id) => void reparse(id)}
            onToggleSubtask={(taskId, subId, done) => void toggleSubtask(taskId, subId, done)}
            onEdit={setEditing}
          />
        )}
        {editing && (
          <TaskEditModal task={editing} onClose={() => setEditing(null)} onSaved={() => void refresh()} />
        )}
      </div>
    </div>
  );
}
