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

import { WailsBindings } from "../bindings/little-timer/internal/app/index";

// Detect Android: Wails v3 sets window.wails on Android
const isAndroid = typeof window !== "undefined" && !!(window as any).wails;

export class WailsAPIClient {
  async getState(): Promise<TimerState> {
    return WailsBindings.GetState();
  }

  async startTimer(habitId?: number, options?: TimerStartOptions): Promise<TimerStartResult> {
    return WailsBindings.StartTimer(
      habitId ?? null,
      options?.mode ?? "",
      options?.workDuration ?? 0,
      options?.restDuration ?? 0,
      options?.loopCount ?? 0
    );
  }

  async finishTimer(): Promise<TimerFinishResult> {
    return WailsBindings.FinishTimer();
  }

  async getTimerProgress(): Promise<TimerProgress> {
    return WailsBindings.GetProgress();
  }

  async resumeTimer(habitId?: number): Promise<ResumeResult> {
    return this.startTimer(habitId);
  }

  async startRest(): Promise<RestResult> {
    return WailsBindings.StartRest();
  }

  async pauseTimer(): Promise<void> {
    return WailsBindings.PauseTimer();
  }

  async resetTimer(): Promise<void> {
    return WailsBindings.ResetTimer();
  }

  async changeMode(mode: "countdown" | "stopwatch"): Promise<void> {
    // Wails bindings don't expose changeMode — this is a no-op on Android
    // since mode is set via StartTimer options
    return Promise.resolve();
  }

  async getSettings(): Promise<any> {
    return WailsBindings.GetSettings();
  }

  async updateSettings(settings: object): Promise<void> {
    return WailsBindings.UpdateSettings(JSON.stringify(settings));
  }

  async getHabitSets(): Promise<HabitSet[]> {
    return WailsBindings.ListHabitSets();
  }

  async createHabitSet(name: string, description: string, color: string): Promise<HabitSet> {
    return WailsBindings.CreateHabitSet(name, description, color);
  }

  async updateHabitSet(id: number, name: string, description: string, color: string, wallpaper?: string): Promise<HabitSet> {
    return WailsBindings.UpdateHabitSet(id, name, description, color, wallpaper ?? "");
  }

  async deleteHabitSet(id: number): Promise<void> {
    return WailsBindings.DeleteHabitSet(id);
  }

  async getHabits(): Promise<Habit[]> {
    return WailsBindings.ListHabits(null);
  }

  async createHabit(setId: number, name: string, goalSeconds: number, color: string): Promise<Habit> {
    return WailsBindings.CreateHabit(setId, name, goalSeconds, color);
  }

  async deleteHabit(id: number): Promise<void> {
    return WailsBindings.DeleteHabit(id);
  }

  async updateHabit(id: number, name: string, goalSeconds: number, color: string, wallpaper?: string): Promise<Habit> {
    return WailsBindings.UpdateHabit(id, name, goalSeconds, color, wallpaper ?? "");
  }

  async createSession(habitId: number, durationSeconds: number, count: number, date: string): Promise<CreateSessionResult> {
    return WailsBindings.CreateSession(habitId, durationSeconds, count, date);
  }

  async getSessions(date?: string, startDate?: string, endDate?: string): Promise<Session[]> {
    return WailsBindings.ListSessions(date ?? "", startDate ?? "", endDate ?? "");
  }

  async getHabitStreak(habitId: number, goalSeconds?: number): Promise<HabitStreak> {
    return WailsBindings.GetHabitStreak(habitId, goalSeconds ?? 0);
  }

  async getHabitDetail(habitId: number, date?: string): Promise<HabitDetail> {
    return WailsBindings.GetHabitDetail(habitId, date ?? "");
  }

  async createBackup(): Promise<BackupCreateResult> {
    return WailsBindings.CreateBackup();
  }

  async listBackups(): Promise<BackupListResult> {
    return WailsBindings.ListBackups();
  }

  async restoreBackup(name: string): Promise<BackupRestoreResult> {
    return WailsBindings.RestoreBackup(name);
  }

  async deleteBackup(name: string): Promise<BackupVerifyResult> {
    return WailsBindings.DeleteBackup(name);
  }

  async verifyBackup(): Promise<BackupVerifyResult> {
    return WailsBindings.VerifyBackup();
  }

  async getMasterPasswordStatus(): Promise<{ has_password: boolean; unlocked: boolean; locked_until: number; unlock_time: number }> {
    return WailsBindings.GetMasterPasswordStatus();
  }

  async setMasterPassword(password: string): Promise<{ success: boolean; error?: string }> {
    return WailsBindings.SetMasterPassword(password);
  }

  async unlockCredentials(password: string): Promise<{ success: boolean; locked_until: number; error?: string }> {
    return WailsBindings.UnlockCredentials(password);
  }

  async lockCredentials(): Promise<{ success: boolean }> {
    return WailsBindings.LockCredentials();
  }

  async getBackupConfig(): Promise<BackupConfig> {
    return WailsBindings.GetBackupConfig();
  }

  async updateBackupConfig(config: BackupConfig): Promise<{ success: boolean; error?: string }> {
    return WailsBindings.UpdateBackupConfig(JSON.stringify(config));
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