import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { APIClient } from "../../utils/apiClient";

/**
 * 后端有一批「成功即 204 No Content」的端点。用 fetchJson 解析空 body 会抛
 * `Unexpected end of JSON input`，表现为「操作成功但前端报错并回滚」。
 * 这些用例锁住：204 必须被当成正常成功。
 */
describe("APIClient 204 No Content", () => {
  const client = new APIClient("http://test.local");

  beforeEach(() => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(new Response(null, { status: 204 })),
    );
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("deleteTask 成功返回 204 时 resolve", async () => {
    await expect(client.deleteTask(7, 3)).resolves.toBeUndefined();
  });

  it("updateSubtask 成功返回 204 时 resolve", async () => {
    await expect(
      client.updateSubtask(7, 11, { title: "新标题" }, 2),
    ).resolves.toBeUndefined();
  });

  it("toggleSubtask 成功返回 204 时 resolve", async () => {
    await expect(client.toggleSubtask(7, 11, "done", 2)).resolves.toBeUndefined();
  });

  it("deleteSubtask 成功返回 204 时 resolve", async () => {
    await expect(client.deleteSubtask(7, 11, 2)).resolves.toBeUndefined();
  });

  it("rejectDailySweep 成功返回 204 时 resolve", async () => {
    await expect(client.rejectDailySweep({ change_id: "c1" })).resolves.toBeUndefined();
  });
});
