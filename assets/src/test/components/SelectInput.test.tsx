import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, fireEvent, screen } from "@testing-library/preact";
import { SelectInput } from "../../components/SelectInput";

describe("SelectInput 组件", () => {
  const mockOnChange = vi.fn();
  const options = [
    { value: "zh", label: "中文" },
    { value: "en", label: "English" },
    { value: "jp", label: "日本語" },
  ];

  beforeEach(() => {
    mockOnChange.mockClear();
  });

  it("应该正确渲染当前值", () => {
    render(
      <SelectInput
        value="zh"
        onChange={mockOnChange}
        options={options}
        label="语言"
      />,
    );

    const select = screen.getByDisplayValue("中文") as HTMLSelectElement;
    expect(select.value).toBe("zh");
  });

  it("应该渲染所有选项", () => {
    const { container } = render(
      <SelectInput
        value="zh"
        onChange={mockOnChange}
        options={options}
        label="语言"
      />,
    );

    const optionElements = container.querySelectorAll("option");
    expect(optionElements.length).toBe(3);
  });

  it("应该在选择变化时调用 onChange", () => {
    render(
      <SelectInput
        value="zh"
        onChange={mockOnChange}
        options={options}
        label="语言"
      />,
    );

    const select = screen.getByDisplayValue("中文");
    fireEvent.change(select, { target: { value: "en" } });

    expect(mockOnChange).toHaveBeenCalledWith("en");
  });

  it("应该支持禁用状态", () => {
    render(
      <SelectInput
        value="zh"
        onChange={mockOnChange}
        options={options}
        label="语言"
        disabled={true}
      />,
    );

    const select = screen.getByDisplayValue("中文") as HTMLSelectElement;
    expect(select.disabled).toBe(true);
  });

  it("应该显示标签", () => {
    const { container } = render(
      <SelectInput
        value="zh"
        onChange={mockOnChange}
        options={options}
        label="选择语言"
      />,
    );

    expect(container.textContent).toContain("选择语言");
  });
});
