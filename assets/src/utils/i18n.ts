// 轻量级 TOML 解析与 i18n 工具，避免额外依赖
// 仅支持本项目使用的简单语法：
// - 表头 [section] / [section.sub]
// - key = "string" | integer | boolean
// - 不支持数组、内联表，请在语言文件中避免

// 使用 Vite 的 glob + eager 将所有语言文件内嵌进打包产物
// 这样不会在运行时依赖外部 i18n 文件，适配单文件/内嵌发布
const rawI18n = import.meta.glob("../../i18n/*.toml", {
  as: "raw",
  eager: true,
});

type Messages = Record<string, unknown>;

type LangCode = "ZH" | "EN" | "JP";

const embeddedLangs: Record<LangCode, string> = {
  ZH: rawI18n["../../i18n/zh.toml"] ?? "",
  EN: rawI18n["../../i18n/en.toml"] ?? "",
  JP: rawI18n["../../i18n/jp.toml"] ?? "",
};

const loaders: Record<LangCode, () => Promise<string>> = {
  ZH: () => Promise.resolve(embeddedLangs.ZH),
  EN: () => Promise.resolve(embeddedLangs.EN),
  JP: () => Promise.resolve(embeddedLangs.JP),
};

const defaultMessages = parseToml(embeddedLangs.ZH);
let messages: Messages = defaultMessages;
let currentLang: LangCode = "ZH";

function ensurePath(root: Messages, path: string[]): any {
  let node: any = root;
  for (const part of path) {
    if (node[part] == null || typeof node[part] !== "object") {
      node[part] = {};
    }
    node = node[part];
  }
  return node;
}

function parseValue(raw: string): any {
  const trimmed = raw.trim();
  if (trimmed.startsWith("\"") && trimmed.endsWith("\"")) {
    return trimmed.slice(1, -1).replace(/\\"/g, '"');
  }
  if (trimmed === "true" || trimmed === "false") {
    return trimmed === "true";
  }
  const num = Number(trimmed);
  if (!Number.isNaN(num)) return num;
  return trimmed;
}

/**
 * 解析 TOML 字符串为消息对象
 * @param {string} raw - 原始 TOML 字符串
 * @returns {Messages} 解析后的消息对象
 */
export function parseToml(raw: string): Messages {
  const result: Messages = {};
  let path: string[] = [];

  raw.split(/\r?\n/).forEach((line) => {
    const trimmed = line.trim();
    if (!trimmed || trimmed.startsWith("#")) return;

    if (trimmed.startsWith("[") && trimmed.endsWith("]")) {
      const section = trimmed.slice(1, -1).trim();
      path = section.split(".");
      ensurePath(result, path);
      return;
    }

    const eqIndex = trimmed.indexOf("=");
    if (eqIndex === -1) return;
    const key = trimmed.slice(0, eqIndex).trim();
    const valueRaw = trimmed.slice(eqIndex + 1).trim();
    const target = ensurePath(result, path);
    target[key] = parseValue(valueRaw);
  });

  return result;
}

/**
 * 深度合并两个消息对象
 * @param {Messages} base - 基础消息对象
 * @param {Messages} override - 覆盖消息对象
 * @returns {Messages} 合并后的消息对象
 */
function deepMerge(base: Messages, override: Messages): Messages {
  const output: Messages = { ...base };
  for (const [k, v] of Object.entries(override)) {
    if (v && typeof v === "object" && !Array.isArray(v)) {
      const baseChild = (base as Record<string, unknown>)[k];
      const nextBase = baseChild && typeof baseChild === "object" && !Array.isArray(baseChild) ? baseChild as Messages : {};
      output[k] = deepMerge(nextBase, v as Messages);
    } else {
      output[k] = v;
    }
  }
  return output;
}

function getPath(obj: any, path: string[]): any {
  let node = obj;
  for (const part of path) {
    if (node == null) return undefined;
    node = node[part];
  }
  return node;
}

/**
 * 获取翻译消息
 * @param {string} path - 消息路径
 * @param {Record<string, string | number>} params - 替换参数
 * @returns {string} 翻译后的消息
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
 * 设置当前语言
 * @param {string} lang - 语言代码，例如 "ZH"、"EN"、"JP"
 */
export async function setLanguage(lang: string): Promise<void> {
  const upper = lang.toUpperCase() as LangCode;
  const loader = loaders[upper] ?? loaders.ZH;
  try {
    const raw = await loader();
    const parsed = parseToml(raw);
    messages = deepMerge(defaultMessages, parsed);
    currentLang = upper;
  } catch (err) {
    console.error("Failed to load language", lang, err);
    messages = defaultMessages;
    currentLang = "ZH";
  }
}

/**
 * 获取当前语言
 * @returns {string} 当前语言代码，例如 "ZH"、"EN"、"JP"
 */
export function getCurrentLanguage(): LangCode {
  return currentLang;
}

/**
 * 获取当前语言的所有消息对象，主要用于调试或导出完整翻译内容
 * @returns {Messages} 当前语言的所有消息对象
 */
export function getMessages(): Messages {
  return messages;
}
