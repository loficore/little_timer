import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/preact";
import { ErrorNotification, OfflineModeIndicator } from "../../components/ErrorNotification";

describe("ErrorNotification 组件", () => {
  it("visible 为 false 时应该返回 null", () => {
    const { container } = render(<ErrorNotification visible={false} />);
    expect(container.firstChild).toBeNull();
  });

  it("visible 为 true 时应该返回 null（当前实现是 stub）", () => {
    const { container } = render(<ErrorNotification visible={true} />);
    expect(container.firstChild).toBeNull();
  });
});

describe("OfflineModeIndicator 组件", () => {
  it("show 为 false 时应该不显示", () => {
    const { container } = render(<OfflineModeIndicator show={false} />);
    expect(container.firstChild).toBeNull();
  });

  it("show 为 true 时应该显示离线提示", () => {
    render(<OfflineModeIndicator show={true} />);
    expect(screen.getByText("连接中断")).toBeTruthy();
    expect(screen.getByText("请检查网络连接")).toBeTruthy();
  });
});