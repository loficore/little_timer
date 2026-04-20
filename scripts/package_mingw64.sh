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
PKG_CMD="${PKG_CMD:-bun}"
EMBED_UI="${EMBED_UI:-true}"
OPTIMIZE_MODE="${OPTIMIZE_MODE:-ReleaseFast}"
TARGET_TRIPLE="${TARGET_TRIPLE:-x86_64-windows-gnu}"

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
  echo "  --release         发布构建（优化级别设为 ReleaseFast）"
  echo "  --debug           调试构建（仅设置优化级别）"
  echo "  --target <triple> 目标三元组（默认: $TARGET_TRIPLE）"
  echo "  --embed-html      内嵌前端 HTML 到后端二进制（默认）"
  echo "  --no-embed-html   不内嵌前端 HTML"
  echo "  --help, -h        显示此帮助"
  echo ""
  echo "示例:"
  echo "  $0 --release --embed-html"
  echo "  $0 --debug --embed-html"
  echo "  $0 --debug --no-embed-html"
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --release)
      OPTIMIZE_MODE="ReleaseFast"
      ;;
    --debug)
      OPTIMIZE_MODE="Debug"
      ;;
    --target)
      if [[ $# -lt 2 ]]; then
        echo "错误: --target 需要一个值" >&2
        exit 1
      fi
      TARGET_TRIPLE="$2"
      shift
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
      echo "错误: 未知参数 '$1'" >&2
      echo "使用 --help 查看可用选项" >&2
      exit 1
      ;;
  esac
  shift
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
  (cd "$ROOT_DIR" && "$ZIG_CMD" build -Dtarget="$TARGET_TRIPLE" -Doptimize="$OPTIMIZE_MODE" -Dembed_ui=true)
else
  (cd "$ROOT_DIR" && "$ZIG_CMD" build -Dtarget="$TARGET_TRIPLE" -Doptimize="$OPTIMIZE_MODE" -Dembed_ui=false)
fi

BIN_PATH="$ROOT_DIR/zig-out/bin/little_timer.exe"
if [[ ! -f "$BIN_PATH" ]]; then
  echo "❌ 未找到可执行文件: $BIN_PATH" >&2
  exit 1
fi

CLI_BIN_PATH="$ROOT_DIR/zig-out/bin/little_timer_cli.exe"
if [[ ! -f "$CLI_BIN_PATH" ]]; then
  echo "❌ 未找到可执行文件: $CLI_BIN_PATH" >&2
  exit 1
fi

# 4) 打包
rm -rf "$STAGE_DIR"
mkdir -p "$STAGE_DIR"
cp -f "$BIN_PATH" "$STAGE_DIR/"
cp -f "$CLI_BIN_PATH" "$STAGE_DIR/"

cat >"$STAGE_DIR/start_gui.bat" <<'EOF'
@echo off
setlocal
start "Little Timer" "%~dp0little_timer.exe"
EOF

cat >"$STAGE_DIR/start_cli.bat" <<'EOF'
@echo off
setlocal
"%~dp0little_timer_cli.exe" %*
EOF

mkdir -p "$DIST_DIR"
tar -czf "$DIST_DIR/$TAR_NAME" -C "$STAGE_DIR" .

echo "✅ 打包完成: $DIST_DIR/$TAR_NAME"
