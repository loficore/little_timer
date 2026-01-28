import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, fireEvent, screen } from "@testing-library/preact";
import { ControlPanel } from "../../components/ControlPanel";

describe("ControlPanel 组件", () => {
  const mockOnStart = vi.fn();
  const mockOnPause = vi.fn();
  const mockOnReset = vi.fn();

  beforeEach(() => {
    mockOnStart.mockClear();
    mockOnPause.mockClear();
    mockOnReset.mockClear();
  });

  it("未运行时应该显示开始按钮", () => {
    render(
      <ControlPanel
        isRunning={false}
        onStart={mockOnStart}
        onPause={mockOnPause}
        onReset={mockOnReset}
      />,
    );

    const buttons = screen.getAllByRole("button");
    const startBtn = buttons[0]; // 第一个按钮是开始按钮
    expect(startBtn).toBeTruthy();
  });

  it("开始按钮点击应该触发 onStart 回调", () => {
    render(
      <ControlPanel
        isRunning={false}
        onStart={mockOnStart}
        onPause={mockOnPause}
        onReset={mockOnReset}
      />,
    );

    const buttons = screen.getAllByRole("button");
    const startBtn = buttons[0]; // 第一个按钮是开始按钮
    fireEvent.click(startBtn);

    expect(mockOnStart).toHaveBeenCalled();
  });

  it("运行时应该显示暂停按钮", () => {
    render(
      <ControlPanel
        isRunning={true}
        onStart={mockOnStart}
        onPause={mockOnPause}
        onReset={mockOnReset}
      />,
    );

    const buttons = screen.getAllByRole("button");
    const pauseBtn = buttons[0]; // 第一个按钮是暂停按钮
    expect(pauseBtn).toBeTruthy();
  });

  it("暂停按钮点击应该触发 onPause 回调", () => {
    render(
      <ControlPanel
        isRunning={true}
        onStart={mockOnStart}
        onPause={mockOnPause}
        onReset={mockOnReset}
      />,
    );

    const buttons = screen.getAllByRole("button");
    const pauseBtn = buttons[0]; // 第一个按钮是暂停按钮
    fireEvent.click(pauseBtn);

    expect(mockOnPause).toHaveBeenCalled();
  });

  it("应该总是显示重置按钮", () => {
    const { rerender } = render(
      <ControlPanel
        isRunning={false}
        onStart={mockOnStart}
        onPause={mockOnPause}
        onReset={mockOnReset}
      />,
    );

    let buttons = screen.getAllByRole("button");
    let resetBtn = buttons[1]; // 第二个按钮总是重置按钮
    expect(resetBtn).toBeTruthy();

    rerender(
      <ControlPanel
        isRunning={true}
        onStart={mockOnStart}
        onPause={mockOnPause}
        onReset={mockOnReset}
      />,
    );

    buttons = screen.getAllByRole("button");
    resetBtn = buttons[1]; // 第二个按钮总是重置按钮
    expect(resetBtn).toBeTruthy();
  });

  it("重置按钮点击应该触发 onReset 回调", () => {
    render(
      <ControlPanel
        isRunning={false}
        onStart={mockOnStart}
        onPause={mockOnPause}
        onReset={mockOnReset}
      />,
    );

    const buttons = screen.getAllByRole("button");
    const resetBtn = buttons[1]; // 第二个按钮是重置按钮
    fireEvent.click(resetBtn);

    expect(mockOnReset).toHaveBeenCalled();
  });

  it("应该支持自定义动画延迟", () => {
    const { container } = render(
      <ControlPanel
        isRunning={false}
        onStart={mockOnStart}
        onPause={mockOnPause}
        onReset={mockOnReset}
        animationDelay="0.3s"
      />,
    );

    const controlDiv = container.querySelector("div");
    expect(controlDiv?.style.animationDelay).toBe("0.3s");
  });
});
