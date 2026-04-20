#!/usr/bin/env bash
set -euo pipefail

# 本地复现 release 打包流程的辅助脚本
# 用法: ./scripts/test_local_release.sh [--dry-run] [--embed-html|--no-embed-html] [--debug]

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(dirname "$SCRIPT_DIR")"

cd "$ROOT_DIR"

ZIG_CMD="${ZIG_CMD:-zig}"
PKG_CMD="${PKG_CMD:-bun}"
CARGO_CMD="${CARGO_CMD:-cargo}"

DRY_RUN=0
EXTRA_ARGS=()

while [[ $# -gt 0 ]]; do
  case "$1" in
    --dry-run)
      DRY_RUN=1; shift ;;
    --*)
      EXTRA_ARGS+=("$1"); shift ;;
    *)
      EXTRA_ARGS+=("$1"); shift ;;
  esac
done

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || {
    echo "错误: 缺少命令: $1" >&2
    exit 1
  }
}

echo "=== 检查环境 ==="
require_cmd "$ZIG_CMD" || true
require_cmd "$PKG_CMD" || true
require_cmd "$CARGO_CMD" || true

echo "ZIG: $(command -v "$ZIG_CMD" || echo 'not found')"
echo "PKG: $(command -v "$PKG_CMD" || echo 'not found')"
echo "CARGO: $(command -v "$CARGO_CMD" || echo 'not found')"

if [[ $DRY_RUN -eq 1 ]]; then
  echo "Dry run: 不会执行生成或打包步骤。显示将执行的命令："
  echo "git-cliff --config cliff-release.toml --latest --strip header > release_notes.md"
  echo "./scripts/package_linux.sh ${EXTRA_ARGS[*]}"
  exit 0
fi

echo "=== 生成 changelog (git-cliff) ==="
if command -v git-cliff >/dev/null 2>&1; then
  git-cliff --config cliff-release.toml --latest --strip header > release_notes.md
  echo "生成 release_notes.md"
else
  echo "警告: 未检测到 git-cliff，尝试用 cargo 安装它（需 Rust/cargo 已安装）"
  if command -v "$CARGO_CMD" >/dev/null 2>&1; then
    $CARGO_CMD install git-cliff
    git-cliff --config cliff-release.toml --latest --strip header > release_notes.md
    echo "生成 release_notes.md"
  else
    echo "无法生成 changelog：既没有 git-cliff 也没有 cargo 可用" >&2
  fi
fi

echo "=== 运行打包脚本 ==="
export ZIG_CMD PKG_CMD
./scripts/package_linux.sh "${EXTRA_ARGS[@]}"

echo "=== 打包产物 ==="
ls -lh dist || true
ls -lh dist/*.tar.gz || echo "未发现 dist/*.tar.gz"

echo "完成。若需要将 job 调度到本地 self-hosted runner，请参考 README 或使用 gh release 触发 release 事件。"
