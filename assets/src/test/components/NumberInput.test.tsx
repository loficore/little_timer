import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, fireEvent, screen } from "@testing-library/preact";
import { NumberInput } from "../../components/NumberInput";

describe("NumberInput 组件", () => {
  const mockOnChange = vi.fn();

  beforeEach(() => {
    mockOnChange.mockClear();
  });

  it("应该正确渲染基本属性", () => {
    render(
      <NumberInput
        value={25}
        onChange={mockOnChange}
        label="测试标签"
        min={1}
        max={100}
      />,
    );

    const input = screen.getByDisplayValue("25");
    expect(input).toBeTruthy();
    expect(input.getAttribute("min")).toBe("1");
    expect(input.getAttribute("max")).toBe("100");
  });

  it("应该在输入变化时调用 onChange", () => {
    render(<NumberInput value={25} onChange={mockOnChange} label="测试标签" />);

    const input = screen.getByDisplayValue("25") as HTMLInputElement;
    fireEvent.change(input, { target: { value: "30" } });

    expect(mockOnChange).toHaveBeenCalledWith(30);
  });

  it("应该正确处理最小值验证", () => {
    render(
      <NumberInput
        value={25}
        onChange={mockOnChange}
        label="测试标签"
        min={10}
      />,
    );

    const input = screen.getByDisplayValue("25") as HTMLInputElement;
    fireEvent.change(input, { target: { value: "5" } });

    // 应该用最小值替代
    expect(mockOnChange).toHaveBeenCalledWith(10);
  });

  it("应该正确处理最大值验证", () => {
    render(
      <NumberInput
        value={25}
        onChange={mockOnChange}
        label="测试标签"
        max={50}
      />,
    );

    const input = screen.getByDisplayValue("25") as HTMLInputElement;
    fireEvent.change(input, { target: { value: "100" } });

    // 应该用最大值替代
    expect(mockOnChange).toHaveBeenCalledWith(50);
  });

  it("应该显示单位", () => {
    const { container } = render(
      <NumberInput
        value={25}
        onChange={mockOnChange}
        label="测试标签"
        unit="分钟"
      />,
    );

    expect(container.textContent).toContain("分钟");
  });

  it("应该支持禁用状态", () => {
    render(
      <NumberInput
        value={25}
        onChange={mockOnChange}
        label="测试标签"
        disabled={true}
      />,
    );

    const input = screen.getByDisplayValue("25") as HTMLInputElement;
    expect(input.disabled).toBe(true);
  });
});
