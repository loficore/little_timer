import { useState } from "preact/hooks";
import type { FunctionalComponent } from "preact";
import { APIClient } from "../utils/apiClient";

interface HabitModalProps {
  isOpen: boolean;
  mode: "set" | "habit";
  setId?: number;
  onClose: () => void;
  onSuccess: () => void;
}

const COLORS = [
  "#6366f1", // indigo
  "#8b5cf6", // violet
  "#ec4899", // pink
  "#ef4444", // red
  "#f97316", // orange
  "#eab308", // yellow
  "#22c55e", // green
  "#14b8a6", // teal
  "#0ea5e9", // sky
  "#3b82f6", // blue
];

export const HabitModal: FunctionalComponent<HabitModalProps> = ({
  isOpen,
  mode,
  setId,
  onClose,
  onSuccess,
}) => {
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [color, setColor] = useState(COLORS[0]);
  const [goalMinutes, setGoalMinutes] = useState(25);
  const [isSubmitting, setIsSubmitting] = useState(false);

  if (!isOpen) return null;

  const handleSubmit = async (e: Event) => {
    e.preventDefault();
    if (!name.trim()) return;

    setIsSubmitting(true);
    try {
      const client = new APIClient(window.location.origin);
      
      if (mode === "set") {
        await client.createHabitSet(name, description, color);
      } else {
        await client.createHabit(setId!, name, goalMinutes * 60, 1, color);
      }
      
      onSuccess();
      setName("");
      setDescription("");
      setColor(COLORS[0]);
      setGoalMinutes(25);
    } catch (err) {
      console.error("Failed to create:", err);
    } finally {
      setIsSubmitting(false);
    }
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center">
      <div className="absolute inset-0 bg-black/50" onClick={onClose} />
      <div className="relative bg-base-100 rounded-lg p-6 w-full max-w-sm mx-4 shadow-xl">
        <h3 className="text-lg font-bold mb-4">
          {mode === "set" ? "创建习惯集" : "添加习惯"}
        </h3>
        
        <form onSubmit={(e) => { void handleSubmit(e); }}>
          <div className="form-control mb-4">
            <label className="label">
              <span className="label-text">名称</span>
            </label>
            <input
              type="text"
              className="input input-bordered"
              placeholder={mode === "set" ? "如：学习习惯" : "如：背单词"}
              value={name}
              onInput={(e) => setName((e.target as HTMLInputElement).value)}
              required
            />
          </div>

          {mode === "set" && (
            <div className="form-control mb-4">
              <label className="label">
                <span className="label-text">描述（可选）</span>
              </label>
              <textarea
                className="textarea textarea-bordered"
                placeholder="简单描述这个习惯集..."
                value={description}
                onInput={(e) => setDescription((e.target as HTMLTextAreaElement).value)}
              />
            </div>
          )}

          {mode === "habit" && (
            <div className="form-control mb-4">
              <label className="label">
                <span className="label-text">目标时间（分钟）</span>
              </label>
              <input
                type="number"
                className="input input-bordered"
                min={1}
                max={180}
                value={goalMinutes}
                onInput={(e) => setGoalMinutes(parseInt((e.target as HTMLInputElement).value) || 25)}
              />
            </div>
          )}

          <div className="form-control mb-6">
            <label className="label">
              <span className="label-text">颜色</span>
            </label>
            <div className="flex gap-2 flex-wrap">
              {COLORS.map((c) => (
                <button
                  key={c}
                  type="button"
                  className={`w-8 h-8 rounded-full ${color === c ? "ring-2 ring-offset-2 ring-base-content" : ""}`}
                  style={{ backgroundColor: c }}
                  onClick={() => setColor(c)}
                />
              ))}
            </div>
          </div>

          <div className="flex gap-2">
            <button
              type="button"
              className="btn btn-ghost flex-1"
              onClick={onClose}
            >
              取消
            </button>
            <button
              type="submit"
              className="btn btn-primary flex-1"
              disabled={isSubmitting || !name.trim()}
            >
              {isSubmitting ? "创建中..." : "创建"}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
};
