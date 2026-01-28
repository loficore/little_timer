import type { FunctionalComponent } from "preact";
import { Mode } from "../utils/share";

interface ModeItem {
  /** 模式枚举值 */
  key: Mode;
  /** 显示标签 */
  label: string;
  /** 模式图标 */
  icon: string;
}

interface ModeSelectorProps {
  /** 模式列表 */
  modes: ModeItem[];
  /** 当前选中的模式 */
  activeMode: Mode;
  /** 模式变更回调 */
  onModeChange: (mode: Mode) => void;
  /** 动画延迟（可选） */
  animationDelay?: string;
}

/**
 * 模式选择器组件 - 管理倒计时/秒表/世界时钟模式切换
 *
 * @example
 * ```tsx
 * <ModeSelector
 *   modes={[
 *     { key: Mode.Countdown, label: "倒计时", icon: "⏱" },
 *     { key: Mode.Stopwatch, label: "秒表", icon: "⏲" },
 *     { key: Mode.WorldClock, label: "世界时钟", icon: "🌐" },
 *   ]}
 *   activeMode={Mode.Countdown}
 *   onModeChange={handleModeChange}
 * />
 * ```
 */
export const ModeSelector: FunctionalComponent<ModeSelectorProps> = ({
  modes,
  activeMode,
  onModeChange,
  animationDelay = "0s",
}) => {
  return (
    <div
      className="mt-6 sm:mt-8 md:mt-12 pt-4 sm:pt-6 md:pt-8 border-t border-border-dark text-center animate-slideUp w-full"
      style={{ animationDelay, animationFillMode: "both" }}
    >
      <h3 className="text-xs font-semibold text-text-secondary-dark uppercase tracking-wider mb-3 sm:mb-4 md:mb-6 px-2">
        切换模式
      </h3>
      <div className="grid grid-cols-3 gap-2 sm:gap-3 md:gap-4 px-2">
        {modes.map(({ key, label, icon }) => (
          <button
            key={key}
            onClick={() => onModeChange(key)}
            className={`p-2 sm:p-3 md:p-4 rounded-xl flex flex-col items-center gap-1 sm:gap-2 text-xs sm:text-sm font-medium transition-all duration-200 hover:scale-105 active:scale-95 min-h-15 sm:min-h-20 ${
              activeMode === key
                ? "bg-accent-dark border-accent-dark text-white"
                : "bg-transparent border border-border-dark text-text-secondary-dark hover:bg-secondary-dark hover:border-accent-dark hover:text-text-primary-dark"
            }`}
          >
            <span className="text-xl sm:text-2xl">{icon}</span>
            <span className="text-center line-clamp-2">{label}</span>
          </button>
        ))}
      </div>
    </div>
  );
};
