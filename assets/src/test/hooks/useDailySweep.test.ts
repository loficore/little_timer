import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, act } from "@testing-library/preact";
import { useDailySweep } from "../../hooks/useDailySweep";
import type { DailySweepProposal } from "../../types/aigtd";

// logger 会额外发一次 /api/log fetch,干扰 fetch 调用计数 → mock 掉。
vi.mock("../../utils/logger", () => ({
  logError: vi.fn(),
}));

// global.fetch 由各用例注入,断言 fetch 调用次数与结果。
const fetchMock = vi.fn();

function makeProposal(): DailySweepProposal {
  return {
    ai_job_id: 7,
    version: 0,
    proposal: {
      change_id: "chg-1",
      task_id: 1,
      action: "reschedule",
      suggested_scheduled_start: 1735689600,
      suggested_scheduled_end: 1735691400,
      reason: "deadline",
    },
  };
}

describe("useDailySweep", () => {
  beforeEach(() => {
    fetchMock.mockReset();
    vi.stubGlobal("fetch", fetchMock);
  });

  it("refresh() fetches /api/daily-sweep/today and stores proposals", async () => {
    fetchMock.mockResolvedValueOnce({
      ok: true,
      status: 200,
      json: async () => ({ proposals: [makeProposal()], source: "manual", swept_at: "2026-10-08T08:00:00Z" }),
      text: async () => "",
    });
    const { result } = renderHook(() => useDailySweep());
    await act(async () => {
      await result.current.refresh();
    });
    expect(result.current.proposals).toHaveLength(1);
    expect(result.current.source).toBe("manual");
    expect(result.current.error).toBeNull();
    expect(fetchMock).toHaveBeenCalledTimes(1);
  });

  it("adopt() 收到 409 时 refetch + 设 conflict error", async () => {
    fetchMock
      .mockResolvedValueOnce({
        ok: false,
        status: 409,
        json: async () => ({ error: "version conflict" }),
        text: async () => "version conflict",
      })
      .mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({ proposals: [] }),
        text: async () => "",
      });

    const { result } = renderHook(() => useDailySweep());
    await act(async () => {
      await result.current.adopt(makeProposal());
    });

    expect(result.current.error).toBeTruthy();
    expect(fetchMock).toHaveBeenCalledTimes(2);
    expect(result.current.proposals).toHaveLength(0);
  });

  it("adopt() 成功时从列表移除该 proposal", async () => {
    const p = makeProposal();
    // 先 refresh 装载一条
    fetchMock.mockResolvedValueOnce({
      ok: true,
      status: 200,
      json: async () => ({ proposals: [p] }),
      text: async () => "",
    });
    // 再 adopt 成功
    fetchMock.mockResolvedValueOnce({
      ok: true,
      status: 200,
      json: async () => ({ task_id: 1, version_after: 1 }),
      text: async () => "",
    });

    const { result } = renderHook(() => useDailySweep());
    await act(async () => {
      await result.current.refresh();
    });
    expect(result.current.proposals).toHaveLength(1);
    await act(async () => {
      await result.current.adopt(p);
    });
    expect(result.current.proposals).toHaveLength(0);
    expect(result.current.error).toBeNull();
  });

  it("reject() 成功后从列表移除该 proposal", async () => {
    const p = makeProposal();
    fetchMock.mockResolvedValueOnce({
      ok: true,
      status: 200,
      json: async () => ({ proposals: [p] }),
      text: async () => "",
    });
    fetchMock.mockResolvedValueOnce({
      ok: true,
      status: 204,
      json: async () => undefined,
      text: async () => "",
    });

    const { result } = renderHook(() => useDailySweep());
    await act(async () => {
      await result.current.refresh();
    });
    await act(async () => {
      await result.current.reject(p);
    });
    expect(result.current.proposals).toHaveLength(0);
  });
});
