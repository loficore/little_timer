/**
 * Little Timer 的平台感知 API 客户端。
 *
 * 在 Android（通过 window.wails 检测）：使用 Wails v3 的 JS↔Go 绑定，
 * 经由 JNI —— 无需 HTTP fetch。
 *
 * 在桌面端：使用现有的基于 fetch() 的 APIClient。
 *
 * 这样同一套前端代码可在两个平台运行，
 * 调用处无需改动 —— 始终使用 getAPIClient()。
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
  CredentialStatus,
  AppVersionInfo,
  Settings,
} from "../types/api";
import type {
  DailySweepSettings,
  DailySweepToday,
  LLMSettingsDTO,
  ReviewProposal,
  ScheduleApplyPlacement,
  SchedulerPlan,
  SubtaskDTO,
  TaskDTO,
  TestConnectionResult,
} from "../types/aigtd";
import { UnsupportedOnPlatformError, type PlatformApi, type TaskScoresPatch } from "./platformApi";

/* eslint-disable @typescript-eslint/require-await */
// 检测 Android：Wails v3 在 Android 上会设置 window.wails
const isAndroid = typeof window !== "undefined" && !!(window as any).wails;

// 仅在 Android 运行时懒加载 Wails 绑定。
let _bindings: {
  TimerService: any;
  SettingsService: any;
  HabitService: any;
  BackupService: any;
} | null = null;

async function _loadBindings() {
  if (_bindings) return _bindings;
  // 使用基于字符串的动态导入，防止 Rollup 在构建期解析。
  // 这些文件由 Wails 生成（仅 Android），在桌面构建中不存在。
  const [ts, ss, hs, bs] = await Promise.all([
    import(/* @vite-ignore */ "../bindings/little-timer/internal/app/timerservice.ts"),
    import(/* @vite-ignore */ "../bindings/little-timer/internal/app/settingsservice.ts"),
    import(/* @vite-ignore */ "../bindings/little-timer/internal/app/habitservice.ts"),
    import(/* @vite-ignore */ "../bindings/little-timer/internal/app/backupservice.ts"),
  ]);
  _bindings = {
    TimerService: ts,
    SettingsService: ss,
    HabitService: hs,
    BackupService: bs,
  };
  return _bindings;
}
/**
 * Wails API 客户端 — 封装所有与后端通信的 Wails Go 绑定方法
 *
 * 显式 `implements PlatformApi`：Android 侧未落地的域（AIGTD / 调度 / LLM 设置 /
 * 凭据）也必须有方法，统一抛 `UnsupportedOnPlatformError`。这些入口本就由
 * `capabilities` 隐藏，方法存在只为让两条 transport 共享同一契约。
 */
export class WailsAPIClient implements PlatformApi {
  // 暴露给 useSSE（其访问 apiClient.baseUrl）—— Android 上不启用 SSE
  baseUrl = "";

  private authToken: string | null = null;

  /**
   * 设置认证 Token。Android 走进程内绑定，无 HTTP header 可带；
   * 保留与 `APIClient` 一致的读写语义，避免调用方分叉。
   * @param token 认证 Token，设为 null 可清除
   */
  setAuthToken(token: string | null): void {
    this.authToken = token;
  }

  /**
   * @returns 当前认证 Token
   */
  getAuthToken(): string | null {
    return this.authToken;
  }

  /**
   * 当前平台未实现某能力时的统一兜底。
   *
   * @param feature 用户可读的功能名
   * @throws {UnsupportedOnPlatformError} 总是抛出
   */
  private unsupported(feature: string): never {
    throw new UnsupportedOnPlatformError(feature);
  }

  private async _b(name: 'TimerService' | 'SettingsService' | 'HabitService' | 'BackupService') {
    const b = await _loadBindings();
    return b[name];
  }

  /**
   * 获取当前计时器状态
   */
  async getState(): Promise<TimerState> {
    const TS = await this._b('TimerService');
    return TS.GetState();
  }

  /**
   * 构建信息。Android 侧 Go 绑定暂未暴露版本服务，如实返回 dev，
   * 不谎报版本号（桌面/Web 走 HTTP /api/version）。
   */
  async getVersion(): Promise<AppVersionInfo> {
    return { version: "dev", git_commit: "unknown", build_time: "" };
  }

  /**
   * 启动计时器
   * @param habitId - 可选习惯 ID
   * @param options - 可选计时配置（模式、时长等）
   */
  async startTimer(habitId?: number, options?: TimerStartOptions): Promise<TimerStartResult> {
    const TS = await this._b('TimerService');
    return TS.StartTimer(
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
    const TS = await this._b('TimerService');
    return TS.FinishTimer();
  }

  /**
   * 获取计时器进度详情
   */
  async getTimerProgress(): Promise<TimerProgress> {
    const TS = await this._b('TimerService');
    return TS.GetProgress();
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
    const TS = await this._b('TimerService');
    return TS.StartRest();
  }

  /**
   * 暂停计时器
   */
  async pauseTimer(): Promise<void> {
    const TS = await this._b('TimerService');
    return TS.PauseTimer();
  }

  /**
   * 重置计时器
   */
  async resetTimer(): Promise<void> {
    const TS = await this._b('TimerService');
    return TS.ResetTimer();
  }

  /**
   * 切换计时模式（仅桌面端，Android 无效）
   */
  async changeMode(_mode: "countdown" | "stopwatch"): Promise<void> {
    // Wails 绑定未暴露 changeMode —— 在 Android 上是空操作，
    // 模式由 StartTimer 参数指定
    return Promise.resolve();
  }

  /**
   * 获取应用设置
   */
  async getSettings(): Promise<Settings> {
    const SS = await this._b('SettingsService');
    return SS.GetSettings();
  }

  /**
   * 更新应用设置
   * @param settings - 设置对象
   */
  async updateSettings(settings: object): Promise<void> {
    const SS = await this._b('SettingsService');
    return SS.UpdateSettings(JSON.stringify(settings));
  }

  /**
   * 获取所有习惯集
   */
  async getHabitSets(): Promise<HabitSet[]> {
    const HS = await this._b('HabitService');
    return HS.ListHabitSets();
  }

  /**
   * 创建新习惯集
   * @param name - 名称
   * @param description - 描述
   * @param color - 颜色
   */
  async createHabitSet(name: string, description: string, color: string): Promise<HabitSet> {
    const HS = await this._b('HabitService');
    return HS.CreateHabitSet(name, description, color);
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
    const HS = await this._b('HabitService');
    return HS.UpdateHabitSet(id, name, description, color, wallpaper ?? "");
  }

  /**
   * 删除习惯集
   * @param id - 习惯集 ID
   */
  async deleteHabitSet(id: number): Promise<void> {
    const HS = await this._b('HabitService');
    return HS.DeleteHabitSet(id);
  }

  /**
   * 获取所有习惯
   */
  async getHabits(): Promise<Habit[]> {
    const HS = await this._b('HabitService');
    return HS.ListHabits(null);
  }

  /**
   * 创建新习惯
   * @param setId - 所属习惯集 ID
   * @param name - 名称
   * @param goalSeconds - 目标秒数
   * @param color - 颜色
   */
  async createHabit(setId: number, name: string, goalSeconds: number, color: string): Promise<Habit> {
    const HS = await this._b('HabitService');
    return HS.CreateHabit(setId, name, goalSeconds, color);
  }

  /**
   * 删除习惯
   * @param id - 习惯 ID
   */
  async deleteHabit(id: number): Promise<void> {
    const HS = await this._b('HabitService');
    return HS.DeleteHabit(id);
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
    const HS = await this._b('HabitService');
    return HS.UpdateHabit(id, name, goalSeconds, color, wallpaper ?? "");
  }

  /**
   * 记录一次习惯完成
   * @param habitId - 习惯 ID
   * @param durationSeconds - 实际时长（秒）
   * @param count - 完成次数
   * @param date - 日期字符串
   */
  async createSession(habitId: number, durationSeconds: number, count: number, date: string): Promise<CreateSessionResult> {
    const HS = await this._b('HabitService');
    return HS.CreateSession(habitId, durationSeconds, count, date);
  }

  /**
   * 获取习惯记录列表
   * @param date - 可选特定日期
   * @param startDate - 可选开始日期
   * @param endDate - 可选结束日期
   */
  async getSessions(date?: string, startDate?: string, endDate?: string): Promise<Session[]> {
    const HS = await this._b('HabitService');
    return HS.ListSessions(date ?? "", startDate ?? "", endDate ?? "");
  }

  /**
   * 获取习惯详情（含今日进度）
   * @param habitId - 习惯 ID
   * @param date - 可选日期
   */
  async getHabitDetail(habitId: number, date?: string): Promise<HabitDetail> {
    const HS = await this._b('HabitService');
    return HS.GetHabitDetail(habitId, date ?? "");
  }

  /**
   * 创建备份
   */
  async createBackup(): Promise<BackupCreateResult> {
    const BS = await this._b('BackupService');
    return BS.CreateBackup();
  }

  /**
   * 列出所有备份
   */
  async listBackups(): Promise<BackupListResult> {
    const BS = await this._b('BackupService');
    return BS.ListBackups();
  }

  /**
   * 从备份恢复
   * @param name - 备份文件名
   */
  async restoreBackup(name: string): Promise<BackupRestoreResult> {
    const BS = await this._b('BackupService');
    return BS.RestoreBackup(name);
  }

  /**
   * 删除备份
   * @param name - 备份文件名
   */
  async deleteBackup(name: string): Promise<BackupVerifyResult> {
    const BS = await this._b('BackupService');
    return BS.DeleteBackup(name);
  }

  /**
   * 验证备份有效性
   */
  async verifyBackup(): Promise<BackupVerifyResult> {
    const BS = await this._b('BackupService');
    return BS.VerifyBackup();
  }

  /**
   * 获取凭据子系统状态（取代已删的主口令状态）。
   *
   * backend === "unavailable" 表示磁盘上有凭据文件但系统密钥环解不开
   * （keyring 被重置 / 条目被删）。这时要展示 detail 里的可操作原因，
   * 而不是引导用户重新填写凭据 —— 那是环境问题，填了也白填。
   */
  async getCredentialStatus(): Promise<CredentialStatus> {
    const BS = await this._b('BackupService');
    return BS.CredentialStatus();
  }

  /**
   * 获取备份配置
   */
  async getBackupConfig(): Promise<BackupConfig> {
    const BS = await this._b('BackupService');
    return BS.GetBackupConfig();
  }

  /**
   * 更新备份配置
   * @param config - 备份配置对象
   */
  async updateBackupConfig(config: BackupConfig): Promise<{ success: boolean; error?: string }> {
    const BS = await this._b('BackupService');
    return BS.UpdateBackupConfig(JSON.stringify(config));
  }

  /**
   * 上传壁纸（Android 不可用，返回空）
   * @param _file - 壁纸文件
   */
  async uploadWallpaper(_file: File): Promise<WallpaperUploadResult> {
    // Android 上壁纸上传不可用（无 HTTP 服务器）；返回空响应以免 UI 出错。
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

  /**
   * 从 URL 获取壁纸（Android 不可用，返回空）
   * @param _url - 壁纸 URL
   */
  async fetchWallpaperByUrl(_url: string): Promise<WallpaperUploadResult> {
    return { filename: "" };
  }
  // ===== GTD Daily Sweep / Review (v12) =====
  // 入口已由 capabilities.dailySweep 隐藏，此处的 throw 是深链兜底的内部
  // 不变量；错误文案即用户可见口径。绑定补齐跟踪见 GitHub issue #41。
  /**
   * 手动触发一次 review（当前平台未实现，见 issue #41）。
   */
  async startDailySweep(): Promise<{ job_ids: number[] }> {
    return this.unsupported("每日梳理");
  }

  /**
   * 拉当日 review 提议（当前平台未实现，见 issue #41）。
   */
  async getDailySweepToday(): Promise<DailySweepToday> {
    return this.unsupported("每日梳理");
  }

  /**
   * 采纳一条提议（当前平台未实现，见 issue #41）。
   * @param _body 采纳请求体
   */
  async adoptDailySweep(_body: {
    change_id: string;
    ai_job_id: number;
    version: number;
    proposal: ReviewProposal;
  }): Promise<{ task_id: number; version_after: number; idempotent?: boolean }> {
    return this.unsupported("每日梳理");
  }

  /**
   * 拒绝一条提议（当前平台未实现，见 issue #41）。
   * @param _body 拒绝请求体
   */
  async rejectDailySweep(_body: { change_id: string }): Promise<void> {
    return this.unsupported("每日梳理");
  }

  /**
   * 读每日梳理设置（当前平台未实现，见 issue #41）。
   */
  async getDailySweepSettings(): Promise<DailySweepSettings> {
    return this.unsupported("每日梳理");
  }

  /**
   * 写每日梳理设置（当前平台未实现，见 issue #41）。
   * @param _patch 要更新的字段
   */
  async updateDailySweepSettings(_patch: {
    enabled?: boolean;
    time?: string;
  }): Promise<DailySweepSettings> {
    return this.unsupported("每日梳理");
  }

  // ===== Android 未实现的域（入口由 capabilities 隐藏）=====
  // 这些方法存在只为满足 PlatformApi：桌面 / Web 走 HTTP，Android 的
  // Wails 绑定尚未暴露对应服务。调用即抛 UnsupportedOnPlatformError。

  /**
   * 提交一条随手记 → 异步拆解（Android 未实现）。
   * @param _rawText 原始文本
   * @param _source 来源
   */
  async captureAIGtd(_rawText: string, _source?: string): Promise<{ task_id: number }> {
    return this.unsupported("AIGTD");
  }

  /**
   * 重新解析一个 task（Android 未实现）。
   * @param _taskId 任务 ID
   * @param _version 任务当前版本
   * @param _newRawText 新原文（可选）
   */
  async reparseAIGtd(
    _taskId: number,
    _version: number,
    _newRawText?: string,
  ): Promise<{ job_id: number }> {
    return this.unsupported("AIGTD");
  }

  /**
   * 查询任务当前 AI 处理状态（Android 未实现）。
   * @param _taskId 任务 ID
   */
  async getAIGtdJob(_taskId: number): Promise<{
    task_id: number;
    ai_status: string;
    ai_error?: string;
    latest_job_id?: number;
    attempts?: number;
  }> {
    return this.unsupported("AIGTD");
  }

  /**
   * 列出顶层任务（Android 未实现）。
   * @param _status 状态过滤
   */
  async listTasks(_status?: string): Promise<TaskDTO[]> {
    return this.unsupported("AIGTD");
  }

  /**
   * 取单条任务（Android 未实现）。
   * @param _id 任务 ID
   */
  async getTask(_id: number): Promise<TaskDTO> {
    return this.unsupported("AIGTD");
  }

  /**
   * 直接创建任务（Android 未实现）。
   * @param _title 标题
   * @param _notes 备注（可选）
   */
  async createTask(_title: string, _notes?: string): Promise<TaskDTO> {
    return this.unsupported("AIGTD");
  }

  /**
   * 局部更新任务（Android 未实现）。
   * @param _id 任务 ID
   * @param _patch 要更新的字段
   */
  async updateTask(_id: number, _patch: Partial<TaskDTO>): Promise<TaskDTO> {
    return this.unsupported("AIGTD");
  }

  /**
   * 删除任务（Android 未实现）。
   * @param _id 任务 ID
   * @param _version 任务当前版本
   */
  async deleteTask(_id: number, _version: number): Promise<void> {
    return this.unsupported("AIGTD");
  }

  /**
   * 切换子任务完成状态（Android 未实现）。
   * @param _taskId 父任务 ID
   * @param _subId 子任务 ID
   * @param _status 目标状态
   * @param _version 子任务自己的版本
   */
  async toggleSubtask(
    _taskId: number,
    _subId: number,
    _status: "active" | "done",
    _version: number,
  ): Promise<void> {
    return this.unsupported("AIGTD");
  }

  /**
   * 更新子任务（Android 未实现）。
   * @param _taskId 父任务 ID
   * @param _subId 子任务 ID
   * @param _patch 要更新的字段
   * @param _version 子任务自己的版本
   */
  async updateSubtask(
    _taskId: number,
    _subId: number,
    _patch: { title?: string; estimated_minutes?: number; status?: "active" | "done" | "archived" },
    _version: number,
  ): Promise<void> {
    return this.unsupported("AIGTD");
  }

  /**
   * 新增子任务（Android 未实现）。
   * @param _taskId 父任务 ID
   * @param _body 新子任务字段
   * @param _version 父任务当前版本
   */
  async createSubtask(
    _taskId: number,
    _body: { title: string; estimated_minutes: number; order_index: number },
    _version: number,
  ): Promise<SubtaskDTO> {
    return this.unsupported("AIGTD");
  }

  /**
   * 删除子任务（Android 未实现）。
   * @param _taskId 父任务 ID
   * @param _subId 子任务 ID
   * @param _version 用于通过 ConflictGuard 的版本号
   */
  async deleteSubtask(_taskId: number, _subId: number, _version: number): Promise<void> {
    return this.unsupported("AIGTD");
  }

  /**
   * 局部更新 5 维评分（Android 未实现）。
   * @param _id 任务 ID
   * @param _body 评分 patch
   */
  async patchTaskScores(_id: number, _body: TaskScoresPatch): Promise<TaskDTO> {
    return this.unsupported("AIGTD");
  }

  /**
   * 读 LLM 设置（Android 未实现）。
   */
  async getLLMSettings(): Promise<LLMSettingsDTO> {
    return this.unsupported("大模型设置");
  }

  /**
   * 写 LLM 设置（Android 未实现）。
   * @param _patch 要更新的字段
   */
  async updateLLMSettings(_patch: {
    provider?: "openai_compat" | "anthropic";
    base_url?: string;
    model?: string;
    max_tokens?: number;
    timeout_seconds?: number;
    api_key?: string;
    clear_key?: boolean;
  }): Promise<LLMSettingsDTO> {
    return this.unsupported("大模型设置");
  }

  /**
   * 测试 LLM 连通性（Android 未实现）。
   * @param _patch 当前表单值
   */
  async testLLMSettings(_patch: {
    provider?: "openai_compat" | "anthropic";
    base_url?: string;
    model?: string;
    max_tokens?: number;
    timeout_seconds?: number;
    api_key?: string;
  }): Promise<TestConnectionResult> {
    return this.unsupported("大模型设置");
  }

  /**
   * 预览某天的排程（Android 未实现）。
   * @param _date 日期（YYYY-MM-DD）
   */
  async runSchedulePreview(_date: string): Promise<SchedulerPlan> {
    return this.unsupported("计划调度");
  }

  /**
   * 落库某天的排程（Android 未实现）。
   * @param _date 日期（YYYY-MM-DD）
   */
  async runScheduleApply(_date: string): Promise<SchedulerPlan> {
    return this.unsupported("计划调度");
  }

  /**
   * 显式提交要落库的 placements（Android 未实现）。
   * @param _placements 带各自 version 的 placement 列表
   */
  async applySchedulePlacements(
    _placements: ScheduleApplyPlacement[],
  ): Promise<{ applied: number }> {
    return this.unsupported("计划调度");
  }

  /**
   * 重置凭据（Android 未实现）。
   */
  async resetCredentials(): Promise<void> {
    return this.unsupported("凭据管理");
  }

  /**
   * 锁定凭据（Android 未实现）。
   */
  async lockCredentials(): Promise<{ success: boolean }> {
    return this.unsupported("凭据管理");
  }
}

export { isAndroid };
