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
