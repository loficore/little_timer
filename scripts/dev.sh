#!/bin/bash
# 开发模式脚本 - Linux/macOS
# 支持两种模式：
#   ./scripts/dev.sh         普通模式 (无 WebView 窗口)
#   ./scripts/dev.sh --webview WebView 模式 (打开 WebView 窗口)

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

cd "$PROJECT_ROOT"

# 解析参数
MODE="http"
USE_STD_HTTP="true"
for arg in "$@"; do
    case $arg in
        --webview)
            MODE="webview"
            ;;
        --no-std-http)
            USE_STD_HTTP="false"
            ;;
        --help|-h)
            echo "用法: $0 [选项]"
            echo "选项:"
            echo "  --webview       启动 WebView 窗口模式"
            echo "  --no-std-http   使用 httpx 而非 std.http.Server"
            echo "  --help          显示此帮助"
            exit 0
            ;;
    esac
done

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

echo "=== 启动后端 (use_std_http=$USE_STD_HTTP) ==="
if command -v zig &> /dev/null; then
    if [ "$MODE" = "webview" ]; then
        zig build -Dembed_ui=false -Doptimize=Debug -Duse_std_http=$USE_STD_HTTP run -- --webview &
        ZIG_PID=$!
    else
        zig build -Dembed_ui=false -Doptimize=Debug -Duse_std_http=$USE_STD_HTTP run &
        ZIG_PID=$!
    fi
else
    echo "错误: 未找到 zig"
    exit 1
fi

echo ""
echo "=== 服务已启动 ==="
echo "前端: http://localhost:5173"
echo "后端: http://localhost:8080"
if [ "$MODE" = "webview" ]; then
    echo "WebView: 已打开窗口指向前端"
fi
echo "HTTP Server: $(if [ "$USE_STD_HTTP" = "true" ]; then echo "std.http.Server"; else echo "httpx"; fi)"
echo ""
echo "按 Ctrl+C 停止所有服务"

wait