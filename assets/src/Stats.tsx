import type { FunctionalComponent } from "preact";
import { useState, useEffect, useRef } from "preact/hooks";
import { Header } from "./components/Header";
import { t } from "./utils/i18n";
import { getAPIClient } from "./utils/apiClientSingleton";
import { formatDurationShort, formatDuration, getToday, getDaysAgo } from "./utils/formatters";

interface StatsPageProps {
  onBackClick: () => void;
}

interface Habit {
  id: number;
  name: string;
  color: string;
}

interface Session {
  habit_id: number;
  duration_seconds: number;
  date: string;
}

type TimeRange = "today" | "week" | "month" | "custom";

export const StatsPage: FunctionalComponent<StatsPageProps> = ({ onBackClick }) => {
  const [timeRange, setTimeRange] = useState<TimeRange>("week");
  const [habits, setHabits] = useState<Habit[]>([]);
  const [sessions, setSessions] = useState<Session[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [selectedHabitId, setSelectedHabitId] = useState<number | null>(null);
  
  const pieChartRef = useRef<HTMLDivElement>(null);
  const barChartRef = useRef<HTMLDivElement>(null);
  const pieChartInstanceRef = useRef<unknown>(null);
  const barChartInstanceRef = useRef<unknown>(null);

  const loadData = async () => {
    setIsLoading(true);
    console.log("[Stats loadData] starting...");
    try {
      const client = getAPIClient();
      console.log("[Stats loadData] fetching habits...");
      const habitsData = await client.getHabits();
      console.log("[Stats loadData] habits response:", habitsData);
      setHabits(Array.isArray(habitsData) ? habitsData : []);
      
      let startDate = "";
      let endDate = "";
      const today = getToday();
      
      if (timeRange === "today") {
        startDate = endDate = today;
      } else if (timeRange === "week") {
        startDate = getDaysAgo(7);
        endDate = today;
      } else if (timeRange === "month") {
        startDate = getDaysAgo(30);
        endDate = today;
      }
      
      console.log("[Stats loadData] fetching sessions:", { startDate, endDate });
      if (startDate && endDate) {
        const sessionsData = await client.getSessions(undefined, startDate, endDate);
        console.log("[Stats loadData] sessions response:", sessionsData);
        setSessions(Array.isArray(sessionsData) ? sessionsData : []);
      } else {
        setSessions([]);
      }
    } catch (e) {
      console.error("[Stats loadData] error:", e);
    } finally {
      setIsLoading(false);
      console.log("[Stats loadData] complete, isLoading=false");
    }
  };

  useEffect(() => {
    console.log("[Stats] timeRange changed, loading data...", timeRange);
    void loadData();
  }, [timeRange]);

  useEffect(() => {
    console.log("[Stats] rendering charts:", { isLoading, sessionsCount: sessions.length, habitsCount: habits.length, selectedHabitId });
    if (!isLoading && sessions.length >= 0) {
      void renderCharts();
    }
    return () => {
      if (pieChartInstanceRef.current && typeof (pieChartInstanceRef.current as { destroy: () => void }).destroy === "function") {
        (pieChartInstanceRef.current as { destroy: () => void }).destroy();
      }
      if (barChartInstanceRef.current && typeof (barChartInstanceRef.current as { destroy: () => void }).destroy === "function") {
        (barChartInstanceRef.current as { destroy: () => void }).destroy();
      }
    };
  }, [isLoading, sessions, habits, selectedHabitId]);

  const renderCharts = async () => {
    console.log("[Stats renderCharts] called with:", { 
      sessionsCount: sessions.length, 
      habitsCount: habits.length, 
      selectedHabitId,
      sessions: sessions.slice(0, 3),
      habits: habits.slice(0, 3)
    });
    
    let targetHabitId: number | null = null;
    if (selectedHabitId !== null) {
      const rawId = selectedHabitId;
      if (typeof rawId === 'string') {
        try {
          targetHabitId = Number(BigInt(rawId));
        } catch {
          targetHabitId = parseInt(rawId, 10);
        }
      } else {
        targetHabitId = Number(rawId);
      }
    }
    
    const filteredSessions = targetHabitId !== null
      ? sessions.filter(s => {
          const sId = Number(s.habit_id);
          return s && Number.isFinite(sId) && sId === targetHabitId;
        })
      : sessions;
    
    console.log("[Stats renderCharts] filteredSessions:", { 
      targetHabitId, 
      count: filteredSessions.length,
      sessions: filteredSessions.map(s => ({ habit_id: s.habit_id, duration: s.duration_seconds }))
    });

    if (pieChartRef.current) {
      if (pieChartInstanceRef.current && typeof (pieChartInstanceRef.current as { destroy: () => void }).destroy === "function") {
        (pieChartInstanceRef.current as { destroy: () => void }).destroy();
        pieChartInstanceRef.current = null;
      }
      
      const canShowPie = !selectedHabitId && sessions.length > 0 && habits.length > 0;
      console.log("[Stats renderCharts] pie chart check:", { canShowPie });
      
      if (canShowPie) {
        const habitTimeMap = new Map<number, number>();
        
        console.log("[Stats renderCharts] sessions for mapping:", sessions.map(s => ({ habit_id: s.habit_id, duration: s.duration_seconds })));
        
        sessions.filter(s => s).forEach((s) => {
          const rawId = s.habit_id;
          let habitIdKey: number;
          
          if (typeof rawId === 'string') {
            try {
              habitIdKey = Number(BigInt(rawId));
            } catch {
              habitIdKey = parseInt(rawId, 10);
            }
          } else {
            habitIdKey = Number(rawId);
          }
          
          if (!Number.isFinite(habitIdKey) || habitIdKey > 9007199254740991 || habitIdKey < -9007199254740991) {
            console.log("[Stats renderCharts] skipping invalid habit_id:", rawId);
            return;
          }
          
          const current = habitTimeMap.get(habitIdKey) || 0;
          habitTimeMap.set(habitIdKey, current + (s.duration_seconds || 0));
        });
        
        console.log("[Stats renderCharts] habitTimeMap after conversion:", Object.fromEntries(habitTimeMap));
        
        const validHabits = habits.filter(h => {
          const hId = Number(h.id);
          return Number.isFinite(hId) && hId <= 9007199254740991 && hId >= -9007199254740991;
        });
        
        const series = validHabits.map((h) => {
          const hId = Number(h.id);
          return habitTimeMap.get(hId) || 0;
        });
        const labels = validHabits.map((h) => h.name);
        const colors = validHabits.map((h) => h.color || "#6366f1");
        
        console.log("[Stats renderCharts] validHabits:", validHabits.map(h => ({ id: h.id, name: h.name })));
        console.log("[Stats renderCharts] series:", series, "habits id:", validHabits.map(h => Number(h.id)));
        
        const hasPositiveData = series.some((s) => s > 0);
        console.log("[Stats renderCharts] pie data:", { series, labels, hasPositiveData });
        
        if (hasPositiveData) {
          const ApexChartsModule = await import("apexcharts");
          const ApexCharts = ApexChartsModule.default;
          
          const pieOptions = {
            series: series,
            labels: labels,
            chart: {
              type: "donut",
              height: 300,
              background: "transparent",
            },
            colors: colors,
            plotOptions: {
              pie: {
                donut: {
                  labels: {
                    show: true,
                    name: { show: true, color: "#fff" },
                    value: { 
                      show: true, 
                      color: "#fff",
                      formatter: (val: unknown) => formatDuration(Number(val))
                    },
                    total: {
                      show: true,
                      label: "总计",
                      color: "#fff",
                      formatter: () => formatDuration(series.reduce((a, b) => a + b, 0))
                    }
                  }
                }
              }
            },
            legend: { position: "bottom", labels: { colors: "#fff" } },
            dataLabels: { enabled: false },
            stroke: { show: false },
            theme: { mode: "dark" }
          };
          
          pieChartInstanceRef.current = new ApexCharts(pieChartRef.current, pieOptions as Record<string, unknown>);
          void (pieChartInstanceRef.current as { render: () => void }).render();
        }
      }
    }
    
    if (barChartRef.current) {
      if (barChartInstanceRef.current && typeof (barChartInstanceRef.current as { destroy: () => void }).destroy === "function") {
        (barChartInstanceRef.current as { destroy: () => void }).destroy();
        barChartInstanceRef.current = null;
      }
      
      console.log("[Stats renderCharts] bar chart check:", { 
        filteredSessionsCount: filteredSessions.length,
        selectedHabitId,
        targetHabitId
      });
      
      const dailyMap = new Map<string, number>();
      filteredSessions.filter(s => s).forEach((s) => {
        const current = dailyMap.get(s.date) || 0;
        dailyMap.set(s.date, current + (s.duration_seconds || 0));
      });
      
      console.log("[Stats renderCharts] dailyMap:", Object.fromEntries(dailyMap));
      
      const sortedDates = Array.from(dailyMap.keys()).sort();
      const seriesData = sortedDates.map(d => Math.round((dailyMap.get(d) || 0) / 60));
      
      console.log("[Stats renderCharts] bar chart:", { sortedDates, seriesData, willRender: seriesData.length > 0 });
      
      if (seriesData.length > 0) {
        const ApexChartsModule = await import("apexcharts");
        const ApexCharts = ApexChartsModule.default;
        
        const barOptions = {
          series: [{ name: "专注分钟", data: seriesData }],
          chart: {
            type: "bar",
            height: 350,
            background: "transparent",
            toolbar: { show: false },
            animations: { enabled: true }
          },
          plotOptions: {
            bar: {
              borderRadius: 4,
              columnWidth: "60%",
              horizontal: false
            }
          },
          xaxis: {
            categories: sortedDates,
            labels: { 
              style: { colors: "#9ca3af" },
              rotate: -45,
              maxHeight: 80
            }
          },
          yaxis: {
            labels: { 
              style: { colors: "#9ca3af" },
              formatter: (val: number) => `${val}m`
            }
          },
          colors: ["#6366f1"],
          grid: { borderColor: "#374151" },
          theme: { mode: "dark" },
          responsive: [{
            breakpoint: 480,
            options: {
              chart: { height: 200 },
              plotOptions: { bar: { columnWidth: "80%" } }
            }
          }]
        };
        
        barChartInstanceRef.current = new ApexCharts(barChartRef.current, barOptions as Record<string, unknown>);
        void (barChartInstanceRef.current as { render: () => void }).render();
      }
    }
  };

  let filteredForStats: { habit_id: unknown; duration_seconds: number }[] = sessions;
  let filteredSessions: { habit_id: unknown; duration_seconds: number; date: string }[] = sessions;
  if (selectedHabitId !== null) {
    const rawId = selectedHabitId;
    let targetId: number;
    if (typeof rawId === 'string') {
      try {
        targetId = Number(BigInt(rawId));
      } catch {
        targetId = parseInt(rawId, 10);
      }
    } else {
      targetId = Number(rawId);
    }
    filteredForStats = sessions.filter(s => {
      const sId = Number(s.habit_id);
      return Number.isFinite(sId) && sId === targetId;
    });
    filteredSessions = filteredForStats as typeof filteredSessions;
  }
  
  const totalSeconds = filteredForStats.reduce((sum, s) => sum + (s?.duration_seconds || 0), 0);
  const totalSessions = filteredForStats.length;

  return (
    <div className="flex flex-col flex-1 bg-transparent pb-16 lg:pb-0 min-h-0">
      <Header
        title={t("stats.title") || "统计"}
        showSettings={false}
        showBack={true}
        onBackClick={onBackClick}
      />

      <div className="my-surface-panel flex gap-2 p-4">
        {(["today", "week", "month"] as TimeRange[]).map((range) => (
          <button
            key={range}
            className={`btn btn-sm ${timeRange === range ? "btn-primary" : "btn-ghost"}`}
            onClick={() => setTimeRange(range)}
          >
            {range === "today" ? t("stats.today") : range === "week" ? t("stats.this_week") : t("stats.this_month")}
          </button>
        ))}
      </div>

      <div className="my-surface-panel flex gap-2 p-4 overflow-x-auto">
        <button
          className={`btn btn-sm ${selectedHabitId === null ? "btn-primary" : "btn-ghost"}`}
          onClick={() => setSelectedHabitId(null)}
        >
          {t("stats.all")}
        </button>
        {habits.map((h) => (
          <button
            key={h.id}
            className={`btn btn-sm ${selectedHabitId === h.id ? "btn-primary" : "btn-ghost"}`}
            onClick={() => setSelectedHabitId(h.id)}
          >
            <span className="w-2 h-2 rounded-full mr-1" style={{ backgroundColor: h.color }} />
            {h.name}
          </button>
        ))}
      </div>

      <div className="flex-1 overflow-y-auto p-4 space-y-6 min-h-0">
        <div className="grid grid-cols-2 gap-4">
          <div className="card my-surface-card">
            <div className="card-body p-4">
              <h3 className="text-sm text-base-content/60">{t("stats.total_focus_time")}</h3>
              <p className="text-2xl font-bold text-primary">{formatDurationShort(totalSeconds)}</p>
            </div>
          </div>
          <div className="card my-surface-card">
            <div className="card-body p-4">
              <h3 className="text-sm text-base-content/60">{t("stats.completion_count")}</h3>
              <p className="text-2xl font-bold text-secondary">{totalSessions}</p>
            </div>
          </div>
        </div>

        {!selectedHabitId && (
          <div className="card my-surface-card">
            <div className="card-body p-4">
              <h3 className="text-lg font-semibold mb-4">{t("stats.time_distribution")}</h3>
              {isLoading ? (
                <div className="h-[300px] flex items-center justify-center">
                  <span className="loading loading-spinner loading-lg"></span>
                </div>
              ) : sessions.length === 0 ? (
                <div className="h-[300px] flex items-center justify-center text-base-content/50">
                  {t("stats.no_data")}
                </div>
              ) : (
                <div ref={pieChartRef}></div>
              )}
            </div>
          </div>
        )}

        <div className="card my-surface-card">
          <div className="card-body p-4">
            <h3 className="text-lg font-semibold mb-4">{t("stats.daily_trend")}</h3>
            {isLoading ? (
              <div className="h-[300px] flex items-center justify-center">
                <span className="loading loading-spinner loading-lg"></span>
              </div>
            ) : filteredSessions.length === 0 ? (
              <div className="h-[300px] flex items-center justify-center text-base-content/50">
                {t("stats.no_data")}
              </div>
            ) : (
              <div ref={barChartRef} className="w-full"></div>
            )}
          </div>
        </div>
      </div>
    </div>
  );
};