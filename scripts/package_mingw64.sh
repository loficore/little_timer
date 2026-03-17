#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ASSETS_DIR="$ROOT_DIR/assets"
DIST_DIR="$ROOT_DIR/dist"
STAGE_DIR="$DIST_DIR/stage"
APP_NAME="little_timer"
VERSION="$(date +%Y%m%d)"
TAR_NAME="${APP_NAME}_${VERSION}_windows_x64.tar.gz"

ZIG_CMD="${ZIG_CMD:-zig}"
PNPM_CMD="${PNPM_CMD:-pnpm}"
EMBED_UI="${EMBED_UI:-1}"
OPTIMIZE_MODE="${OPTIMIZE_MODE:-ReleaseSafe}"

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || {
    echo "❌ 缺少命令: $1" >&2
    exit 1
  }
}

require_cmd "$ZIG_CMD"
require_cmd "$PNPM_CMD"
require_cmd "tar"

# 1) 构建前端
pushd "$ASSETS_DIR" >/dev/null
if [[ ! -d node_modules ]]; then
  "$PNPM_CMD" install
fi
"$PNPM_CMD" build
popd >/dev/null

# 2) 修复：确保 i18n 文件进入构建产物（Vite 默认不会复制原始 toml）
mkdir -p "$ASSETS_DIR/dist/i18n"
cp -f "$ASSETS_DIR/i18n/"*.toml "$ASSETS_DIR/dist/i18n/"

# 3) 构建 Zig
if [[ "$EMBED_UI" == "1" || "$EMBED_UI" == "true" ]]; then
  (cd "$ROOT_DIR" && "$ZIG_CMD" build -Doptimize="$OPTIMIZE_MODE" -Dembed_ui=true)
else
  (cd "$ROOT_DIR" && "$ZIG_CMD" build -Doptimize="$OPTIMIZE_MODE" -Dembed_ui=false)
fi

BIN_PATH="$ROOT_DIR/zig-out/bin/little_timer.exe"
if [[ ! -f "$BIN_PATH" ]]; then
  echo "❌ 未找到可执行文件: $BIN_PATH" >&2
  exit 1
fi

# 4) 打包
rm -rf "$STAGE_DIR"
mkdir -p "$STAGE_DIR"
cp -f "$BIN_PATH" "$STAGE_DIR/"
cp -f "$ROOT_DIR/settings.toml" "$STAGE_DIR/"

mkdir -p "$DIST_DIR"
tar -czf "$DIST_DIR/$TAR_NAME" -C "$STAGE_DIR" .

echo "✅ 打包完成: $DIST_DIR/$TAR_NAME"
