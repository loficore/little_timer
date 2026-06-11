import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent } from "@testing-library/preact";
import { ErrorBoundary, PageErrorBoundary } from "../../components/ErrorBoundary";

vi.mock("../../utils/logger", () => ({
  logError: vi.fn(),
}));

describe("ErrorBoundary 组件", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("正常渲染子组件", () => {
    const { container } = render(
      <ErrorBoundary>
        <div>正常内容</div>
      </ErrorBoundary>
    );

    expect(container.textContent).toContain("正常内容");
  });

  it("子组件出错时显示错误 UI", () => {
    const ThrowError = () => {
      throw new Error("测试错误");
    };

    const { container } = render(
      <ErrorBoundary>
        <ThrowError />
      </ErrorBoundary>
    );

    expect(container.textContent).toContain("出错了");
    expect(container.textContent).toContain("测试错误");
  });

  it("子组件出错时显示重新加载按钮", () => {
    const ThrowError = () => {
      throw new Error("测试错误");
    };

    render(
      <ErrorBoundary>
        <ThrowError />
      </ErrorBoundary>
    );

    expect(screen.getByRole("button", { name: /重新加载/i })).toBeTruthy();
  });

  it("应该显示重新加载按钮文本", () => {
    const ThrowError = () => {
      throw new Error("测试错误");
    };

    render(
      <ErrorBoundary>
        <ThrowError />
      </ErrorBoundary>
    );

    expect(screen.getByRole("button", { name: /重新加载/i })).toBeTruthy();
  });

  it("提供 fallback 时显示 fallback 内容", () => {
    const ThrowError = () => {
      throw new Error("测试错误");
    };

    const fallbackContent = <div data-testid="fallback">自定义错误 UI</div>;

    const { container } = render(
      <ErrorBoundary fallback={fallbackContent}>
        <ThrowError />
      </ErrorBoundary>
    );

    expect(screen.getByTestId("fallback")).toBeTruthy();
    expect(container.textContent).not.toContain("出错了");
  });

  it("提供 onError 回调时应该调用", () => {
    const ThrowError = () => {
      throw new Error("测试错误");
    };

    const onError = vi.fn();

    render(
      <ErrorBoundary onError={onError}>
        <ThrowError />
      </ErrorBoundary>
    );

    expect(onError).toHaveBeenCalled();
    expect(onError.mock.calls[0][0]).toBeInstanceOf(Error);
  });
});

describe("PageErrorBoundary 组件", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("正常渲染子组件", () => {
    const { container } = render(
      <PageErrorBoundary>
        <div>页面内容</div>
      </PageErrorBoundary>
    );

    expect(container.textContent).toContain("页面内容");
  });

  it("子组件出错时显示默认标题", () => {
    const ThrowError = () => {
      throw new Error("页面加载错误");
    };

    render(
      <PageErrorBoundary>
        <ThrowError />
      </PageErrorBoundary>
    );

    expect(screen.getByText("页面加载失败")).toBeTruthy();
  });

  it("支持自定义标���", () => {
    const ThrowError = () => {
      throw new Error("页面加载错误");
    };

    render(
      <PageErrorBoundary title="自定义标题">
        <ThrowError />
      </PageErrorBoundary>
    );

    expect(screen.getByText("自定义标题")).toBeTruthy();
  });

  it("应该显示重新加载页面按钮文本", () => {
    const ThrowError = () => {
      throw new Error("页面加载错误");
    };

    render(
      <PageErrorBoundary>
        <ThrowError />
      </PageErrorBoundary>
    );

    expect(screen.getByRole("button", { name: /重新加载页面/i })).toBeTruthy();
  });
});