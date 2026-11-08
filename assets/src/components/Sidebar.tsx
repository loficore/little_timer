/**
 * 侧边栏导航组件
 * 展示应用主要页面（计时、习惯、统计、设置）的入口
 */

import type { FunctionalComponent } from "preact";
import { t } from "../utils/i18n";
import { useAppVersion } from "../hooks/useAppVersion";
import { capabilities, type PlatformCapabilities } from "../utils/platformApi";import { StarIconComponent, TimerIconComponent, HabitsIconComponent, ChartIcon, SettingsIcon, CalendarIcon, SparklesIconComponent } from "../utils/icons";

type Page = "timer" | "habits" | "aigtd" | "plan" | "stats" | "settings";

interface SidebarProps {
    /** 当前所在页面 */
    currentPage: Page;
    /** 切换页面回调 */
    onNavigate: (page: Page) => void;
}

/**
 * 导航项。`capability` 指向能力表字段：平台不支持时整条入口不渲染，
 * 而不是先渲染再在运行期抛错。
 */
const navItems: {
    id: Page;
    labelKey: string;
    capability: keyof PlatformCapabilities;
    icon: preact.JSX.Element;
}[] = [
    {
        id: "timer" as const,
        labelKey: "nav.timer",
        capability: "timer" as const,
        icon: (
            <TimerIconComponent className="h-5 w-5" />
        ),
    },
    {
        id: "habits" as const,
        labelKey: "nav.habits",
        capability: "habits" as const,
        icon: (
            <HabitsIconComponent className="h-5 w-5" />
        ),
    },
    {
        id: "aigtd" as const,
        labelKey: "aigtd.nav_label",
        capability: "tasks" as const,
        icon: (
            <SparklesIconComponent className="h-5 w-5" />
        ),
    },
    {
        id: "plan" as const,
        labelKey: "plan.nav_label",
        capability: "schedule" as const,
        icon: (
            <CalendarIcon className="h-5 w-5" />
        ),
    },
    {
        id: "stats" as const,
        labelKey: "nav.stats",
        capability: "habits" as const,
        icon: (
            <ChartIcon className="h-5 w-5" />
        ),
    },
    {
        id: "settings" as const,
        labelKey: "nav.settings",
        capability: "settings" as const,
        icon: (
            <SettingsIcon className="h-5 w-5" />
        ),
    },
];

export const Sidebar: FunctionalComponent<SidebarProps> = ({ currentPage, onNavigate }) => {
    const versionInfo = useAppVersion();
    const visibleItems = navItems.filter((item) => capabilities[item.capability]);
    return (
        <aside className="my-sidebar flex flex-col w-60 h-full shrink-0">
            {/* Logo */}
            <div className="p-4 shadow-[inset_0_-1px_0_color-mix(in_oklab,var(--my-outline)_22%,transparent)]">
                <h1 className="text-xl font-bold flex items-center gap-2 text-[var(--my-on-surface)] opacity-90">
                    <StarIconComponent />
                    <span className="text-[var(--my-on-surface)]">Little Timer</span>
                </h1>
            </div>

            {/* 导航 */}
            <nav className="flex-1 p-2" aria-label={t("common.app_name")}>
                {visibleItems.map((item) => (
                    <button
                        key={item.id}
                        data-testid={`nav-${item.id}`}
                        aria-current={currentPage === item.id ? "page" : undefined}
                        className={`my-sidebar-nav-btn ${currentPage === item.id ? "is-active" : ""}`}
                        onClick={() => onNavigate(item.id)}
                    >
                        {item.icon}
                        <span className="font-medium">{t(item.labelKey)}</span>
                    </button>
                ))}
            </nav>

            {/* 底部：版本号唯一来源是后端 /api/version（发布时 = git tag） */}
            <div className="p-4 text-center text-sm text-[var(--my-on-surface)] opacity-60 shadow-[inset_0_1px_0_color-mix(in_oklab,var(--my-outline)_18%,transparent)]">
                {versionInfo && (
                    <p data-testid="app-version" title={versionInfo.gitCommit}>
                        v{versionInfo.version}
                    </p>
                )}
            </div>
        </aside>
    );
};
