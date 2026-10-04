import { useState } from "preact/hooks";
import { t } from "../../utils/i18n";

interface CaptureBoxProps {
  onCapture: (text: string) => Promise<unknown>;
  disabled?: boolean;
}

/**
 * CaptureBox —— 随手记输入框。Cmd/Ctrl + Enter 提交。
 */
export function CaptureBox({ onCapture, disabled }: CaptureBoxProps) {
  const [text, setText] = useState("");
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  const submit = async () => {
    const trimmed = text.trim();
    if (!trimmed) {
      setErr(t("aigtd.error_required"));
      return;
    }
    if (trimmed.length > 2000) {
      setErr(t("aigtd.error_too_long"));
      return;
    }
    setBusy(true);
    setErr(null);
    try {
      await onCapture(trimmed);
      setText("");
    } catch (e) {
      setErr(e instanceof Error ? e.message : String(e));
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="rounded-lg bg-base-200 p-3">
      <div className="flex gap-2">
        <textarea
          data-testid="capture-input"
          className="textarea textarea-bordered flex-1"
          placeholder={t("aigtd.capture_placeholder")}
          value={text}
          rows={1}
          maxLength={2000}
          disabled={disabled || busy}
          onInput={(e) => setText((e.currentTarget as HTMLTextAreaElement).value)}
          onKeyDown={(e) => {
            if (e.key === "Enter" && (e.metaKey || e.ctrlKey || !e.shiftKey)) {
              e.preventDefault();
              void submit();
            }
          }}
        />
        <button
          className="btn btn-primary"
          disabled={disabled || busy || !text.trim()}
          onClick={() => void submit()}
          title={t("aigtd.capture_hint")}
        >
          →
        </button>
      </div>
      {err && <p className="mt-1 text-sm text-error">{err}</p>}
      <p className="mt-1 text-xs opacity-60">{t("aigtd.capture_hint")}</p>
    </div>
  );
}
