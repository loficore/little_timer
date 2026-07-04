#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ASSETS_DIR="$ROOT_DIR/assets"
NEO_SRC_DIR="$ROOT_DIR/neo-src"
DIST_DIR="$ROOT_DIR/dist"
STAGE_DIR="$DIST_DIR/stage"
APP_NAME="little_timer"
VERSION="$(date +%Y%m%d)"

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || {
    echo "❌ 缺少命令: $1" >&2
    exit 1
  }
}

require_cmd "go"
require_cmd "pnpm"
require_cmd "tar"

show_help() {
  echo "用法: $0 [选项]"
  echo "选项:"
  echo "  --version <ver>   指定版本号（默认日期）"
  echo "  --help, -h        显示此帮助"
  echo ""
  echo "示例:"
  echo "  $0 --version 1.0.0"
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --version)
      VERSION="$2"
      shift
      ;;
    --help|-h)
      show_help
      exit 0
      ;;
    *)
      echo "错误: 未知参数 '$1'" >&2
      exit 1
      ;;
  esac
  shift
done

# 1) 构建前端
echo "=== 构建前端 ==="
cd "$ASSETS_DIR"
if [[ ! -d node_modules ]]; then
  pnpm install
fi
pnpm run build

# 2) 修复：确保 i18n 文件进入构建产物
mkdir -p "$ASSETS_DIR/dist/i18n"
cp -f "$ASSETS_DIR/i18n/"*.toml "$ASSETS_DIR/dist/i18n/"

# 3) 构建 Go 后端（内嵌前端）
echo "=== 构建 Go 后端（内嵌前端）==="
cd "$NEO_SRC_DIR"
go build -tags embed_ui -ldflags="-s -w" -o bin/server ./cmd/server

BIN_PATH="$NEO_SRC_DIR/bin/server"
if [[ ! -f "$BIN_PATH" ]]; then
  echo "❌ 未找到可执行文件: $BIN_PATH" >&2
  exit 1
fi

# 4) 打包
echo "=== 打包 ==="
rm -rf "$STAGE_DIR"
mkdir -p "$STAGE_DIR"
cp -f "$BIN_PATH" "$STAGE_DIR/"

# 规范化版本字符串
SANITIZED_VERSION="$(echo "$VERSION" | sed -E 's/[^A-Za-z0-9._-]/_/g')"
TAR_NAME="${APP_NAME}-${SANITIZED_VERSION}-linux-x64.tar.gz"

mkdir -p "$DIST_DIR"
tar -czf "$DIST_DIR/$TAR_NAME" -C "$STAGE_DIR" .

echo "✅ 打包完成: $DIST_DIR/$TAR_NAME"
