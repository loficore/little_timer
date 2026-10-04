/**
 * AI GTD 前端类型(与后端 DTO 对齐)。
 */

export interface SubtaskDTO {
  id: number;
  task_id: number;
  title: string;
  status: "pending" | "done" | "skipped";
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
