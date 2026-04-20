import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/preact";
import { PickerNumberInput } from "../../components/PickerNumberInput";

describe("PickerNumberInput", () => {
  it("应该渲染输入框和加减按钮", () => {
    const onChange = vi.fn();
    render(
      <PickerNumberInput
        value={25}
        onChange={onChange}
      />
    );

    const input = screen.getByRole("textbox") as HTMLInputElement;
    expect(input).toBeTruthy();
    expect(input.value).toBe("25");

    const buttons = screen.getAllByRole("button");
    expect(buttons.length).toBe(2);
  });

  it("应该显示标签", () => {
    const onChange = vi.fn();
    render(
      <PickerNumberInput
        value={10}
        onChange={onChange}
        label="分钟"
      />
    );

    expect(screen.getByText("分钟")).toBeTruthy();
  });

  it("应该显示单位", () => {
    const onChange = vi.fn();
    render(
      <PickerNumberInput
        value={5}
        onChange={onChange}
        unit="分钟"
      />
    );

    expect(screen.getByText("分钟")).toBeTruthy();
  });

  it("应该显示提示文字", () => {
    const onChange = vi.fn();
    render(
      <PickerNumberInput
        value={0}
        onChange={onChange}
        hint="最小值"
      />
    );

    expect(screen.getByText("最小值")).toBeTruthy();
  });

  it("增加按钮应该调用 onChange", () => {
    const onChange = vi.fn();
    render(
      <PickerNumberInput
        value={5}
        min={0}
        max={99}
        onChange={onChange}
      />
    );

    const buttons = screen.getAllByRole("button");
    fireEvent.click(buttons[0]);

    expect(onChange).toHaveBeenCalledWith(6);
  });

  it("减少按钮应该调用 onChange", () => {
    const onChange = vi.fn();
    render(
      <PickerNumberInput
        value={5}
        min={0}
        max={99}
        onChange={onChange}
      />
    );

    const buttons = screen.getAllByRole("button");
    fireEvent.click(buttons[1]);

    expect(onChange).toHaveBeenCalledWith(4);
  });

  it("值达到最大值时增加按钮应该禁用", () => {
    const onChange = vi.fn();
    render(
      <PickerNumberInput
        value={99}
        min={0}
        max={99}
        onChange={onChange}
      />
    );

    const buttons = screen.getAllByRole("button");
    expect((buttons[0] as HTMLButtonElement).disabled).toBe(true);
  });

  it("值达到最小值时减少按钮应该禁用", () => {
    const onChange = vi.fn();
    render(
      <PickerNumberInput
        value={0}
        min={0}
        max={99}
        onChange={onChange}
      />
    );

    const buttons = screen.getAllByRole("button");
    expect((buttons[1] as HTMLButtonElement).disabled).toBe(true);
  });

  it("禁用状态下按钮应该禁用", () => {
    const onChange = vi.fn();
    render(
      <PickerNumberInput
        value={50}
        min={0}
        max={99}
        onChange={onChange}
        disabled={true}
      />
    );

    const input = screen.getByRole("textbox") as HTMLInputElement;
    expect(input.disabled).toBe(true);

    const buttons = screen.getAllByRole("button");
    expect((buttons[0] as HTMLButtonElement).disabled).toBe(true);
    expect((buttons[1] as HTMLButtonElement).disabled).toBe(true);
  });

  it("输入空字符串时不应该调用 onChange", () => {
    const onChange = vi.fn();
    const { container } = render(
      <PickerNumberInput
        value={25}
        min={0}
        max={99}
        onChange={onChange}
      />
    );

    const input = container.querySelector("input");
    fireEvent.change(input!, { target: { value: "" } });

    expect(onChange).not.toHaveBeenCalled();
  });

  it("输入非数字时应该被忽略", () => {
    const onChange = vi.fn();
    const { container } = render(
      <PickerNumberInput
        value={25}
        min={0}
        max={99}
        onChange={onChange}
      />
    );

    const input = container.querySelector("input");
    fireEvent.change(input!, { target: { value: "abc" } });

    expect(onChange).not.toHaveBeenCalled();
  });

  it("输入值超过最大值时应该被限制", () => {
    const onChange = vi.fn();
    const { container } = render(
      <PickerNumberInput
        value={25}
        min={0}
        max={99}
        onChange={onChange}
      />
    );

    const input = container.querySelector("input");
    fireEvent.change(input!, { target: { value: "150" } });

    expect(onChange).toHaveBeenCalledWith(99);
  });

  it("输入值小于最小值时应该被限制", () => {
    const onChange = vi.fn();
    const { container } = render(
      <PickerNumberInput
        value={25}
        min={0}
        max={99}
        onChange={onChange}
      />
    );

    const input = container.querySelector("input");
    fireEvent.change(input!, { target: { value: "-5" } });

    expect(onChange).toHaveBeenCalledWith(0);
  });

  it("默认 min 值应该是 0", () => {
    const onChange = vi.fn();
    render(
      <PickerNumberInput
        value={0}
        max={99}
        onChange={onChange}
      />
    );

    const buttons = screen.getAllByRole("button");
    expect((buttons[1] as HTMLButtonElement).disabled).toBe(true);
  });

  it("默认 max 值应该是 99", () => {
    const onChange = vi.fn();
    render(
      <PickerNumberInput
        value={99}
        min={0}
        onChange={onChange}
      />
    );

    const buttons = screen.getAllByRole("button");
    expect((buttons[0] as HTMLButtonElement).disabled).toBe(true);
  });
});
