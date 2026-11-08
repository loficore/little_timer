#!/usr/bin/env bash
# 计算发布版本号；CI 里写入 $GITHUB_ENV，本地用 --print 只输出版本。
#
# 四个 release job（linux-packages / windows-installer / android-apk / publish）
# 共用这一份：各处复制的话，改一处漏一处就会出现「三个产物带 tag 版本、另三个带
# commit hash」这种 Release 页上看不出来的分裂。本地入口（justfile、
# scripts/package-linux.sh）也用 --print 取同一个值。
#
# 版本来源（按优先级）：
#   1. RELEASE_TAG 环境变量（release 事件的 tag_name / workflow_dispatch 的输入）
#   2. 当前 commit 处的 git tag（去掉 v 前缀）
#   3. 短 commit hash（浅克隆无 tag 时的回退）
#
# 用法：
#   RELEASE_TAG=<tag> bash scripts/ci-version.sh   # 写 $GITHUB_ENV（CI）
#   bash scripts/ci-version.sh --print             # 只把版本打到 stdout
set -euo pipefail

cd "$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

VERSION="${RELEASE_TAG:-}"
VERSION="${VERSION#v}"
if [ -z "$VERSION" ]; then
    VERSION="$(git describe --tags --exact-match 2>/dev/null || true)"
fi
if [ -z "$VERSION" ]; then
    VERSION="$(git rev-parse --short HEAD)"
fi
VERSION="${VERSION#v}"

if [ "${1:-}" = "--print" ]; then
    printf '%s\n' "$VERSION"
    exit 0
fi

echo "Using VERSION=$VERSION"
if [ -n "${GITHUB_ENV:-}" ]; then
    echo "VERSION=$VERSION" >> "$GITHUB_ENV"
else
    echo "GITHUB_ENV is not set; skipping env export (local run)" >&2
fi
