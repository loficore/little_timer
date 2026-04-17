#!/bin/bash
# 构建脚本 - Linux/macOS
# 用法: ./scripts/build.sh [--debug|--release] [--embed-html|--no-embed-html] [--std-http|--no-std-http]

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

cd "$PROJECT_ROOT"

# 解析参数
EMBED_UI="false"
OPTIMIZE="Release"
USE_STD_HTTP="true"
for arg in "$@"; do
    case $arg in
        --release)
            OPTIMIZE="Release"
            ;;
        --debug)
            OPTIMIZE="Debug"
            ;;
        --embed-html|--embed-ui)
            EMBED_UI="true"
            ;;
        --no-embed-html|--no-embed-ui)
            EMBED_UI="false"
            ;;
        --std-http)
            USE_STD_HTTP="true"
            ;;
        --no-std-http)
            USE_STD_HTTP="false"
            ;;
        --help|-h)
            echo "用法: $0 [选项]"
            echo "选项:"
            echo "  --release         发布构建（仅设置优化级别）"
            echo "  --debug           调试构建（仅设置优化级别）"
            echo "  --embed-html      内嵌前端 HTML 到后端二进制"
            echo "  --no-embed-html   不内嵌前端 HTML（默认）"
            echo "  --std-http        使用 std.http.Server（默认）"
            echo "  --no-std-http     使用 httpx"
            echo "  --help            显示此帮助"
            echo ""
            echo "示例:"
            echo "  $0 --release --embed-html"
            echo "  $0 --debug --embed-html"
            echo "  $0 --debug --no-embed-html"
            echo "  $0 --debug --no-embed-html --no-std-http"
            exit 0
            ;;
        *)
            echo "错误: 未知参数 '$arg'"
            echo "使用 --help 查看可用选项"
            exit 1
            ;;
    esac
done

echo "=== 构建前端 ==="
cd assets
if command -v bun &> /dev/null; then
    bun install
    bun run build
else
    echo "错误: 未找到 bun"
    exit 1
fi
cd ..

echo "=== 构建后端 (Optimize=$OPTIMIZE, EmbedUI=$EMBED_UI, UseStdHttp=$USE_STD_HTTP) ==="
if command -v zig &> /dev/null; then
    if [ "$EMBED_UI" = "true" ]; then
        zig build -Doptimize=$OPTIMIZE -Dembed_ui=true -Duse_std_http=$USE_STD_HTTP
    else
        zig build -Doptimize=$OPTIMIZE -Duse_std_http=$USE_STD_HTTP
    fi
else
    echo "错误: 未找到 zig"
    exit 1
fi

echo "=== 构建完成 ==="
echo "运行: ./zig-out/bin/little_timer"
