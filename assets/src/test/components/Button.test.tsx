import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, fireEvent, screen } from "@testing-library/preact";
import { Button } from "../../components/Button";

describe("Button 组件", () => {
  const mockOnClick = vi.fn();

  beforeEach(() => {
    mockOnClick.mockClear();
  });

  it("应该渲染按钮文本", () => {
    render(<Button onClick={mockOnClick}>点击我</Button>);

    expect(screen.getByText("点击我")).toBeTruthy();
  });

  it("点击应该触发回调", () => {
    render(<Button onClick={mockOnClick}>点击</Button>);

    fireEvent.click(screen.getByText("点击"));

    expect(mockOnClick).toHaveBeenCalled();
  });

  it("应该支持不同的样式变体", () => {
    const { rerender, container } = render(
      <Button variant="primary">主要按钮</Button>,
    );

    expect(container.querySelector("button")?.className).toContain(
      "bg-accent-dark",
    );

    rerender(<Button variant="secondary">次要按钮</Button>);

    expect(container.querySelector("button")?.className).toContain(
      "bg-secondary-dark",
    );

    rerender(<Button variant="danger">危险按钮</Button>);

    expect(container.querySelector("button")?.className).toContain(
      "bg-red-600",
    );

    rerender(<Button variant="ghost">幽灵按钮</Button>);

    expect(container.querySelector("button")?.className).toContain(
      "bg-transparent",
    );
  });

  it("应该支持不同的尺寸", () => {
    const { rerender, container } = render(<Button size="sm">小按钮</Button>);

    expect(container.querySelector("button")?.className).toContain("text-xs");

    rerender(<Button size="md">中等按钮</Button>);

    expect(container.querySelector("button")?.className).toContain("text-sm");

    rerender(<Button size="lg">大按钮</Button>);

    expect(container.querySelector("button")?.className).toContain("text-base");
  });

  it("应该支持显示图标", () => {
    render(<Button icon="▶">开始</Button>);

    const button = screen.getByText("开始").parentElement;
    expect(button?.textContent).toContain("▶");
  });

  it("应该支持禁用状态", () => {
    render(
      <Button disabled={true} onClick={mockOnClick}>
        禁用按钮
      </Button>,
    );

    const button = screen.getByRole("button") as HTMLButtonElement;
    expect(button.disabled).toBe(true);
    fireEvent.click(button);

    // 禁用时不应该触发回调
    expect(mockOnClick).not.toHaveBeenCalled();
  });

  it("应该支持自定义 className", () => {
    const { container } = render(
      <Button className="custom-class">按钮</Button>,
    );

    expect(container.querySelector("button")?.className).toContain(
      "custom-class",
    );
  });

  it("应该支持标题提示", () => {
    render(<Button title="这是一个提示">按钮</Button>);

    const button = screen.getByRole("button");
    expect(button.getAttribute("title")).toBe("这是一个提示");
  });

  it("应该支持多个变体和尺寸组合", () => {
    const { container } = render(
      <Button variant="danger" size="lg" icon="🗑">
        删除
      </Button>,
    );

    const button = container.querySelector("button");
    expect(button?.className).toContain("bg-red-600");
    expect(button?.className).toContain("text-base");
  });
});
