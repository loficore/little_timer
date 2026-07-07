#!/bin/bash
# 构建 Android APK
# 用法: ./scripts/build-android.sh [--package-only]
#
#   --package-only   仅运行 Gradle 打包（假设 .so 已编译好）

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
cd "$PROJECT_ROOT"

PACKAGE_ONLY=false
for arg in "$@"; do
    case $arg in
        --package-only) PACKAGE_ONLY=true ;;
    esac
done

echo "=== Building Android APK ==="

# Step 1: Generate Wails bindings (必需，否则 Android 无法调用 Go 方法)
if [ "$PACKAGE_ONLY" = "false" ]; then
    echo "--- Generate bindings ---"
    # wails3 generate bindings 需要 -tags=android 才能识别 main_android.go 中的 service
    TEMP_BINDINGS=$(mktemp -d)
    cd neo-src && \
    go run github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0-alpha2.114 generate bindings \
        -ts -clean \
        -f '-tags=android' \
        -d "$TEMP_BINDINGS" \
        ./cmd/server \
        2>&1 | grep -v "^WARNING\|^# " || true
    cd ..

    # 把生成的 bindings 复制到 assets/src/bindings/ (Vite 会处理并内联到 index.html)
    BINDINGS_SRC="$TEMP_BINDINGS/little-timer/internal/app"
    if [ -d "$BINDINGS_SRC" ]; then
        mkdir -p assets/src/bindings/little-timer/internal/app
        cp -r "$BINDINGS_SRC"/* assets/src/bindings/little-timer/internal/app/

        # 修复导入路径: @wailsio/runtime → /wails/runtime.js
        find assets/src/bindings -name "*.ts" -exec sed -i 's|@wailsio/runtime|/wails/runtime.js|g' {} \;
        echo "    bindings generated: $(find assets/src/bindings -name '*.ts' | wc -l) files"
    else
        echo "    WARNING: no bindings generated (check wails3 output above)"
    fi
    rm -rf "$TEMP_BINDINGS"

    # Step 2: 前端构建 (Vite 会把 bindings 内联到 dist/index.html)
    echo "--- Frontend ---"
    cd assets
    pnpm install
    pnpm run build
    cd ..

    # Step 3: 同步到 Go embed 路径
    cp assets/dist/index.html neo-src/cmd/server/assets/index.html

    # 确保 runtime.js 注入存在 (幂等: 只加一次)
    if ! grep -q 'src="/wails/runtime.js"' neo-src/cmd/server/assets/index.html; then
        sed -i 's|</head>|<script type="module" src="/wails/runtime.js"></script></head>|' \
            neo-src/cmd/server/assets/index.html
    fi
fi

# Step 3b: 复制 runtime.js 到 Android assets (Wails 运行时需要)
RUNTIME_JS_SRC="$HOME/.asdf/installs/golang/1.26.3/packages/pkg/mod/github.com/wailsapp/wails/v3@v3.0.0-alpha2.115/internal/assetserver/bundledassets/runtime.js"
mkdir -p android/app/src/main/assets/wails
if [ -f "$RUNTIME_JS_SRC" ]; then
    cp "$RUNTIME_JS_SRC" android/app/src/main/assets/wails/runtime.js
fi

# Step 4: Go → Android .so
NDK_ROOT="${ANDROID_NDK_HOME:-$ANDROID_HOME/ndk/26.3.11579264}"
if [ ! -d "$NDK_ROOT" ]; then
    echo "Error: Android NDK not found at $NDK_ROOT"
    echo "Set ANDROID_NDK_HOME or ANDROID_HOME/ndk/26.3.11579264"
    exit 1
fi

HOST_TAG="$(uname -s | grep -q Darwin && echo darwin-x86_64 || echo linux-x86_64)"
TOOLCHAIN="$NDK_ROOT/toolchains/llvm/prebuilt/$HOST_TAG"

mkdir -p android/app/src/main/jniLibs/arm64-v8a
mkdir -p android/app/src/main/jniLibs/x86_64

echo "--- Go shared lib: arm64-v8a ---"
(cd neo-src && \
CC="$TOOLCHAIN/bin/aarch64-linux-android21-clang" \
CXX="$TOOLCHAIN/bin/aarch64-linux-android21-clang++" \
CGO_ENABLED=1 GOOS=android GOARCH=arm64 \
    go build -buildmode=c-shared -tags android,debug -buildvcs=false \
    -o ../android/app/src/main/jniLibs/arm64-v8a/libwails.so ./cmd/server)

echo "--- Go shared lib: x86_64 ---"
(cd neo-src && \
CC="$TOOLCHAIN/bin/x86_64-linux-android21-clang" \
CXX="$TOOLCHAIN/bin/x86_64-linux-android21-clang++" \
CGO_ENABLED=1 GOOS=android GOARCH=amd64 \
    go build -buildmode=c-shared -tags android,debug -buildvcs=false \
    -o ../android/app/src/main/jniLibs/x86_64/libwails.so ./cmd/server)

# Step 5: Gradle assemble
echo "--- Gradle assembleDebug ---"
cd android
./gradlew assembleDebug
cd ..
mkdir -p bin
cp android/app/build/outputs/apk/debug/app-debug.apk bin/LittleTimer.apk

echo "=== APK: bin/LittleTimer.apk ==="
