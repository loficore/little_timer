import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/preact";
import { StopwatchSettings } from "../../components/StopwatchSettings";

vi.mock("../../utils/i18n", () => ({
  t: (key: string, params?: Record<string, unknown>) => {
    const translations: Record<string, string> = {
      "settings.stopwatch.max_hours": "Max Hours",
      "settings.stopwatch.max_hours_hint": "Max: {hours} hours",
    };
    let result = translations[key] || key;
    if (params) {
      Object.entries(params).forEach(([k, v]) => {
        result = result.replace(`{${k}}`, String(v));
      });
    }
    return result;
  },
}));

describe("StopwatchSettings", () => {
  it("应该渲染秒表设置", () => {
    const { container } = render(
      <StopwatchSettings
        config={{ max_seconds: 86400 }}
        onChange={vi.fn()}
      />
    );

    expect(container.querySelector(".space-y-4")).toBeTruthy();
  });

  it("应该显示最大小时数标签", () => {
    render(
      <StopwatchSettings
        config={{ max_seconds: 86400 }}
        onChange={vi.fn()}
      />
    );

    expect(screen.getByText("Max Hours")).toBeTruthy();
  });

  it("应该显示小时数提示", () => {
    render(
      <StopwatchSettings
        config={{ max_seconds: 86400 }}
        onChange={vi.fn()}
      />
    );

    expect(screen.getByText(/Max:/)).toBeTruthy();
  });

  it("应该传递配置给 NumberInput", () => {
    const { container } = render(
      <StopwatchSettings
        config={{ max_seconds: 3600 }}
        onChange={vi.fn()}
      />
    );

    const input = container.querySelector("input");
    expect(input).toBeTruthy();
  });

  it("变化时应该调用 onChange", () => {
    const onChange = vi.fn();
    const { container } = render(
      <StopwatchSettings
        config={{ max_seconds: 3600 }}
        onChange={onChange}
      />
    );

    const increaseButton = container.querySelectorAll("button")[0];
    fireEvent.click(increaseButton);

    expect(onChange).toHaveBeenCalled();
  });

  it("默认应该有动画", () => {
    const { container } = render(
      <StopwatchSettings
        config={{ max_seconds: 86400 }}
        onChange={vi.fn()}
      />
    );

    const div = container.querySelector("div");
    expect(div?.className).toContain("animate-slideUp");
  });

  it("isAnimated 为 false 时不应该有动画", () => {
    const { container } = render(
      <StopwatchSettings
        config={{ max_seconds: 86400 }}
        onChange={vi.fn()}
        isAnimated={false}
      />
    );

    const div = container.querySelector("div");
    expect(div?.className).not.toContain("animate-slideUp");
  });
});
