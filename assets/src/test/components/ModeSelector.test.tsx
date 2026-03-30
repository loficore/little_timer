import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, fireEvent, screen } from "@testing-library/preact";
import { ModeSelector } from "../../components/ModeSelector";
import { Mode } from "../../utils/share";

describe("ModeSelector 组件", () => {
  const mockOnModeChange = vi.fn();

  beforeEach(() => {
    mockOnModeChange.mockClear();
  });

  it("应该渲染所有模式按钮", () => {
    const modes = [
      { key: Mode.Countdown, label: "倒计时", icon: null },
      { key: Mode.Stopwatch, label: "秒表", icon: null },
    ];
    
    render(
      <ModeSelector
        modes={modes}
        activeMode={Mode.Countdown}
        onModeChange={mockOnModeChange}
      />,
    );

    expect(screen.getByText("倒计时")).toBeTruthy();
    expect(screen.getByText("秒表")).toBeTruthy();
  });

  it("应该高亮激活的模式", () => {
    const modes = [
      { key: Mode.Countdown, label: "倒计时", icon: null },
      { key: Mode.Stopwatch, label: "秒表", icon: null },
    ];
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
    const modes = [
      { key: Mode.Countdown, label: "倒计时", icon: null },
      { key: Mode.Stopwatch, label: "秒表", icon: null },
    ];
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
    const modes = [
      { key: Mode.Countdown, label: "倒计时", icon: null },
      { key: Mode.Stopwatch, label: "秒表", icon: null },
    ];
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
    const modes = [
      { key: Mode.Countdown, label: "倒计时", icon: null },
      { key: Mode.Stopwatch, label: "秒表", icon: null },
    ];
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
    const modes = [
      { key: Mode.Countdown, label: "倒计时", icon: null },
      { key: Mode.Stopwatch, label: "秒表", icon: null },
    ];
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
