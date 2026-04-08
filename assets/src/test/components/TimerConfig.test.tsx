import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/preact";
import { TimerConfig } from "../../components/TimerConfig";

describe("TimerConfig", () => {
  const defaultConfig = {
    mode: "countdown" as const,
    workDuration: 1500,
    restDuration: 300,
    loopCount: 0,
  };

  it("倒计时模式且未运行时应该渲染配置面板", () => {
    const { container } = render(
      <TimerConfig
        config={defaultConfig}
        isRunning={false}
        isCountdownMode={true}
        onChange={vi.fn()}
      />
    );

    const inputs = container.querySelectorAll("input");
    expect(inputs.length).toBe(3);
  });

  it("运行时应该隐藏配置面板", () => {
    const { container } = render(
      <TimerConfig
        config={defaultConfig}
        isRunning={true}
        isCountdownMode={true}
        onChange={vi.fn()}
      />
    );

    expect(container.firstChild).toBeNull();
  });

  it("正计时模式应该隐藏配置面板", () => {
    const { container } = render(
      <TimerConfig
        config={{ ...defaultConfig, mode: "stopwatch" }}
        isRunning={false}
        isCountdownMode={false}
        onChange={vi.fn()}
      />
    );

    expect(container.firstChild).toBeNull();
  });

  it("应该显示正确的初始值", () => {
    const { container } = render(
      <TimerConfig
        config={defaultConfig}
        isRunning={false}
        isCountdownMode={true}
        onChange={vi.fn()}
      />
    );

    const inputs = container.querySelectorAll("input");
    expect(inputs[0].value).toBe("25");
    expect(inputs[1].value).toBe("5");
  });

  it("轮次为空时应该显示无穷大标识", () => {
    const { container } = render(
      <TimerConfig
        config={{ ...defaultConfig, loopCount: 0 }}
        isRunning={false}
        isCountdownMode={true}
        onChange={vi.fn()}
      />
    );

    const inputs = container.querySelectorAll("input");
    expect(inputs[2].value).toBe("0");
  });

  it("倒计时模式且未运行时应该包含配置项", () => {
    render(
      <TimerConfig
        config={defaultConfig}
        isRunning={false}
        isCountdownMode={true}
        onChange={vi.fn()}
      />
    );

    expect(screen.getByDisplayValue("25")).toBeDefined();
    expect(screen.getByDisplayValue("5")).toBeDefined();
  });
});
