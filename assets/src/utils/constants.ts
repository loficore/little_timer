/**
 * 常量定义
 * 统一所有重复使用的常量
 */

export const TIMER_DEFAULTS = {
  WORK_DURATION: 25 * 60,
  REST_DURATION: 5 * 60,
  LOOP_COUNT: 0,
} as const;

export const STORAGE_KEYS = {
  WALLPAPER: "global_wallpaper",
  WALLPAPER_DEBUG: "debug_wallpaper",
  LAYOUT_DENSITY: "layout_density",
  TIME_DISPLAY_STYLE: "time_display_style",
  LIGHT_STYLE: "lt_light_style",
  THEME_MODE: "lt_theme_mode",
} as const;

export const DEFAULT_API_URL = "http://localhost:8013";

export type LayoutDensity = "compact" | "normal" | "spacious";

export const LAYOUT_DENSITY_OPTIONS: { value: LayoutDensity; label: string }[] = [
  { value: "compact", label: "紧凑" },
  { value: "normal", label: "标准" },
  { value: "spacious", label: "宽松" },
];

export type TimeDisplayStyle = "classic" | "seven_segment";

export const TIME_DISPLAY_STYLE_OPTIONS: { value: TimeDisplayStyle; label: string }[] = [
  { value: "classic", label: "经典" },
  { value: "seven_segment", label: "数码管" },
];

export const TIMER_MODES = {
  COUNTDOWN: "countdown",
  STOPWATCH: "stopwatch",
} as const;

export type Page = "timer" | "habits" | "stats" | "settings";

export const ALLOWED_WALLPAPER_DOMAINS = [
  "imgur.com",
  "unsplash.com",
  "picsum.photos",
] as const;

export const WALLPAPER_FALLBACK_GRADIENT =
  "linear-gradient(135deg, #0d0d0d 0%, #1a1a1a 50%, #0d0d0d 100%)";

export const WALLPAPER_LOCAL_PREFIX = "local:";

/**
 * 解析壁纸 URL：如果是 local: 前缀，拼接为后端图片服务路径
 * @param value 壁纸原始值
 * @param baseUrl API 基础 URL
 */
export function resolveWallpaperUrl(value: string): string {
    if (value.startsWith(WALLPAPER_LOCAL_PREFIX)) {
        const filename = value.slice(WALLPAPER_LOCAL_PREFIX.length);
        return `/api/wallpapers/${filename}`;
    }
    return value;
}

/**
 * 校验壁纸 URL 是否安全
 * @param url 壁纸 URL
 * @returns 是否允许
 */
export function isAllowedWallpaperUrl(url: string): boolean {
    if (url.startsWith(WALLPAPER_LOCAL_PREFIX)) return true;
    if (url.startsWith("/")) return true;
    if (!url.startsWith("http://") && !url.startsWith("https://")) return false;
    try {
        const hostname = new URL(url).hostname;
        return ALLOWED_WALLPAPER_DOMAINS.some(
            (d) => hostname === d || hostname.endsWith("." + d)
        );
    } catch {
        return false;
    }
}

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
  WALLPAPERS: "/api/wallpapers",
} as const;

export const APP_VERSION = "1.0.0";

export const DEFAULT_TIMEZONE = 8;

export const LANGUAGE_OPTIONS = [
  { value: "ZH", label: "中文" },
  { value: "EN", label: "English" },
] as const;

export const THEME_MODES = {
  LIGHT: "light",
  DARK: "dark",
  AUTO: "auto",
} as const;
