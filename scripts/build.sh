#!/bin/bash
# 构建脚本 - Linux/macOS
# 用法: ./scripts/build.sh [--debug|--release] [--embed-html|--no-embed-html] [--go]
#
#   --go            仅构建 Go 后端（跳过 Zig 前端 + 后端）
#   --zig           仅构建 Zig 后端（默认行为，去掉 --go 即可）
#   --both          构建 Zig + Go 两个后端（前端只构建一次）
#
# 不带参数时行为同原来：构建前端 + Zig 后端。

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

cd "$PROJECT_ROOT"

# 解析参数
BUILD_MODE="zig"      # "zig" | "go" | "both"
EMBED_UI="false"
OPTIMIZE="ReleaseFast"

for arg in "$@"; do
    case $arg in
        --release)
            OPTIMIZE="ReleaseFast"
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
        --go)
            BUILD_MODE="go"
            ;;
        --zig)
            BUILD_MODE="zig"
            ;;
        --both)
            BUILD_MODE="both"
            ;;
        --help|-h)
            echo "用法: $0 [选项]"
            echo "选项:"
            echo "  --release         发布构建（仅设置优化级别）"
            echo "  --debug           调试构建（仅设置优化级别）"
            echo "  --embed-html      内嵌前端 HTML 到后端二进制"
            echo "  --no-embed-html   不内嵌前端 HTML（默认）"
            echo "  --go              仅构建 Go 后端（跳过 Zig + 前端）"
            echo "  --zig             仅构建 Zig 后端（默认）"
            echo "  --both            构建 Zig + Go 两个后端"
            echo "  --help            显示此帮助"
            echo ""
            echo "示例:"
            echo "  $0 --release --embed-html    # Zig 发行版（内嵌前端）"
            echo "  $0 --go                     # 仅 Go 后端"
            echo "  $0 --both                   # Zig + Go 双后端"
            exit 0
            ;;
        *)
            echo "错误: 未知参数 '$arg'"
            echo "使用 --help 查看可用选项"
            exit 1
            ;;
    esac
done

# ── 前端构建（zig / both 模式需要） ────────────────────────────────
if [ "$BUILD_MODE" != "go" ]; then
    echo "=== 构建前端 ==="
    cd assets
    if command -v pnpm &> /dev/null; then
        pnpm install
        pnpm run build
    else
        echo "错误: 未找到 pnpm"
        exit 1
    fi
    cd ..
fi

# ── Zig 后端 ─────────────────────────────────────────────────────────
if [ "$BUILD_MODE" = "zig" ] || [ "$BUILD_MODE" = "both" ]; then
    echo "=== 构建 Zig 后端 (Optimize=$OPTIMIZE, EmbedUI=$EMBED_UI) ==="
    if command -v zig &> /dev/null; then
        if [ "$EMBED_UI" = "true" ]; then
            zig build -Doptimize=$OPTIMIZE -Dembed_ui=true
        else
            zig build -Doptimize=$OPTIMIZE
        fi
    else
        echo "警告: 未找到 zig，跳过 Zig 后端构建"
    fi
fi

# ── Go 后端 ─────────────────────────────────────────────────────────
if [ "$BUILD_MODE" = "go" ] || [ "$BUILD_MODE" = "both" ]; then
    echo "=== 构建 Go 后端 (Optimize=$OPTIMIZE, EmbedUI=$EMBED_UI) ==="
    cd neo-src
    GO_LDFLAGS=""
    if [ "$EMBED_UI" = "true" ]; then
        GO_LDFLAGS="-tags embed_ui"
        echo "  (Go build: embed_ui enabled)"
    fi
    if [ "$OPTIMIZE" = "ReleaseFast" ] || [ "$OPTIMIZE" = "ReleaseSafe" ] || [ "$OPTIMIZE" = "ReleaseSmall" ]; then
        go build -ldflags="$GO_LDFLAGS" -o bin/server ./cmd/server/
    else
        # Debug 模式保留调试信息
        go build -ldflags="$GO_LDFLAGS" -gcflags="all=-N -l" -o bin/server ./cmd/server/
    fi
    cd ..
    echo "  Go binary: neo-src/bin/server"
fi

echo "=== 构建完成 ==="
[ "$BUILD_MODE" = "zig" ] && echo "运行 Zig: ./zig-out/bin/little_timer"
[ "$BUILD_MODE" = "go" ]  && echo "运行 Go:  ./neo-src/bin/server serve --http-only"
[ "$BUILD_MODE" = "both" ] && echo "Zig: ./zig-out/bin/little_timer  |  Go: ./neo-src/bin/server serve --http-only"
