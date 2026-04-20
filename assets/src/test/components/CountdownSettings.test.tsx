import { describe, it, expect, vi } from "vitest";
import { render } from "@testing-library/preact";
import { CountdownSettings } from "../../components/CountdownSettings";

vi.mock("../../utils/i18n", () => ({
  t: (key: string, params?: Record<string, unknown>) => {
    const translations: Record<string, string> = {
      "settings.countdown.duration": "Duration",
      "settings.countdown.duration_hint": "Duration hint",
      "settings.countdown.loop_mode": "Loop Mode",
      "settings.countdown.loop_enable": "Enable Loop",
      "settings.countdown.loop_count": "Loop Count",
      "settings.countdown.loop_count_hint": "Loop count hint",
      "settings.countdown.loop_interval": "Loop Interval",
      "settings.countdown.loop_interval_hint": "Interval hint",
    };
    const text = translations[key] || key;
    if (params) {
      return text.replace(/{(\w+)}/g, (_, k) => String(params[k]));
    }
    return text;
  },
}));

vi.mock("../../components/TimeInput", () => ({
  TimeInput: ({ value, onChange }: { value: number; onChange: (v: number) => void }) => (
    <input type="number" data-testid="time-input" value={value} onChange={() => onChange(100)} />
  ),
}));

vi.mock("../../components/NumberInput", () => ({
  NumberInput: ({ value, onChange }: { value: number; onChange: (v: number) => void }) => (
    <input type="number" data-testid="number-input" value={value} onChange={() => onChange(5)} />
  ),
}));

vi.mock("../../components/CheckboxInput", () => ({
  CheckboxInput: ({ value, onChange }: { value: boolean; onChange: (v: boolean) => void }) => (
    <input type="checkbox" data-testid="checkbox-input" checked={value} onChange={() => onChange(!value)} />
  ),
}));

describe("CountdownSettings", () => {
  const defaultConfig = {
    duration_seconds: 1500,
    loop: false,
    loop_count: 0,
    loop_interval_seconds: 300,
  };

  const defaultProps = {
    config: defaultConfig,
    onChange: vi.fn(),
  };

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("应该渲染时间输入组件", () => {
    const { getByTestId } = render(<CountdownSettings {...defaultProps} />);
    expect(getByTestId("time-input")).toBeTruthy();
  });

  it("showLoopControls 为 false 时不应该显示循环控制", () => {
    const { queryByText } = render(
      <CountdownSettings {...defaultProps} showLoopControls={false} />
    );

    expect(queryByText("Loop Mode")).toBeNull();
    expect(queryByText("Enable Loop")).toBeNull();
  });

  describe("循环控制", () => {
    it("loop 为 false 时不应该显示循环次数和间隔", () => {
      const { queryByText } = render(
        <CountdownSettings {...defaultProps} config={{ ...defaultConfig, loop: false }} />
      );

      expect(queryByText("Loop Count")).toBeNull();
      expect(queryByText("Loop Interval")).toBeNull();
    });

    it("loop 为 true 时应该显示循环次数和间隔", () => {
      const { getByText, getByTestId } = render(
        <CountdownSettings
          {...defaultProps}
          config={{ ...defaultConfig, loop: true }}
        />
      );

      expect(getByText("Loop Count")).toBeTruthy();
      expect(getByText("Loop Interval")).toBeTruthy();
      expect(getByTestId("checkbox-input")).toBeTruthy();
    });
  });

  describe("onChange 回调", () => {
    it("时间变化时应该调用 onChange", () => {
      const onChange = vi.fn();
      const { getByTestId } = render(
        <CountdownSettings {...defaultProps} onChange={onChange} />
      );

      getByTestId("time-input").dispatchEvent(new Event("change", { bubbles: true }));

      expect(onChange).toHaveBeenCalled();
    });

    it("loop 变化时应该调用 onChange", () => {
      const onChange = vi.fn();
      const { getByTestId } = render(
        <CountdownSettings
          {...defaultProps}
          config={{ ...defaultConfig, loop: true }}
          onChange={onChange}
        />
      );

      getByTestId("checkbox-input").dispatchEvent(new Event("change", { bubbles: true }));

      expect(onChange).toHaveBeenCalled();
    });
  });

  describe("动画", () => {
    it("isAnimated 为 true 时应该有动画类", () => {
      const { container } = render(
        <CountdownSettings {...defaultProps} isAnimated={true} />
      );

      const element = container.querySelector("div");
      expect(element?.className).toContain("animate-slideUp");
    });

    it("isAnimated 为 false 时不应该有动画类", () => {
      const { container } = render(
        <CountdownSettings {...defaultProps} isAnimated={false} />
      );

      const element = container.querySelector("div");
      expect(element?.className).not.toContain("animate-slideUp");
    });
  });
});
