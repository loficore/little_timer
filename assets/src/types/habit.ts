import type { Habit, HabitSet, Session, HabitDetail } from "./api";

/**
 * DTO 单一来源：`HabitSet` / `Habit` / `Session` / `HabitDetail` 的声明在
 * `types/api.ts`，这里只做转发，避免两份定义各自漂移。
 */
export type { Habit, HabitSet, Session, HabitDetail };

/**
 * @description 包含今日进度的习惯信息
 */
export interface HabitWithProgress extends Habit {
  /** 今日累计时长（秒） */
  today_seconds: number;
  /** 今日完成次数 */
  today_count: number;
  /** 当前进度百分比（0-100） */
  progress: number;
}

/**
 * @description 包含习惯列表的集合信息
 */
export interface HabitSetWithHabits extends HabitSet {
  /** 集合中的习惯列表 */
  habits: HabitWithProgress[];
}

/**
 * @description 每日习惯统计数据
 */
export interface DailyStats {
  /** 统计日期 */
  date: string;
  /** 当日总时长（秒） */
  total_seconds: number;
  /** 当日总完成次数 */
  total_count: number;
  /** 各习惯的当日统计 */
  habits: {
    /** 习惯 ID */
    habit_id: number;
    /** 习惯名称 */
    habit_name: string;
    /** 习惯颜色 */
    habit_color: string;
    /** 习惯累计时长（秒） */
    seconds: number;
    /** 习惯完成次数 */
    count: number;
  }[];
}

/**
 * @description 习惯统计汇总数据
 */
export interface StatsSummary {
  /** 有记录的总天数 */
  total_days: number;
  /** 累计总时长（秒） */
  total_seconds: number;
  /** 累计总会话数 */
  total_sessions: number;
  /** 日均时长（秒） */
  average_per_day: number;
  /** 目标完成率 */
  completion_rate: number;
}

/**
 * @description 统计查询的日期范围
 */
export interface DateRange {
  /** 开始日期 */
  start: string;
  /** 结束日期 */
  end: string;
}
