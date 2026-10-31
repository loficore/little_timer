#!/usr/bin/env bash
# 把 assets/dist 同步到 cmd/server/assets，供 -tags embed_ui 嵌入。
# 只替换 index.html / fonts / i18n，保留 Wails 生成的 bindings/。
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SRC="$ROOT/assets/dist"
DST="$ROOT/neo-src/cmd/server/assets"

if [ ! -f "$SRC/index.html" ]; then
    echo "missing $SRC/index.html; run 'cd assets && pnpm run build' first" >&2
    exit 1
fi
if [ ! -d "$SRC/fonts" ]; then
    echo "missing $SRC/fonts" >&2
    exit 1
fi

mkdir -p "$DST"
cp -f "$SRC/index.html" "$DST/index.html"
rm -rf "$DST/fonts"
cp -a "$SRC/fonts" "$DST/fonts"
# i18n/*.toml 由 Vite 以 raw 方式内联进 index.html。仍保留可选的目录拷贝，
# 方便将来改回运行时加载；源不存在时清掉上一轮的残留。
rm -rf "$DST/i18n"
if [ -d "$SRC/i18n" ]; then
    cp -a "$SRC/i18n" "$DST/i18n"
fi

echo "staged embedded UI: $DST"
