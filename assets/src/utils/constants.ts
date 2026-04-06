/**
 * 常量定义
 * 统一所有重复使用的常量
 */

// 计时器默认值
export const TIMER_DEFAULTS = {
  WORK_DURATION: 25 * 60, // 25 分钟
  REST_DURATION: 5 * 60,  // 5 分钟
  LOOP_COUNT: 0,
} as const;

// 存储键名
export const STORAGE_KEYS = {
  WALLPAPER: "global_wallpaper",
  WALLPAPER_DEBUG: "debug_wallpaper",
  LAYOUT_DENSITY: "layout_density",
  TIME_DISPLAY_STYLE: "time_display_style",
} as const;

// 布局密度选项
export type LayoutDensity = "compact" | "normal" | "spacious";

export const LAYOUT_DENSITY_OPTIONS: { value: LayoutDensity; label: string }[] = [
  { value: "compact", label: "紧凑" },
  { value: "normal", label: "标准" },
  { value: "spacious", label: "宽松" },
];

// 时间显示风格
export type TimeDisplayStyle = "classic" | "seven_segment";

export const TIME_DISPLAY_STYLE_OPTIONS: { value: TimeDisplayStyle; label: string }[] = [
  { value: "classic", label: "经典" },
  { value: "seven_segment", label: "数码管" },
];

// 计时模式
export const TIMER_MODES = {
  COUNTDOWN: "countdown",
  STOPWATCH: "stopwatch",
} as const;

// 页面类型
export type Page = "timer" | "habits" | "stats" | "settings";

// 壁纸回退
export const WALLPAPER_FALLBACK_GRADIENT =
  "linear-gradient(135deg, #0d0d0d 0%, #1a1a1a 50%, #0d0d0d 100%)";

// API 端点
export const API_ENDPOINTS = {
  STATE: "/api/state",
  START: "/api/start",
  PAUSE: "/api/pause",
  RESET: "/api/reset",
  MODE: "/api/mode",
  SETTINGS: "/api/settings",
  TIMER_PROGRESS: "/api/timer/progress",
  TIMER_FINISH: "/api/timer/finish",
  TIMER_REST: "/api/timer/rest",
  HABIT_SETS: "/api/habit-sets",
  HABITS: "/api/habits",
  SESSIONS: "/api/sessions",
  EVENTS: "/api/events",
} as const;

// 应用版本
export const APP_VERSION = "1.0.0";

// 默认时区
export const DEFAULT_TIMEZONE = 8;

// 语言选项
export const LANGUAGE_OPTIONS = [
  { value: "ZH", label: "中文" },
  { value: "EN", label: "English" },
] as const;

// 主题模式
export const THEME_MODES = {
  LIGHT: "light",
  DARK: "dark",
  AUTO: "auto",
} as const;
