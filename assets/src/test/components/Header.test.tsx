import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, fireEvent, screen } from "@testing-library/preact";
import { Header } from "../../components/Header";

describe("Header 组件", () => {
  const mockOnSettings = vi.fn();
  const mockOnBack = vi.fn();

  beforeEach(() => {
    mockOnSettings.mockClear();
    mockOnBack.mockClear();
  });

  it("应该正确渲染标题", () => {
    const { container } = render(
      <Header title="测试标题" showSettings={false} />,
    );

    expect(container.textContent).toContain("测试标题");
  });

  it("应该显示设置按钮", () => {
    render(
      <Header
        title="标题"
        showSettings={true}
        onSettingsClick={mockOnSettings}
      />,
    );

    const settingsBtn = screen.getByTitle(/设置/i);
    expect(settingsBtn).toBeTruthy();
    expect(settingsBtn.textContent).toContain("⚙");
  });

  it("设置按钮点击应该触发回调", () => {
    render(
      <Header
        title="标题"
        showSettings={true}
        onSettingsClick={mockOnSettings}
      />,
    );

    const settingsBtn = screen.getByTitle(/设置/i);
    fireEvent.click(settingsBtn);

    expect(mockOnSettings).toHaveBeenCalled();
  });

  it("应该显示返回按钮", () => {
    render(<Header title="标题" showBack={true} onBackClick={mockOnBack} />);

    const backBtn = screen.getByTitle(/返回|back/i);
    expect(backBtn).toBeTruthy();
    expect(backBtn.textContent).toContain("←");
  });

  it("返回按钮点击应该触发回调", () => {
    render(<Header title="标题" showBack={true} onBackClick={mockOnBack} />);

    const backBtn = screen.getByTitle(/返回|back/i);
    fireEvent.click(backBtn);

    expect(mockOnBack).toHaveBeenCalled();
  });

  it("应该支持同时显示返回和设置按钮", () => {
    render(
      <Header
        title="标题"
        showBack={true}
        onBackClick={mockOnBack}
        showSettings={true}
        onSettingsClick={mockOnSettings}
      />,
    );

    const backBtn = screen.getByTitle(/返回|back/i);
    const settingsBtn = screen.getByTitle(/设置/i);

    expect(backBtn).toBeTruthy();
    expect(settingsBtn).toBeTruthy();
  });
});
