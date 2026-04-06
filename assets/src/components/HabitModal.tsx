import { useState, useEffect } from "preact/hooks";
import type { FunctionalComponent } from "preact";
import { APIClient } from "../utils/apiClient";
import type { HabitSet, Habit } from "../types/habit";
import { WallpaperSelector } from "./WallpaperSelector";
import { PickerNumberInput } from "./PickerNumberInput";
import { t } from "../utils/i18n";

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

const normalize_hex = (value: string): string | null => {
  const trimmed = value.trim().toLowerCase();
  const match = trimmed.match(/^#?([0-9a-f]{3}|[0-9a-f]{6})$/i);
  if (!match) return null;
  const hex = match[1];
  if (hex.length === 3) {
    return `#${hex[0]}${hex[0]}${hex[1]}${hex[1]}${hex[2]}${hex[2]}`;
  }
  return `#${hex}`;
};

const normalize_rgb_to_hex = (value: string): string | null => {
  const trimmed = value.trim().toLowerCase();
  const rgb_match = trimmed.match(/^rgb\(\s*(\d{1,3})\s*,\s*(\d{1,3})\s*,\s*(\d{1,3})\s*\)$/i);
  if (!rgb_match) return null;
  const r = Number(rgb_match[1]);
  const g = Number(rgb_match[2]);
  const b = Number(rgb_match[3]);
  if ([r, g, b].some((n) => Number.isNaN(n) || n < 0 || n > 255)) return null;
  const to_hex = (n: number) => n.toString(16).padStart(2, "0");
  return `#${to_hex(r)}${to_hex(g)}${to_hex(b)}`;
};

const normalize_color_value = (value: string): string | null => {
  return normalize_hex(value) ?? normalize_rgb_to_hex(value);
};

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
  const [colorInput, setColorInput] = useState(COLORS[0]);
  const [colorInputError, setColorInputError] = useState("");
  const [isSubmitting, setIsSubmitting] = useState(false);

  const isEdit = !!editData;
  const isHabitEdit = isEdit && mode === "habit";
  const isSetEdit = isEdit && mode === "set";

  useEffect(() => {
    if (editData) {
      setName(editData.name);
      setColor(editData.color || COLORS[0]);
      setColorInput(editData.color || COLORS[0]);
      setColorInputError("");
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
      setColorInput(COLORS[0]);
      setColorInputError("");
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
      setColorInput(COLORS[0]);
      setColorInputError("");
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
      return isSetEdit ? t("modal.edit_set") : t("modal.create_set");
    }
    return isHabitEdit ? t("modal.edit_habit") : t("modal.add_habit");
  };

  const getSubmitText = () => {
    if (isSubmitting) return t("button.saving");
    return isEdit ? t("button.save") : t("button.create");
  };

  const handleColorSelect = (value: string) => {
    setColor(value);
    setColorInput(value);
    setColorInputError("");
  };

  const handleColorInputBlur = () => {
    const normalized = normalize_color_value(colorInput);
    if (!normalized) {
      setColorInputError(t("modal.color_invalid"));
      return;
    }
    setColor(normalized);
    setColorInput(normalized);
    setColorInputError("");
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center" style={{ background: 'rgba(0,0,0,0.2)', backdropFilter: 'blur(4px)' }}>
      <div className="absolute inset-0" onClick={onClose} />
      <div className="relative my-surface-card rounded-xl p-6 w-full max-w-sm mx-4">
        <h3 className="text-lg font-bold mb-4">{getTitle()}</h3>
        
        <form onSubmit={(e) => { void handleSubmit(e); }}>
          <div className="form-control mb-4">
            <label className="label">
              <span className="label-text">{t("modal.name")}</span>
            </label>
            <input
              type="text"
              className="my-input"
              placeholder={mode === "set" ? t("modal.name_placeholder_set") : t("modal.name_placeholder_habit")}
              value={name}
              onInput={(e) => setName((e.target as HTMLInputElement).value)}
              required
            />
          </div>

          {mode === "set" && (
            <div className="form-control mb-4">
              <label className="label">
                <span className="label-text">{t("modal.description")}</span>
              </label>
              <textarea
                className="my-input min-h-[80px] resize-none"
                placeholder={t("modal.description_placeholder")}
                value={description}
                onInput={(e) => setDescription((e.target as HTMLTextAreaElement).value)}
              />
            </div>
          )}

          {mode === "habit" && (
            <div className="form-control mb-4">
              <label className="label">
                <span className="label-text">{t("modal.goal_duration")}</span>
              </label>
              <div className="flex gap-3 items-center">
                <PickerNumberInput
                  value={goalHours}
                  min={0}
                  max={9999}
                  onChange={setGoalHours}
                />
                <span className="text-sm">{t("modal.hours")}</span>
                <PickerNumberInput
                  value={goalMinutes}
                  min={0}
                  max={59}
                  onChange={setGoalMinutes}
                />
                <span className="text-sm">{t("modal.minutes")}</span>
              </div>
              <label className="label">
                <span className="label-text-alt text-error">
                  {(goalHours * 60 + goalMinutes) === 0 ? t("modal.goal_error") : t("modal.goal_summary", { hours: goalHours, minutes: goalMinutes, total: (goalHours * 60 + goalMinutes) })}
                </span>
              </label>
            </div>
          )}

          <div className="form-control mb-6">
            <label className="label">
              <span className="label-text">{t("modal.color")}</span>
            </label>
            <div className="flex gap-2 flex-wrap mb-3">
              {COLORS.map((c) => (
                <button
                  key={c}
                  type="button"
                  className={`w-9 h-9 rounded-full border-2 transition-all ${
                    color.toLowerCase() === c.toLowerCase()
                      ? "border-primary ring-2 ring-primary/30"
                      : "border-base-300 hover:border-base-content/40 hover:scale-105"
                  }`}
                  style={{ backgroundColor: c }}
                  onClick={() => handleColorSelect(c)}
                />
              ))}
            </div>
            <div className="flex items-center gap-2">
              <input
                type="text"
                className="my-input flex-1"
                value={colorInput}
                placeholder="#6366f1 或 rgb(99,102,241)"
                onInput={(e) => {
                  setColorInput((e.target as HTMLInputElement).value);
                  if (colorInputError) setColorInputError("");
                }}
                onBlur={handleColorInputBlur}
              />
              <input
                type="color"
                className="w-10 h-10 rounded-lg cursor-pointer border border-base-300 bg-transparent"
                value={normalize_hex(color) ?? COLORS[0]}
                onChange={(e) => handleColorSelect((e.target as HTMLInputElement).value)}
                title={t("modal.select_color")}
              />
            </div>
            {colorInputError && (
              <label className="label">
                <span className="label-text-alt text-error">{colorInputError}</span>
              </label>
            )}
          </div>

          <WallpaperSelector value={wallpaper} onChange={setWallpaper} />

          <div className="flex gap-2">
            <button
              type="button"
              className="btn btn-ghost flex-1"
              onClick={onClose}
            >
              {t("button.cancel")}
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
