import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/preact";
import { SevenSegmentDisplay } from "../../components/SevenSegmentDisplay";

describe("SevenSegmentDisplay", () => {
  it("应该渲染显示值", () => {
    render(<SevenSegmentDisplay value="12:30" />);

    const display = screen.getByRole("img");
    expect(display).toBeDefined();
  });

  it("应该设置正确的 aria-label", () => {
    render(<SevenSegmentDisplay value="12:30" />);

    const display = screen.getByRole("img");
    expect(display.getAttribute("aria-label")).toBe("12:30");
  });

  it("应该渲染冒号字符", () => {
    render(<SevenSegmentDisplay value="12:30" />);

    const display = screen.getByRole("img");
    expect(display.innerHTML).toContain("seven-segment-colon");
  });

  it("应该应用自定义 className", () => {
    render(<SevenSegmentDisplay value="00:00" className="custom-class" />);

    const display = screen.getByRole("img");
    expect(display.className).toContain("custom-class");
  });

  it("应该包含默认 className", () => {
    render(<SevenSegmentDisplay value="00:00" />);

    const display = screen.getByRole("img");
    expect(display.className).toContain("seven-segment-display");
  });

  it("应该处理空字符串", () => {
    const { container } = render(<SevenSegmentDisplay value="" />);

    expect(container.firstChild).toBeDefined();
  });

  it("应该为每个字符创建 SevenSegmentDigit", () => {
    render(<SevenSegmentDisplay value="AB" />);

    const display = screen.getByRole("img");
    const digits = display.querySelectorAll(".seven-segment-char");
    expect(digits.length).toBe(2);
  });

  it("应该渲染数字0-9的segments", () => {
    render(<SevenSegmentDisplay value="0" />);

    const display = screen.getByRole("img");
    const segments = display.querySelectorAll(".seven-segment-seg.is-on");
    expect(segments.length).toBe(6);
  });

  it("应该渲染数字1的segments", () => {
    render(<SevenSegmentDisplay value="1" />);

    const display = screen.getByRole("img");
    const segments = display.querySelectorAll(".seven-segment-seg.is-on");
    expect(segments.length).toBe(2);
  });
});
