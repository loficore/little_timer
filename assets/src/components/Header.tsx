import type { FunctionalComponent } from "preact";
import { t } from "../utils/i18n";
import { ArrowLeftIconComponent, SettingsIcon, ChartIcon } from "../utils/icons";

interface HeaderProps {
  /** 标题文本 */
  title?: string;
  /** 是否显示设置按钮 */
  showSettings?: boolean;
  /** 设置按钮点击回调 */
  onSettingsClick?: () => void;
  /** 是否显示统计按钮 */
  showStats?: boolean;
  /** 统计按钮点击回调 */
  onStatsClick?: () => void;
  /** 是否显示返回按钮 */
  showBack?: boolean;
  /** 返回按钮点击回调 */
  onBackClick?: () => void;
}

export const Header: FunctionalComponent<HeaderProps> = ({
  title = t("common.app_name"),
  showSettings = true,
  onSettingsClick,
  showStats = false,
  onStatsClick,
  showBack = false,
  onBackClick,
}) => {
  return (
    <div className="navbar bg-base-200 border-b border-base-300 shrink-0">
      <div className="flex-1">
        {showBack && (
          <button
            onClick={onBackClick}
            title={t("common.back")}
            className="btn btn-ghost btn-sm gap-1"
          >
            <ArrowLeftIconComponent />
            <span>{t("common.back")}</span>
          </button>
        )}
      </div>
      <div className="flex-none">
        <h1 className="text-lg font-semibold">{title}</h1>
      </div>
      <div className="flex-1 flex justify-end gap-2">
        {showStats && (
          <button
            onClick={onStatsClick}
            title={t("stats.title") || "统计"}
            className="btn btn-ghost btn-circle"
          >
            <ChartIcon />
          </button>
        )}
        {showSettings && (
          <button
            onClick={onSettingsClick}
            title={t("common.settings_title")}
            className="btn btn-ghost btn-circle"
          >
            <SettingsIcon />
          </button>
        )}
      </div>
    </div>
  );
};
