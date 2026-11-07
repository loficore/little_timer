import { describe, it, expect } from "vitest";
import { capabilities, UnsupportedOnPlatformError } from "../../utils/platformApi";
import { APIClient } from "../../utils/apiClient";
import type { PlatformApi } from "../../utils/platformApi";

describe("platformApi", () => {
  it("桌面环境启用全部能力", () => {
    expect(capabilities.timer).toBe(true);
    expect(capabilities.habits).toBe(true);
    expect(capabilities.tasks).toBe(true);
    expect(capabilities.schedule).toBe(true);
  });

  it("UnsupportedOnPlatformError 暴露稳定 name", () => {
    const err = new UnsupportedOnPlatformError("AIGTD");
    expect(err.name).toBe("UnsupportedOnPlatformError");
    expect(err.message).toContain("AIGTD");
  });

  it("APIClient 满足 PlatformApi", () => {
    const client: PlatformApi = new APIClient("http://localhost");
    expect(client).toBeInstanceOf(APIClient);
  });
});
