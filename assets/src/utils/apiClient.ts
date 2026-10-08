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
import type {
    TaskDTO,
    LLMSettingsDTO,
    SchedulerPlan,
    DailySweepToday,
    DailySweepSettings,
    ReviewProposal,
} from "../types/aigtd";

/**
 * API 客户端，用于与后端 API 进行交互
 */

/**
 * VersionConflictError —— PATCH/PUT/DELETE 返回 409 时抛出的 sentinel。
 *
 * 后端 ConflictGuard 在 storage 层 updateWithVersion 影响 0 行时统一返回
 * HTTP 409 + code=version_conflict,前端在拿到这个错误时应:
 *   1) 用 getTask(id) 拉最新版本与字段;
 *   2) 重新提交(有界重试 1 次),或把决策交回用户(展示冲突横幅)。
 *
 * 不复用普通 Error(仅 message 含 "409"),保证 instanceof 判定稳。
 */
export class VersionConflictError extends Error {
    constructor(message = "version conflict") {
        super(message);
        this.name = "VersionConflictError";
    }
}

/**
 * TaskScoresPatch —— PATCH /api/tasks/:id/scores 的请求体形状。
 *
 * 指针语义:除 version 外所有字段可选;后端 omitempty,缺 = "不动"。
 * version 必须带,后端 ConflictGuard 缺则 400。
 */
export interface TaskScoresPatch {
    version: number;
    priority_score?: number;
    urgency_score?: number;
    energy_required?: number;
    context_tag?: string;
}

export class APIClient {
    public baseUrl: string;
    private authToken: string | null = null;

    /**
     * ensureMutationVersion —— 给 POST/PATCH/PUT/DELETE 注入 ConflictGuard
     * 要求的 `If-Match: "<version>"` header。
     *
     * 设计:
     *  - GET/HEAD/OPTIONS 原样返回(读不需要乐观锁)。
     *  - 已显式设置 If-Match(如 task 真实 version / capture 的 "0")不覆盖。
     *  - 其余资源(backup/settings/timer/wallpaper/habit/...)缺省 `"0"`。
     *    中间件 Task 3 仅做存在性校验,正确性校验仅 tasks 行(Task 4),
     *    所以对无版本语义的资源送 "0" 即可满足 400 防呆。
     *
     * 集中放在 fetchJson 是为了**不漏** —— Critical finding 指出手动
     * 加头的方法在 review 时漏了 ~20 个,这里用唯一出口一次性覆盖。
     */
    private ensureMutationVersion(init?: RequestInit): RequestInit | undefined {
        if (!init) return init;
        const method = (init.method ?? "GET").toUpperCase();
        if (method === "GET" || method === "HEAD" || method === "OPTIONS") {
            return init;
        }
        const headers: Record<string, string> = {
            ...((init.headers ?? {}) as Record<string, string>),
        };
        if (!headers["If-Match"]) {
            headers["If-Match"] = '"0"';
        }
        return { ...init, headers };
    }

    /**
     * 内部辅助函数：统一处理 fetch 响应和 JSON 解析
     * @param url 请求 URL
     * @param options RequestInit 选项
     * @returns Promise<T> 解析后的 JSON 数据
     */
    private async fetchJson<T>(url: string, options?: RequestInit): Promise<T> {
        const res = await fetch(url, this.ensureMutationVersion(options));
        if (!res.ok) {
            throw new Error(`HTTP ${res.status}: ${await res.text()}`);
        }
        return res.json() as Promise<T>;
    }

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
        return this.fetchJson<TimerState>(`${this.baseUrl}/api/state`);
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
        return this.fetchJson<TimerStartResult>(`${this.baseUrl}/api/start`, {
            method: "POST",
            headers: Object.keys(body).length > 0 ? { "Content-Type": "application/json" } : {},
            body: Object.keys(body).length > 0 ? JSON.stringify(body) : undefined,
        });
    }

    /**
     * 结束计时器（停止并计入统计）
     * @returns {Promise<TimerFinishResult>} 累计时间
     */
    async finishTimer(): Promise<TimerFinishResult> {
        return this.fetchJson<TimerFinishResult>(`${this.baseUrl}/api/timer/finish`, { method: "POST" });
    }

    /**
     * 获取计时进度（用于刷新恢复）
     * @returns {Promise<TimerProgress>} 计时进度
     */
    async getTimerProgress(): Promise<TimerProgress> {
        return this.fetchJson<TimerProgress>(`${this.baseUrl}/api/timer/progress`);
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
        return this.fetchJson<RestResult>(`${this.baseUrl}/api/timer/rest`, { method: "POST" });
    }

    /**
     * 暂停计时器
     * @returns {Promise<void>} 返回一个 Promise，表示操作完成
     */
    async pauseTimer(): Promise<void> {
        return this.fetchJson<void>(`${this.baseUrl}/api/pause`, { method: "POST" });
    }

    /**
     * 重置计时器
     */
    async resetTimer(): Promise<void> {
        return this.fetchJson<void>(`${this.baseUrl}/api/reset`, { method: "POST" });
    }

    /**
     * 切换计时器模式
     * @param {"countdown" | "stopwatch"} mode 目标模式
     */
    async changeMode(mode: "countdown" | "stopwatch"): Promise<void> {
        return this.fetchJson<void>(`${this.baseUrl}/api/mode`, {
            method: "POST",
            body: mode,
        });
    }

    /**
     * 获取设置
     * @returns {Promise<Settings>} 返回一个 Promise，解析为 Settings 对象
     */
    async getSettings(): Promise<Settings> {
        return this.fetchJson<Settings>(`${this.baseUrl}/api/settings`);
    }

    /**
     * 更新设置
     * @param {object} settings 要更新的设置对象
     * @returns {Promise<void>} 返回一个 Promise，表示操作完成
     */
    async updateSettings(settings: object): Promise<void> {
        return this.fetchJson<void>(`${this.baseUrl}/api/settings`, {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify(settings),
        });
    }

    /**
     * 获取习惯集列表
     * @returns {Promise<HabitSet[]>} 返回一个 Promise，解析为习惯集列表
     */
    async getHabitSets(): Promise<HabitSet[]> {
        return this.fetchJson<HabitSet[]>(`${this.baseUrl}/api/habit-sets`);
    }

    /**
     * 创建习惯集
     * @param {string} name
     * @param {string} description
     * @param {string} color
     * @returns {Promise<HabitSet>} 返回一个 Promise，解析为创建的习惯集对象
     */
    async createHabitSet(name: string, description: string, color: string): Promise<HabitSet> {
        return this.fetchJson<HabitSet>(`${this.baseUrl}/api/habit-sets`, {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({ name, description, color }),
        });
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
        return this.fetchJson<HabitSet>(`${this.baseUrl}/api/habit-sets/${id}`, {
            method: "PUT",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({ name, description, color, wallpaper: wallpaper || "" }),
        });
    }

    /**
     * 删除习惯集
     * @param {number} id 习惯集 ID
     * @returns {Promise<void>}
     */
    async deleteHabitSet(id: number): Promise<void> {
        return this.fetchJson<void>(`${this.baseUrl}/api/habit-sets/${id}`, {
            method: "DELETE",
        });
    }

    /**
     * 获取习惯列表
     * @returns {Promise<Habit[]>} 返回一个 Promise，解析为习惯列表
     */
    async getHabits(): Promise<Habit[]> {
        return this.fetchJson<Habit[]>(`${this.baseUrl}/api/habits`);
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
        return this.fetchJson<Habit>(`${this.baseUrl}/api/habits`, {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({
                set_id: setId,
                name,
                goal_seconds: goalSeconds,
                color,
            }),
        });
    }

    /**
     * 删除习惯
     * @param {number} id
     * @returns {Promise<void>} 返回一个 Promise
     */
    async deleteHabit(id: number): Promise<void> {
        return this.fetchJson<void>(`${this.baseUrl}/api/habits/${id}`, {
            method: "DELETE",
        });
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
        return this.fetchJson<Habit>(`${this.baseUrl}/api/habits/${id}`, {
            method: "PUT",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({
                name,
                goal_seconds: goalSeconds,
                color,
                wallpaper: wallpaper || "",
            }),
        });
    }

    /**
     * 创建记录
     * @param {number} habitId 习惯 ID
     * @param {number} durationSeconds 持续时间（秒）
     * @param {number} count 次数
     * @param {string} date 日期
     * @returns {Promise<CreateSessionResult>} 返回一个 Promise，解析为创建的记录对象
     */
    async createSession(habitId: number, durationSeconds: number, count: number, date: string): Promise<CreateSessionResult> {
        return this.fetchJson<CreateSessionResult>(`${this.baseUrl}/api/sessions`, {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({
                habit_id: habitId,
                duration_seconds: durationSeconds,
                count,
                date,
            }),
        });
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

        return this.fetchJson<Session[]>(`${this.baseUrl}/api/sessions?${params.toString()}`);
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
        return this.fetchJson<HabitDetail>(`${this.baseUrl}/api/habits/${habitId}/detail?${params.toString()}`);
    }

    /**
     * 创建数据库备份
     * @returns 解析为结果对象的 Promise，含成功状态、backup_path 或错误信息
     */
    async createBackup(): Promise<BackupCreateResult> {
        return this.fetchJson<BackupCreateResult>(`${this.baseUrl}/api/backup/create`, { method: "POST" });
    }

    /**
     * 获取备份列表
     * @returns 解析为备份信息数组的 Promise
     */
    async listBackups(): Promise<BackupListResult> {
        return this.fetchJson<BackupListResult>(`${this.baseUrl}/api/backup/list`);
    }

    /**
     * 从备份恢复数据库
     * @param name - 要从中恢复的备份文件名
     * @returns 解析为成功状态或错误信息的 Promise
     */
    async restoreBackup(name: string): Promise<BackupRestoreResult> {
        return this.fetchJson<BackupRestoreResult>(`${this.baseUrl}/api/backup/restore`, {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({ name }),
        });
    }

    /**
     * 删除指定备份
     * @param name - 要删除的备份文件名
     * @returns 解析为成功状态或错误信息的 Promise
     */
    async deleteBackup(name: string): Promise<BackupVerifyResult> {
        return this.fetchJson<BackupVerifyResult>(`${this.baseUrl}/api/backup/${encodeURIComponent(name)}`, {
            method: "DELETE",
        });
    }

    /**
     * 验证备份目标配置是否有效
     * @returns 解析为成功状态或错误信息的 Promise
     */
    async verifyBackup(): Promise<BackupVerifyResult> {
        return this.fetchJson<BackupVerifyResult>(`${this.baseUrl}/api/backup/verify`, { method: "POST" });
    }

    /**
     * 获取主密码状态
     * @returns 返回含 has_password、unlocked、locked_until、unlock_time 的 Promise
     */
    async getMasterPasswordStatus(): Promise<{
        has_password: boolean;
        unlocked: boolean;
        locked_until: number;
        unlock_time: number;
    }> {
        return this.fetchJson<{
            has_password: boolean;
            unlocked: boolean;
            locked_until: number;
            unlock_time: number;
        }>(`${this.baseUrl}/api/backup/master-password`);
    }

    /**
     * 设置主密码
     * @param password - 新主密码
     * @returns 返回含成功状态的 Promise
     */
    async setMasterPassword(password: string): Promise<{ success: boolean; error?: string }> {
        return this.fetchJson<{ success: boolean; error?: string }>(`${this.baseUrl}/api/backup/master-password`, {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({ password }),
        });
    }

    /**
     * 解锁凭证
     * @param password - 主密码
     * @returns 返回含成功状态的 Promise
     */
    async unlockCredentials(password: string): Promise<{ success: boolean; locked_until: number; error?: string }> {
        return this.fetchJson<{ success: boolean; locked_until: number; error?: string }>(`${this.baseUrl}/api/backup/unlock`, {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({ password }),
        });
    }

    /**
     * 锁定凭证
     * @returns 返回含成功状态的 Promise
     */
    async lockCredentials(): Promise<{ success: boolean }> {
        return this.fetchJson<{ success: boolean }>(`${this.baseUrl}/api/backup/lock`, {
            method: "POST",
        });
    }

    /**
     * 获取当前备份配置
     * @returns 解析为 BackupConfig 对象的 Promise
     */
    async getBackupConfig(): Promise<BackupConfig> {
        return this.fetchJson<BackupConfig>(`${this.baseUrl}/api/backup/config`);
    }

    /**
     * 更新备份配置
     * @param config - 含更新后设置的 BackupConfig 对象
     * @returns 解析为成功状态或错误信息的 Promise
     */
    async updateBackupConfig(config: BackupConfig): Promise<{ success: boolean; error?: string }> {
        return this.fetchJson<{ success: boolean; error?: string }>(`${this.baseUrl}/api/backup/config`, {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify(config),
        });
    }

    /**
     * 上传壁纸图片
     * @param {File} file 图片文件
     * @returns {Promise<WallpaperUploadResult>} 上传后的文件名
     */
    async uploadWallpaper(file: File): Promise<WallpaperUploadResult> {
        const formData = new FormData();
        formData.append("file", file);

        // v11:ConflictGuard 要求 mutation 带 If-Match。uploadWallpaper 用 raw
        // fetch(FormData 不能走 fetchJson 那条 Content-Type 自动注入路径),
        // 这里手动补 header。wallpaper 无版本语义,送 "0"。
        const response = await fetch(`${this.baseUrl}/api/wallpapers`, {
            method: "POST",
            headers: { "If-Match": '"0"' },
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
        return this.fetchJson<WallpaperListResult[]>(`${this.baseUrl}/api/wallpapers`);
    }

    /**
     * 删除指定壁纸
     * @param {string} filename 文件名
     */
    async deleteWallpaper(filename: string): Promise<WallpaperDeleteResult> {
        return this.fetchJson<WallpaperDeleteResult>(`${this.baseUrl}/api/wallpapers/${encodeURIComponent(filename)}`, {
            method: "DELETE",
        });
    }

    /**
     * 从 URL 获取壁纸
     * @param {string} url 壁纸 URL
     * @returns {Promise<WallpaperUploadResult>} 上传后的文件名
     */
    async fetchWallpaperByUrl(url: string): Promise<WallpaperUploadResult> {
        return this.fetchJson<WallpaperUploadResult>(`${this.baseUrl}/api/wallpapers/from-url`, {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({ url }),
        });
    }

    // ===== AI GTD =====

    /** 提交一条随手记 → 异步拆解。返回 task_id。
     *
     *  v11:ConflictGuard 要求所有 mutation 带 version。capture 是创建场景,
     *  任务尚未存在,带 `If-Match: "0"` + body version=0 满足中间件即可。
     */
    async captureAIGtd(rawText: string, source?: string): Promise<{ task_id: number }> {
        return this.fetchJson<{ task_id: number }>(`${this.baseUrl}/api/aigtd/capture`, {
            method: "POST",
            headers: {
                "Content-Type": "application/json",
                "If-Match": '"0"',
            },
            body: JSON.stringify({ version: 0, raw_text: rawText, source: source ?? "manual" }),
        });
    }

    /** 重新解析一个 task(可选覆盖原文)。
     *
     *  v11:ConflictGuard 要求 version;reparse 若带 newRawText 走 storage.UpdateTask
     *  乐观锁校验,不带则 handler 忽略 version,但中间件仍要求。统一 required。
     */
    async reparseAIGtd(taskId: number, version: number, newRawText?: string): Promise<{ job_id: number }> {
        return this.fetchJson<{ job_id: number }>(`${this.baseUrl}/api/aigtd/reparse/${taskId}`, {
            method: "POST",
            headers: {
                "Content-Type": "application/json",
                "If-Match": `"${version}"`,
            },
            body: JSON.stringify({
                version,
                ...(newRawText ? { raw_text: newRawText } : {}),
            }),
        });
    }

    /** 查询任务当前 AI 处理状态。 */
    async getAIGtdJob(taskId: number): Promise<{
        task_id: number;
        ai_status: string;
        ai_error?: string;
        latest_job_id?: number;
        attempts?: number;
    }> {
        return this.fetchJson(`${this.baseUrl}/api/aigtd/jobs?task_id=${taskId}`);
    }

    /** 列出顶层任务(可按状态过滤)。 */
    async listTasks(status?: string): Promise<TaskDTO[]> {
        const q = status ? `?status=${encodeURIComponent(status)}` : "";
        return this.fetchJson<TaskDTO[]>(`${this.baseUrl}/api/tasks${q}`);
    }

    /** 取单条任务(含子任务)。 */
    async getTask(id: number): Promise<TaskDTO> {
        return this.fetchJson<TaskDTO>(`${this.baseUrl}/api/tasks/${id}`);
    }

    /** 直接创建任务(不经过 AI)。
     *
     *  v11:ConflictGuard 要求 mutation 带 version。create 任务尚未存在,
     *  带 `If-Match: "0"` + body version=0 满足中间件。
     */
    async createTask(title: string, notes?: string): Promise<TaskDTO> {
        return this.fetchJson<TaskDTO>(`${this.baseUrl}/api/tasks`, {
            method: "POST",
            headers: {
                "Content-Type": "application/json",
                "If-Match": '"0"',
            },
            body: JSON.stringify({ version: 0, title, notes }),
        });
    }

    /** 局部更新任务(指针语义)。
     *
     *  v11:patch 必须包含 version(从 task.version 读取),既走 If-Match header
     *  也回填到 body,handler 通过 middleware.GetVersion(c) 取值做乐观锁。
     */
    async updateTask(id: number, patch: Partial<TaskDTO>): Promise<TaskDTO> {
        const { version } = patch;
        const headers: Record<string, string> = { "Content-Type": "application/json" };
        if (typeof version === "number") {
            headers["If-Match"] = `"${version}"`;
        }
        return this.fetchJson<TaskDTO>(`${this.baseUrl}/api/tasks/${id}`, {
            method: "PATCH",
            headers,
            body: JSON.stringify(patch),
        });
    }

    /** 删除任务。
     *
     *  v11:ConflictGuard 要求 mutation 带 version;handler TaskDelete 当前未
     *  使用 version 做业务校验,但中间件强制 0/缺版本 400。
     */
    async deleteTask(id: number, version: number): Promise<void> {
        await this.fetchJson<void>(`${this.baseUrl}/api/tasks/${id}`, {
            method: "DELETE",
            headers: { "If-Match": `"${version}"` },
        });
    }

    /** 切换子任务完成状态。
     *
     *  v11:ConflictGuard 要求 mutation 带 version;handler SubtaskUpdate 当前
     *  未使用 version 做业务校验,统一带上以满足中间件。
     */
    async toggleSubtask(
        taskId: number,
        subId: number,
        status: "active" | "done",
        version: number,
    ): Promise<void> {
        await this.fetchJson<void>(`${this.baseUrl}/api/tasks/${taskId}/subtasks/${subId}`, {
            method: "PATCH",
            headers: {
                "Content-Type": "application/json",
                "If-Match": `"${version}"`,
            },
            body: JSON.stringify({ version, status }),
        });
    }

    /** 局部更新 5 维评分(pointer 语义)。
     *
     *  v11:走 PATCH /api/tasks/:id/scores,ConflictGuard 强制 version。
     *  优先 If-Match header(ETag 风格,handler 优先级 > body)。
     *  body 仍然带 version,作为 fallback 满足中间件的 body-peek 探测。
     *  409 由后端返回 `{"code":"version_conflict"}`,此处统一抛 VersionConflictError
     *  供上层 (useAITasks.patchScores / ScoreEditPanel) 做有界重试或冲突横幅。
     */
    async patchTaskScores(id: number, body: TaskScoresPatch): Promise<TaskDTO> {
        const { version, ...fields } = body;
        const payload = { version, ...fields };
        const res = await fetch(`${this.baseUrl}/api/tasks/${id}/scores`, {
            method: "PATCH",
            headers: {
                "Content-Type": "application/json",
                "If-Match": `"${version}"`,
            },
            body: JSON.stringify(payload),
        });
        if (res.status === 409) {
            throw new VersionConflictError();
        }
        if (!res.ok) {
            throw new Error(`HTTP ${res.status}: ${await res.text()}`);
        }
        return res.json() as Promise<TaskDTO>;
    }

    /** 读 LLM 设置。 */
    async getLLMSettings(): Promise<LLMSettingsDTO> {
        return this.fetchJson<LLMSettingsDTO>(`${this.baseUrl}/api/settings/llm`);
    }

    /** 写 LLM 设置(任意字段可选)。 */
    async updateLLMSettings(patch: {
        provider?: "openai_compat" | "anthropic";
        base_url?: string;
        model?: string;
        max_tokens?: number;
        timeout_seconds?: number;
        api_key?: string;
        clear_key?: boolean;
    }): Promise<LLMSettingsDTO> {
        return this.fetchJson<LLMSettingsDTO>(`${this.baseUrl}/api/settings/llm`, {
            method: "PUT",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify(patch),
        });
    }

    // ===== GTD Daily Sweep / Review (v12) =====

    /** 手动触发一次 review:POST /api/daily-sweep/start → { job_ids }。 */
    async startDailySweep(): Promise<{ job_ids: number[] }> {
        return this.fetchJson<{ job_ids: number[] }>(`${this.baseUrl}/api/daily-sweep/start`, {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({}),
        });
    }

    /** 拉当日 review 提议:GET /api/daily-sweep/today。 */
    async getDailySweepToday(): Promise<DailySweepToday> {
        return this.fetchJson<DailySweepToday>(`${this.baseUrl}/api/daily-sweep/today`);
    }

    /** 采纳一条提议:POST /api/daily-sweep/adopt(幂等;409 = version 冲突)。 */
    async adoptDailySweep(body: {
        change_id: string;
        ai_job_id: number;
        version: number;
        proposal: ReviewProposal;
    }): Promise<{ task_id: number; version_after: number; idempotent?: boolean }> {
        return this.fetchJson<{ task_id: number; version_after: number; idempotent?: boolean }>(
            `${this.baseUrl}/api/daily-sweep/adopt`,
            {
                method: "POST",
                headers: { "Content-Type": "application/json" },
                body: JSON.stringify(body),
            },
        );
    }

    /** 拒绝一条提议:POST /api/daily-sweep/reject → 204。 */
    async rejectDailySweep(body: { change_id: string }): Promise<void> {
        return this.fetchJson<void>(`${this.baseUrl}/api/daily-sweep/reject`, {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify(body),
        });
    }

    /** 读 Daily Sweep 设置。 */
    async getDailySweepSettings(): Promise<DailySweepSettings> {
        return this.fetchJson<DailySweepSettings>(`${this.baseUrl}/api/settings/daily-sweep`);
    }

    /** 写 Daily Sweep 设置(字段可选)。非法时间后端 400 daily_sweep.invalid_time。 */
    async updateDailySweepSettings(patch: {
        enabled?: boolean;
        time?: string;
    }): Promise<DailySweepSettings> {
        return this.fetchJson<DailySweepSettings>(`${this.baseUrl}/api/settings/daily-sweep`, {
            method: "PUT",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify(patch),
        });
    }

    // ===== Scheduler (Task 9) =====

    /**
     * runSchedulePreview —— POST /api/schedule/run (mode=preview)。
     *
     * 返回后端 `scheduler.Plan` 的原样 JSON(PascalCase 键)。不落库。
     * 走 fetchJson → 继承 ConflictGuard 的 If-Match 注入(Task 8 retrofit)。
     */
    async runSchedulePreview(date: string): Promise<SchedulerPlan> {
        return this.fetchJson<SchedulerPlan>(`${this.baseUrl}/api/schedule/run`, {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({ date, mode: "preview" }),
        });
    }

    /**
     * runScheduleApply —— POST /api/schedule/run (mode=apply)。
     *
     * 服务端在事务内乐观锁写入 preview 出来的 placements(任一冲突 → 409 全滚)。
     * 走 fetchJson → 继承 If-Match 注入。
     */
    async runScheduleApply(date: string): Promise<SchedulerPlan> {
        return this.fetchJson<SchedulerPlan>(`${this.baseUrl}/api/schedule/run`, {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({ date, mode: "apply" }),
        });
    }

    /**
     * applySchedulePlacements —— POST /api/schedule/apply。
     *
     * 客户端显式提交要落库的 placements(带各自 version)。事务性:任一
     * version 冲突 → 后端 409 全事务回滚,这里抛 VersionConflictError 供上层
     * 展示冲突并刷新。
     *
     * 该端点需要识别 409 语义,而 `fetchJson` 只抛普通 Error(拿不到 status),
     * 故与 `patchTaskScores` 一样用 raw fetch;raw 路径需手动补 ConflictGuard
     * 要求的 `If-Match`(scheduler 无 per-resource 版本语义,送 `"0"`)。
     */
    async applySchedulePlacements(
        placements: ScheduleApplyPlacement[],
    ): Promise<{ applied: number }> {
        const res = await fetch(`${this.baseUrl}/api/schedule/apply`, {
            method: "POST",
            headers: {
                "Content-Type": "application/json",
                "If-Match": '"0"',
            },
            body: JSON.stringify({ placements }),
        });
        if (res.status === 409) {
            throw new VersionConflictError();
        }
        if (!res.ok) {
            throw new Error(`HTTP ${res.status}: ${await res.text()}`);
        }
        return res.json() as Promise<{ applied: number }>;
    }
}
