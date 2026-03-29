import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, fireEvent, screen } from "@testing-library/preact";
import { ModeSelector } from "../../components/ModeSelector";
import { Mode } from "../../utils/share";
import { ClockIconComponent } from "../../utils/icons";

describe("ModeSelector 组件", () => {
  const mockOnModeChange = vi.fn();

  const modes = [
    { key: Mode.Countdown, label: "倒计时", icon: <ClockIconComponent /> },
    { key: Mode.Stopwatch, label: "秒表", icon: <ClockIconComponent /> },
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

    expect(countdownBtn?.className).toContain("btn-primary");
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

    expect(stopwatchBtn?.className).toContain("btn-outline");
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
    expect(countdownBtn?.className).toContain("btn-primary");

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
    expect(countdownBtn?.className).toContain("btn-outline");

    const stopwatchBtn = Array.from(container.querySelectorAll("button")).find(
      (btn) => btn.textContent?.includes("秒表"),
    );
    expect(stopwatchBtn?.className).toContain("btn-primary");
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
