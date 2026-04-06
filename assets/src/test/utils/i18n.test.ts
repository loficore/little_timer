import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { t, setLanguage, getCurrentLanguage, getMessages, parseToml } from "../../utils/i18n";

vi.mock("../../i18n/*.toml", () => ({
  ZH: `
[common]
save_success = "保存成功"
save_hint = "点击保存设置"

[validation]
save_error = "保存失败: {error}"
required = "{field} 为必填项"

[timer]
start = "开始"
pause = "暂停"
  `,
  EN: `
[common]
save_success = "Saved successfully"
  `,
  JP: `
[common]
save_success = "保存成功（日语）"
  `,
}), { virtual: true });

describe("parseToml", () => {
  it("应该解析简单的 TOML", () => {
    const result = parseToml(`
[section]
key = "value"
`);
    expect(result).toEqual({ section: { key: "value" } });
  });

  it("应该解析嵌套节", () => {
    const result = parseToml(`
[parent.child]
key = "value"
`);
    expect(result).toEqual({ parent: { child: { key: "value" } } });
  });

  it("应该解析字符串值", () => {
    const result = parseToml('key = "hello world"');
    expect(result).toEqual({ key: "hello world" });
  });

  it("应该解析数字值", () => {
    const result = parseToml("count = 42");
    expect(result).toEqual({ count: 42 });
  });

  it("应该解析布尔值", () => {
    const result = parseToml("enabled = true");
    expect(result).toEqual({ enabled: true });
  });

  it("应该忽略注释行", () => {
    const result = parseToml(`
# 这是一个注释
key = "value"
`);
    expect(result).toEqual({ key: "value" });
  });

  it("应该处理转义字符", () => {
    const result = parseToml('key = "hello \\"world\\""');
    expect(result).toEqual({ key: 'hello "world"' });
  });
});

describe("t", () => {
  beforeEach(async () => {
    await setLanguage("ZH");
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("路径不存在时返回原路径", () => {
    expect(t("nonexistent.key")).toBe("nonexistent.key");
  });
});

describe("setLanguage", () => {
  it("应该切换到英文", async () => {
    await setLanguage("EN");
    expect(getCurrentLanguage()).toBe("EN");
  });

  it("应该切换到日文", async () => {
    await setLanguage("JP");
    expect(getCurrentLanguage()).toBe("JP");
  });

  it("语言不区分大小写", async () => {
    await setLanguage("en");
    expect(getCurrentLanguage()).toBe("EN");
  });
});

describe("getMessages", () => {
  it("应该返回当前语言的消息对象", () => {
    const messages = getMessages();
    expect(messages).toHaveProperty("common");
  });
});
