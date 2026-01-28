import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, fireEvent, screen } from "@testing-library/preact";
import { ModeSelector } from "../../components/ModeSelector";
import { Mode } from "../../utils/share";

describe("ModeSelector 组件", () => {
  const mockOnModeChange = vi.fn();

  const modes = [
    { key: Mode.Countdown, label: "倒计时", icon: "⏱" },
    { key: Mode.Stopwatch, label: "秒表", icon: "⏲" },
    { key: Mode.WorldClock, label: "世界时钟", icon: "🌐" },
  ];

  beforeEach(() => {
    mockOnModeChange.mockClear();
  });

  it("应该渲染所有模式按钮", () => {
    render(
      <ModeSelector
        modes={modes}
        activeMode={Mode.Countdown}
        onModeChange={mockOnModeChange}
      />,
    );

    expect(screen.getByText("倒计时")).toBeTruthy();
    expect(screen.getByText("秒表")).toBeTruthy();
    expect(screen.getByText("世界时钟")).toBeTruthy();
  });

  it("应该显示所有模式的图标", () => {
    const { container } = render(
      <ModeSelector
        modes={modes}
        activeMode={Mode.Countdown}
        onModeChange={mockOnModeChange}
      />,
    );

    expect(container.textContent).toContain("⏱");
    expect(container.textContent).toContain("⏲");
    expect(container.textContent).toContain("🌐");
  });

  it("应该高亮激活的模式", () => {
    const { container } = render(
      <ModeSelector
        modes={modes}
        activeMode={Mode.Countdown}
        onModeChange={mockOnModeChange}
      />,
    );

    const buttons = container.querySelectorAll("button");
    const countdownBtn = Array.from(buttons).find((btn) =>
      btn.textContent?.includes("倒计时"),
    );

    expect(countdownBtn?.className).toContain("bg-accent-dark");
    expect(countdownBtn?.className).toContain("text-white");
  });

  it("非激活模式应该有不同样式", () => {
    const { container } = render(
      <ModeSelector
        modes={modes}
        activeMode={Mode.Countdown}
        onModeChange={mockOnModeChange}
      />,
    );

    const buttons = container.querySelectorAll("button");
    const stopwatchBtn = Array.from(buttons).find((btn) =>
      btn.textContent?.includes("秒表"),
    );

    expect(stopwatchBtn?.className).toContain("border-border-dark");
    expect(stopwatchBtn?.className).not.toContain("bg-accent-dark");
  });

  it("点击模式按钮应该触发回调", () => {
    render(
      <ModeSelector
        modes={modes}
        activeMode={Mode.Countdown}
        onModeChange={mockOnModeChange}
      />,
    );

    const stopwatchBtn = screen.getByText("秒表");
    fireEvent.click(stopwatchBtn);

    expect(mockOnModeChange).toHaveBeenCalledWith(Mode.Stopwatch);
  });

  it("切换模式时应该更新高亮", () => {
    const { rerender, container } = render(
      <ModeSelector
        modes={modes}
        activeMode={Mode.Countdown}
        onModeChange={mockOnModeChange}
      />,
    );

    let countdownBtn = Array.from(container.querySelectorAll("button")).find(
      (btn) => btn.textContent?.includes("倒计时"),
    );
    expect(countdownBtn?.className).toContain("bg-accent-dark");

    rerender(
      <ModeSelector
        modes={modes}
        activeMode={Mode.Stopwatch}
        onModeChange={mockOnModeChange}
      />,
    );

    countdownBtn = Array.from(container.querySelectorAll("button")).find(
      (btn) => btn.textContent?.includes("倒计时"),
    );
    expect(countdownBtn?.className).not.toContain("bg-accent-dark");

    const stopwatchBtn = Array.from(container.querySelectorAll("button")).find(
      (btn) => btn.textContent?.includes("秒表"),
    );
    expect(stopwatchBtn?.className).toContain("bg-accent-dark");
  });

  it("应该支持自定义动画延迟", () => {
    const { container } = render(
      <ModeSelector
        modes={modes}
        activeMode={Mode.Countdown}
        onModeChange={mockOnModeChange}
        animationDelay="0.4s"
      />,
    );

    const modeDiv = container.querySelector("div");
    expect(modeDiv?.style.animationDelay).toBe("0.4s");
  });
});
