import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { t, setLanguage, getCurrentLanguage, getMessages } from "../../utils/i18n";

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
