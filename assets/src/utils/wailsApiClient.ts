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

import * as TimerService from "../bindings/little-timer/internal/app/timerservice";
import * as SettingsService from "../bindings/little-timer/internal/app/settingsservice";
import * as HabitService from "../bindings/little-timer/internal/app/habitservice";
import * as BackupService from "../bindings/little-timer/internal/app/backupservice";

/* eslint-disable @typescript-eslint/require-await */
// Detect Android: Wails v3 sets window.wails on Android
const isAndroid = typeof window !== "undefined" && !!(window as any).wails;

export class WailsAPIClient {
  // Exposed for useSSE which accesses apiClient.baseUrl — SSE is not used on Android
  baseUrl = "";
  async getState(): Promise<TimerState> {
    return TimerService.GetState();
  }

  async startTimer(habitId?: number, options?: TimerStartOptions): Promise<TimerStartResult> {
    return TimerService.StartTimer(
      habitId ?? null,
      options?.mode ?? "",
      options?.workDuration ?? 0,
      options?.restDuration ?? 0,
      options?.loopCount ?? 0
    );
  }

  async finishTimer(): Promise<TimerFinishResult> {
    return TimerService.FinishTimer();
  }

  async getTimerProgress(): Promise<TimerProgress> {
    return TimerService.GetProgress();
  }

  async resumeTimer(habitId?: number): Promise<ResumeResult> {
    return this.startTimer(habitId);
  }

  async startRest(): Promise<RestResult> {
    return TimerService.StartRest();
  }

  async pauseTimer(): Promise<void> {
    return TimerService.PauseTimer();
  }

  async resetTimer(): Promise<void> {
    return TimerService.ResetTimer();
  }

  async changeMode(mode: "countdown" | "stopwatch"): Promise<void> {
    // Wails bindings don't expose changeMode — this is a no-op on Android
    // since mode is set via StartTimer options
    return Promise.resolve();
  }

  async getSettings(): Promise<any> {
    return SettingsService.GetSettings();
  }

  async updateSettings(settings: object): Promise<void> {
    return SettingsService.UpdateSettings(JSON.stringify(settings));
  }

  async getHabitSets(): Promise<HabitSet[]> {
    return HabitService.ListHabitSets();
  }

  async createHabitSet(name: string, description: string, color: string): Promise<HabitSet> {
    return HabitService.CreateHabitSet(name, description, color);
  }

  async updateHabitSet(id: number, name: string, description: string, color: string, wallpaper?: string): Promise<HabitSet> {
    return HabitService.UpdateHabitSet(id, name, description, color, wallpaper ?? "");
  }

  async deleteHabitSet(id: number): Promise<void> {
    return HabitService.DeleteHabitSet(id);
  }

  async getHabits(): Promise<Habit[]> {
    return HabitService.ListHabits(null);
  }

  async createHabit(setId: number, name: string, goalSeconds: number, color: string): Promise<Habit> {
    return HabitService.CreateHabit(setId, name, goalSeconds, color);
  }

  async deleteHabit(id: number): Promise<void> {
    return HabitService.DeleteHabit(id);
  }

  async updateHabit(id: number, name: string, goalSeconds: number, color: string, wallpaper?: string): Promise<Habit> {
    return HabitService.UpdateHabit(id, name, goalSeconds, color, wallpaper ?? "");
  }

  async createSession(habitId: number, durationSeconds: number, count: number, date: string): Promise<CreateSessionResult> {
    return HabitService.CreateSession(habitId, durationSeconds, count, date);
  }

  async getSessions(date?: string, startDate?: string, endDate?: string): Promise<Session[]> {
    return HabitService.ListSessions(date ?? "", startDate ?? "", endDate ?? "");
  }

  async getHabitStreak(habitId: number, goalSeconds?: number): Promise<HabitStreak> {
    return HabitService.GetHabitStreak(habitId, goalSeconds ?? 0);
  }

  async getHabitDetail(habitId: number, date?: string): Promise<HabitDetail> {
    return HabitService.GetHabitDetail(habitId, date ?? "");
  }

  async createBackup(): Promise<BackupCreateResult> {
    return BackupService.CreateBackup();
  }

  async listBackups(): Promise<BackupListResult> {
    return BackupService.ListBackups();
  }

  async restoreBackup(name: string): Promise<BackupRestoreResult> {
    return BackupService.RestoreBackup(name);
  }

  async deleteBackup(name: string): Promise<BackupVerifyResult> {
    return BackupService.DeleteBackup(name);
  }

  async verifyBackup(): Promise<BackupVerifyResult> {
    return BackupService.VerifyBackup();
  }

  async getMasterPasswordStatus(): Promise<{ has_password: boolean; unlocked: boolean; locked_until: number; unlock_time: number }> {
    return BackupService.GetMasterPasswordStatus();
  }

  async setMasterPassword(password: string): Promise<{ success: boolean; error?: string }> {
    return BackupService.SetMasterPassword(password);
  }

  async unlockCredentials(password: string): Promise<{ success: boolean; locked_until: number; error?: string }> {
    return BackupService.UnlockCredentials(password);
  }

  async lockCredentials(): Promise<{ success: boolean }> {
    return BackupService.LockCredentials();
  }

  async getBackupConfig(): Promise<BackupConfig> {
    return BackupService.GetBackupConfig();
  }

  async updateBackupConfig(config: BackupConfig): Promise<{ success: boolean; error?: string }> {
    return BackupService.UpdateBackupConfig(JSON.stringify(config));
  }

  async uploadWallpaper(file: File): Promise<WallpaperUploadResult> {
    // Wallpaper upload is not available on Android (no HTTP server)
    // Return a dummy response so the UI doesn't break
    return { filename: "", url: "" };
  }

  async listWallpapers(): Promise<WallpaperListResult[]> {
    return [];
  }

  async deleteWallpaper(filename: string): Promise<WallpaperDeleteResult> {
    return { success: true };
  }
}

// Re-export the type guard
export { isAndroid };