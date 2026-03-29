import { useState } from "preact/hooks";
import { HomePage } from "./Homepage.tsx";
import { SettingsPage } from "./Settings.tsx";
import { StatsPage } from "./Stats.tsx";
import { ErrorNotification } from "./components/ErrorNotification.tsx";
import type { Habit } from "./types/habit.ts";

type Page = "home" | "habits" | "timer" | "stats" | "settings";

interface AppState {
  page: Page;
  selectedSetId: number | null;
  selectedHabit: Habit | null;
}

export const App = () => {
  const [state, setState] = useState<AppState>({
    page: "home",
    selectedSetId: null,
    selectedHabit: null,
  });

  const navigateTo = (page: Page) => {
    setState(prev => ({ ...prev, page, selectedSetId: null, selectedHabit: null }));
  };

  const selectSet = (setId: number) => {
    setState(prev => ({ ...prev, page: "habits", selectedSetId: setId, selectedHabit: null }));
  };

  const selectHabit = (habit: Habit) => {
    setState(prev => ({ ...prev, page: "timer", selectedHabit: habit }));
  };

  const goBack = () => {
    if (state.page === "timer") {
      setState(prev => ({ ...prev, page: "habits", selectedHabit: null }));
    } else if (state.page === "habits") {
      setState(prev => ({ ...prev, page: "home", selectedSetId: null }));
    } else {
      setState(prev => ({ ...prev, page: "home" }));
    }
  };

  return (
    <>
      <ErrorNotification visible={true} />

      {state.page === "home" && (
        <HomePage
          onStatsClick={() => navigateTo("stats")}
          onSetClick={selectSet}
        />
      )}
      {state.page === "habits" && (
        <HomePage
          onBackClick={goBack}
          onStatsClick={() => navigateTo("stats")}
          selectedSetId={state.selectedSetId}
          onHabitClick={selectHabit}
        />
      )}
      {state.page === "timer" && (
        <HomePage
          onBackClick={goBack}
          onStatsClick={() => navigateTo("stats")}
          selectedHabit={state.selectedHabit}
        />
      )}
      {state.page === "settings" && (
        <SettingsPage onBackClick={goBack} />
      )}
      {state.page === "stats" && (
        <StatsPage onBackClick={goBack} />
      )}

      {/* Bottom Navigation */}
      <nav className="btm-nav btm-nav-md lg:hidden fixed bottom-0 w-full z-50">
        <a
          className={state.page === "home" || state.page === "habits" || state.page === "timer" ? "active" : ""}
          onClick={() => navigateTo("home")}
        >
          <svg xmlns="http://www.w3.org/2000/svg" className="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M3 12l2-2m0 0l7-7 7 7M5 10v10a1 1 0 001 1h3m10-11l2 2m-2-2v10a1 1 0 01-1 1h-3m-6 0a1 1 0 001-1v-4a1 1 0 011-1h2a1 1 0 011 1v4a1 1 0 001 1m-6 0h6" />
          </svg>
          <span className="btm-nav-label">首页</span>
        </a>
        <a
          className={state.page === "stats" ? "active" : ""}
          onClick={() => navigateTo("stats")}
        >
          <svg xmlns="http://www.w3.org/2000/svg" className="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z" />
          </svg>
          <span className="btm-nav-label">统计</span>
        </a>
        <a
          className={state.page === "settings" ? "active" : ""}
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
