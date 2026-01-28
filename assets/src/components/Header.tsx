import type { FunctionalComponent } from "preact";
import { t } from "../utils/i18n";

interface HeaderProps {
  /** 标题文本 */
  title?: string;
  /** 是否显示设置按钮 */
  showSettings?: boolean;
  /** 设置按钮点击回调 */
  onSettingsClick?: () => void;
  /** 是否显示返回按钮 */
  showBack?: boolean;
  /** 返回按钮点击回调 */
  onBackClick?: () => void;
}

/**
 * 应用头部组件 - 统一管理标题栏样式和交互
 *
 * @example
 * ```tsx
 * <Header
 *   title={t("common.app_name")}
 *   showSettings={true}
 *   onSettingsClick={handleSettings}
 * />
 * ```
 */
export const Header: FunctionalComponent<HeaderProps> = ({
  title = t("common.app_name"),
  showSettings = true,
  onSettingsClick,
  showBack = false,
  onBackClick,
}) => {
  return (
    <div className="flex justify-between items-center px-4 sm:px-6 md:px-8 py-3 sm:py-4 md:py-6 border-b border-border-dark shrink-0">
      {/* 左侧按钮 */}
      <div className="w-10 shrink-0 flex items-center justify-center">
        {showBack && (
          <button
            onClick={onBackClick}
            title={t("common.back")}
            className="w-10 h-10 flex items-center justify-center rounded-xl bg-transparent border border-border-dark text-text-secondary-dark font-semibold cursor-pointer transition-all duration-200 hover:bg-secondary-dark hover:border-accent-dark hover:text-text-primary-dark hover:scale-110 active:bg-tertiary-dark active:scale-95"
          >
            ←
          </button>
        )}
      </div>

      {/* 中间标题 */}
      <h1 className="text-lg sm:text-xl md:text-2xl font-semibold text-text-primary-dark truncate pr-2">
        {title}
      </h1>

      {/* 右侧按钮 */}
      <div className="w-10 shrink-0 flex items-center justify-center">
        {showSettings && (
          <button
            onClick={onSettingsClick}
            title={t("common.settings_title")}
            className="w-10 h-10 flex items-center justify-center rounded-xl bg-transparent border-0 cursor-pointer transition-all duration-200 text-text-secondary-dark hover:bg-secondary-dark hover:text-text-primary-dark hover:scale-110 active:bg-tertiary-dark"
          >
            ⚙
          </button>
        )}
      </div>
    </div>
  );
};
