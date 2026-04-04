import { useState, useEffect } from "preact/hooks";
import type { FunctionalComponent } from "preact";
import { APIClient } from "../utils/apiClient";
import type { HabitSet, Habit } from "../types/habit";
import { WallpaperSelector } from "./WallpaperSelector";

interface HabitModalProps {
  isOpen: boolean;
  mode: "set" | "habit";
  editData?: HabitSet | Habit | null;
  setId?: number;
  onClose: () => void;
  onSuccess: () => void;
}

const COLORS = [
  "#6366f1",
  "#8b5cf6",
  "#ec4899",
  "#ef4444",
  "#f97316",
  "#eab308",
  "#22c55e",
  "#14b8a6",
  "#0ea5e9",
  "#3b82f6",
];

export const HabitModal: FunctionalComponent<HabitModalProps> = ({
  isOpen,
  mode,
  editData,
  setId,
  onClose,
  onSuccess,
}) => {
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [color, setColor] = useState(COLORS[0]);
  const [wallpaper, setWallpaper] = useState("");
  const [goalHours, setGoalHours] = useState(0);
  const [goalMinutes, setGoalMinutes] = useState(25);
  const [isSubmitting, setIsSubmitting] = useState(false);

  const isEdit = !!editData;
  const isHabitEdit = isEdit && mode === "habit";
  const isSetEdit = isEdit && mode === "set";

  useEffect(() => {
    if (editData) {
      setName(editData.name);
      setColor(editData.color || COLORS[0]);
      const wallpaperValue = (editData as { wallpaper?: string }).wallpaper;
      setWallpaper(wallpaperValue || "");
      if ("description" in editData) {
        setDescription(editData.description || "");
      }
      if ("goal_seconds" in editData) {
        const total = editData.goal_seconds || 1500;
        setGoalHours(Math.floor(total / 3600));
        setGoalMinutes(Math.floor((total % 3600) / 60));
      }
    } else {
      setName("");
      setDescription("");
      setColor(COLORS[0]);
      setWallpaper("");
      setGoalHours(0);
      setGoalMinutes(25);
    }
  }, [editData, isOpen]);

  if (!isOpen) return null;

  const handleSubmit = async (e: Event) => {
    e.preventDefault();
    if (!name.trim()) return;

    const totalSeconds = (goalHours * 60 + goalMinutes) * 60;
    if (mode === "habit" && totalSeconds === 0) return;

    setIsSubmitting(true);
    try {
      const client = new APIClient(window.location.origin);
      
      if (mode === "set") {
        if (isSetEdit) {
          await client.updateHabitSet(editData.id, name, description, color, wallpaper);
        } else {
          await client.createHabitSet(name, description, color);
        }
      } else {
        if (isHabitEdit) {
          await client.updateHabit(editData.id, name, totalSeconds, color, wallpaper);
        } else {
          await client.createHabit(setId!, name, totalSeconds, color);
        }
      }
      
      onSuccess();
      setName("");
      setDescription("");
      setColor(COLORS[0]);
      setWallpaper("");
      setGoalHours(0);
      setGoalMinutes(25);
    } catch (err) {
      console.error("Failed to save:", err);
    } finally {
      setIsSubmitting(false);
    }
  };

  const getTitle = () => {
    if (mode === "set") {
      return isSetEdit ? "编辑习惯集" : "创建习惯集";
    }
    return isHabitEdit ? "编辑习惯" : "添加习惯";
  };

  const getSubmitText = () => {
    if (isSubmitting) return "保存中...";
    return isEdit ? "保存" : "创建";
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center">
      <div className="absolute inset-0 bg-black/50" onClick={onClose} />
      <div className="relative bg-base-100 rounded-lg p-6 w-full max-w-sm mx-4 shadow-xl">
        <h3 className="text-lg font-bold mb-4">{getTitle()}</h3>
        
        <form onSubmit={(e) => { void handleSubmit(e); }}>
          <div className="form-control mb-4">
            <label className="label">
              <span className="label-text">名称</span>
            </label>
            <input
              type="text"
              className="my-input"
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
                className="my-input min-h-[80px] resize-none"
                placeholder="简单描述这个习惯集..."
                value={description}
                onInput={(e) => setDescription((e.target as HTMLTextAreaElement).value)}
              />
            </div>
          )}

          {mode === "habit" && (
            <div className="form-control mb-4">
              <label className="label">
                <span className="label-text">目标时长</span>
              </label>
              <div className="flex gap-2 items-center">
                <input
                  type="number"
                  className="my-input w-20"
                  min={0}
                  max={9999}
                  value={goalHours}
                  onInput={(e) => setGoalHours(parseInt((e.target as HTMLInputElement).value) || 0)}
                />
                <span className="text-sm">小时</span>
                <input
                  type="number"
                  className="my-input w-20"
                  min={0}
                  max={59}
                  value={goalMinutes}
                  onInput={(e) => setGoalMinutes(parseInt((e.target as HTMLInputElement).value) || 0)}
                />
                <span className="text-sm">分钟</span>
              </div>
              <label className="label">
                <span className="label-text-alt text-error">
                  {(goalHours * 60 + goalMinutes) === 0 ? "请设置目标时长" : `目标: ${goalHours}h ${goalMinutes}m = ${(goalHours * 60 + goalMinutes)} 分钟`}
                </span>
              </label>
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

          <WallpaperSelector value={wallpaper} onChange={setWallpaper} />

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
              disabled={isSubmitting || !name.trim() || (mode === "habit" && (goalHours * 60 + goalMinutes) === 0)}
            >
              {getSubmitText()}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
};
