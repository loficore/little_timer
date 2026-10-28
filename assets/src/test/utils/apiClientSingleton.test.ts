import { describe, it, expect, vi, beforeEach } from "vitest";

describe("supportsDailySweep", () => {
  beforeEach(() => { vi.resetModules(); vi.unstubAllGlobals(); });

  it("web 构建(window.wails 不存在)为 true", async () => {
    const { supportsDailySweep } = await import("../../utils/apiClientSingleton");
    expect(supportsDailySweep).toBe(true);
  });

  it("window.wails 存在时为 false", async () => {
    vi.stubGlobal("window", { ...globalThis.window, wails: {} });
    const { supportsDailySweep } = await import("../../utils/apiClientSingleton");
    expect(supportsDailySweep).toBe(false);
  });
});
