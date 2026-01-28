import { describe, it, expect } from "vitest";
import { render } from "@testing-library/preact";
import { FormGroup } from "../../components/FormGroup";

describe("FormGroup 组件", () => {
  it("应该渲染标签", () => {
    const { container } = render(
      <FormGroup label="测试标签">
        <input type="text" />
      </FormGroup>,
    );

    expect(container.textContent).toContain("测试标签");
  });

  it("应该渲染子内容（输入框）", () => {
    const { container } = render(
      <FormGroup label="标签">
        <input type="text" placeholder="输入框" />
      </FormGroup>,
    );

    const input = container.querySelector("input");
    expect(input?.placeholder).toBe("输入框");
  });

  it("应该显示提示信息", () => {
    const { container } = render(
      <FormGroup label="标签" hint="这是提示信息">
        <input type="text" />
      </FormGroup>,
    );

    expect(container.textContent).toContain("这是提示信息");
  });

  it("应该显示错误信息", () => {
    const { container } = render(
      <FormGroup label="标签" error="这是错误信息">
        <input type="text" />
      </FormGroup>,
    );

    expect(container.textContent).toContain("这是错误信息");
  });

  it("错误信息显示时不应该显示提示信息", () => {
    const { container } = render(
      <FormGroup label="标签" hint="这是提示" error="这是错误">
        <input type="text" />
      </FormGroup>,
    );

    expect(container.textContent).toContain("这是错误");
    expect(container.textContent).not.toContain("这是提示");
  });

  it("必填字段应该显示红色星号", () => {
    const { container } = render(
      <FormGroup label="必填字段" required={true}>
        <input type="text" />
      </FormGroup>,
    );

    const star = container.querySelector("span.text-red-500");
    expect(star?.textContent).toBe("*");
  });

  it("应该支持垂直布局（默认）", () => {
    const { container } = render(
      <FormGroup label="标签" layout="vertical">
        <input type="text" />
      </FormGroup>,
    );

    const wrapper = container.querySelector("div");
    expect(wrapper?.className).toContain("flex-col");
  });

  it("应该支持水平布局", () => {
    const { container } = render(
      <FormGroup label="标签" layout="horizontal">
        <input type="text" />
      </FormGroup>,
    );

    const wrapper = container.querySelector("div");
    expect(wrapper?.className).toContain("flex-row");
  });

  it("应该支持自定义 className", () => {
    const { container } = render(
      <FormGroup label="标签" className="custom-class">
        <input type="text" />
      </FormGroup>,
    );

    expect(container.querySelector("div")?.className).toContain("custom-class");
  });

  it("标签应该有正确的样式", () => {
    const { container } = render(
      <FormGroup label="标签">
        <input type="text" />
      </FormGroup>,
    );

    const label = container.querySelector("label");
    expect(label?.className).toContain("font-medium");
    expect(label?.className).toContain("text-text-primary-dark");
  });

  it("错误信息应该有错误样式", () => {
    const { container } = render(
      <FormGroup label="标签" error="错误消息">
        <input type="text" />
      </FormGroup>,
    );

    const errorSpan = Array.from(container.querySelectorAll("span")).find(
      (span) => span.textContent?.includes("错误消息"),
    );

    expect(errorSpan?.className).toContain("text-red-500");
  });
});
