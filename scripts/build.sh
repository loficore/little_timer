#!/bin/bash
# 构建脚本 - Linux/macOS
# 用法: ./scripts/build.sh [--release]

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

cd "$PROJECT_ROOT"

# 解析参数
EMBED_UI="false"
OPTIMIZE="Release"
for arg in "$@"; do
    case $arg in
        --release)
            OPTIMIZE="Release"
            EMBED_UI="true"
            ;;
        --debug)
            OPTIMIZE="Debug"
            EMBED_UI="false"
            ;;
        --help|-h)
            echo "用法: $0 [选项]"
            echo "选项:"
            echo "  --release  发布构建（内嵌 UI）"
            echo "  --debug    调试构建"
            echo "  --help     显示此帮助"
            exit 0
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

echo "=== 构建后端 (Optimize=$OPTIMIZE, EmbedUI=$EMBED_UI) ==="
if command -v zig &> /dev/null; then
    if [ "$EMBED_UI" = "true" ]; then
        zig build -Doptimize=$OPTIMIZE -Dembed_ui=true
    else
        zig build -Doptimize=$OPTIMIZE
    fi
else
    echo "错误: 未找到 zig"
    exit 1
fi

echo "=== 构建完成 ==="
echo "运行: ./zig-out/bin/little_timer"
