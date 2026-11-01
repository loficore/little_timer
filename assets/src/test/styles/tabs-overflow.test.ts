import { describe, it, expect } from "vitest";
import { readFileSync } from "fs";
import { resolve } from "path";

describe(".my-tabs-scroll 横向溢出策略(R6)", () => {
  const css = readFileSync(
    resolve(__dirname, "../../styles/components/controls.css"),
    "utf-8",
  );
  // 窄屏规则追加在文件末尾，从这里往后切片断言（比用正则抠块稳）
  const idx = css.indexOf("max-width: 639px");
  const narrow = idx >= 0 ? css.slice(idx) : "";

  it("有窄屏规则块", () => {
    expect(idx).toBeGreaterThan(-1);
  });

  it("只作用于设置页的滚动变体，不波及 PlanPage 的两 tab", () => {
    expect(narrow).toContain(".my-tabs.my-tabs-scroll");
    expect(narrow).not.toMatch(/^\s*\.my-tabs\s*\{/m);
  });

  it("显式允许横向滚动并带两端渐隐", () => {
    expect(narrow).toContain("overflow-x: auto");
    expect(narrow).toContain("mask-image");
  });

  it("滚动变体里的 tab 不被 flex:1 压扁", () => {
    expect(narrow).toContain("flex: 0 0 auto");
  });
});
