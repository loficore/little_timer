#!/usr/bin/env bash
# 计算发布版本号并写入 $GITHUB_ENV。
#
# 四个 job（linux-packages / windows-installer / android-apk / publish）共用这一份：
# 各处复制的话，改一处漏一处就会出现「三个产物带 tag 版本、另三个带 commit hash」
# 这种 Release 页上看不出来的分裂。
#
# 版本来源（按优先级）：
#   1. RELEASE_TAG 环境变量（release 事件的 tag_name / workflow_dispatch 的输入）
#   2. 当前 commit 处的 git tag（去掉 v 前缀）
#   3. 短 commit hash
#
# 用法：RELEASE_TAG=<tag> bash scripts/ci-version.sh
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

echo "Using VERSION=$VERSION"
echo "VERSION=$VERSION" >> "$GITHUB_ENV"
