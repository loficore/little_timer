import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent } from "@testing-library/preact";
import { TimerControls } from "../../components/TimerControls";

describe("TimerControls 组件", () => {
  const mockOnStart = vi.fn();
  const mockOnPause = vi.fn();
  const mockOnResume = vi.fn();
  const mockOnReset = vi.fn();
  const mockOnSkip = vi.fn();
  const mockOnFinish = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("未运行时应该显示开始按钮", () => {
    render(
      <TimerControls
        isRunning={false}
        isPaused={false}
        isFinished={false}
        isCountdownMode={false}
        onStart={mockOnStart}
        onPause={mockOnPause}
        onResume={mockOnResume}
        onReset={mockOnReset}
        onSkip={mockOnSkip}
        onFinish={mockOnFinish}
      />
    );

    expect(screen.getByText("开始")).toBeTruthy();
  });

  it("运行时应该显示暂停按钮", () => {
    render(
      <TimerControls
        isRunning={true}
        isPaused={false}
        isFinished={false}
        isCountdownMode={false}
        onStart={mockOnStart}
        onPause={mockOnPause}
        onResume={mockOnResume}
        onReset={mockOnReset}
        onSkip={mockOnSkip}
        onFinish={mockOnFinish}
      />
    );

    expect(screen.getByText("暂停")).toBeTruthy();
  });

  it("暂停时应该显示继续按钮", () => {
    render(
      <TimerControls
        isRunning={true}
        isPaused={true}
        isFinished={false}
        isCountdownMode={false}
        onStart={mockOnStart}
        onPause={mockOnPause}
        onResume={mockOnResume}
        onReset={mockOnReset}
        onSkip={mockOnSkip}
        onFinish={mockOnFinish}
      />
    );

    expect(screen.getByText("继续")).toBeTruthy();
  });

  it("已完成时应该显示再计一次", () => {
    render(
      <TimerControls
        isRunning={false}
        isPaused={false}
        isFinished={true}
        isCountdownMode={false}
        onStart={mockOnStart}
        onPause={mockOnPause}
        onResume={mockOnResume}
        onReset={mockOnReset}
        onSkip={mockOnSkip}
        onFinish={mockOnFinish}
      />
    );

    expect(screen.getByText("再计一次")).toBeTruthy();
  });

  it("点击开始应该调用 onStart", () => {
    render(
      <TimerControls
        isRunning={false}
        isPaused={false}
        isFinished={false}
        isCountdownMode={false}
        onStart={mockOnStart}
        onPause={mockOnPause}
        onResume={mockOnResume}
        onReset={mockOnReset}
        onSkip={mockOnSkip}
        onFinish={mockOnFinish}
      />
    );

    fireEvent.click(screen.getByText("开始"));
    expect(mockOnStart).toHaveBeenCalled();
  });

  it("点击暂停应该调用 onPause", () => {
    render(
      <TimerControls
        isRunning={true}
        isPaused={false}
        isFinished={false}
        isCountdownMode={false}
        onStart={mockOnStart}
        onPause={mockOnPause}
        onResume={mockOnResume}
        onReset={mockOnReset}
        onSkip={mockOnSkip}
        onFinish={mockOnFinish}
      />
    );

    fireEvent.click(screen.getByText("暂停"));
    expect(mockOnPause).toHaveBeenCalled();
  });

  it("点击继续应该调用 onResume", () => {
    render(
      <TimerControls
        isRunning={true}
        isPaused={true}
        isFinished={false}
        isCountdownMode={false}
        onStart={mockOnStart}
        onPause={mockOnPause}
        onResume={mockOnResume}
        onReset={mockOnReset}
        onSkip={mockOnSkip}
        onFinish={mockOnFinish}
      />
    );

    fireEvent.click(screen.getByText("继续"));
    expect(mockOnResume).toHaveBeenCalled();
  });

  it("倒计时模式运行时应该显示跳过按钮", () => {
    const { container } = render(
      <TimerControls
        isRunning={true}
        isPaused={false}
        isFinished={false}
        isCountdownMode={true}
        onStart={mockOnStart}
        onPause={mockOnPause}
        onResume={mockOnResume}
        onReset={mockOnReset}
        onSkip={mockOnSkip}
        onFinish={mockOnFinish}
      />
    );

    expect(screen.getByText("跳过")).toBeTruthy();
  });

  it("运行时应该显示结束按钮", () => {
    render(
      <TimerControls
        isRunning={true}
        isPaused={false}
        isFinished={false}
        isCountdownMode={false}
        onStart={mockOnStart}
        onPause={mockOnPause}
        onResume={mockOnResume}
        onReset={mockOnReset}
        onSkip={mockOnSkip}
        onFinish={mockOnFinish}
      />
    );

    expect(screen.getByText("结束")).toBeTruthy();
  });

  it("点击重置应该调用 onReset", () => {
    render(
      <TimerControls
        isRunning={true}
        isPaused={false}
        isFinished={false}
        isCountdownMode={false}
        onStart={mockOnStart}
        onPause={mockOnPause}
        onResume={mockOnResume}
        onReset={mockOnReset}
        onSkip={mockOnSkip}
        onFinish={mockOnFinish}
      />
    );

    fireEvent.click(screen.getByText("重置"));
    expect(mockOnReset).toHaveBeenCalled();
  });
});
