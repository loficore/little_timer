// API 响应类型定义

export interface TimerState {
  time: number;
  elapsed?: number;
  mode: "countdown" | "stopwatch";
  is_running: boolean;
  is_finished: boolean;
  in_rest: boolean;
  loop_remaining: number;
  loop_total: number;
  rest_remaining: number;
  timezone: number;
  habit_id?: number;
}

export interface BasicSettings {
  timezone: number;
  language: string;
  default_mode: string;
  theme_mode: string;
  wallpaper?: string;
  sound_enabled?: boolean;
  sound_tick?: boolean;
  sound_finish?: boolean;
  sound_volume?: number;
}

export interface CountdownDefaults {
  duration_seconds: number;
  loop: boolean;
  loop_count: number;
  loop_interval_seconds: number;
}

export interface StopwatchDefaults {
  max_seconds: number;
}

export interface Settings {
  basic: BasicSettings;
  countdown?: CountdownDefaults;
  stopwatch?: StopwatchDefaults;
  world_clock?: object;
}

export interface HabitSet {
  id: number;
  name: string;
  description: string;
  color: string;
  wallpaper?: string;
  created_at?: string;
}

export interface Habit {
  id: number;
  set_id: number;
  name: string;
  goal_seconds: number;
  color: string;
  wallpaper?: string;
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

export interface HabitDetail {
  id: number;
  name: string;
  goal_seconds: number;
  color: string;
  today_seconds: number;
  streak: number;
  progress_percent: number;
}

export interface HabitStreak {
  habit_id: number;
  streak: number;
}

export interface TimerProgress {
  session_id: number | null;
  habit_id: number | null;
  mode: string;
  is_running: boolean;
  is_paused: boolean;
  is_finished: boolean;
  elapsed_seconds: number;
  remaining_seconds: number;
  in_rest: boolean;
}

export interface TimerStartResult {
  habit_id: number | null;
  session_id?: number;
}

export interface TimerFinishResult {
  status: string;
  elapsed_seconds: number;
  session_id?: number;
}

export interface RestResult {
  rest_seconds: number;
}

export interface ResumeResult {
  habit_id: number | null;
}

export interface CreateSessionResult {
  id?: number;
  habit_id: number;
  duration_seconds: number;
  count: number;
  date: string;
}

export interface BackupConfig {
  enabled: boolean;
  target_type: "local" | "webdav" | "s3";
  local_path?: string;
  webdav_url?: string;
  webdav_username?: string;
  webdav_password?: string;
  s3_endpoint?: string;
  s3_bucket?: string;
  s3_region?: string;
  s3_access_key?: string;
  s3_secret_key?: string;
  s3_path_prefix?: string;
  auto_interval_hours?: number;
  max_backups?: number;
}

export interface BackupListItem {
  name: string;
  timestamp: number;
  size_bytes: number;
}

export interface BackupListResult {
  success: boolean;
  backups: BackupListItem[];
  error?: string;
}

export interface BackupCreateResult {
  success: boolean;
  backup_path?: string;
  error?: string;
}

export interface BackupRestoreResult {
  success: boolean;
  error?: string;
}

export interface BackupVerifyResult {
  success: boolean;
  error?: string;
}

export interface WallpaperUploadResult {
  filename: string;
}

export interface WallpaperListResult {
  name: string;
}

export interface WallpaperDeleteResult {
  success: boolean;
}

export interface TimerStartOptions {
  mode?: "countdown" | "stopwatch";
  workDuration?: number;
  restDuration?: number;
  loopCount?: number;
}