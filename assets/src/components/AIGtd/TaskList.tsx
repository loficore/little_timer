import { useState } from "preact/hooks";
import type { TaskDTO } from "../../types/aigtd";
import { TaskCard } from "./TaskCard";

interface TaskListProps {
  tasks: TaskDTO[];
  onArchive: (id: number) => void;
  onReparse: (id: number) => void;
  onToggleSubtask: (taskId: number, subId: number, done: boolean) => void;
  onEdit: (task: TaskDTO) => void;
}

type Tab = "all" | "processing" | "parsed";

/**
 * TaskList —— 顶层任务列表(隐藏 archived/rejected,后端 ListTopLevel 默认行为)。
 */
export function TaskList({ tasks, onArchive, onReparse, onToggleSubtask, onEdit }: TaskListProps) {
  const [tab, setTab] = useState<Tab>("all");

  const filtered = tasks.filter((t) => {
    if (tab === "processing") return t.ai_status === "pending" || t.ai_status === "processing";
    if (tab === "parsed") return t.ai_status === "done" && (t.subtasks?.length ?? 0) > 0;
    return true;
  });

  if (tasks.length === 0) return null;

  return (
    <div className="space-y-2">
      <div role="tablist" className="tabs tabs-bordered">
        <button
          role="tab"
          className={`tab ${tab === "all" ? "tab-active" : ""}`}
          onClick={() => setTab("all")}
        >
          All ({tasks.length})
        </button>
        <button
          role="tab"
          className={`tab ${tab === "processing" ? "tab-active" : ""}`}
          onClick={() => setTab("processing")}
        >
          AI ({tasks.filter((t) => t.ai_status === "pending" || t.ai_status === "processing").length})
        </button>
        <button
          role="tab"
          className={`tab ${tab === "parsed" ? "tab-active" : ""}`}
          onClick={() => setTab("parsed")}
        >
          Parsed ({tasks.filter((t) => t.ai_status === "done" && (t.subtasks?.length ?? 0) > 0).length})
        </button>
      </div>

      {filtered.length === 0 ? (
        <p className="py-4 text-center opacity-60">—</p>
      ) : (
        filtered.map((t) => (
          <TaskCard
            key={t.id}
            task={t}
            onArchive={onArchive}
            onReparse={onReparse}
            onToggleSubtask={onToggleSubtask}
            onEdit={onEdit}
          />
        ))
      )}
    </div>
  );
}
