/**
 * AI GTD 前端类型(与后端 DTO 对齐)。
 */

export interface SubtaskDTO {
  id: number;
  task_id: number;
  title: string;
  status: "active" | "done" | "archived";
  estimated_minutes: number;
  order_index: number;
}

export interface TaskDTO {
  id: number;
  parent_id?: number | null;
  raw_text?: string;
  title: string;
  notes?: string;
  status: "inbox" | "active" | "done" | "archived" | "rejected";
  due_date?: string | null;
  scheduled_start?: number | null;
  scheduled_end?: number | null;
  pinned: boolean;
  estimated_minutes: number;
  order_index: number;
  source: string;
  ai_status: "pending" | "processing" | "done" | "error";
  ai_error?: string;
  subtasks?: SubtaskDTO[];
  // v11:乐观锁版本号(server 每次 UpdateTask ++)。所有 mutation 必须带回。
  version: number;
  // v11:5 维调度评分(scheduler §3.1)。context_tag 由 LLM 预填 + 用户可改;
  // priority/urgency/energy 同上;blocked_by 仅用户显式标(不经 LLM)。
  priority_score: number;
  urgency_score: number;
  energy_required: number;
  context_tag: string;
  blocked_by?: number[];
  user_edited_title?: boolean;
  created_at: string;
  updated_at: string;
  parsed_at?: string | null;
}

export interface JobStatusDTO {
  task_id: number;
  ai_status: TaskDTO["ai_status"];
  ai_error?: string;
  latest_job_id?: number;
  attempts?: number;
}

export interface LLMSettingsDTO {
  provider: "openai_compat" | "anthropic";
  base_url: string;
  model: string;
  max_tokens: number;
  timeout_seconds: number;
  api_key_set: boolean;
  api_key_masked?: string;
}

export interface TestConnectionResult {
  ok: boolean;
  latency_ms?: number;
  model_echo?: string;
  error?: string;
}

// ===== Scheduler（Task 9）=====

/**
 * SchedulerPlacement —— `POST /api/schedule/run` 返回的单个 placement。
 *
 * ⚠️ 关键决策（PascalCase vs camelCase）：
 * `scheduler.Placement`（Go）**没有 json tag**，`encoding/json` 会按导出字段名
 * 原样输出，即 `TaskID` / `ScheduledStart` / `ScheduledEnd` / `Reason`。
 * 因此前端类型直接采用 **PascalCase** 与后端保持一致，避免在 apiClient 层
 * 维护一层易漏字段的适配器。显示层需要的 camelCase 在组件边界内自行转换。
 */
export interface SchedulerPlacement {
  TaskID: number;
  ScheduledStart: number; // unix 秒
  ScheduledEnd: number; // unix 秒
  Reason: string;
}

/**
 * SchedulerPlan —— `POST /api/schedule/run` 的响应。
 *
 * 同为 Go 默认 JSON 键（PascalCase）：`Date` / `Placements` /
 * `Unscheduled` / `Warnings`。
 */
export interface SchedulerPlan {
  Date: string; // YYYY-MM-DD
  Placements: SchedulerPlacement[];
  Unscheduled: number[]; // 排不下的 task id
  Warnings: string[];
}

/** 简名别名（对齐 Task 9 brief 的命名）。 */
export type Plan = SchedulerPlan;
export type Placement = SchedulerPlacement;

/**
 * ScheduleApplyPlacement —— `POST /api/schedule/apply` 的请求体单条。
 *
 * 注意：与响应不同，请求体由 handler 的 `schedulePlacement` 结构体显式声明了
 * snake_case json tag，因此这里必须是 snake_case。
 */
export interface ScheduleApplyPlacement {
  task_id: number;
  scheduled_start: number;
  scheduled_end: number;
  version: number;
}

// ===== GTD Daily Sweep / Review (v12) =====

/** 子任务变更 op 集(delta Δ3)。 */
export interface SuggestedSubChange {
  subtask_id?: number | null;
  op: "delete" | "modify_title" | "add" | "mark_done";
  new_title?: string | null;
  reason?: string;
}

/** AI review 提议(与后端 domain.ReviewProposal 对齐)。 */
export interface ReviewProposal {
  change_id: string;
  task_id: number;
  action: "reschedule" | "drop" | "modify_subtasks" | "no_op";
  suggested_scheduled_start?: number | null;
  suggested_scheduled_end?: number | null;
  suggested_subtask_changes?: SuggestedSubChange[];
  reason: string;
}

/** today 出参里的一条提议:proposal 本体 + 来源 job + 当前 task version + 是否已采纳。 */
export interface DailySweepProposal {
  ai_job_id: number;
  version: number;
  is_adopted?: boolean;
  proposal: ReviewProposal;
}

/** GET /api/daily-sweep/today 出参。 */
export interface DailySweepToday {
  swept_at?: string;
  source?: "cron" | "manual";
  proposals: DailySweepProposal[];
}

/** GET /api/settings/daily-sweep 出参。 */
export interface DailySweepSettings {
  enabled: boolean;
  time: string;
}
