import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { getAPIClient, resetAPIClient, capabilities } from "../../utils/apiClientSingleton";

vi.mock("../../utils/apiClient", () => ({
  APIClient: vi.fn().mockImplementation(() => ({})),
}));

describe("apiClientSingleton", () => {
  beforeEach(() => {
    resetAPIClient();
    vi.clearAllMocks();
  });

  afterEach(() => {
    resetAPIClient();
  });

  it("getAPIClient returns an instance", () => {
    const client = getAPIClient();
    expect(client).toBeTruthy();
  });

  it("getAPIClient returns the same instance (singleton)", () => {
    const client1 = getAPIClient();
    const client2 = getAPIClient();
    expect(client1).toBe(client2);
  });

  it("resetAPIClient allows creating a new instance", () => {
    const client1 = getAPIClient();
    resetAPIClient();
    const client2 = getAPIClient();
    expect(client1).not.toBe(client2);
  });

  it("multiple calls return same instance until reset", () => {
    const client1 = getAPIClient();
    const client2 = getAPIClient();
    const client3 = getAPIClient();
    expect(client1).toBe(client2);
    expect(client2).toBe(client3);
  });
});

describe("capabilities", () => {
  beforeEach(() => { vi.resetModules(); vi.unstubAllGlobals(); });

  it("capabilities 在模块加载时即可读，不是异步函数", () => {
    expect(typeof capabilities.tasks).toBe("boolean");
    expect(typeof capabilities.timer).toBe("boolean");
  });

  it("web 构建(window.wails 不存在)tasks / schedule / dailySweep 为 true", () => {
    expect(capabilities.tasks).toBe(true);
    expect(capabilities.schedule).toBe(true);
    expect(capabilities.dailySweep).toBe(true);
  });

  it("window.wails 存在时 tasks / schedule / dailySweep 为 false", async () => {
    vi.stubGlobal("window", { ...globalThis.window, wails: {} });
    const mod = await import("../../utils/apiClientSingleton");
    expect(mod.capabilities.tasks).toBe(false);
    expect(mod.capabilities.schedule).toBe(false);
    expect(mod.capabilities.dailySweep).toBe(false);
  });

  it("Android 首调返回可排队的代理，方法不是 undefined", async () => {
    vi.stubGlobal("window", { ...globalThis.window, wails: {} });
    const mod = await import("../../utils/apiClientSingleton");
    const client = mod.getAPIClient();
    // 绑定尚未加载完成时，方法存在且是函数（调用会排队，不会立刻抛 TypeError）。
    expect(typeof client.listTasks).toBe("function");
    expect(typeof client.getState).toBe("function");
  });
});

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
