import type { FunctionalComponent } from "preact";
import { t } from "../utils/i18n";

interface ControlPanelProps {
  /** 计时器是否正在运行 */
  isRunning: boolean;
  /** 开始按钮点击回调 */
  onStart: () => void;
  /** 暂停按钮点击回调 */
  onPause: () => void;
  /** 重置按钮点击回调 */
  onReset: () => void;
  /** 动画延迟（可选） */
  animationDelay?: string;
}

/**
 * 计时器控制面板组件 - 管理开始/暂停/重置按钮
 *
 * @example
 * ```tsx
 * <ControlPanel
 *   isRunning={isRunning}
 *   onStart={handleStart}
 *   onPause={handlePause}
 *   onReset={handleReset}
 *   animationDelay="0.3s"
 * />
 * ```
 */
export const ControlPanel: FunctionalComponent<ControlPanelProps> = ({
  isRunning,
  onStart,
  onPause,
  onReset,
  animationDelay = "0s",
}) => {
  return (
    <div
      className="flex gap-2 sm:gap-3 md:gap-4 my-4 sm:my-6 md:my-8 justify-center flex-wrap animate-slideUp w-full"
      style={{ animationDelay, animationFillMode: "both" }}
    >
      {!isRunning ? (
        <button
          onClick={onStart}
          className="btn-primary flex-1 sm:flex-none sm:w-32 md:w-40 flex items-center justify-center gap-1 sm:gap-2 text-sm sm:text-base"
        >
          <span className="text-lg sm:text-xl">▶</span>
          <span className="hidden sm:inline">{t("home.start")}</span>
        </button>
      ) : (
        <button
          onClick={onPause}
          className="btn-primary flex-1 sm:flex-none sm:w-32 md:w-40 flex items-center justify-center gap-1 sm:gap-2 text-sm sm:text-base"
        >
          <span className="text-lg sm:text-xl">⏸</span>
          <span className="hidden sm:inline">{t("home.pause")}</span>
        </button>
      )}
      <button
        onClick={onReset}
        className="btn-secondary flex-1 sm:flex-none sm:w-32 md:w-40 flex items-center justify-center gap-1 sm:gap-2 text-sm sm:text-base"
      >
        <span className="text-lg sm:text-xl">↻</span>
        <span className="hidden sm:inline">{t("home.reset")}</span>
      </button>
    </div>
  );
};
