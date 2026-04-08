import type { FunctionalComponent } from "preact";
import { useState, useEffect, useRef, useMemo } from "preact/hooks";
import { Header } from "./components/Header";
import { t } from "./utils/i18n";
import { getAPIClient } from "./utils/apiClientSingleton";
import { formatDurationShort, formatDuration, getToday, getDaysAgo } from "./utils/formatters";
import type { ApexOptions, ApexCharts as ApexChartsClass } from "apexcharts";
import { isPerfDebugEnabled, logPerf } from "./utils/logger";

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

const parseHabitId = (raw: unknown): number | null => {
  if (typeof raw === "number") {
    return Number.isFinite(raw) ? raw : null;
  }

  if (typeof raw === "string") {
    const trimmed = raw.trim();
    if (!trimmed) return null;

    const numeric = Number(trimmed);
    return Number.isFinite(numeric) ? numeric : null;
  }

  return null;
};

const getIsWebView = (): boolean => {
  if (typeof window === "undefined") return false;
  return !!window.webui;
};

export const StatsPage: FunctionalComponent<StatsPageProps> = ({ onBackClick }) => {
  const [timeRange, setTimeRange] = useState<TimeRange>("week");
  const [habits, setHabits] = useState<Habit[]>([]);
  const [sessions, setSessions] = useState<Session[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [selectedHabitId, setSelectedHabitId] = useState<number | null>(null);
  
  const pieChartRef = useRef<HTMLDivElement>(null);
  const barChartRef = useRef<HTMLDivElement>(null);
  const pieChartInstanceRef = useRef<ApexChartsClass | null>(null);
  const barChartInstanceRef = useRef<ApexChartsClass | null>(null);
  const ApexCtorRef = useRef<typeof ApexChartsClass | null>(null);
  const isWebViewRef = useRef(getIsWebView());

  const loadApexCtor = async (): Promise<typeof ApexChartsClass> => {
    if (!ApexCtorRef.current) {
      const ApexChartsModule = await import("apexcharts");
      ApexCtorRef.current = ApexChartsModule.default;
    }
    return ApexCtorRef.current;
  };

  const loadData = async () => {
    const startAt = performance.now();
    setIsLoading(true);
    try {
      const client = getAPIClient();
      const habitsData = await client.getHabits();
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
      
      if (startDate && endDate) {
        const sessionsData = await client.getSessions(undefined, startDate, endDate);
        setSessions(Array.isArray(sessionsData) ? sessionsData : []);
      } else {
        setSessions([]);
      }

      logPerf("Stats.loadData.success", {
        timeRange,
        habits: Array.isArray(habitsData) ? habitsData.length : 0,
        durationMs: Math.round(performance.now() - startAt),
      });
    } catch (e) {
      console.error("[Stats] loadData error:", e);
      logPerf("Stats.loadData.error", {
        timeRange,
        durationMs: Math.round(performance.now() - startAt),
      });
    } finally {
      setIsLoading(false);
    }
  };

  useEffect(() => {
    void loadData();
  }, [timeRange]);

  useEffect(() => {
    return () => {
      pieChartInstanceRef.current?.destroy();
      barChartInstanceRef.current?.destroy();
      pieChartInstanceRef.current = null;
      barChartInstanceRef.current = null;
    };
  }, []);

  const normalizedSelectedHabitId = useMemo(() => {
    if (selectedHabitId === null) return null;
    return parseHabitId(selectedHabitId);
  }, [selectedHabitId]);

  const filteredSessions = useMemo(() => {
    if (normalizedSelectedHabitId === null) return sessions;
    return sessions.filter((s) => parseHabitId(s.habit_id) === normalizedSelectedHabitId);
  }, [sessions, normalizedSelectedHabitId]);

  const pieData = useMemo(() => {
    const canShow = normalizedSelectedHabitId === null && sessions.length > 0 && habits.length > 0;
    if (!canShow) {
      return { canShow: false, series: [] as number[], labels: [] as string[], colors: [] as string[] };
    }

    const totalsByHabit = new Map<number, number>();
    for (const session of sessions) {
      const habitId = parseHabitId(session.habit_id);
      if (habitId === null) continue;
      totalsByHabit.set(habitId, (totalsByHabit.get(habitId) || 0) + (session.duration_seconds || 0));
    }

    const validHabits = habits.filter((habit) => parseHabitId(habit.id) !== null);
    const series = validHabits.map((habit) => totalsByHabit.get(parseHabitId(habit.id) as number) || 0);
    const hasPositiveData = series.some((value) => value > 0);

    if (!hasPositiveData) {
      return { canShow: false, series: [] as number[], labels: [] as string[], colors: [] as string[] };
    }

    return {
      canShow: true,
      series,
      labels: validHabits.map((habit) => habit.name),
      colors: validHabits.map((habit) => habit.color || "#6366f1"),
    };
  }, [sessions, habits, normalizedSelectedHabitId]);

  const barData = useMemo(() => {
    const dailyMap = new Map<string, number>();
    for (const session of filteredSessions) {
      dailyMap.set(session.date, (dailyMap.get(session.date) || 0) + (session.duration_seconds || 0));
    }

    const sortedDates = Array.from(dailyMap.keys()).sort();
    const seriesData = sortedDates.map((date) => Math.round((dailyMap.get(date) || 0) / 60));

    return {
      categories: sortedDates,
      seriesData,
    };
  }, [filteredSessions]);

  useEffect(() => {
    if (isLoading) return;

    const updateStartAt = performance.now();

    const renderPieChart = async () => {
      const pieStartAt = performance.now();
      if (!pieChartRef.current) return;

      if (!pieData.canShow) {
        pieChartInstanceRef.current?.destroy();
        pieChartInstanceRef.current = null;
        return;
      }

      const totalSeconds = pieData.series.reduce((sum, value) => sum + value, 0);
      const pieOptions: ApexOptions = {
        series: pieData.series,
        labels: pieData.labels,
        chart: {
          type: "donut",
          height: 300,
          background: "transparent",
          animations: { enabled: !isWebViewRef.current },
        },
        colors: pieData.colors,
        plotOptions: {
          pie: {
            donut: {
              labels: {
                show: true,
                name: { show: true, color: "#fff" },
                value: {
                  show: true,
                  color: "#fff",
                  formatter: (val: number) => formatDuration(val),
                },
                total: {
                  show: true,
                  label: "总计",
                  color: "#fff",
                  formatter: () => formatDuration(totalSeconds),
                },
              },
            },
          },
        },
        legend: { position: "bottom", labels: { colors: "#fff" } },
        dataLabels: { enabled: false },
        stroke: { show: false },
        theme: { mode: "dark" },
      };

      if (!pieChartInstanceRef.current) {
        const ApexCharts = await loadApexCtor();
        pieChartInstanceRef.current = new ApexCharts(pieChartRef.current, pieOptions);
        await pieChartInstanceRef.current.render();
        logPerf("Stats.chart.pie.initialRender", {
          points: pieData.series.length,
          durationMs: Math.round(performance.now() - pieStartAt),
        });
        return;
      }

      pieChartInstanceRef.current.updateOptions(pieOptions, false, false);
      pieChartInstanceRef.current.updateSeries(pieData.series, false);
      logPerf("Stats.chart.pie.update", {
        points: pieData.series.length,
        durationMs: Math.round(performance.now() - pieStartAt),
      });
    };

    const renderBarChart = async () => {
      const barStartAt = performance.now();
      if (!barChartRef.current) return;

      if (barData.seriesData.length === 0) {
        barChartInstanceRef.current?.destroy();
        barChartInstanceRef.current = null;
        return;
      }

      const barOptions: ApexOptions = {
        series: [{ name: "专注分钟", data: barData.seriesData }],
        chart: {
          type: "bar",
          height: 350,
          background: "transparent",
          toolbar: { show: false },
          animations: { enabled: !isWebViewRef.current },
        },
        plotOptions: {
          bar: {
            borderRadius: 4,
            columnWidth: "60%",
            horizontal: false,
          },
        },
        xaxis: {
          categories: barData.categories,
          labels: {
            style: { colors: "#9ca3af" },
          },
        },
        yaxis: {
          labels: {
            style: { colors: "#9ca3af" },
            formatter: (val: number) => `${val}m`,
          },
        },
        colors: ["#6366f1"],
        grid: { borderColor: "#374151" },
        theme: { mode: "dark" },
        responsive: [{
          breakpoint: 480,
          options: {
            chart: { height: 200 },
            plotOptions: { bar: { columnWidth: "80%" } },
          },
        }],
      };

      if (!barChartInstanceRef.current) {
        const ApexCharts = await loadApexCtor();
        barChartInstanceRef.current = new ApexCharts(barChartRef.current, barOptions);
        await barChartInstanceRef.current.render();
        logPerf("Stats.chart.bar.initialRender", {
          points: barData.seriesData.length,
          durationMs: Math.round(performance.now() - barStartAt),
        });
        return;
      }

      barChartInstanceRef.current.updateOptions(barOptions, false, false);
      barChartInstanceRef.current.updateSeries([{ name: "专注分钟", data: barData.seriesData }], false);
      logPerf("Stats.chart.bar.update", {
        points: barData.seriesData.length,
        durationMs: Math.round(performance.now() - barStartAt),
      });
    };

    void renderPieChart();
    void renderBarChart();

    requestAnimationFrame(() => {
      logPerf("Stats.effect.frame", {
        sessions: sessions.length,
        filteredSessions: filteredSessions.length,
        totalEffectMs: Math.round(performance.now() - updateStartAt),
      });
    });
  }, [isLoading, pieData, barData]);

  useEffect(() => {
    if (!isPerfDebugEnabled()) return;
    logPerf("Stats.filter.changed", {
      timeRange,
      selectedHabitId,
      sessions: sessions.length,
      filtered: filteredSessions.length,
      piePoints: pieData.series.length,
      barPoints: barData.seriesData.length,
    });
  }, [timeRange, selectedHabitId, sessions.length, filteredSessions.length, pieData.series.length, barData.seriesData.length]);

  const totalSeconds = useMemo(() => {
    return filteredSessions.reduce((sum, session) => sum + (session.duration_seconds || 0), 0);
  }, [filteredSessions]);

  const totalSessions = filteredSessions.length;

  return (
    <div className="flex flex-col flex-1 bg-transparent pb-16 lg:pb-0 min-h-0">
      <Header
        title={t("stats.title") || "统计"}
        showSettings={false}
        showBack={true}
        onBackClick={onBackClick}
      />

      <div className="flex-1 overflow-y-auto px-4 pb-4 space-y-6 min-h-0">
        <div className="my-surface-panel flex flex-col gap-4 p-4">
          <div className="flex flex-wrap gap-2">
            {(["today", "week", "month"] as TimeRange[]).map((range) => (
              <button
                key={range}
                className={`my-filter-btn ${timeRange === range ? "my-filter-btn-active" : ""}`}
                onClick={() => setTimeRange(range)}
              >
                {range === "today" ? t("stats.today") : range === "week" ? t("stats.this_week") : t("stats.this_month")}
              </button>
            ))}
          </div>

          <div className="h-px bg-[color:color-mix(in_oklab,var(--my-outline)_18%,transparent)]" />

          <div className="flex gap-2 overflow-x-auto pb-1">
            <button
              className={`my-filter-btn ${selectedHabitId === null ? "my-filter-btn-active" : ""}`}
              onClick={() => setSelectedHabitId(null)}
            >
              {t("stats.all")}
            </button>
            {habits.map((h) => (
              <button
                key={h.id}
                className={`my-filter-btn ${selectedHabitId === h.id ? "my-filter-btn-active" : ""}`}
                onClick={() => setSelectedHabitId(h.id)}
              >
                <span className="w-2 h-2 rounded-full mr-1" style={{ backgroundColor: h.color }} />
                {h.name}
              </button>
            ))}
          </div>
        </div>

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
            ) : barData.seriesData.length === 0 ? (
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