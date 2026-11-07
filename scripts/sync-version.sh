#!/usr/bin/env bash
# 发布时把版本号写进前端包与 Android 元数据。
#
# 用户可见版本只由 scripts/ci-version.sh 计算；assets/package.json 与
# android/app/build.gradle 里的 versionName 是构建系统的元数据，平时保留仓库中的
# 状态，只有 release 流水线在同一份临时 checkout 里用本脚本对齐 tag。
# 不要把它接到本地 just 任务上 —— 那会污染开发者的已跟踪文件。
#
# 用法：bash scripts/sync-version.sh <version>
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
VER="${1:-}"
if [ -z "$VER" ]; then
    echo "usage: $(basename "$0") <version>" >&2
    exit 1
fi
# 版本会进 JSON 字符串和 Gradle 字符串字面量：只放行安全字符，
# 同时挡住 sed/正则元字符与引号。
if ! [[ "$VER" =~ ^[A-Za-z0-9._+-]+$ ]]; then
    echo "invalid version '$VER': use letters, digits, dot, underscore, plus or hyphen" >&2
    exit 1
fi

cd "$ROOT"

node -e '
const fs = require("fs");
const version = process.argv[1];
const targets = [
  { path: "assets/package.json", re: /("version"\s*:\s*)"[^"]*"/ },
  { path: "android/app/build.gradle", re: /(versionName\s+)"[^"]*"/ },
];
for (const { path, re } of targets) {
  const source = fs.readFileSync(path, "utf8");
  if (!re.test(source)) {
    console.error(`cannot find version field in ${path}`);
    process.exit(1);
  }
  fs.writeFileSync(path, source.replace(re, `$1"${version}"`));
}
' "$VER"

echo "Synced version $VER to assets/package.json and android/app/build.gradle"
