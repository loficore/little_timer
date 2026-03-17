import type { FunctionalComponent } from "preact";

interface ErrorNotificationProps {
  visible?: boolean;
}

export const ErrorNotification: FunctionalComponent<ErrorNotificationProps> = ({
  visible = false,
}) => {
  if (!visible) {
    return null;
  }

  return null;
};

export const OfflineModeIndicator: FunctionalComponent<{
  show?: boolean;
}> = ({ show = false }) => {
  return show ? (
    <div className="fixed bottom-4 left-4 right-4 sm:left-6 sm:right-6 md:left-8 md:right-8 bg-yellow-700 border-l-4 border-yellow-500 p-3 sm:p-4 rounded-lg text-white text-xs sm:text-sm shadow-lg z-40">
      <div className="flex items-center gap-2 sm:gap-3">
        <span className="text-lg flex-shrink-0">⚠️</span>
        <div>
          <strong>连接中断</strong>
          <p className="text-yellow-100 mt-1">请检查网络连接</p>
        </div>
      </div>
    </div>
  ) : null;
};
