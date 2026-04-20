#!/usr/bin/env node
/**
 * i18n 校验脚本
 * 检查内容:
 * 1. 各语言版本中的 key 是否同步
 * 2. TOML 格式是否正确
 * 3. 是否有缺失的翻译
 */

import fs from "node:fs";
import path from "node:path";

const I18N_DIR = path.join(import.meta.dirname, "..", "assets", "i18n");
const LANGUAGES = ["zh", "en", "jp"];

/**
 * 解析 TOML 文件
 */
function parseToml(content) {
  const lines = content.split(/\r?\n/);
  const result = {};
  let currentPath = [];

  for (const line of lines) {
    const trimmed = line.trim();

    // 跳过注释和空行
    if (!trimmed || trimmed.startsWith("#")) continue;

    // 处理 section header - 支持嵌套如 [errors.connection]
    if (trimmed.startsWith("[") && trimmed.endsWith("]")) {
      const section = trimmed.slice(1, -1).trim();
      currentPath = section.split(".");
      let node = result;
      for (const part of currentPath) {
        if (!node[part]) {
          node[part] = {};
        }
        node = node[part];
      }
      continue;
    }

    // 处理 key = value
    const eqIndex = trimmed.indexOf("=");
    if (eqIndex === -1) continue;

    const key = trimmed.slice(0, eqIndex).trim();
    let value = trimmed.slice(eqIndex + 1).trim();

    // 解析值
    if (value.startsWith('"') && value.endsWith('"')) {
      value = value.slice(1, -1).replace(/\\"/g, '"');
    } else if (value === "true" || value === "false") {
      value = value === "true";
    } else {
      const num = Number(value);
      value = Number.isNaN(num) ? value : num;
    }

    // 设置值到正确的嵌套位置
    if (currentPath.length === 0) {
      // 全局 key
      result[key] = value;
    } else {
      // 嵌套 key
      let node = result;
      for (let i = 0; i < currentPath.length; i++) {
        const part = currentPath[i];
        if (i === currentPath.length - 1) {
          node[part][key] = value;
        } else {
          node = node[part];
        }
      }
    }
  }

  return result;
}

/**
 * 获取所有 key 的扁平路径
 */
function flattenKeys(obj, prefix = []) {
  const keys = [];

  for (const [key, value] of Object.entries(obj)) {
    const currentPath = [...prefix, key];

    if (value && typeof value === "object" && !Array.isArray(value)) {
      keys.push(...flattenKeys(value, currentPath));
    } else {
      keys.push(currentPath.join("."));
    }
  }

  return keys;
}

/**
 * 检查 TOML 格式
 */
function checkTomlFormat(content, filename) {
  const errors = [];
  const lines = content.split(/\r?\n/);
  let bracketBalance = 0;

  for (let i = 0; i < lines.length; i++) {
    const line = lines[i].trim();
    const lineNum = i + 1;

    // 检查括号平衡
    for (const char of line) {
      if (char === "[") bracketBalance++;
      if (char === "]") bracketBalance--;
    }

    // 检查无效字符
    if (line.includes("\t")) {
      errors.push(`Line ${lineNum}: 使用了 tab 缩进，建议使用空格`);
    }

    // 检查行尾空格
    if (line !== line.replace(/\s+$/, "")) {
      errors.push(`Line ${lineNum}: 行尾有多余空格`);
    }

    // 检查不完整的引号
    const quoteCount = (line.match(/"/g) || []).length;
    if (quoteCount % 2 !== 0 && !line.startsWith("#")) {
      errors.push(`Line ${lineNum}: 引号未配对`);
    }
  }

  if (bracketBalance !== 0) {
    errors.push(`括号不匹配: [ 和 ] 数量差 ${bracketBalance}`);
  }

  return errors;
}

async function main() {
  console.log("🔍 检查 i18n 文件...\n");

  const files = {};
  const allKeys = new Set();
  const languageKeys = {};

  // 加载所有语言文件
  for (const lang of LANGUAGES) {
    const filePath = path.join(I18N_DIR, `${lang}.toml`);

    if (!fs.existsSync(filePath)) {
      console.error(`❌ 找不到文件: ${filePath}`);
      process.exit(1);
    }

    const content = fs.readFileSync(filePath, "utf-8");
    files[lang] = content;

    // 检查格式
    console.log(`📄 检查 ${lang}.toml 格式...`);
    const formatErrors = checkTomlFormat(content, `${lang}.toml`);

    if (formatErrors.length > 0) {
      console.error(`  ❌ 格式错误:`);
      for (const err of formatErrors) {
        console.error(`    - ${err}`);
      }
    } else {
      console.log(`  ✅ 格式正确`);
    }

    // 解析并收集 key
    const parsed = parseToml(content);
    const keys = flattenKeys(parsed);
    languageKeys[lang] = new Set(keys);

    for (const key of keys) {
      allKeys.add(key);
    }
  }

  console.log(`\n📊 检查 key 同步情况...`);

  const totalKeys = allKeys.size;
  let missingCount = 0;
  let extraCount = 0;

  // 检查每种语言的 key
  for (const lang of LANGUAGES) {
    const keys = languageKeys[lang];
    const missing = [...allKeys].filter((k) => !keys.has(k));
    const extra = [...keys].filter((k) => !allKeys.has(k));

    console.log(`\n${lang}.toml:`);
    console.log(`  总 key 数: ${keys.size} / ${totalKeys}`);

    if (missing.length > 0) {
      missingCount += missing.length;
      console.log(`  ❌ 缺失 ${missing.length} 个 key:`);
      for (const k of missing.slice(0, 10)) {
        console.log(`    - ${k}`);
      }
      if (missing.length > 10) {
        console.log(`    ... 还有 ${missing.length - 10} 个`);
      }
    } else {
      console.log(`  ✅ 无缺失`);
    }

    if (extra.length > 0) {
      extraCount += extra.length;
      console.log(`  ⚠️  多出 ${extra.length} 个 key:`);
      for (const k of extra.slice(0, 10)) {
        console.log(`    + ${k}`);
      }
      if (extra.length > 10) {
        console.log(`    ... 还有 ${extraCount - 10} 个`);
      }
    }
  }

  // 总结
  console.log("\n📈 总结:");
  console.log(`  总 key 数: ${totalKeys}`);
  console.log(`  缺失 key: ${missingCount}`);
  console.log(`  多余 key: ${extraCount}`);

  if (missingCount > 0 || extraCount > 0) {
    console.log("\n❌ i18n 同步检查未通过");
    process.exit(1);
  } else {
    console.log("\n✅ i18n 同步检查通过");
    process.exit(0);
  }
}
main();