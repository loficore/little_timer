// 习惯追踪相关类型定义

export interface HabitSet {
  id: number;
  name: string;
  description: string;
  color: string;
  created_at?: string;
}

export interface Habit {
  id: number;
  set_id: number;
  name: string;
  goal_seconds: number;
  color: string;
  created_at?: string;
}

export interface Session {
  id: number;
  habit_id: number;
  duration_seconds: number;
  count: number;
  started_at: string;
  date: string;
}

export interface HabitWithProgress extends Habit {
  today_seconds: number;
  today_count: number;
  progress: number; // 0-100 percentage
}

export interface HabitSetWithHabits extends HabitSet {
  habits: HabitWithProgress[];
}

export interface DailyStats {
  date: string;
  total_seconds: number;
  total_count: number;
  habits: {
    habit_id: number;
    habit_name: string;
    habit_color: string;
    seconds: number;
    count: number;
  }[];
}

export interface StatsSummary {
  total_days: number;
  total_seconds: number;
  total_sessions: number;
  current_streak: number;
  longest_streak: number;
  average_per_day: number;
  completion_rate: number;
}

export interface DateRange {
  start: string;
  end: string;
}
