import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, fireEvent, screen } from "@testing-library/preact";
import { CheckboxInput } from "../../components/CheckboxInput";

describe("CheckboxInput 组件", () => {
  const mockOnChange = vi.fn();

  beforeEach(() => {
    mockOnChange.mockClear();
  });

  it("应该正确渲染勾选状态", () => {
    render(
      <CheckboxInput value={true} onChange={mockOnChange} label="启用循环" />,
    );

    const checkbox = screen.getByRole("checkbox") as HTMLInputElement;
    expect(checkbox.checked).toBe(true);
  });

  it("应该正确渲染未勾选状态", () => {
    render(
      <CheckboxInput value={false} onChange={mockOnChange} label="启用循环" />,
    );

    const checkbox = screen.getByRole("checkbox") as HTMLInputElement;
    expect(checkbox.checked).toBe(false);
  });

  it("应该在点击时切换状态", () => {
    render(
      <CheckboxInput value={false} onChange={mockOnChange} label="启用循环" />,
    );

    const checkbox = screen.getByRole("checkbox");
    fireEvent.click(checkbox);

    expect(mockOnChange).toHaveBeenCalledWith(true);
  });

  it("应该显示标签文本", () => {
    const { container } = render(
      <CheckboxInput value={false} onChange={mockOnChange} label="启用循环" />,
    );

    expect(container.textContent).toContain("启用循环");
  });

  it("应该支持禁用状态", () => {
    render(
      <CheckboxInput
        value={false}
        onChange={mockOnChange}
        label="启用循环"
        disabled={true}
      />,
    );

    const checkbox = screen.getByRole("checkbox") as HTMLInputElement;
    expect(checkbox.disabled).toBe(true);
  });

  it("禁用时不应该触发 onChange", () => {
    render(
      <CheckboxInput
        value={false}
        onChange={mockOnChange}
        label="启用循环"
        disabled={true}
      />,
    );

    const checkbox = screen.getByRole("checkbox");
    fireEvent.click(checkbox);

    // 禁用状态下不会触发 onChange
    expect(mockOnChange).not.toHaveBeenCalled();
  });
});
