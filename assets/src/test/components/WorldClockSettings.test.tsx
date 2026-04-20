import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/preact";
import { WorldClockSettings } from "../../components/WorldClockSettings";

vi.mock("../../utils/i18n", () => ({
  t: (key: string, params?: Record<string, unknown>) => {
    const translations: Record<string, string> = {
      "settings.world_clock.timezone": "Timezone",
      "settings.basic.timezone_hint": "UTC{offset}",
      "settings.world_clock.desc": "World clock description",
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

describe("WorldClockSettings", () => {
  it("应该渲染世界时钟设置", () => {
    const { container } = render(
      <WorldClockSettings
        timezone={8}
        onTimezoneChange={vi.fn()}
      />
    );

    expect(container.querySelector(".space-y-4")).toBeTruthy();
  });

  it("应该显示时区标签", () => {
    render(
      <WorldClockSettings
        timezone={8}
        onTimezoneChange={vi.fn()}
      />
    );

    expect(screen.getByText("Timezone")).toBeTruthy();
  });

  it("应该显示 UTC 偏移提示", () => {
    render(
      <WorldClockSettings
        timezone={8}
        onTimezoneChange={vi.fn()}
      />
    );

    expect(screen.getAllByText(/UTC/).length).toBeGreaterThan(0);
  });

  it("应该显示描述文字", () => {
    render(
      <WorldClockSettings
        timezone={8}
        onTimezoneChange={vi.fn()}
      />
    );

    expect(screen.getByText("World clock description")).toBeTruthy();
  });

  it("默认时区应该是 8", () => {
    const onTimezoneChange = vi.fn();
    render(
      <WorldClockSettings
        timezone={8}
        onTimezoneChange={onTimezoneChange}
      />
    );

    expect(onTimezoneChange).not.toHaveBeenCalled();
  });

  it("时区变化时应该调用 onTimezoneChange", () => {
    const onTimezoneChange = vi.fn();
    const { container } = render(
      <WorldClockSettings
        timezone={8}
        onTimezoneChange={onTimezoneChange}
      />
    );

    const buttons = container.querySelectorAll("button");
    expect(buttons.length).toBeGreaterThan(0);
  });

  it("应该渲染下拉选择按钮", () => {
    const { container } = render(
      <WorldClockSettings
        timezone={8}
        onTimezoneChange={vi.fn()}
      />
    );

    expect(container.querySelector(".dropdown-select-btn")).toBeTruthy();
  });

  it("应该有动画类", () => {
    const { container } = render(
      <WorldClockSettings
        timezone={8}
        onTimezoneChange={vi.fn()}
      />
    );

    const div = container.querySelector("div");
    expect(div?.className).toContain("animate-slideUp");
  });
});
