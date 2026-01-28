import { describe, it, expect } from "vitest";
import { render } from "@testing-library/preact";
import { FormSection } from "../../components/FormSection";

describe("FormSection 组件", () => {
  it("应该渲染分组标题", () => {
    const { container } = render(
      <FormSection title="基本设置">
        <div>内容</div>
      </FormSection>,
    );

    expect(container.textContent).toContain("基本设置");
  });

  it("应该渲染分组描述", () => {
    const { container } = render(
      <FormSection title="基本设置" description="这是应用的基本设置">
        <div>内容</div>
      </FormSection>,
    );

    expect(container.textContent).toContain("这是应用的基本设置");
  });

  it("应该渲染子内容", () => {
    const { container } = render(
      <FormSection title="标题">
        <div>测试内容</div>
      </FormSection>,
    );

    expect(container.textContent).toContain("测试内容");
  });

  it("没有标题时应该只渲染内容", () => {
    const { container } = render(
      <FormSection>
        <div>内容</div>
      </FormSection>,
    );

    expect(container.textContent).toContain("内容");
    expect(container.querySelector("h3")).toBeFalsy();
  });

  it("应该支持自定义 className", () => {
    const { container } = render(
      <FormSection className="custom-class">
        <div>内容</div>
      </FormSection>,
    );

    expect(container.querySelector("div")?.className).toContain("custom-class");
  });

  it("标题和描述应该有正确的样式", () => {
    const { container } = render(
      <FormSection title="标题" description="描述">
        <div>内容</div>
      </FormSection>,
    );

    const title = container.querySelector("h3");
    const description = container.querySelector("p");

    expect(title?.className).toContain("font-semibold");
    expect(description?.className).toContain("italic");
  });
});
