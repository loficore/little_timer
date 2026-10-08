import { useEffect, useState } from "preact/hooks";
import { getAPIClient } from "../../utils/apiClientSingleton";
import { t } from "../../utils/i18n";

/**
 * DailySweepSettings —— 设置页 "Daily Sweep" Tab 内容。
 *
 * 字段:enabled 开关 + HH:MM 时间(5 分钟步进)。非 5 分钟倍数在提交时被
 * 后端正则 `^(?:[01]\d|2[0-3]):[0-5]\d$` 接受(HH:MM 任意,非仅 5 分倍),
 * 但前端限制为 5 分步进以贴合用户预期。
 */
export function DailySweepSettings() {
  const client = getAPIClient();
  const [cfg, setCfg] = useState<{ enabled: boolean; time: string } | null>(null);
  const [enabled, setEnabled] = useState(true);
  const [time, setTime] = useState("08:00");
  const [saving, setSaving] = useState(false);
  const [feedback, setFeedback] = useState<string | null>(null);

  useEffect(() => {
    void client.getDailySweepSettings().then((c) => {
      setCfg(c);
      setEnabled(c.enabled);
      setTime(c.time);
    });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const onSave = async () => {
    if (!cfg) return;
    setSaving(true);
    setFeedback(null);
    try {
      const next = await client.updateDailySweepSettings({ enabled, time });
      setCfg(next);
      setFeedback(t("common.save_success"));
    } catch (e) {
      setFeedback(e instanceof Error ? e.message : String(e));
    } finally {
      setSaving(false);
    }
  };

  if (!cfg) {
    return <p className="opacity-60">…</p>;
  }

  return (
    <div className="space-y-3">
      <div className="form-control">
        <label className="label cursor-pointer justify-start gap-3">
          <input
            type="checkbox"
            className="toggle toggle-primary"
            data-testid="enabled-toggle"
            checked={enabled}
            onChange={(e) => setEnabled((e.currentTarget as HTMLInputElement).checked)}
          />
          <span className="label-text">{t("dailySweep.settings.enabled")}</span>
        </label>
      </div>
      <div className="form-control">
        <label className="label">
          <span className="label-text">{t("dailySweep.settings.time")}</span>
        </label>
        <input
          type="time"
          className="input input-bordered"
          step={300}
          data-testid="time-input"
          value={time}
          onInput={(e) => setTime((e.currentTarget as HTMLInputElement).value)}
        />
      </div>
      <div className="flex items-center gap-2">
        <button
          className="btn btn-primary btn-sm"
          onClick={() => void onSave()}
          disabled={saving}
          data-testid="save-btn"
        >
          {saving ? t("common.saving") : t("common.save")}
        </button>
        {feedback && <span className="text-xs opacity-70">{feedback}</span>}
      </div>
    </div>
  );
}
