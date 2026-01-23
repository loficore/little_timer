import type { FunctionalComponent } from "preact";
import { useEffect, useState } from "preact/hooks";
import {
  ConnectionState,
  onConnectionStateChange,
  getRetryCount,
} from "../utils/webui-manager";
import { t } from "../utils/i18n";

interface ErrorNotificationProps {
  visible?: boolean;
  onDismiss?: () => void;
  autoHideDuration?: number;
}

/**
 * 网络错误通知组件
 * 显示 WebUI 连接状态和错误信息
 */
export const ErrorNotification: FunctionalComponent<ErrorNotificationProps> = ({
  visible = false,
  onDismiss,
  autoHideDuration = 5000,
}) => {
  const [state, setState] = useState<ConnectionState | null>(null);
  const [error, setError] = useState<string>("");
  const [show, setShow] = useState(visible);
  const [retryCount, setRetryCount] = useState(0);

  useEffect(() => {
    // 监听连接状态变化
    const unsubscribe = onConnectionStateChange(
      ConnectionState.DISCONNECTED,
      (newState) => {
        setState(newState);
        setShow(true);
        setError(t("errors.connection.disconnected"));
      },
    );

    const unsubscribeError = onConnectionStateChange(
      ConnectionState.ERROR,
      (newState, err) => {
        setState(newState);
        setShow(true);
        setError(err?.message || t("errors.connection.unknown"));
      },
    );

    const unsubscribeReconnecting = onConnectionStateChange(
      ConnectionState.RECONNECTING,
      (newState) => {
        setState(newState);
        setShow(true);
        setRetryCount(getRetryCount());
      },
    );

    const unsubscribeConnected = onConnectionStateChange(
      ConnectionState.CONNECTED,
      () => {
        setState(ConnectionState.CONNECTED);
        if (autoHideDuration > 0) {
          setTimeout(() => {
            setShow(false);
            onDismiss?.();
          }, autoHideDuration);
        }
      },
    );

    return () => {
      unsubscribe();
      unsubscribeError();
      unsubscribeReconnecting();
      unsubscribeConnected();
    };
  }, [autoHideDuration, onDismiss]);

  if (!show) {
    return null;
  }

  const getStatusIcon = () => {
    switch (state) {
      case ConnectionState.DISCONNECTED:
        return "📡";
      case ConnectionState.RECONNECTING:
        return "🔄";
      case ConnectionState.ERROR:
        return "❌";
      case ConnectionState.CONNECTED:
        return "✅";
      default:
        return "⚠️";
    }
  };

  const getStatusColor = () => {
    switch (state) {
      case ConnectionState.CONNECTED:
        return "bg-green-600 border-green-500";
      case ConnectionState.RECONNECTING:
        return "bg-yellow-600 border-yellow-500 animate-pulse";
      case ConnectionState.ERROR:
      case ConnectionState.DISCONNECTED:
        return "bg-red-600 border-red-500";
      default:
        return "bg-gray-600 border-gray-500";
    }
  };

  return (
    <div
      className={`fixed top-4 left-4 right-4 sm:left-6 sm:right-6 md:left-8 md:right-8 z-50 rounded-lg border-l-4 p-4 sm:p-5 md:p-6 shadow-lg animate-slideDown ${getStatusColor()} text-white`}
      role="alert"
    >
      <div className="flex items-start gap-3 sm:gap-4">
        <span className="text-2xl flex-shrink-0">{getStatusIcon()}</span>
        <div className="flex-1 min-w-0">
          <h3 className="font-semibold text-sm sm:text-base mb-1">
            {state === ConnectionState.CONNECTED
              ? t("errors.status.connected")
              : state === ConnectionState.RECONNECTING
                ? t("errors.status.reconnecting", { attempt: retryCount })
                : state === ConnectionState.ERROR
                  ? t("errors.status.error")
                  : t("errors.status.disconnected")}
          </h3>
          <p className="text-xs sm:text-sm opacity-90 break-words">{error}</p>
        </div>

        {state === ConnectionState.CONNECTED && (
          <button
            onClick={() => {
              setShow(false);
              onDismiss?.();
            }}
            className="flex-shrink-0 ml-2 text-white hover:opacity-75 transition-opacity p-1 rounded"
            title="关闭"
          >
            ✕
          </button>
        )}
      </div>
    </div>
  );
};

/**
 * 离线模式指示器
 * 当连接失败时显示降级方案提示
 */
export const OfflineModeIndicator: FunctionalComponent<{
  show?: boolean;
}> = ({ show = false }) => {
  return show ? (
    <div className="fixed bottom-4 left-4 right-4 sm:left-6 sm:right-6 md:left-8 md:right-8 bg-yellow-700 border-l-4 border-yellow-500 p-3 sm:p-4 rounded-lg text-white text-xs sm:text-sm shadow-lg z-40">
      <div className="flex items-center gap-2 sm:gap-3">
        <span className="text-lg flex-shrink-0">⚠️</span>
        <div>
          <strong>{t("errors.offline.title")}</strong>
          <p className="text-yellow-100 mt-1">{t("errors.offline.message")}</p>
        </div>
      </div>
    </div>
  ) : null;
};
