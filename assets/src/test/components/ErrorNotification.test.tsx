import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, waitFor, cleanup } from "@testing-library/preact";
import { ErrorNotification, OfflineModeIndicator } from "../../components/ErrorNotification";

describe("ErrorNotification 组件", () => {
  afterEach(() => {
    cleanup();
  });

  it("visible 为 false 时应该返回 null", () => {
    const { container } = render(<ErrorNotification visible={false} />);
    expect(container.firstChild).toBeNull();
  });

  it("visible 为 true 但无 message 时应该返回 null", () => {
    const { container } = render(<ErrorNotification visible={true} />);
    expect(container.firstChild).toBeNull();
  });

  it("visible 为 true 且有 message 时应该显示错误通知", async () => {
    vi.useFakeTimers();
    render(<ErrorNotification visible={true} message="保存失败" />);
    expect(screen.getByText("保存失败")).toBeTruthy();
    expect(screen.getByText("操作失败")).toBeTruthy();
    vi.useRealTimers();
  });

  it("5 秒后自动消失", async () => {
    vi.useFakeTimers();
    const onDismiss = vi.fn();
    render(<ErrorNotification visible={true} message="测试错误" onDismiss={onDismiss} />);
    expect(screen.getByText("测试错误")).toBeTruthy();
    vi.advanceTimersByTime(5000);
    await waitFor(() => {
      expect(onDismiss).toHaveBeenCalled();
    });
    vi.useRealTimers();
  });

  it("点击关闭按钮应该调用 onDismiss", async () => {
    vi.useFakeTimers();
    const onDismiss = vi.fn();
    render(<ErrorNotification visible={true} message="测试错误" onDismiss={onDismiss} />);
    const closeBtn = screen.getByLabelText("关闭");
    closeBtn.click();
    expect(onDismiss).toHaveBeenCalled();
    vi.useRealTimers();
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