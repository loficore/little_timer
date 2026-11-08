import { describe, it, expect } from "vitest";
import { WailsAPIClient } from "../../utils/wailsApiClient";
import { UnsupportedOnPlatformError } from "../../utils/platformApi";

const DAILY_SWEEP_METHODS: Array<[string, () => Promise<unknown>]> = (() => {
  const c = new WailsAPIClient();
  return [
    ["startDailySweep", () => c.startDailySweep()],
    ["getDailySweepToday", () => c.getDailySweepToday()],
    ["adoptDailySweep", () => c.adoptDailySweep({ change_id: "c1", ai_job_id: 1, version: 1, proposal: { change_id: "c1", task_id: 1, action: "no_op", reason: "" } })],
    ["rejectDailySweep", () => c.rejectDailySweep({ change_id: "c1" })],
    ["getDailySweepSettings", () => c.getDailySweepSettings()],
    ["updateDailySweepSettings", () => c.updateDailySweepSettings({})],
  ];
})();

// Android 上 Wails 绑定未暴露的域：入口已由 capabilities 隐藏，
// 这些方法存在只为满足 PlatformApi，并给出可读的兜底错误。
const UNSUPPORTED_METHODS: Array<[string, () => Promise<unknown>]> = (() => {
  const c = new WailsAPIClient();
  return [
    ["captureAIGtd", () => c.captureAIGtd("x")],
    ["createTask", () => c.createTask("t")],
    ["listTasks", () => c.listTasks()],
    ["getTask", () => c.getTask(1)],
    ["updateTask", () => c.updateTask(1, {})],
    ["deleteTask", () => c.deleteTask(1, 1)],
    ["createSubtask", () => c.createSubtask(1, { title: "t", estimated_minutes: 1, order_index: 0 }, 1)],
    ["updateSubtask", () => c.updateSubtask(1, 2, {}, 1)],
    ["deleteSubtask", () => c.deleteSubtask(1, 2, 1)],
    ["toggleSubtask", () => c.toggleSubtask(1, 2, "done", 1)],
    ["patchTaskScores", () => c.patchTaskScores(1, { version: 1 })],
    ["reparseAIGtd", () => c.reparseAIGtd(1, 1)],
    ["getAIGtdJob", () => c.getAIGtdJob(1)],
    ["getLLMSettings", () => c.getLLMSettings()],
    ["updateLLMSettings", () => c.updateLLMSettings({})],
    ["testLLMSettings", () => c.testLLMSettings({})],
    ["runSchedulePreview", () => c.runSchedulePreview("2026-11-07")],
    ["runScheduleApply", () => c.runScheduleApply("2026-11-07")],
    ["applySchedulePlacements", () => c.applySchedulePlacements([])],
    ["resetCredentials", () => c.resetCredentials()],
    ["lockCredentials", () => c.lockCredentials()],
  ];
})();

const assertUnsupported = async (call: () => Promise<unknown>) => {
  const err = await call().catch((e) => e as Error);
  expect(err).toBeInstanceOf(UnsupportedOnPlatformError);
  expect(err.name).toBe("UnsupportedOnPlatformError");
  // 内部术语不得泄露给用户。
  expect(err.message).not.toMatch(/not implemented|Wails|Android/i);
};

describe("WailsAPIClient daily_sweep 占位", () => {
  // daily_sweep 的补齐归 issue #41，这里保持既有文案不变。
  it.each(DAILY_SWEEP_METHODS)("%s 以用户口径文案拒绝且不含内部术语", async (_name, call) => {
    await expect(call()).rejects.toThrow("每日梳理在当前平台不可用");
    const err = await call().catch((e) => e as Error);
    expect(err.message).not.toMatch(/not implemented|Wails|Android/i);
  });
});

describe("WailsAPIClient 平台不支持的方法", () => {
  it.each(UNSUPPORTED_METHODS)("%s 抛 UnsupportedOnPlatformError 且不含内部术语", async (_name, call) => {
    await assertUnsupported(call);
  });
});

describe("WailsAPIClient 与 PlatformApi 一致的认证接口", () => {
  it("setAuthToken / getAuthToken 与 APIClient 语义一致", () => {
    const c = new WailsAPIClient();
    expect(c.getAuthToken()).toBeNull();
    c.setAuthToken("token-1");
    expect(c.getAuthToken()).toBe("token-1");
    c.setAuthToken(null);
    expect(c.getAuthToken()).toBeNull();
  });
});
