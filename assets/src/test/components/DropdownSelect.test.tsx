import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/preact";
import { DropdownSelect } from "../../components/DropdownSelect";

describe("DropdownSelect", () => {
  const options = [
    { value: "1", label: "选项一" },
    { value: "2", label: "选项二" },
    { value: "3", label: "选项三" },
  ];

  it("应该渲染选择器按钮", () => {
    render(
      <DropdownSelect
        value="1"
        options={options}
        onChange={vi.fn()}
      />
    );

    expect(screen.getByRole("button")).toBeDefined();
  });

  it("应该显示选中项的标签", () => {
    render(
      <DropdownSelect
        value="2"
        options={options}
        onChange={vi.fn()}
      />
    );

    expect(screen.getByText("选项二")).toBeDefined();
  });

  it("点击应该打开下拉菜单", () => {
    render(
      <DropdownSelect
        value="1"
        options={options}
        onChange={vi.fn()}
      />
    );

    const button = screen.getByRole("button");
    fireEvent.click(button);

    expect(screen.getAllByText("选项二").length).toBeGreaterThan(0);
  });

  it("选择选项应该触发 onChange", () => {
    const onChange = vi.fn();

    render(
      <DropdownSelect
        value="1"
        options={options}
        onChange={onChange}
      />
    );

    const button = screen.getByRole("button");
    fireEvent.click(button);

    const allOptions = screen.getAllByText("选项二");
    fireEvent.click(allOptions[allOptions.length - 1]);

    expect(onChange).toHaveBeenCalledWith("2");
  });

  it("禁用时不应该响应点击", () => {
    const onChange = vi.fn();

    render(
      <DropdownSelect
        value="1"
        options={options}
        onChange={onChange}
        disabled={true}
      />
    );

    const button = screen.getByRole("button");
    fireEvent.click(button);

    expect(onChange).not.toHaveBeenCalled();
  });

  it("空选项数组应该正常渲染", () => {
    const { container } = render(
      <DropdownSelect
        value=""
        options={[]}
        onChange={vi.fn()}
      />
    );

    expect(container.firstChild).toBeDefined();
  });
});
