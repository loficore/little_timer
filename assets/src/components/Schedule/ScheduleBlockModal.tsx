import { useState } from "preact/hooks";
import { Modal } from "../common/Modal";
import { t } from "../../utils/i18n";
import type { TimelineTask } from "./types";

export interface ScheduleBlockModalProps {
  task: TimelineTask;
  /** 所选日期的 YYYY-MM-DD（本地时区）。 */
  date: string;
  onSave: (startSec: number, endSec: number) => Promise<void> | void;
  onRemove: () => Promise<void> | void;
  onClose: () => void;
}

/** unix 秒 → 本地时区 HH:MM。 */
export function toHHMM(sec: number): string {
  const d = new Date(sec * 1000);
  return `${String(d.getHours()).padStart(2, "0")}:${String(d.getMinutes()).padStart(2, "0")}`;
}

/** 日期 + HH:MM（本地时区）→ unix 秒；解析失败返回 null。 */
export function toUnix(date: string, hhmm: string): number | null {
  const [y, m, d] = date.split("-").map((s) => Number(s));
  const [hh, mm] = hhmm.split(":").map((s) => Number(s));
  if (!y || !m || !d || Number.isNaN(hh) || Number.isNaN(mm)) return null;
  return Math.floor(new Date(y, m - 1, d, hh, mm, 0, 0).getTime() / 1000);
}

/**
 * ScheduleBlockModal —— 手动微调某个时间块（AI 排完/应用后都能改）。
 *
 * 只编辑「当天的起止时刻」；结束必须晚于开始，跨午夜不支持（会提示）。
 */
export function ScheduleBlockModal({
  task,
  date,
  onSave,
  onRemove,
  onClose,
}: ScheduleBlockModalProps) {
  const [start, setStart] = useState(toHHMM(task.scheduled_start));
  const [end, setEnd] = useState(toHHMM(task.scheduled_end));
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const startSec = toUnix(date, start);
  const endSec = toUnix(date, end);
  const timeInvalid = startSec === null || endSec === null || endSec <= startSec;

  const run = async (fn: () => Promise<void> | void) => {
    setBusy(true);
    setError(null);
    try {
      await fn();
      onClose();
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setBusy(false);
    }
  };

  return (
    <Modal
      title={t("schedule.edit_block")}
      onClose={onClose}
      testId="schedule-block-modal"
      footer={
        <>
          <button
            type="button"
            className="btn btn-ghost mr-auto text-error"
            data-testid="block-remove"
            disabled={busy}
            onClick={() => void run(onRemove)}
          >
            {t("schedule.block_remove")}
          </button>
          <button
            type="button"
            className="btn btn-ghost"
            data-testid="block-cancel"
            disabled={busy}
            onClick={onClose}
          >
            {t("button.cancel")}
          </button>
          <button
            type="button"
            className="btn btn-primary"
            data-testid="block-save"
            disabled={busy || timeInvalid}
            onClick={() => {
              if (timeInvalid || startSec === null || endSec === null) return;
              void run(() => onSave(startSec, endSec));
            }}
          >
            {busy ? t("button.saving") : t("button.save")}
          </button>
        </>
      }
    >
      <div className="space-y-4">
        <p className="m-0 text-sm text-[var(--my-on-surface-variant)]">
          {date} · {task.title}
        </p>

        <div className="grid grid-cols-2 gap-3">
          <label className="block space-y-1.5">
            <span className="text-sm font-medium text-[var(--my-on-surface)]">
              {t("schedule.block_start")}
            </span>
            <input
              type="time"
              className="my-input w-full"
              data-testid="block-start"
              value={start}
              onInput={(e) => setStart((e.currentTarget as HTMLInputElement).value)}
            />
          </label>
          <label className="block space-y-1.5">
            <span className="text-sm font-medium text-[var(--my-on-surface)]">
              {t("schedule.block_end")}
            </span>
            <input
              type="time"
              className="my-input w-full"
              data-testid="block-end"
              value={end}
              onInput={(e) => setEnd((e.currentTarget as HTMLInputElement).value)}
            />
          </label>
        </div>

        {timeInvalid && (
          <p className="m-0 text-sm text-error" data-testid="block-time-error" role="alert">
            {t("schedule.block_invalid")}
          </p>
        )}
        {error && (
          <p className="m-0 text-sm text-error" role="alert" data-testid="block-save-error">
            {error}
          </p>
        )}
      </div>
    </Modal>
  );
}
