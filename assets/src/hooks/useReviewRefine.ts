/**
 * useReviewRefine —— Review 提案精化 hook(issue #32)。
 *
 * 流程:
 *   1) refine(aiJobId, originalChangeId, chipHint, userMessage, taskId)
 *      → POST /api/daily-sweep/refine
 *   2) 后端返:
 *        - chip_hint="重写" → { reparse_job_id, mode: "rewrite" }
 *        - 其他           → { job_id, mode: "refine" }
 *   3) 前端轮询 GET /api/aigtd/jobs?task_id,按 latest_job_id + latest_job_status
 *      判定目标 job 是否完成(不看 task.ai_status —— 避免上一次 job 遗留的
 *      'done' 造成即时误判):
 *        - mode="refine": 完成时读 response_payload,解析为 ReviewProposal[] 取首条
 *        - mode="rewrite": 完成时不返 proposal,由上层关闭 panel 并刷新列表
 *
 * 状态机:idle → submitting → waiting → done | error
 */
import { useCallback, useEffect, useRef, useState } from "preact/hooks";
import { getAPIClient } from "../utils/apiClientSingleton";
import { logError } from "../utils/logger";
import type { ReviewProposal } from "../types/aigtd";

export type RefineStatus = "idle" | "submitting" | "waiting" | "done" | "error";

export interface UseReviewRefineOptions {
    /** 轮询间隔(ms),默认 1500 */
    pollIntervalMs?: number;
}

export interface RefineSuccess {
    status: "done";
    mode: "refine" | "rewrite";
    /** refine 模式下为新 proposal;rewrite 模式下为 null(由上层刷新 task 列表) */
    proposal: ReviewProposal | null;
}

export interface RefineError {
    status: "error";
    message: string;
}

export type RefineResult = RefineSuccess | RefineError;

const POLL_MAX_ATTEMPTS = 60; // 60 × 1500ms ≈ 90s 上限

export function useReviewRefine(opts: UseReviewRefineOptions = {}) {
    const pollIntervalMs = opts.pollIntervalMs ?? 1500;
    const [status, setStatus] = useState<RefineStatus>("idle");
    const [error, setError] = useState<string | null>(null);
    const [proposal, setProposal] = useState<ReviewProposal | null>(null);
    const [lastMode, setLastMode] = useState<"refine" | "rewrite" | null>(null);
    const cancelledRef = useRef(false);

    const reset = useCallback(() => {
        cancelledRef.current = true;
        cancelledRef.current = false;
        setStatus("idle");
        setError(null);
        setProposal(null);
        setLastMode(null);
    }, []);

    const refine = useCallback(
        async (
            aiJobId: number,
            originalChangeId: string,
            chipHint: string,
            userMessage: string,
            taskId: number,
        ): Promise<RefineResult> => {
            cancelledRef.current = false;
            setStatus("submitting");
            setError(null);
            setProposal(null);
            setLastMode(null);
            try {
                const resp = await getAPIClient().refineDailySweep({
                    ai_job_id: aiJobId,
                    original_change_id: originalChangeId,
                    chip_hint: chipHint,
                    user_message: userMessage,
                });
                setLastMode(resp.mode);
                setStatus("waiting");

                const targetJobId = resp.mode === "rewrite" ? resp.reparse_job_id : resp.job_id;
                const job = await pollJobDone(taskId, targetJobId, pollIntervalMs, cancelledRef);
                if (!job.done) {
                    setError(job.message);
                    setStatus("error");
                    return { status: "error", message: job.message };
                }
                if (resp.mode === "rewrite") {
                    setStatus("done");
                    return { status: "done", mode: "rewrite", proposal: null };
                }
                const newProposal = parseFirstProposal(job.responsePayload);
                if (!newProposal) {
                    const msg = "refine 完成但未解析出 proposal";
                    setError(msg);
                    setStatus("error");
                    return { status: "error", message: msg };
                }
                setProposal(newProposal);
                setStatus("done");
                return { status: "done", mode: "refine", proposal: newProposal };
            } catch (e) {
                const msg = e instanceof Error ? e.message : String(e);
                setError(msg);
                setStatus("error");
                logError("useReviewRefine.refine failed", e instanceof Error ? e : new Error(msg));
                return { status: "error", message: msg };
            }
        },
        [pollIntervalMs],
    );

    useEffect(() => {
        return () => {
            cancelledRef.current = true;
        };
    }, []);

    return { status, error, proposal, lastMode, refine, reset };
}

interface PollDoneResult {
    done: boolean;
    message: string;
    responsePayload?: string;
}

/** 轮询 job 直到目标 job 完成(按 job id 判定,避免 task.ai_status 旧值误判)。 */
async function pollJobDone(
    taskId: number,
    targetJobId: number,
    intervalMs: number,
    cancelledRef: { current: boolean },
): Promise<PollDoneResult> {
    for (let i = 0; i < POLL_MAX_ATTEMPTS; i++) {
        await sleep(intervalMs);
        if (cancelledRef.current) {
            return { done: false, message: "cancelled" };
        }
        try {
            const j = await getAPIClient().getAIGtdJob(taskId);
            if (j.latest_job_id === targetJobId) {
                if (j.latest_job_status === "success") {
                    return { done: true, message: "", responsePayload: j.response_payload };
                }
                if (j.latest_job_status === "failed") {
                    return { done: false, message: j.ai_error ?? "job failed" };
                }
            }
        } catch {
            // 忽略瞬时网络错误,继续轮询
        }
    }
    return { done: false, message: "polling timeout" };
}

/** 从 ai_jobs.response_payload(LLM 原始输出 {"proposals":[...]})取首条 proposal。 */
function parseFirstProposal(payload?: string): ReviewProposal | null {
    if (!payload) return null;
    try {
        const parsed = JSON.parse(payload) as { proposals?: ReviewProposal[] };
        const list = parsed.proposals ?? [];
        return list.length > 0 ? list[0] : null;
    } catch {
        return null;
    }
}

function sleep(ms: number) {
    return new Promise<void>((r) => setTimeout(r, ms));
}
