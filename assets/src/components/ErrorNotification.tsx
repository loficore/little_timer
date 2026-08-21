/**
 * 顶部错误通知组件
 * 显示一条错误消息，5 秒后自动消失，可手动关闭
 * 同文件内还导出 OfflineModeIndicator 离线提示横幅
 */

import type { FunctionalComponent } from "preact";
import { useState, useEffect } from "preact/hooks";
import { t } from "../utils/i18n";

interface ErrorNotificationProps {
  /** 是否显示通知 */
  visible?: boolean;
  /** 错误消息内容 */
  message?: string;
  /** 关闭通知回调 */
  onDismiss?: () => void;
}

export const ErrorNotification: FunctionalComponent<ErrorNotificationProps> = ({
  visible = false,
  message,
  onDismiss,
}) => {
  const [displayMessage, setDisplayMessage] = useState<string | null>(null);
  const [isShowing, setIsShowing] = useState(false);

  useEffect(() => {
    if (visible && message) {
      setDisplayMessage(message);
      setIsShowing(true);
      const timer = setTimeout(() => {
        setIsShowing(false);
        onDismiss?.();
      }, 5000);
      return () => clearTimeout(timer);
    } else if (!visible) {
      setIsShowing(false);
    }
  }, [visible, message, onDismiss]);

  if (!isShowing || !displayMessage) {
    return null;
  }

  return (
    <div className="fixed top-4 left-1/2 -translate-x-1/2 z-50 animate-slide-down">
      <div className="bg-[color-mix(in_oklab,rgba(239,68,68,0.22)_88%,var(--my-surface))] border border-[color-mix(in_oklab,rgba(239,68,68,0.35)_60%,transparent)] rounded-lg p-3 sm:p-4 shadow-lg max-w-md">
        <div className="flex items-start gap-2 sm:gap-3">
          <span className="text-lg flex-shrink-0">⚠️</span>
          <div className="flex-1 min-w-0">
            <strong className="text-[var(--my-on-surface)] text-sm sm:text-base">{t("errors.operation_failed")}</strong>
            <p className="text-[#fca5a5] mt-1 text-xs sm:text-sm break-words">{displayMessage}</p>
          </div>
          <button
            type="button"
            onClick={() => {
              setIsShowing(false);
              onDismiss?.();
            }}
            className="text-[#fca5a5] hover:text-[var(--my-on-surface)] flex-shrink-0"
            aria-label={t("errors.close")}
          >
            ✕
          </button>
        </div>
      </div>
    </div>
  );
};

export const OfflineModeIndicator: FunctionalComponent<{
  show?: boolean;
}> = ({ show = false }) => {
  return show ? (
    <div className="fixed bottom-4 left-4 right-4 sm:left-6 sm:right-6 md:left-8 md:right-8 alert alert-warning p-3 sm:p-4 rounded-lg text-[var(--my-on-surface)] text-xs sm:text-sm shadow-lg z-40">
      <div className="flex items-center gap-2 sm:gap-3">
        <span className="text-lg flex-shrink-0">⚠️</span>
        <div>
          <strong>{t("errors.disconnected")}</strong>
          <p className="text-[var(--my-on-surface-variant)] mt-1">{t("errors.check_network")}</p>
        </div>
      </div>
    </div>
  ) : null;
};