import { describe, it, expect } from "vitest";
import { WailsAPIClient } from "../../utils/wailsApiClient";

const METHODS: Array<[string, () => Promise<unknown>]> = (() => {
  const c = new WailsAPIClient();
  return [
    ["startDailySweep", () => c.startDailySweep()],
    ["getDailySweepToday", () => c.getDailySweepToday()],
    ["adoptDailySweep", () => c.adoptDailySweep({})],
    ["rejectDailySweep", () => c.rejectDailySweep({})],
    ["getDailySweepSettings", () => c.getDailySweepSettings()],
    ["updateDailySweepSettings", () => c.updateDailySweepSettings({})],
  ];
})();

describe("WailsAPIClient daily_sweep 占位", () => {
  it.each(METHODS)("%s 以用户口径文案拒绝且不含内部术语", async (_name, call) => {
    await expect(call()).rejects.toThrow("每日梳理在当前平台不可用");
    const err = await call().catch((e) => e as Error);
    expect(err.message).not.toMatch(/not implemented|Wails|Android/i);
  });
});
