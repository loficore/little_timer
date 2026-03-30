import { useState } from "preact/hooks";
import { Sidebar } from "./components/Sidebar";
import { HabitsPage } from "./HabitsPage";
import { SettingsPage } from "./Settings.tsx";
import { StatsPage } from "./Stats.tsx";
import { ErrorNotification } from "./components/ErrorNotification.tsx";
import type { Habit } from "./types/habit.ts";

type Page = "habits" | "stats" | "settings";

export const App = () => {
  const [page, setPage] = useState<Page>("habits");
  const [selectedHabit, setSelectedHabit] = useState<Habit | null>(null);

  const navigateTo = (newPage: Page) => {
    setPage(newPage);
    setSelectedHabit(null);
  };

  const handleHabitClick = (habit: Habit | null) => {
    setSelectedHabit(habit);
  };

  return (
    <>
      <ErrorNotification visible={true} />

      <div className="flex h-screen bg-base-100">
        {/* 侧边栏 - 桌面端 */}
        <div className="hidden lg:block lg:flex shrink-0">
          <Sidebar currentPage={page} onNavigate={navigateTo} />
        </div>

        {/* 主内容区 */}
        <main className="flex-1 flex flex-col overflow-hidden pb-20 lg:pb-0">
          {page === "habits" && (
            <HabitsPage
              selectedHabit={selectedHabit}
              onHabitClick={handleHabitClick}
              onStatsClick={() => navigateTo("stats")}
              onSettingsClick={() => navigateTo("settings")}
            />
          )}
          {page === "stats" && (
            <StatsPage onBackClick={() => navigateTo("habits")} />
          )}
          {page === "settings" && (
            <SettingsPage onBackClick={() => navigateTo("habits")} />
          )}
        </main>
      </div>

      {/* 底部导航 - 移动端 */}
      <nav
        className="btm-nav btm-nav-md lg:hidden fixed inset-x-0 bottom-0 w-full z-50"
        data-testid="bottom-nav"
      >
        <a
          data-testid="nav-habits"
          className={page === "habits" ? "active" : ""}
          onClick={() => navigateTo("habits")}
        >
          <svg xmlns="http://www.w3.org/2000/svg" className="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2m-6 9l2 2 4-4" />
          </svg>
          <span className="btm-nav-label">习惯</span>
        </a>
        <a
          data-testid="nav-stats"
          className={page === "stats" ? "active" : ""}
          onClick={() => navigateTo("stats")}
        >
          <svg xmlns="http://www.w3.org/2000/svg" className="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z" />
          </svg>
          <span className="btm-nav-label">统计</span>
        </a>
        <a
          data-testid="nav-settings"
          className={page === "settings" ? "active" : ""}
          onClick={() => navigateTo("settings")}
        >
          <svg xmlns="http://www.w3.org/2000/svg" className="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" />
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
          </svg>
          <span className="btm-nav-label">设置</span>
        </a>
      </nav>
    </>
  );
};
