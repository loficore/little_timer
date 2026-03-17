#!/bin/bash
# 开发模式脚本 - Linux/macOS
# 前端 dev server + 后端并行运行
# 用法: ./scripts/dev.sh

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

cd "$PROJECT_ROOT"

cleanup() {
    echo ""
    echo "=== 关闭服务 ==="
    [ -n "$VITE_PID" ] && kill "$VITE_PID" 2>/dev/null
    [ -n "$ZIG_PID" ] && kill "$ZIG_PID" 2>/dev/null
    exit 0
}

trap cleanup SIGINT SIGTERM

echo "=== 启动前端 Dev Server ==="
cd assets
if command -v bun &> /dev/null; then
    bun run dev &
    VITE_PID=$!
else
    echo "错误: 未找到 bun"
    exit 1
fi
cd ..

echo "等待前端服务启动..."
sleep 3

echo "=== 启动后端 ==="
if command -v zig &> /dev/null; then
    zig build -Dembed_ui=true -Doptimize=Debug run &
    ZIG_PID=$!
else
    echo "错误: 未找到 zig"
    exit 1
fi

echo ""
echo "=== 服务已启动 ==="
echo "前端: http://localhost:5173"
echo "后端: http://localhost:8080"
echo ""
echo "按 Ctrl+C 停止所有服务"

wait
