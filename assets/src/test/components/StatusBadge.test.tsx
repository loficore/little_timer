import { describe, it, expect } from "vitest";
import { render } from "@testing-library/preact";
import { StatusBadge } from "../../components/StatusBadge";

describe("StatusBadge 组件", () => {
  it("应该正确渲染运行状态", () => {
    const { container } = render(
      <StatusBadge status="running" label="运行中" />,
    );

    expect(container.textContent).toContain("运行中");
    expect(container.querySelector("span")?.className).toContain("badge-primary");
  });

  it("应该正确渲染暂停状态", () => {
    const { container } = render(
      <StatusBadge status="paused" label="已暂停" />,
    );

    expect(container.textContent).toContain("已暂停");
    expect(container.querySelector("span")?.className).toContain("badge-neutral");
  });

  it("应该正确渲染完成状态", () => {
    const { container } = render(
      <StatusBadge status="finished" label="已完成" />,
    );

    expect(container.textContent).toContain("已完成");
    expect(container.querySelector("span")?.className).toContain("badge-success");
  });

  it("应该支持动画延迟", () => {
    const { container } = render(
      <StatusBadge status="running" label="运行中" animationDelay="0.2s" />,
    );

    const badge = container.querySelector("span");
    expect(badge?.style.animationDelay).toBe("0.2s");
  });

  it("运行状态应该有脉冲动画", () => {
    const { container } = render(
      <StatusBadge status="running" label="运行中" />,
    );

    expect(container.querySelector("span")?.className).toContain("animate-slideUp");
  });
});
