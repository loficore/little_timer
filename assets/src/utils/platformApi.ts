/**
 * PlatformApi —— 前端 transport 的唯一契约。
 *
 * 前端有两条 transport：
 *  - 桌面 / Web：`APIClient`（HTTP fetch）
 *  - Android：`WailsAPIClient`（Wails v3 JS↔Go 绑定）
 *
 * 两个实现都显式 `implements PlatformApi`，任何一侧漏实现方法都会让
 * `pnpm run build:check` 失败，而不是把缺口推迟到运行期。
 *
 * 能力表 `capabilities` 必须是**模块级常量**，不能做成 client 实例属性：
 * Android 首次调用返回的 Proxy 会把任意属性读取包装成 async 函数，
 * 实例上读到的能力永远是 truthy 的函数。
 */

import type {
  AppVersionInfo,
  BackupConfig,
  BackupCreateResult,
  BackupListResult,
  BackupRestoreResult,
  BackupVerifyResult,
  CreateSessionResult,
  CredentialStatus,
  Habit,
  HabitDetail,
  HabitSet,
  RestResult,
  ResumeResult,
  Session,
  Settings,
  TimerFinishResult,
  TimerProgress,
  TimerStartOptions,
  TimerStartResult,
  TimerState,
  WallpaperDeleteResult,
  WallpaperListResult,
  WallpaperUploadResult,
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

/**
 * TaskScoresPatch —— PATCH /api/tasks/:id/scores 的请求体形状。
 *
 * 指针语义：除 version 外所有字段可选；后端 omitempty，缺 = "不动"。
 * version 必须带，后端 ConflictGuard 缺则 400。
 */
export interface TaskScoresPatch {
  /** 乐观锁版本号，必须与后端当前版本一致。 */
  version: number;
  /** 优先级评分。 */
  priority_score?: number;
  /** 紧急度评分。 */
  urgency_score?: number;
  /** 所需精力。 */
  energy_required?: number;
  /** 上下文标签。 */
  context_tag?: string;
}

/**
 * 平台能力表：`true` 表示该域在当前平台可用。
 *
 * 导航与页面据此过滤入口，而不是先渲染再在运行期抛错。
 */
export interface PlatformCapabilities {
  /** 计时器（状态、开始/暂停/结束、模式切换）。 */
  timer: boolean;
  /** 习惯集、习惯、会话与习惯详情。 */
  habits: boolean;
  /** 备份的创建 / 列表 / 恢复 / 删除 / 校验与备份配置。 */
  backup: boolean;
  /** 应用设置读写。 */
  settings: boolean;
  /** AIGTD 任务（capture / 解析 / 子任务 / 评分）。 */
  tasks: boolean;
  /** 调度预览与落库。 */
  schedule: boolean;
  /** LLM 设置的读写与连通性测试。 */
  llmSettings: boolean;
  /** 凭据状态查询、重置与锁定。 */
  credentials: boolean;
  /** 壁纸上传 / 列表 / 删除 / 远程拉取。 */
  wallpaper: boolean;
  /** 每日梳理（daily sweep）与 review 采纳。 */
  dailySweep: boolean;
}

/**
 * 是否 Android（Wails 运行时）。Wails v3 在 Android 上注入 `window.wails`。
 *
 * 桌面浏览器没有该全局，因此不会被误判为 Android。
 */
const isAndroid = typeof window !== "undefined" && !!(window as any).wails;

/**
 * 当前平台的能力表。
 *
 * 模块级常量：Wails 首调 Proxy 只影响实例属性读取，不影响这里。
 */
export const capabilities: PlatformCapabilities = {
  timer: true,
  habits: true,
  backup: true,
  settings: true,
  tasks: !isAndroid,
  schedule: !isAndroid,
  llmSettings: !isAndroid,
  credentials: !isAndroid,
  wallpaper: !isAndroid,
  dailySweep: !isAndroid,
};

/**
 * 当前平台未实现某能力时抛出的错误。
 *
 * 这是深链兜底的内部不变量（入口已由 `capabilities` 隐藏），
 * message 使用用户可读口径，不得泄露 "not implemented" 之类的内部术语。
 */
export class UnsupportedOnPlatformError extends Error {
  /**
   * @param feature 用户可读的功能名，例如 "AIGTD"。
   */
  constructor(feature: string) {
    super(`${feature}在当前平台不可用`);
    this.name = "UnsupportedOnPlatformError";
  }
}

/**
 * 计时器域：状态读取、开始/暂停/结束/重置、模式切换与认证 header。
 */
export interface TimerApi {
  /** API 基础 URL（SSE 等旁路连接需要）。 */
  baseUrl: string;
  /**
   * 设置认证 Token。
   * @param token 认证 Token，设为 null 可清除
   */
  setAuthToken(token: string | null): void;
  /** @returns 当前认证 Token */
  getAuthToken(): string | null;
  /** @returns 当前计时器状态 */
  getState(): Promise<TimerState>;
  /** @returns 进程级构建信息（版本号 / commit / 构建时间） */
  getVersion(): Promise<AppVersionInfo>;
  /**
   * 开始计时器
   * @param habitId 习惯 ID（可选）
   * @param options 计时配置（可选）
   */
  startTimer(habitId?: number, options?: TimerStartOptions): Promise<TimerStartResult>;
  /** 结束计时器（停止并计入统计） */
  finishTimer(): Promise<TimerFinishResult>;
  /** 获取计时进度（用于刷新恢复） */
  getTimerProgress(): Promise<TimerProgress>;
  /**
   * 恢复计时器（从暂停继续）
   * @param habitId 习惯 ID（可选）
   */
  resumeTimer(habitId?: number): Promise<ResumeResult>;
  /** 开始休息 */
  startRest(): Promise<RestResult>;
  /** 暂停计时器 */
  pauseTimer(): Promise<void>;
  /** 重置计时器 */
  resetTimer(): Promise<void>;
  /**
   * 切换计时器模式
   * @param mode 目标模式
   */
  changeMode(mode: "countdown" | "stopwatch"): Promise<void>;
}

/**
 * 设置域：应用设置与 LLM 设置。
 */
export interface SettingsApi {
  /** @returns 应用设置 */
  getSettings(): Promise<Settings>;
  /**
   * 更新设置
   * @param settings 要更新的设置对象
   */
  updateSettings(settings: object): Promise<void>;
  /** @returns LLM 设置 */
  getLLMSettings(): Promise<LLMSettingsDTO>;
  /**
   * 写 LLM 设置（任意字段可选）
   * @param patch 要更新的字段
   */
  updateLLMSettings(patch: {
    provider?: "openai_compat" | "anthropic";
    base_url?: string;
    model?: string;
    max_tokens?: number;
    timeout_seconds?: number;
    api_key?: string;
    clear_key?: boolean;
  }): Promise<LLMSettingsDTO>;
  /**
   * 测试 LLM 连通性（业务失败以 ok=false 返回，不抛异常）
   * @param patch 当前表单值；api_key 留空表示沿用已保存的 Key
   */
  testLLMSettings(patch: {
    provider?: "openai_compat" | "anthropic";
    base_url?: string;
    model?: string;
    max_tokens?: number;
    timeout_seconds?: number;
    api_key?: string;
  }): Promise<TestConnectionResult>;
}

/**
 * 习惯域：习惯集 / 习惯 / 会话 / 习惯详情。
 */
export interface HabitApi {
  /** @returns 习惯集列表 */
  getHabitSets(): Promise<HabitSet[]>;
  /**
   * 创建习惯集
   * @param name 名称
   * @param description 描述
   * @param color 颜色
   */
  createHabitSet(name: string, description: string, color: string): Promise<HabitSet>;
  /**
   * 更新习惯集
   * @param id 习惯集 ID
   * @param name 名称
   * @param description 描述
   * @param color 颜色
   * @param wallpaper 壁纸（可选）
   */
  updateHabitSet(
    id: number,
    name: string,
    description: string,
    color: string,
    wallpaper?: string,
  ): Promise<HabitSet>;
  /**
   * 删除习惯集
   * @param id 习惯集 ID
   */
  deleteHabitSet(id: number): Promise<void>;
  /** @returns 习惯列表 */
  getHabits(): Promise<Habit[]>;
  /**
   * 创建习惯
   * @param setId 习惯集 ID
   * @param name 习惯名称
   * @param goalSeconds 目标时间（秒）
   * @param color 颜色
   */
  createHabit(setId: number, name: string, goalSeconds: number, color: string): Promise<Habit>;
  /**
   * 删除习惯
   * @param id 习惯 ID
   */
  deleteHabit(id: number): Promise<void>;
  /**
   * 更新习惯
   * @param id 习惯 ID
   * @param name 名称
   * @param goalSeconds 目标时长（秒）
   * @param color 颜色
   * @param wallpaper 壁纸（可选）
   */
  updateHabit(
    id: number,
    name: string,
    goalSeconds: number,
    color: string,
    wallpaper?: string,
  ): Promise<Habit>;
  /**
   * 创建记录
   * @param habitId 习惯 ID
   * @param durationSeconds 持续时间（秒）
   * @param count 次数
   * @param date 日期
   */
  createSession(
    habitId: number,
    durationSeconds: number,
    count: number,
    date: string,
  ): Promise<CreateSessionResult>;
  /**
   * 获取记录列表
   * @param date 日期
   * @param startDate 开始日期
   * @param endDate 结束日期
   */
  getSessions(date?: string, startDate?: string, endDate?: string): Promise<Session[]>;
  /**
   * 获取习惯详情
   * @param habitId 习惯 ID
   * @param date 日期（可选，默认今天）
   */
  getHabitDetail(habitId: number, date?: string): Promise<HabitDetail>;
}

/**
 * 备份域：备份文件与备份配置。
 */
export interface BackupApi {
  /** 创建数据库备份 */
  createBackup(): Promise<BackupCreateResult>;
  /** @returns 备份列表 */
  listBackups(): Promise<BackupListResult>;
  /**
   * 从备份恢复数据库
   * @param name 要从中恢复的备份文件名
   */
  restoreBackup(name: string): Promise<BackupRestoreResult>;
  /**
   * 删除指定备份
   * @param name 要删除的备份文件名
   */
  deleteBackup(name: string): Promise<BackupVerifyResult>;
  /** 验证备份目标配置是否有效 */
  verifyBackup(): Promise<BackupVerifyResult>;
  /** @returns 当前备份配置 */
  getBackupConfig(): Promise<BackupConfig>;
  /**
   * 更新备份配置
   * @param config 含更新后设置的备份配置
   */
  updateBackupConfig(config: BackupConfig): Promise<{ success: boolean; error?: string }>;
}

/**
 * 凭据域：凭据子系统状态与破坏性操作。
 */
export interface CredentialApi {
  /** @returns 凭据子系统状态 */
  getCredentialStatus(): Promise<CredentialStatus>;
  /** 重置凭据：删除主密钥并清空全部凭据（破坏性） */
  resetCredentials(): Promise<void>;
  /** 锁定凭据 */
  lockCredentials(): Promise<{ success: boolean }>;
}

/**
 * 壁纸域：上传 / 列表 / 删除 / 远程拉取。
 */
export interface WallpaperApi {
  /**
   * 上传壁纸图片
   * @param file 图片文件
   */
  uploadWallpaper(file: File): Promise<WallpaperUploadResult>;
  /** @returns 已上传的壁纸列表 */
  listWallpapers(): Promise<WallpaperListResult[]>;
  /**
   * 删除指定壁纸
   * @param filename 文件名
   */
  deleteWallpaper(filename: string): Promise<WallpaperDeleteResult>;
  /**
   * 从 URL 获取壁纸
   * @param url 壁纸 URL
   */
  fetchWallpaperByUrl(url: string): Promise<WallpaperUploadResult>;
}

/**
 * AIGTD 域：任务 / 子任务 / 解析任务状态。
 */
export interface TaskApi {
  /**
   * 提交一条随手记 → 异步拆解
   * @param rawText 原始文本
   * @param source 来源（默认 manual）
   */
  captureAIGtd(rawText: string, source?: string): Promise<{ task_id: number }>;
  /**
   * 重新解析一个 task（可选覆盖原文）
   * @param taskId 任务 ID
   * @param version 任务当前版本
   * @param newRawText 新原文（可选）
   */
  reparseAIGtd(
    taskId: number,
    version: number,
    newRawText?: string,
  ): Promise<{ job_id: number }>;
  /**
   * 查询任务当前 AI 处理状态
   * @param taskId 任务 ID
   */
  getAIGtdJob(taskId: number): Promise<{
    task_id: number;
    ai_status: string;
    ai_error?: string;
    latest_job_id?: number;
    attempts?: number;
  }>;
  /**
   * 列出顶层任务（可按状态过滤）
   * @param status 状态过滤
   */
  listTasks(status?: string): Promise<TaskDTO[]>;
  /**
   * 取单条任务（含子任务）
   * @param id 任务 ID
   */
  getTask(id: number): Promise<TaskDTO>;
  /**
   * 直接创建任务（不经过 AI）
   * @param title 标题
   * @param notes 备注（可选）
   */
  createTask(title: string, notes?: string): Promise<TaskDTO>;
  /**
   * 局部更新任务（指针语义）
   * @param id 任务 ID
   * @param patch 要更新的字段（version 必带）
   */
  updateTask(id: number, patch: Partial<TaskDTO>): Promise<TaskDTO>;
  /**
   * 删除任务
   * @param id 任务 ID
   * @param version 任务当前版本
   */
  deleteTask(id: number, version: number): Promise<void>;
  /**
   * 切换子任务完成状态
   * @param taskId 父任务 ID
   * @param subId 子任务 ID
   * @param status 目标状态
   * @param version 子任务自己的版本（不是父任务的）
   */
  toggleSubtask(
    taskId: number,
    subId: number,
    status: "active" | "done",
    version: number,
  ): Promise<void>;
  /**
   * 更新子任务（标题 / 分钟数 / 状态）
   * @param taskId 父任务 ID
   * @param subId 子任务 ID
   * @param patch 要更新的字段
   * @param version 子任务自己的版本
   */
  updateSubtask(
    taskId: number,
    subId: number,
    patch: { title?: string; estimated_minutes?: number; status?: "active" | "done" | "archived" },
    version: number,
  ): Promise<void>;
  /**
   * 新增子任务
   * @param taskId 父任务 ID
   * @param body 新子任务字段
   * @param version 父任务当前版本
   */
  createSubtask(
    taskId: number,
    body: { title: string; estimated_minutes: number; order_index: number },
    version: number,
  ): Promise<SubtaskDTO>;
  /**
   * 删除子任务
   * @param taskId 父任务 ID
   * @param subId 子任务 ID
   * @param version 用于通过 ConflictGuard 的版本号
   */
  deleteSubtask(taskId: number, subId: number, version: number): Promise<void>;
  /**
   * 局部更新 5 维评分（指针语义）
   * @param id 任务 ID
   * @param body 评分 patch（version 必带）
   */
  patchTaskScores(id: number, body: TaskScoresPatch): Promise<TaskDTO>;
}

/**
 * 调度域：预览 / 落库 / 显式提交 placements。
 */
export interface ScheduleApi {
  /**
   * 预览某天的排程（不落库）
   * @param date 日期（YYYY-MM-DD）
   */
  runSchedulePreview(date: string): Promise<SchedulerPlan>;
  /**
   * 落库某天的排程（服务端事务 + 乐观锁）
   * @param date 日期（YYYY-MM-DD）
   */
  runScheduleApply(date: string): Promise<SchedulerPlan>;
  /**
   * 显式提交要落库的 placements（事务性，冲突全滚）
   * @param placements 带各自 version 的 placement 列表
   */
  applySchedulePlacements(
    placements: ScheduleApplyPlacement[],
  ): Promise<{ applied: number }>;
}

/**
 * 每日梳理域：review 触发、提议采纳 / 拒绝与设置。
 */
export interface DailySweepApi {
  /** 手动触发一次 review */
  startDailySweep(): Promise<{ job_ids: number[] }>;
  /** @returns 当日的 review 提议 */
  getDailySweepToday(): Promise<DailySweepToday>;
  /**
   * 采纳一条提议（幂等；409 = version 冲突）
   * @param body 采纳请求体
   */
  adoptDailySweep(body: {
    change_id: string;
    ai_job_id: number;
    version: number;
    proposal: ReviewProposal;
  }): Promise<{ task_id: number; version_after: number; idempotent?: boolean }>;
  /**
   * 拒绝一条提议
   * @param body 拒绝请求体
   */
  rejectDailySweep(body: { change_id: string }): Promise<void>;
  /** @returns 每日梳理设置 */
  getDailySweepSettings(): Promise<DailySweepSettings>;
  /**
   * 写每日梳理设置（字段可选；非法时间后端 400）
   * @param patch 要更新的字段
   */
  updateDailySweepSettings(patch: {
    enabled?: boolean;
    time?: string;
  }): Promise<DailySweepSettings>;
}

/**
 * 两条 transport 必须共同满足的完整契约。
 */
export type PlatformApi = TimerApi &
  SettingsApi &
  HabitApi &
  BackupApi &
  CredentialApi &
  WallpaperApi &
  TaskApi &
  ScheduleApi &
  DailySweepApi;
