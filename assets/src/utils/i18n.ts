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

const defaultMessages = toml.parse<Messages>(embeddedLangs.ZH);
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

export function setLanguage(lang: string): void {
  const upper = lang.toUpperCase() as LangCode;
  try {
    messages = defaultMessages;
    Object.assign(messages, toml.parse<Messages>(embeddedLangs[upper] ?? ""));
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
