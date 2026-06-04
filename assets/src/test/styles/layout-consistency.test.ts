import { describe, it, expect } from "vitest";
import { readFileSync } from "fs";
import { resolve } from "path";

describe("layout.css 样式一致性测试", () => {
  const cssPath = resolve(__dirname, "../../styles/components/layout.css");
  let cssContent: string;

  beforeEach(() => {
    cssContent = readFileSync(cssPath, "utf-8");
  });

  it("my-topbar::before 底边透明度应为 15%（非 34%）", () => {
    const topbarBeforeMatch = cssContent.match(/\.my-topbar::before\s*\{[^}]+\}/s);
    expect(topbarBeforeMatch).toBeTruthy();

    const block = topbarBeforeMatch![0];
    expect(block).toContain("border-bottom");
    expect(block).toContain("15%");
    expect(block).not.toContain("34%");
  });

  it("my-sidebar::after 渐变叠加层应已被删除（无 .my-sidebar::after 规则）", () => {
    const sidebarAfterMatch = cssContent.match(/\.my-sidebar::after\s*\{/);
    expect(sidebarAfterMatch).toBeNull();
  });

  it("my-clock-glass > div 应包含 align-items: center 和 justify-content: center", () => {
    const glassDivMatch = cssContent.match(/\.my-clock-glass > div\s*\{[^}]+\}/s);
    expect(glassDivMatch).toBeTruthy();

    const block = glassDivMatch![0];
    expect(block).toContain("display: flex");
    expect(block).toContain("align-items: center");
    expect(block).toContain("justify-content: center");
  });

  it("my-clock-glass > div 的时钟垂直偏移默认值为 10px", () => {
    const glassDivMatch = cssContent.match(/\.my-clock-glass > div\s*\{[^}]+\}/s);
    expect(glassDivMatch).toBeTruthy();

    const block = glassDivMatch![0];
    expect(block).toContain("--clock-vertical-offset");
    expect(block).toContain("10px");
  });
});