import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, fireEvent } from "@testing-library/preact";
import { TabPanel } from "../../components/TabPanel";

describe("TabPanel 组件", () => {
  const mockOnTabChange = vi.fn();
  const tabs = [
    { id: "basic", label: "基本设置", icon: "⚙️" },
    { id: "countdown", label: "倒计时", icon: "⏱️" },
    { id: "stopwatch", label: "正计时", icon: "⏲️" },
  ];

  beforeEach(() => {
    mockOnTabChange.mockClear();
  });

  it("应该正确渲染所有标签页", () => {
    const { container } = render(
      <TabPanel tabs={tabs} activeTab="basic" onTabChange={mockOnTabChange}>
        <div>内容区域</div>
      </TabPanel>,
    );

    // 检查所有标签页是否渲染
    expect(container.textContent).toContain("基本设置");
    expect(container.textContent).toContain("倒计时");
    expect(container.textContent).toContain("正计时");
  });

  it("应该高亮激活的标签页", () => {
    const { container } = render(
      <TabPanel tabs={tabs} activeTab="countdown" onTabChange={mockOnTabChange}>
        <div>内容区域</div>
      </TabPanel>,
    );

    const buttons = container.querySelectorAll("button");
    const countdownButton = Array.from(buttons).find((btn) =>
      btn.textContent?.includes("倒计时"),
    );

    // 激活的标签页应该有特定的样式类
    expect(countdownButton?.className).toContain("text-accent-dark");
  });

  it("应该在点击标签页时调用 onTabChange", () => {
    const { container } = render(
      <TabPanel tabs={tabs} activeTab="basic" onTabChange={mockOnTabChange}>
        <div>内容区域</div>
      </TabPanel>,
    );

    const buttons = container.querySelectorAll("button");
    const stopwatchButton = Array.from(buttons).find((btn) =>
      btn.textContent?.includes("正计时"),
    );

    if (stopwatchButton) {
      fireEvent.click(stopwatchButton);
      expect(mockOnTabChange).toHaveBeenCalledWith("stopwatch");
    }
  });

  it("应该渲染子内容", () => {
    const { container } = render(
      <TabPanel tabs={tabs} activeTab="basic" onTabChange={mockOnTabChange}>
        <div data-testid="content">测试内容</div>
      </TabPanel>,
    );

    expect(container.textContent).toContain("测试内容");
  });
});
