import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/preact";

const mocks = vi.hoisted(() => ({
  getVersion: vi.fn(),
}));

vi.mock("../../utils/apiClientSingleton", () => ({
  getAPIClient: () => ({ getVersion: mocks.getVersion }),
}));

vi.mock("../../utils/i18n", () => ({ t: (key: string) => key }));

import { Sidebar } from "../../components/Sidebar";

describe("Sidebar 版本号", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("显示 /api/version 返回的版本，而不是写死的 1.0.0", async () => {
    mocks.getVersion.mockResolvedValue({
      version: "2.3.1",
      git_commit: "abc1234",
      build_time: "2026-11-02T00:00:00Z",
    });

    const { container } = render(<Sidebar currentPage="timer" onNavigate={() => {}} />);

    const el = await screen.findByTestId("app-version");
    expect(el.textContent).toBe("v2.3.1");
    expect(container.textContent).not.toContain("1.0.0");
  });

  it("拿到版本前不渲染任何版本文字（避免闪一下旧值）", () => {
    mocks.getVersion.mockReturnValue(new Promise(() => {}));
    render(<Sidebar currentPage="timer" onNavigate={() => {}} />);
    expect(screen.queryByTestId("app-version")).toBeNull();
  });

  it("接口失败时回退 vdev，不谎报版本", async () => {
    mocks.getVersion.mockRejectedValue(new Error("boom"));
    render(<Sidebar currentPage="timer" onNavigate={() => {}} />);
    const el = await screen.findByTestId("app-version");
    expect(el.textContent).toBe("vdev");
  });
});
