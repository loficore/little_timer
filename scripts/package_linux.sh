#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ASSETS_DIR="$ROOT_DIR/assets"
DIST_DIR="$ROOT_DIR/dist"
STAGE_DIR="$DIST_DIR/stage"
APP_NAME="little_timer"
VERSION="$(date +%Y%m%d)"
TAR_NAME="${APP_NAME}_${VERSION}_linux_x64.tar.gz"

ZIG_CMD="${ZIG_CMD:-zig}"
PKG_CMD="${PKG_CMD:-bun}"
EMBED_UI="${EMBED_UI:-false}"
# Zig 0.15+ 使用具体 OptimizeMode：ReleaseFast/ReleaseSafe/ReleaseSmall/Debug
OPTIMIZE_MODE="${OPTIMIZE_MODE:-ReleaseFast}"

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || {
    echo "❌ 缺少命令: $1" >&2
    exit 1
  }
}

require_cmd "$ZIG_CMD"
require_cmd "$PKG_CMD"
require_cmd "tar"

show_help() {
  echo "用法: $0 [选项]"
  echo "选项:"
  echo "  --release         发布构建（仅设置优化级别）"
  echo "  --debug           调试构建（仅设置优化级别）"
  echo "  --embed-html      内嵌前端 HTML 到后端二进制"
  echo "  --no-embed-html   不内嵌前端 HTML（默认）"
  echo "  --help, -h        显示此帮助"
  echo ""
  echo "示例:"
  echo "  $0 --release --embed-html"
  echo "  $0 --debug --embed-html"
  echo "  $0 --debug --no-embed-html"
}

for arg in "$@"; do
  case "$arg" in
    --release)
      OPTIMIZE_MODE="ReleaseFast"
      ;;
    --debug)
      OPTIMIZE_MODE="Debug"
      ;;
    --embed-html|--embed-ui)
      EMBED_UI="true"
      ;;
    --no-embed-html|--no-embed-ui)
      EMBED_UI="false"
      ;;
    --help|-h)
      show_help
      exit 0
      ;;
    *)
      echo "错误: 未知参数 '$arg'" >&2
      echo "使用 --help 查看可用选项" >&2
      exit 1
      ;;
  esac
done

# 1) 构建前端
pushd "$ASSETS_DIR" >/dev/null
if [[ ! -d node_modules ]]; then
  "$PKG_CMD" install
fi
"$PKG_CMD" run build
popd >/dev/null

# 2) 修复：确保 i18n 文件进入构建产物（Vite 默认不会复制原始 toml）
mkdir -p "$ASSETS_DIR/dist/i18n"
cp -f "$ASSETS_DIR/i18n/"*.toml "$ASSETS_DIR/dist/i18n/"

# 3) 构建 Zig
if [[ "$EMBED_UI" == "1" || "$EMBED_UI" == "true" ]]; then
  "$ZIG_CMD" build -Doptimize="$OPTIMIZE_MODE" -Dembed_ui=true
else
  "$ZIG_CMD" build -Doptimize="$OPTIMIZE_MODE" -Dembed_ui=false
fi

BIN_PATH="$ROOT_DIR/zig-out/bin/little_timer"
if [[ ! -f "$BIN_PATH" ]]; then
  echo "❌ 未找到可执行文件: $BIN_PATH" >&2
  exit 1
fi

# 4) 打包
rm -rf "$STAGE_DIR"
mkdir -p "$STAGE_DIR"
cp -f "$BIN_PATH" "$STAGE_DIR/"

mkdir -p "$DIST_DIR"
tar -czf "$DIST_DIR/$TAR_NAME" -C "$STAGE_DIR" .

echo "✅ 打包完成: $DIST_DIR/$TAR_NAME"
