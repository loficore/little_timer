import toml from "toml";

// Vite root-absolute glob (resolved from project root, not from this file's location)
const rawI18n = import.meta.glob("/i18n/*.toml", {
  as: "raw",
  eager: true,
});

type Messages = Record<string, unknown>;

type LangCode = "ZH" | "EN" | "JP";

const embeddedLangs: Record<LangCode, string> = {
  ZH: rawI18n["/i18n/zh.toml"] ?? "",
  EN: rawI18n["/i18n/en.toml"] ?? "",
  JP: rawI18n["/i18n/jp.toml"] ?? "",
};

const defaultMessages = toml.parse(embeddedLangs.ZH);
let messages: Messages = defaultMessages;
let currentLang: LangCode = "ZH";

function getPath(obj: any, path: string[]): any {
  let node = obj;
  for (const part of path) {
    if (node == null) return undefined;
    node = node[part];
  }
  return node;
}

/**
 * 根据路径查找翻译文本，支持参数替换
 * @param path - 翻译键路径，格式如 "timer.stopwatch"
 * @param params - 可选参数对象，用于替换文本中的 {key}
 */
export function t(path: string, params?: Record<string, string | number>): string {
  const parts = path.split(".");
  const val = (getPath(messages, parts) ?? getPath(defaultMessages, parts)) as
    | string
    | number
    | undefined;
  if (val == null) return path;
  const str = String(val);
  if (!params) return str;
  return Object.keys(params).reduce((acc, key) => acc.replace(`{${key}}`, String(params[key])), str);
}

/**
 * 设置当前语言，会重新加载对应语言的翻译文件
 * @param lang - 语言代码，如 "ZH"、"EN"、"JP"
 */
export function setLanguage(lang: string): void {
  const upper = lang.toUpperCase() as LangCode;
  try {
    messages = defaultMessages;
    Object.assign(messages, toml.parse(embeddedLangs[upper] ?? ""));
    currentLang = upper;
  } catch (err) {
    console.error("Failed to load language", lang, err);
    messages = defaultMessages;
    currentLang = "ZH";
  }
}


/**
 * 获取当前语言代码
 * @returns 当前语言，如 "ZH"、"EN" 或 "JP"
 */
export function getCurrentLanguage(): LangCode {
  return currentLang;
}

/**
 * 获取当前语言的全部翻译数据
 * @returns Messages 对象，键为翻译路径，值为翻译文本
 */
export function getMessages(): Messages {
  return messages;
}
