/**
 * 格式化工具函数
 * 统一所有时间格式化逻辑
 */

export type TimerMode = "stopwatch" | "countdown";

/**
 * 格式化持续时间（秒）为 HH:MM:SS 格式
 * 用于倒计时/秒表显示
 */
export const formatDuration = (totalSeconds: number): string => {
  const hours = Math.floor(totalSeconds / 3600);
  const minutes = Math.floor((totalSeconds % 3600) / 60);
  const seconds = Math.floor(totalSeconds % 60);

  if (hours > 0) {
    return `${hours}:${String(minutes).padStart(2, "0")}:${String(seconds).padStart(2, "0")}`;
  }
  return `${String(minutes).padStart(2, "0")}:${String(seconds).padStart(2, "0")}`;
};

/**
 * 格式化持续时间为简写格式
 * 用于统计卡片和列表显示
 * 例如: 2h 30min, 0h 45min
 */
export const formatDurationShort = (totalSeconds: number): string => {
  const hours = Math.floor(totalSeconds / 3600);
  const minutes = Math.floor((totalSeconds % 3600) / 60);
  return `${hours}h ${minutes}min`;
};

/**
 * 将分钟数转换为秒数
 */
export const minutesToSeconds = (minutes: number): number => {
  return minutes * 60;
};

/**
 * 将秒数转换为分钟数（向下取整）
 */
export const secondsToMinutes = (seconds: number): number => {
  return Math.floor(seconds / 60);
};

/**
 * 格式化目标时长（秒）用于显示
 */
export const formatGoalDuration = (goalSeconds: number): string => {
  const hours = Math.floor(goalSeconds / 3600);
  const minutes = Math.floor((goalSeconds % 3600) / 60);

  if (hours > 0) {
    return `${hours} 小时 ${minutes} 分钟`;
  }
  return `${minutes} 分钟`;
};

/**
 * 计算进度百分比
 */
export const calculateProgress = (
  current: number,
  goal: number
): number => {
  if (goal <= 0) return 0;
  return Math.min(Math.floor((current * 100) / goal), 100);
};

/**
 * 格式化日期为 YYYY-MM-DD
 */
export const formatDate = (date: Date): string => {
  return date.toISOString().split("T")[0];
};

/**
 * 获取今天的日期字符串
 */
export const getToday = (): string => {
  return formatDate(new Date());
};

/**
 * 获取 N 天前的日期
 */
export const getDaysAgo = (days: number): string => {
  const date = new Date();
  date.setDate(date.getDate() - days);
  return formatDate(date);
};
