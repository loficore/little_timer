import type {
  TimerState,
  Settings,
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

/**
 * API 客户端，用于与后端 API 进行交互
 */
export class APIClient {
    public baseUrl: string;
    private authToken: string | null = null;

    /**
     * 构造函数，接受 API 基础 URL
     * @param {string} baseUrl API 基础 URL，例如 http://localhost:8000
     */
    constructor(baseUrl: string) {
        this.baseUrl = baseUrl;
    }

    /**
     * 设置认证 Token（用于 Authorization header）
     * @param {string | null} token 认证 Token，设为 null 可清除
     */
    setAuthToken(token: string | null): void {
        this.authToken = token;
    }

    /**
     * 获取当前认证 Token
     * @returns {string | null} 当前 Token
     */
    getAuthToken(): string | null {
        return this.authToken;
    }

    /**
     * 获取当前计时器状态
     * @returns {Promise<TimerState>} 返回一个 Promise，解析为 TimerState 对象
     */
    async getState(): Promise<TimerState> {
        const response = await fetch(`${this.baseUrl}/api/state`);
        if (!response.ok) {
            throw new Error(`Error fetching state: ${response.statusText}`);
        }
        return await response.json();
    }

    /**
     * 开始计时器
     * @param {number} [habitId] 习惯 ID（可选）
     * @returns {Promise<TimerStartResult>} 返回一个 Promise，表示操作完成
     */
    async startTimer(habitId?: number, options?: TimerStartOptions): Promise<TimerStartResult> {
        const body: Record<string, string | number | boolean> = {};
        if (habitId) body.habit_id = habitId;
        if (options) {
            if (options.mode) body.mode = options.mode;
            if (options.workDuration) body.work_duration = options.workDuration;
            if (options.restDuration) body.rest_duration = options.restDuration;
            if (options.loopCount) body.loop_count = options.loopCount;
        }
        const response = await fetch(`${this.baseUrl}/api/start`, {
            method: "POST",
            headers: Object.keys(body).length > 0 ? { "Content-Type": "application/json" } : {},
            body: Object.keys(body).length > 0 ? JSON.stringify(body) : undefined,
        });
        if (!response.ok) {
            throw new Error(`Error starting timer: ${response.statusText}`);
        }
        return await response.json();
    }

    /**
     * 结束计时器（停止并计入统计）
     * @returns {Promise<TimerFinishResult>} 累计时间
     */
    async finishTimer(): Promise<TimerFinishResult> {
        const response = await fetch(`${this.baseUrl}/api/timer/finish`, { method: "POST" });
        if (!response.ok) {
            throw new Error(`Error finishing timer: ${response.statusText}`);
        }
        return await response.json();
    }

    /**
     * 获取计时进度（用于刷新恢复）
     * @returns {Promise<TimerProgress>} 计时进度
     */
    async getTimerProgress(): Promise<TimerProgress> {
        const response = await fetch(`${this.baseUrl}/api/timer/progress`);
        if (!response.ok) {
            throw new Error(`Error getting timer progress: ${response.statusText}`);
        }
        return await response.json();
    }

    /**
     * 恢复计时器（从暂停继续）
     * @param {number} [habitId] 习惯 ID（可选）
     */
    async resumeTimer(habitId?: number): Promise<ResumeResult> {
        return this.startTimer(habitId);
    }

    /**
     * 开始休息
     * @returns {Promise<RestResult>} 休息时长
     */
    async startRest(): Promise<RestResult> {
        const response = await fetch(`${this.baseUrl}/api/timer/rest`, { method: "POST" });
        if (!response.ok) {
            throw new Error(`Error starting rest: ${response.statusText}`);
        }
        return await response.json();
    }

    /**
     * 暂停计时器
     * @returns {Promise<void>} 返回一个 Promise，表示操作完成
     */
    async pauseTimer(): Promise<void> {
        const response = await fetch(`${this.baseUrl}/api/pause`, { method: "POST" });
        if (!response.ok) {
            throw new Error(`Error pausing timer: ${response.statusText}`);
        }
    }

    /**
     * 重置计时器
     */
    async resetTimer(): Promise<void> {
        const response = await fetch(`${this.baseUrl}/api/reset`, { method: "POST" });
        if (!response.ok) {
            throw new Error(`Error resetting timer: ${response.statusText}`);
        }
    }

    /**
     * 切换计时器模式
     * @param {"countdown" | "stopwatch"} mode 目标模式
     */
    async changeMode(mode: "countdown" | "stopwatch"): Promise<void> {
        const response = await fetch(`${this.baseUrl}/api/mode`, {
            method: "POST",
            body: mode,
        });
        if (!response.ok) {
            throw new Error(`Error changing mode: ${response.statusText}`);
        }
    }

    /**
     * 获取设置
     * @returns {Promise<Settings>} 返回一个 Promise，解析为 Settings 对象
     */
    async getSettings(): Promise<Settings> {
        const response = await fetch(`${this.baseUrl}/api/settings`);
        if (!response.ok) {
            throw new Error(`Error fetching settings: ${response.statusText}`);
        }
        return await response.json();
    }

    /**
     * 更新设置
     * @param {object} settings 要更新的设置对象
     * @returns {Promise<void>} 返回一个 Promise，表示操作完成
     */
    async updateSettings(settings: object): Promise<void> {
        const response = await fetch(`${this.baseUrl}/api/settings`, {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify(settings),
        });
        if (!response.ok) {
            throw new Error(`Error updating settings: ${response.statusText}`);
        }
    }

    // === 习惯集 API ===

    /**
     * 获取习惯集列表
     * @returns {Promise<HabitSet[]>} 返回一个 Promise，解析为习惯集列表
     */
    async getHabitSets(): Promise<HabitSet[]> {
        const response = await fetch(`${this.baseUrl}/api/habit-sets`);
        if (!response.ok) {
            throw new Error(`Error fetching habit sets: ${response.statusText}`);
        }
        return await response.json();
    }

    /**
     * 创建习惯集
     * @param {string} name
     * @param {string} description
     * @param {string} color
     * @returns {Promise<HabitSet>} 返回一个 Promise，解析为创建的习惯集对象
     */
    async createHabitSet(name: string, description: string, color: string): Promise<HabitSet> {
        const response = await fetch(`${this.baseUrl}/api/habit-sets`, {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({ name, description, color }),
        });
        if (!response.ok) {
            throw new Error(`Error creating habit set: ${response.statusText}`);
        }
        return await response.json();
    }

    /**
     * 更新习惯集
     * @param {number} id 习惯集 ID
     * @param {string} name 名称
     * @param {string} description 描述
     * @param {string} color 颜色
     * @param {string} wallpaper 壁纸（可选）
     * @returns {Promise<HabitSet>}
     */
    async updateHabitSet(id: number, name: string, description: string, color: string, wallpaper?: string): Promise<HabitSet> {
        const response = await fetch(`${this.baseUrl}/api/habit-sets/${id}`, {
            method: "PUT",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({ name, description, color, wallpaper: wallpaper || "" }),
        });
        if (!response.ok) {
            throw new Error(`Error updating habit set: ${response.statusText}`);
        }
        return await response.json();
    }

    /**
     * 删除习惯集
     * @param {number} id 习惯集 ID
     * @returns {Promise<void>}
     */
    async deleteHabitSet(id: number): Promise<void> {
        const response = await fetch(`${this.baseUrl}/api/habit-sets/${id}`, {
            method: "DELETE",
        });
        if (!response.ok) {
            throw new Error(`Error deleting habit set: ${response.statusText}`);
        }
    }

    // === 习惯 API ===

    /**
     * 获取习惯列表
     * @returns {Promise<Habit[]>} 返回一个 Promise，解析为习惯列表
     */
    async getHabits(): Promise<Habit[]> {
        const response = await fetch(`${this.baseUrl}/api/habits`);
        if (!response.ok) {
            throw new Error(`Error fetching habits: ${response.statusText}`);
        }
        return await response.json();
    }

    /**
     * 创建习惯
     * @param {number} setId 习惯集 ID
     * @param {string} name 习惯名称
     * @param {number} goalSeconds 目标时间（秒）
     * @param {string} color 颜色
     * @returns {Promise<Habit>}
     */
    async createHabit(setId: number, name: string, goalSeconds: number, color: string): Promise<Habit> {
        const response = await fetch(`${this.baseUrl}/api/habits`, {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({
                set_id: setId,
                name,
                goal_seconds: goalSeconds,
                color,
            }),
        });
        if (!response.ok) {
            throw new Error(`Error creating habit: ${response.statusText}`);
        }
        return await response.json();
    }

    /**
     * 删除习惯
     * @param {number} id
     * @returns {Promise<void>} 返回一个 Promise
     */
    async deleteHabit(id: number): Promise<void> {
        const response = await fetch(`${this.baseUrl}/api/habits/${id}`, {
            method: "DELETE",
        });
        if (!response.ok) {
            throw new Error(`Error deleting habit: ${response.statusText}`);
        }
    }

    /**
     * 更新习惯
     * @param {number} id 习惯 ID
     * @param {string} name 名称
     * @param {number} goalSeconds 目标时长（秒）
     * @param {string} color 颜色
     * @param {string} wallpaper 壁纸（可选）
     * @returns {Promise<Habit>}
     */
    async updateHabit(id: number, name: string, goalSeconds: number, color: string, wallpaper?: string): Promise<Habit> {
        const response = await fetch(`${this.baseUrl}/api/habits/${id}`, {
            method: "PUT",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({
                name,
                goal_seconds: goalSeconds,
                color,
                wallpaper: wallpaper || "",
            }),
        });
        if (!response.ok) {
            throw new Error(`Error updating habit: ${response.statusText}`);
        }
        return await response.json();
    }

    // === 记录 API ===

    /**
     * 创建记录
     * @param {number} habitId 习惯 ID
     * @param {number} durationSeconds 持续时间（秒）
     * @param {number} count 次数
     * @param {string} date 日期
     * @returns {Promise<CreateSessionResult>} 返回一个 Promise，解析为创建的记录对象
     */
    async createSession(habitId: number, durationSeconds: number, count: number, date: string): Promise<CreateSessionResult> {
        const response = await fetch(`${this.baseUrl}/api/sessions`, {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({
                habit_id: habitId,
                duration_seconds: durationSeconds,
                count,
                date,
            }),
        });
        if (!response.ok) {
            throw new Error(`Error creating session: ${response.statusText}`);
        }
        return await response.json();
    }

    /**
     * 获取记录列表
     * @param {string} date 日期
     * @param {string} startDate 开始日期
     * @param {string} endDate 结束日期
     * @returns {Promise<Session[]>} 返回一个 Promise，解析为记录列表
     */
    async getSessions(date?: string, startDate?: string, endDate?: string): Promise<Session[]> {
        const params = new URLSearchParams();
        if (date) params.set("date", date);
        if (startDate) params.set("start_date", startDate);
        if (endDate) params.set("end_date", endDate);

        const response = await fetch(`${this.baseUrl}/api/sessions?${params.toString()}`);
        if (!response.ok) {
            throw new Error(`Error fetching sessions: ${response.statusText}`);
        }
        return await response.json();
    }

    /**
     * 获取习惯连胜
     * @param {number} habitId 习惯 ID
     * @param {number} goalSeconds 目标时长
     * @returns {Promise<HabitStreak>} 返回一个 Promise，解析为包含习惯 ID 和连胜数的对象
     */
    async getHabitStreak(habitId: number, goalSeconds?: number): Promise<HabitStreak> {
        const params = new URLSearchParams();
        if (goalSeconds) params.set("goal_seconds", goalSeconds.toString());
        const response = await fetch(`${this.baseUrl}/api/habits/${habitId}/streak?${params.toString()}`);
        if (!response.ok) {
            throw new Error(`Error fetching streak: ${response.statusText}`);
        }
        return await response.json();
    }

    /**
     * 获取习惯详情
     * @param {number} habitId 习惯 ID
     * @param {string} [date] 日期（可选，默认今天）
     * @returns {Promise<HabitDetail>} 返回习惯详情对象
     */
    async getHabitDetail(habitId: number, date?: string): Promise<HabitDetail> {
        const params = new URLSearchParams();
        if (date) params.set("date", date);
        const response = await fetch(`${this.baseUrl}/api/habits/${habitId}/detail?${params.toString()}`);
        if (!response.ok) {
            throw new Error(`Error fetching habit detail: ${response.statusText}`);
        }
        return await response.json();
    }

    // === 备份 API ===

    /**
     * 创建数据库备份
     * @returns Promise resolving to result object with success status, backup_path, or error message
     */
    async createBackup(): Promise<BackupCreateResult> {
        const response = await fetch(`${this.baseUrl}/api/backup/create`, { method: "POST" });
        const data = await response.json();
        return data;
    }

    /**
     * 获取备份列表
     * @returns Promise resolving to array of backup info
     */
    async listBackups(): Promise<BackupListResult> {
        const response = await fetch(`${this.baseUrl}/api/backup/list`);
        if (!response.ok) {
            throw new Error(`Error listing backups: ${response.statusText}`);
        }
        return await response.json();
    }

    /**
     * 从备份恢复数据库
     * @param name - Backup filename to restore from
     * @returns Promise resolving to success status or error message
     */
    async restoreBackup(name: string): Promise<BackupRestoreResult> {
        const response = await fetch(`${this.baseUrl}/api/backup/restore`, {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({ name }),
        });
        return await response.json();
    }

    /**
     * 删除指定备份
     * @param name - Backup filename to delete
     * @returns Promise resolving to success status or error message
     */
    async deleteBackup(name: string): Promise<BackupVerifyResult> {
        const response = await fetch(`${this.baseUrl}/api/backup/${encodeURIComponent(name)}`, {
            method: "DELETE",
        });
        return await response.json();
    }

    /**
     * 验证备份目标配置是否有效
     * @returns Promise resolving to success status or error message
     */
    async verifyBackup(): Promise<BackupVerifyResult> {
        const response = await fetch(`${this.baseUrl}/api/backup/verify`, { method: "POST" });
        return await response.json();
    }

    /**
     * 获取主密码状态
     * @returns Promise with has_password, unlocked, locked_until, unlock_time
     */
    async getMasterPasswordStatus(): Promise<{
        has_password: boolean;
        unlocked: boolean;
        locked_until: number;
        unlock_time: number;
    }> {
        const response = await fetch(`${this.baseUrl}/api/backup/master-password`);
        return await response.json();
    }

    /**
     * 设置主密码
     * @param password - New master password
     * @returns Promise with success status
     */
    async setMasterPassword(password: string): Promise<{ success: boolean; error?: string }> {
        const response = await fetch(`${this.baseUrl}/api/backup/master-password`, {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({ password }),
        });
        return await response.json();
    }

    /**
     * 解锁凭证
     * @param password - Master password
     * @returns Promise with success status
     */
    async unlockCredentials(password: string): Promise<{ success: boolean; locked_until: number; error?: string }> {
        const response = await fetch(`${this.baseUrl}/api/backup/unlock`, {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({ password }),
        });
        return await response.json();
    }

    /**
     * 锁定凭证
     * @returns Promise with success status
     */
    async lockCredentials(): Promise<{ success: boolean }> {
        const response = await fetch(`${this.baseUrl}/api/backup/lock`, {
            method: "POST",
        });
        return await response.json();
    }

    /**
     * 获取当前备份配置
     * @returns Promise resolving to BackupConfig object
     */
    async getBackupConfig(): Promise<BackupConfig> {
        const response = await fetch(`${this.baseUrl}/api/backup/config`);
        if (!response.ok) {
            throw new Error(`Error fetching backup config: ${response.statusText}`);
        }
        return await response.json();
    }

    /**
     * 更新备份配置
     * @param config - BackupConfig object with updated settings
     * @returns Promise resolving to success status or error message
     */
    async updateBackupConfig(config: BackupConfig): Promise<{ success: boolean; error?: string }> {
        const response = await fetch(`${this.baseUrl}/api/backup/config`, {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify(config),
        });
        return await response.json();
    }

    // === 壁纸 API ===

    /**
     * 上传壁纸图片
     * @param {File} file 图片文件
     * @returns {Promise<WallpaperUploadResult>} 上传后的文件名
     */
    async uploadWallpaper(file: File): Promise<WallpaperUploadResult> {
        const formData = new FormData();
        formData.append("file", file);

        const response = await fetch(`${this.baseUrl}/api/wallpapers`, {
            method: "POST",
            body: formData,
        });
        if (!response.ok) {
            throw new Error(`Error uploading wallpaper: ${response.statusText}`);
        }
        return await response.json();
    }

    /**
     * 获取已上传的壁纸列表
     * @returns {Promise<WallpaperListResult[]>}
     */
    async listWallpapers(): Promise<WallpaperListResult[]> {
        const response = await fetch(`${this.baseUrl}/api/wallpapers`);
        if (!response.ok) {
            throw new Error(`Error listing wallpapers: ${response.statusText}`);
        }
        return await response.json();
    }

    /**
     * 删除指定壁纸
     * @param {string} filename 文件名
     */
    async deleteWallpaper(filename: string): Promise<WallpaperDeleteResult> {
        const response = await fetch(`${this.baseUrl}/api/wallpapers/${encodeURIComponent(filename)}`, {
            method: "DELETE",
        });
        if (!response.ok) {
            throw new Error(`Error deleting wallpaper: ${response.statusText}`);
        }
        return await response.json();
    }
}
