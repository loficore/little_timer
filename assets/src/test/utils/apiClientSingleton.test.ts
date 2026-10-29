import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { getAPIClient, resetAPIClient } from "../../utils/apiClientSingleton";

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
