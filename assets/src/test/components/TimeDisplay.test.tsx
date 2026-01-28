import { describe, it, expect } from "vitest";
import { render } from "@testing-library/preact";
import { TimeDisplay } from "../../components/TimeDisplay";

describe("TimeDisplay 组件", () => {
  it("应该正确显示时间", () => {
    const { container } = render(
      <TimeDisplay time="25:30:45" isRunning={false} />,
    );

    expect(container.textContent).toContain("25:30:45");
  });

  it("运行时应该有强调色", () => {
    const { container } = render(
      <TimeDisplay time="25:30:45" isRunning={true} />,
    );

    expect(container.querySelector("div")?.className).toContain(
      "text-accent-dark",
    );
  });

  it("暂停时不应该有强调色", () => {
    const { container } = render(
      <TimeDisplay time="25:30:45" isRunning={false} />,
    );

    const timeDiv = container.querySelector("div");
    expect(timeDiv?.className).toContain("text-text-primary-dark");
    expect(timeDiv?.className).not.toContain("text-accent-dark");
  });

  it("动画激活时应该添加动画类", () => {
    const { container } = render(
      <TimeDisplay time="25:30:45" isRunning={false} isAnimating={true} />,
    );

    expect(container.querySelector("div")?.className).toContain(
      "time-transition--active",
    );
  });

  it("应该支持自定义动画延迟", () => {
    const { container } = render(
      <TimeDisplay time="25:30:45" isRunning={false} animationDelay="0.3s" />,
    );

    const timeDiv = container.querySelector("div");
    expect(timeDiv?.style.animationDelay).toBe("0.3s");
  });

  it("应该使用等宽字体", () => {
    const { container } = render(
      <TimeDisplay time="25:30:45" isRunning={false} />,
    );

    expect(container.querySelector("div")?.className).toContain("font-mono");
  });

  it("应该支持大字体显示", () => {
    const { container } = render(
      <TimeDisplay time="25:30:45" isRunning={false} />,
    );

    expect(container.querySelector("div")?.className).toContain("text-4xl");
    expect(container.querySelector("div")?.className).toContain("sm:text-6xl");
    expect(container.querySelector("div")?.className).toContain("md:text-8xl");
  });
});
