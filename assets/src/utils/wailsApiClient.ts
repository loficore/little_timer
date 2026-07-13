/**
 * Platform-aware API client for Little Timer.
 *
 * On Android (detected via window.wails): uses Wails v3 JS↔Go bindings
 * via JNI — no HTTP fetch needed.
 *
 * On desktop: uses the existing fetch()-based APIClient.
 *
 * This allows the same frontend code to work on both platforms
 * without changing call sites — they keep using getAPIClient().
 */

import type {
  TimerState,
  TimerProgress,
  TimerStartOptions,
  TimerStartResult,
  TimerFinishResult,
  RestResult,
  ResumeResult,
  HabitSet,
  Habit,
  HabitDetail,
  HabitStreak,
  Session,
  CreateSessionResult,
  BackupConfig,
  BackupListResult,
  BackupCreateResult,
  BackupRestoreResult,
  BackupVerifyResult,
  WallpaperUploadResult,
  WallpaperListResult,
  WallpaperDeleteResult,
} from "../types/api";

import * as TimerService from "../bindings/little-timer/internal/app/timerservice.js";
import * as SettingsService from "../bindings/little-timer/internal/app/settingsservice.js";
import * as HabitService from "../bindings/little-timer/internal/app/habitservice.js";
import * as BackupService from "../bindings/little-timer/internal/app/backupservice.js";

/* eslint-disable @typescript-eslint/require-await */
// Detect Android: Wails v3 sets window.wails on Android
const isAndroid = typeof window !== "undefined" && !!(window as any).wails;

/**
 * Wails API 客户端 — 封装所有与后端通信的 Wails Go 绑定方法
 */
export class WailsAPIClient {
  // Exposed for useSSE which accesses apiClient.baseUrl — SSE is not used on Android
  baseUrl = "";
  /**
   * 获取当前计时器状态
   */
  async getState(): Promise<TimerState> {
    return TimerService.GetState();
  }

  /**
   * 启动计时器
   * @param habitId - 可选习惯 ID
   * @param options - 可选计时配置（模式、时长等）
   */
  async startTimer(habitId?: number, options?: TimerStartOptions): Promise<TimerStartResult> {
    return TimerService.StartTimer(
      habitId ?? null,
      options?.mode ?? "",
      options?.workDuration ?? 0,
      options?.restDuration ?? 0,
      options?.loopCount ?? 0
    );
  }

  /**
   * 结束计时器
   */
  async finishTimer(): Promise<TimerFinishResult> {
    return TimerService.FinishTimer();
  }

  /**
   * 获取计时器进度详情
   */
  async getTimerProgress(): Promise<TimerProgress> {
    return TimerService.GetProgress();
  }

  /**
   * 恢复计时器（等同于 startTimer）
   */
  async resumeTimer(habitId?: number): Promise<ResumeResult> {
    return this.startTimer(habitId);
  }

  /**
   * 开始休息时段
   */
  async startRest(): Promise<RestResult> {
    return TimerService.StartRest();
  }

  /**
   * 暂停计时器
   */
  async pauseTimer(): Promise<void> {
    return TimerService.PauseTimer();
  }

  /**
   * 重置计时器
   */
  async resetTimer(): Promise<void> {
    return TimerService.ResetTimer();
  }

  /**
   * 切换计时模式（仅桌面端，Android 无效）
   */
  async changeMode(_mode: "countdown" | "stopwatch"): Promise<void> {
    // Wails bindings don't expose changeMode — this is a no-op on Android
    // since mode is set via StartTimer options
    return Promise.resolve();
  }

  /**
   * 获取应用设置
   */
  async getSettings(): Promise<any> {
    return SettingsService.GetSettings();
  }

  /**
   * 更新应用设置
   * @param settings - 设置对象
   */
  async updateSettings(settings: object): Promise<void> {
    return SettingsService.UpdateSettings(JSON.stringify(settings));
  }

  /**
   * 获取所有习惯集
   */
  async getHabitSets(): Promise<HabitSet[]> {
    return HabitService.ListHabitSets();
  }

  /**
   * 创建新习惯集
   * @param name - 名称
   * @param description - 描述
   * @param color - 颜色
   */
  async createHabitSet(name: string, description: string, color: string): Promise<HabitSet> {
    return HabitService.CreateHabitSet(name, description, color);
  }

  /**
   * 更新习惯集
   * @param id - 习惯集 ID
   * @param name - 名称
   * @param description - 描述
   * @param color - 颜色
   * @param wallpaper - 可选壁纸
   */
  async updateHabitSet(id: number, name: string, description: string, color: string, wallpaper?: string): Promise<HabitSet> {
    return HabitService.UpdateHabitSet(id, name, description, color, wallpaper ?? "");
  }

  /**
   * 删除习惯集
   * @param id - 习惯集 ID
   */
  async deleteHabitSet(id: number): Promise<void> {
    return HabitService.DeleteHabitSet(id);
  }

  /**
   * 获取所有习惯
   */
  async getHabits(): Promise<Habit[]> {
    return HabitService.ListHabits(null);
  }

  /**
   * 创建新习惯
   * @param setId - 所属习惯集 ID
   * @param name - 名称
   * @param goalSeconds - 目标秒数
   * @param color - 颜色
   */
  async createHabit(setId: number, name: string, goalSeconds: number, color: string): Promise<Habit> {
    return HabitService.CreateHabit(setId, name, goalSeconds, color);
  }

  /**
   * 删除习惯
   * @param id - 习惯 ID
   */
  async deleteHabit(id: number): Promise<void> {
    return HabitService.DeleteHabit(id);
  }

  /**
   * 更新习惯
   * @param id - 习惯 ID
   * @param name - 名称
   * @param goalSeconds - 目标秒数
   * @param color - 颜色
   * @param wallpaper - 可选壁纸
   */
  async updateHabit(id: number, name: string, goalSeconds: number, color: string, wallpaper?: string): Promise<Habit> {
    return HabitService.UpdateHabit(id, name, goalSeconds, color, wallpaper ?? "");
  }

  /**
   * 记录一次习惯完成
   * @param habitId - 习惯 ID
   * @param durationSeconds - 实际时长（秒）
   * @param count - 完成次数
   * @param date - 日期字符串
   */
  async createSession(habitId: number, durationSeconds: number, count: number, date: string): Promise<CreateSessionResult> {
    return HabitService.CreateSession(habitId, durationSeconds, count, date);
  }

  /**
   * 获取习惯记录列表
   * @param date - 可选特定日期
   * @param startDate - 可选开始日期
   * @param endDate - 可选结束日期
   */
  async getSessions(date?: string, startDate?: string, endDate?: string): Promise<Session[]> {
    return HabitService.ListSessions(date ?? "", startDate ?? "", endDate ?? "");
  }

  /**
   * 获取习惯连续完成天数
   * @param habitId - 习惯 ID
   * @param goalSeconds - 可选目标秒数
   */
  async getHabitStreak(habitId: number, goalSeconds?: number): Promise<HabitStreak> {
    return HabitService.GetHabitStreak(habitId, goalSeconds ?? 0);
  }

  /**
   * 获取习惯详情（含今日进度和连续天数）
   * @param habitId - 习惯 ID
   * @param date - 可选日期
   */
  async getHabitDetail(habitId: number, date?: string): Promise<HabitDetail> {
    return HabitService.GetHabitDetail(habitId, date ?? "");
  }

  /**
   * 创建备份
   */
  async createBackup(): Promise<BackupCreateResult> {
    return BackupService.CreateBackup();
  }

  /**
   * 列出所有备份
   */
  async listBackups(): Promise<BackupListResult> {
    return BackupService.ListBackups();
  }

  /**
   * 从备份恢复
   * @param name - 备份文件名
   */
  async restoreBackup(name: string): Promise<BackupRestoreResult> {
    return BackupService.RestoreBackup(name);
  }

  /**
   * 删除备份
   * @param name - 备份文件名
   */
  async deleteBackup(name: string): Promise<BackupVerifyResult> {
    return BackupService.DeleteBackup(name);
  }

  /**
   * 验证备份有效性
   */
  async verifyBackup(): Promise<BackupVerifyResult> {
    return BackupService.VerifyBackup();
  }

  /**
   * 获取主密码状态（是否设置、是否解锁、锁定剩余时间）
   */
  async getMasterPasswordStatus(): Promise<{ has_password: boolean; unlocked: boolean; locked_until: number; unlock_time: number }> {
    return BackupService.GetMasterPasswordStatus();
  }

  /**
   * 设置主密码
   * @param password - 密码
   */
  async setMasterPassword(password: string): Promise<{ success: boolean; error?: string }> {
    return BackupService.SetMasterPassword(password);
  }

  /**
   * 解锁凭证（使用主密码）
   * @param password - 主密码
   */
  async unlockCredentials(password: string): Promise<{ success: boolean; locked_until: number; error?: string }> {
    return BackupService.UnlockCredentials(password);
  }

  /**
   * 锁定凭证
   */
  async lockCredentials(): Promise<{ success: boolean }> {
    return BackupService.LockCredentials();
  }

  /**
   * 获取备份配置
   */
  async getBackupConfig(): Promise<BackupConfig> {
    return BackupService.GetBackupConfig();
  }

  /**
   * 更新备份配置
   * @param config - 备份配置对象
   */
  async updateBackupConfig(config: BackupConfig): Promise<{ success: boolean; error?: string }> {
    return BackupService.UpdateBackupConfig(JSON.stringify(config));
  }

  /**
   * 上传壁纸（Android 不可用，返回空）
   * @param _file - 壁纸文件
   */
  async uploadWallpaper(_file: File): Promise<WallpaperUploadResult> {
    // Wallpaper upload is not available on Android (no HTTP server)
    // Return a dummy response so the UI doesn't break
    return { filename: "" };
  }

  /**
   * 列出已上传壁纸（Android 不可用，返回空）
   */
  async listWallpapers(): Promise<WallpaperListResult[]> {
    return [];
  }

  /**
   * 删除壁纸（Android 不可用）
   * @param _filename - 壁纸文件名
   */
  async deleteWallpaper(_filename: string): Promise<WallpaperDeleteResult> {
    return { success: true };
  }
}

// Re-export the type guard
export { isAndroid };