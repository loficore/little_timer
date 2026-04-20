#!/bin/bash
# 运行脚本 - Linux/macOS
# 用法: ./scripts/run.sh

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

cd "$PROJECT_ROOT"

EXE_PATH="$PROJECT_ROOT/zig-out/bin/little_timer"

if [ ! -f "$EXE_PATH" ]; then
    echo "未找到可执行文件，正在构建..."
    "$SCRIPT_DIR/build.sh" --release
fi

echo "=== 启动 Little Timer ==="
"$EXE_PATH"
