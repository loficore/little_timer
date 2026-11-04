#!/usr/bin/env bash
# 产物命名门禁：校验 dist/ 顶层每个文件的命名是否符合
#
#   little-timer-<版本>-<平台/引擎>-<架构>[-<种类>].<ext>
#
# 只看 dist 顶层（dist/stage、dist/package 是中间目录，不参与判定）。
#
# 用法：
#   scripts/check-artifact-names.sh <version> [dist_dir]
#
# 退出码：
#   0  dist 存在、顶层至少有一个文件，且每个文件都命中允许清单、未命中禁止模式
#   1  dist 不存在、顶层无文件，或存在不合规文件（逐个打印 "<文件名>: <原因>"）
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

if [ "$#" -lt 1 ] || [ -z "${1:-}" ]; then
    echo "usage: $(basename "$0") <version> [dist_dir]" >&2
    exit 1
fi

VERSION="$1"
DIST="${2:-$ROOT/dist}"

# 转义 ERE 元字符（版本号里 `.` `+` 等会落在元字符集合内）
esc() { printf '%s' "$1" | sed 's/[][(){}.*+?^$|]/\\&/g'; }

V="$(esc "$VERSION")"
# justfile 打包 tar.gz 时会把版本号里 [^A-Za-z0-9._-] 的字符替成 _，
# rpm/deb 与其余平台产物用原样版本号 —— 两种形态都要放行。
SV="$(esc "$(printf '%s' "$VERSION" | sed -E 's/[^A-Za-z0-9._-]/_/g')")"

ALLOWED=(
    "^little-timer-${V}-webkit40\\.el9\\.x86_64\\.rpm$"
    "^little-timer-${V}-webkit41-x86_64\\.deb$"
    "^little-timer-${V}-linux-x86_64\\.tar\\.gz$"
    "^little-timer-${SV}-linux-x86_64\\.tar\\.gz$"
    "^little-timer-${V}-windows-x86_64-setup\\.exe$"
    "^little-timer-${V}-windows-x86_64-portable\\.zip$"
    "^little-timer-${V}-android-universal-debug\\.apk$"
)

FORBIDDEN=(
    '[0-9]webkit'
    '_amd64\.'
    '-x64\.'
    'little_timer-'
    '^server\.exe$'
    '^LittleTimer\.apk$'
)

if [ ! -d "$DIST" ]; then
    echo "error: dist dir does not exist: $DIST" >&2
    exit 1
fi

files=()
while IFS= read -r -d '' f; do
    files+=("$f")
done < <(find "$DIST" -maxdepth 1 -type f -print0 | sort -z)

if [ "${#files[@]}" -eq 0 ]; then
    echo "error: no artifact files found in $DIST" >&2
    exit 1
fi

violations=0
for f in "${files[@]}"; do
    name="$(basename "$f")"
    reason=""

    for pat in "${FORBIDDEN[@]}"; do
        # `--` 必须有：`-x64\.` 以 `-` 开头，会被 grep 当成选项
        if printf '%s\n' "$name" | grep -Eq -- "$pat"; then
            reason="forbidden pattern: $pat"
            break
        fi
    done

    if [ -z "$reason" ]; then
        allowed=0
        for pat in "${ALLOWED[@]}"; do
            if printf '%s\n' "$name" | grep -Eq -- "$pat"; then
                allowed=1
                break
            fi
        done
        [ "$allowed" -eq 1 ] || reason="not in the allowed name list"
    fi

    if [ -n "$reason" ]; then
        echo "$name: $reason"
        violations=$((violations + 1))
    fi
done

if [ "$violations" -gt 0 ]; then
    echo "error: $violations artifact name(s) rejected" >&2
    exit 1
fi

echo "✅ ${#files[@]} artifact name(s) conform to the naming spec"
