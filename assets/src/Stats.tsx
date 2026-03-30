import type { FunctionalComponent } from "preact";
import { useState, useEffect, useRef } from "preact/hooks";
import { Header } from "./components/Header";
import { t } from "./utils/i18n";
import { APIClient } from "./utils/apiClient";
import ApexCharts from "apexcharts";

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
  const pieChartInstance = useRef<ApexCharts | null>(null);
  const barChartInstance = useRef<ApexCharts | null>(null);

  useEffect(() => {
    void loadData();
  }, [timeRange]);

  useEffect(() => {
    if (!isLoading && sessions.length >= 0) {
      renderCharts();
    }
    return () => {
      if (pieChartInstance.current) {
        pieChartInstance.current.destroy();
      }
      if (barChartInstance.current) {
        barChartInstance.current.destroy();
      }
    };
  }, [isLoading, sessions, habits, selectedHabitId]);

  const loadData = async () => {
    setIsLoading(true);
    try {
      const client = new APIClient(window.location.origin);
      const habitsData = await client.getHabits();
      setHabits(Array.isArray(habitsData) ? habitsData : []);
      
      let startDate = "";
      let endDate = "";
      const today = new Date().toISOString().split("T")[0];
      
      if (timeRange === "today") {
        startDate = endDate = today;
      } else if (timeRange === "week") {
        const weekAgo = new Date();
        weekAgo.setDate(weekAgo.getDate() - 7);
        startDate = weekAgo.toISOString().split("T")[0];
        endDate = today;
      } else if (timeRange === "month") {
        const monthAgo = new Date();
        monthAgo.setDate(monthAgo.getDate() - 30);
        startDate = monthAgo.toISOString().split("T")[0];
        endDate = today;
      }
      
      if (startDate && endDate) {
        const sessionsData = await client.getSessions(undefined, startDate, endDate);
        setSessions(Array.isArray(sessionsData) ? sessionsData : []);
      } else {
        setSessions([]);
      }
    } catch (e) {
      console.error("Failed to load stats:", e);
    } finally {
      setIsLoading(false);
    }
  };

  const renderCharts = () => {
    const filteredSessions = selectedHabitId 
      ? sessions.filter(s => s && s.habit_id === selectedHabitId)
      : sessions;

    // Pie chart - time distribution (only when no habit filter)
    if (pieChartRef.current) {
      if (pieChartInstance.current) {
        pieChartInstance.current.destroy();
        pieChartInstance.current = null;
      }
      
      if (!selectedHabitId && sessions.length > 0) {
        const habitTimeMap = new Map<number, number>();
        sessions.filter(s => s).forEach((s) => {
          const current = habitTimeMap.get(s.habit_id) || 0;
          habitTimeMap.set(s.habit_id, current + (s.duration_seconds || 0));
        });
        
        const series = habits.map((h) => habitTimeMap.get(h.id) || 0);
        const labels = habits.map((h) => h.name);
        
        if (series.some((s) => s > 0)) {
          const pieOptions: ApexCharts.ApexOptions = {
            series: series,
            labels: labels,
            chart: {
              type: "donut",
              height: 300,
              background: "transparent",
            },
            colors: habits.map((h) => h.color || "#6366f1"),
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
          
          // eslint-disable-next-line @typescript-eslint/no-unsafe-argument
          pieChartInstance.current = new ApexCharts(pieChartRef.current, pieOptions as any);
          void pieChartInstance.current.render();
        }
      }
    }
    
    // Bar chart - daily trend
    if (barChartRef.current) {
      if (barChartInstance.current) {
        barChartInstance.current.destroy();
        barChartInstance.current = null;
      }
      
      const dailyMap = new Map<string, number>();
      filteredSessions.filter(s => s).forEach((s) => {
        const current = dailyMap.get(s.date) || 0;
        dailyMap.set(s.date, current + (s.duration_seconds || 0));
      });
      
      const sortedDates = Array.from(dailyMap.keys()).sort();
      const seriesData = sortedDates.map(d => Math.round((dailyMap.get(d) || 0) / 60));
      
      if (seriesData.length > 0) {
        const barOptions: ApexCharts.ApexOptions = {
          series: [{ name: "专注分钟", data: seriesData }],
          chart: {
            type: "bar",
            height: 300,
            background: "transparent",
            toolbar: { show: false }
          },
          plotOptions: {
            bar: {
              borderRadius: 4,
              columnWidth: "60%"
            }
          },
          xaxis: {
            categories: sortedDates,
            labels: { style: { colors: "#9ca3af" } }
          },
          yaxis: {
            labels: { 
              style: { colors: "#9ca3af" },
              formatter: (val: number) => `${val}m`
            }
          },
          colors: ["#6366f1"],
          grid: { borderColor: "#374151" },
          theme: { mode: "dark" }
        };
        
        // eslint-disable-next-line @typescript-eslint/no-unsafe-argument
        barChartInstance.current = new ApexCharts(barChartRef.current, barOptions as any);
        void barChartInstance.current.render();
      }
    }
  };

  const formatDuration = (seconds: number): string => {
    const hours = Math.floor(seconds / 3600);
    const minutes = Math.floor((seconds % 3600) / 60);
    if (hours > 0) return `${hours}h ${minutes}m`;
    return `${minutes}m`;
  };

  const filteredSessions = selectedHabitId 
    ? sessions.filter(s => s && s.habit_id === selectedHabitId)
    : sessions;
  const totalSeconds = filteredSessions.reduce((sum, s) => sum + (s?.duration_seconds || 0), 0);
  const totalSessions = filteredSessions.length;

  return (
    <div className="flex flex-col flex-1 bg-base-100 dark:bg-base-100 transition-colors duration-300 overflow-hidden pb-16 lg:pb-0">
      <Header
        title={t("stats.title") || "统计"}
        showSettings={false}
        showBack={true}
        onBackClick={onBackClick}
      />

      {/* Time Range Selector */}
      <div className="flex gap-2 p-4 border-b border-base-300">
        {(["today", "week", "month"] as TimeRange[]).map((range) => (
          <button
            key={range}
            className={`btn btn-sm ${timeRange === range ? "btn-primary" : "btn-ghost"}`}
            onClick={() => setTimeRange(range)}
          >
            {range === "today" ? "今日" : range === "week" ? "本周" : "本月"}
          </button>
        ))}
      </div>

      {/* Habit Filter */}
      <div className="flex gap-2 p-4 border-b border-base-300 overflow-x-auto">
        <button
          className={`btn btn-sm ${selectedHabitId === null ? "btn-primary" : "btn-ghost"}`}
          onClick={() => setSelectedHabitId(null)}
        >
          全部
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

      <div className="flex-1 overflow-y-auto p-4 space-y-6">
        {/* Summary Cards */}
        <div className="grid grid-cols-2 gap-4">
          <div className="card bg-base-200 shadow-lg">
            <div className="card-body p-4">
              <h3 className="text-sm text-base-content/60">总专注时间</h3>
              <p className="text-2xl font-bold text-primary">{formatDuration(totalSeconds)}</p>
            </div>
          </div>
          <div className="card bg-base-200 shadow-lg">
            <div className="card-body p-4">
              <h3 className="text-sm text-base-content/60">完成次数</h3>
              <p className="text-2xl font-bold text-secondary">{totalSessions}</p>
            </div>
          </div>
        </div>

        {/* Pie Chart */}
        {!selectedHabitId && (
          <div className="card bg-base-200 shadow-lg">
            <div className="card-body p-4">
              <h3 className="text-lg font-semibold mb-4">时间分布</h3>
              {isLoading ? (
                <div className="h-[300px] flex items-center justify-center">
                  <span className="loading loading-spinner loading-lg"></span>
                </div>
              ) : sessions.length === 0 ? (
                <div className="h-[300px] flex items-center justify-center text-base-content/50">
                  暂无数据
                </div>
              ) : (
                <div ref={pieChartRef}></div>
              )}
            </div>
          </div>
        )}

        {/* Bar Chart */}
        <div className="card bg-base-200 shadow-lg">
          <div className="card-body p-4">
            <h3 className="text-lg font-semibold mb-4">每日趋势</h3>
            {isLoading ? (
              <div className="h-[300px] flex items-center justify-center">
                <span className="loading loading-spinner loading-lg"></span>
              </div>
            ) : filteredSessions.length === 0 ? (
              <div className="h-[300px] flex items-center justify-center text-base-content/50">
                暂无数据
              </div>
            ) : (
              <div ref={barChartRef}></div>
            )}
          </div>
        </div>
      </div>
    </div>
  );
};