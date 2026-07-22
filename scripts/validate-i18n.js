#!/usr/bin/env node

import fs from "node:fs";
import path from "node:path";
import { createRequire } from "node:module";

const assetsNodeModules = path.join(import.meta.dirname, "..", "assets", "node_modules");
const require = createRequire(path.join(assetsNodeModules, "validate-i18n.js"));
const toml = require("toml");

const I18N_DIR = path.join(import.meta.dirname, "..", "assets", "i18n");
const LANGUAGES = [
  { code: "ZH", filename: "zh.toml", name: "Chinese" },
  { code: "EN", filename: "en.toml", name: "English" },
  { code: "JP", filename: "jp.toml", name: "Japanese" },
];

function flattenTOML(data, prefix = "") {
  const result = {};

  for (const [key, value] of Object.entries(data)) {
    const fullKey = prefix ? `${prefix}.${key}` : key;

    if (typeof value === "object" && value !== null && !Array.isArray(value)) {
      const isLeaf = Object.values(value).every(
        (v) => typeof v === "string" || typeof v === "number"
      );

      if (isLeaf) {
        for (const [subKey, subValue] of Object.entries(value)) {
          const leafKey = `${fullKey}.${subKey}`;
          result[leafKey] = String(subValue);
        }
      } else {
        Object.assign(result, flattenTOML(value, fullKey));
      }
    }
  }

  return result;
}

function parseTOMLFile(filepath) {
  try {
    const content = fs.readFileSync(filepath, "utf-8");
    const parsed = toml.parse(content);
    const keys = flattenTOML(parsed);
    return { keys, raw: parsed };
  } catch (err) {
    const message = err instanceof Error ? err.message : String(err);
    console.error(`Failed to parse ${path.basename(filepath)}: ${message}`);
    return null;
  }
}

function findTrueDuplicates(keys) {
  const seen = new Set();
  const duplicates = [];

  for (const fullKey of Object.keys(keys)) {
    if (seen.has(fullKey)) {
      duplicates.push(fullKey);
    }
    seen.add(fullKey);
  }

  return duplicates;
}

function main() {
  console.log("Validating i18n TOML files...\n");

  const results = [];

  for (const { code, filename, name } of LANGUAGES) {
    const filepath = path.join(I18N_DIR, filename);
    const parsed = parseTOMLFile(filepath);

    if (!parsed) {
      console.error(`❌ ${name} (${code}): Failed to parse TOML`);
      process.exit(1);
    }

    results.push({
      lang: code,
      name,
      keys: parsed.keys,
      filepath,
    });
  }

  let hasDuplicateErrors = false;

  for (const { lang, name, keys } of results) {
    const duplicates = findTrueDuplicates(keys);
    if (duplicates.length > 0) {
      console.error(`❌ ${name} (${lang}): Duplicate keys found:`);
      for (const key of duplicates) {
        console.error(`   - "${key}" appears more than once`);
      }
      hasDuplicateErrors = true;
    }
  }

  if (hasDuplicateErrors) {
    console.error("\n❌ Validation failed: duplicate keys found");
    process.exit(1);
  }

  const allKeys = new Set();
  for (const { keys } of results) {
    for (const key of Object.keys(keys)) {
      allKeys.add(key);
    }
  }

  const missing = {
    ZH: new Set(),
    EN: new Set(),
    JP: new Set(),
  };

  const extras = {
    ZH: new Set(),
    EN: new Set(),
    JP: new Set(),
  };

  for (const key of allKeys) {
    const presentIn = [];

    for (const { lang, keys } of results) {
      if (key in keys) {
        presentIn.push(lang);
      }
    }

    if (presentIn.length < 3) {
      const missingLangs = LANGUAGES.map((l) => l.code).filter(
        (code) => !presentIn.includes(code)
      );

      for (const missingLang of missingLangs) {
        missing[missingLang].add(key);
      }

      for (const lang of presentIn) {
        extras[lang].add(key);
      }
    }
  }

  let hasErrors = false;

  for (const { code, name } of LANGUAGES) {
    if (missing[code].size > 0) {
      console.error(`❌ ${name} (${code}) missing keys: ${Array.from(missing[code]).join(", ")}`);
      hasErrors = true;
    }
  }

  for (const { code, name } of LANGUAGES) {
    if (extras[code].size > 0) {
      const onlyInThisLang = Array.from(extras[code]).filter((key) => {
        const presentInOthers = results
          .filter((r) => r.lang !== code)
          .some((r) => key in r.keys);
        return !presentInOthers;
      });

      if (onlyInThisLang.length > 0) {
        console.error(`❌ ${name} (${code}) has extra keys (not in other files): ${onlyInThisLang.join(", ")}`);
        hasErrors = true;
      }
    }
  }

  console.log("\n--- Summary ---");
  for (const { lang, name, keys } of results) {
    console.log(`${name} (${lang}): ${Object.keys(keys).length} keys`);
  }

  const totalMissing = Object.values(missing).reduce((sum, set) => sum + set.size, 0);
  console.log(`\nTotal missing keys: ${totalMissing}`);

  if (hasErrors) {
    console.error("\n❌ Validation failed: key parity check failed");
    process.exit(1);
  }

  console.log("\n✅ All i18n files are valid!");
  process.exit(0);
}

main();
