// 轻量级 TOML 解析与 i18n 工具，避免额外依赖
// 仅支持本项目使用的简单语法：
// - 表头 [section] / [section.sub]
// - key = "string" | integer | boolean
// - 不支持数组、内联表，请在语言文件中避免

import zhRaw from "../../i18n/zh.toml?raw";
import enRaw from "../../i18n/en.toml?raw";
import jpRaw from "../../i18n/jp.toml?raw";

type Messages = Record<string, unknown>;

type LangCode = "ZH" | "EN" | "JP";

const loaders: Record<LangCode, () => Promise<string>> = {
  ZH: async () => zhRaw,
  EN: async () => enRaw,
  JP: async () => jpRaw,
};

const defaultMessages = parseToml(zhRaw);
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

function deepMerge(base: Messages, override: Messages): Messages {
  const output: Messages = { ...base };
  for (const [k, v] of Object.entries(override)) {
    if (v && typeof v === "object" && !Array.isArray(v)) {
      const baseChild = (base as any)[k];
      const nextBase = baseChild && typeof baseChild === "object" && !Array.isArray(baseChild) ? baseChild : {};
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

export function getCurrentLanguage(): LangCode {
  return currentLang;
}

export function getMessages(): Messages {
  return messages;
}
