#!/usr/bin/env bash
# Windows 产物打包：前端 → Go → NSIS 安装器 → 便携 zip。
# 自足：调用方只需给版本号，不用先构建前端。
#
#   scripts/package-windows.sh <version>
#
# 产出：
#   dist/little-timer-<version>-windows-x86_64-setup.exe
#   dist/little-timer-<version>-windows-x86_64-portable.zip
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

VERSION="${1:?usage: package-windows.sh <version>}"

die() {
    echo "error: $*" >&2
    exit 1
}

# NSIS 的 VIProductVersion 要求严格的 X.X.X.X，而版本号可能带非数字后缀
# （例如临时验收 tag v0.0.0ci）。取数字前缀补齐成四段，其余部分丢弃。
file_version() {
    local v="${1%%[!0-9.]*}" p i
    local -a parts
    IFS='.' read -r -a parts <<<"$v"
    local -a out=()
    for i in 0 1 2 3; do
        p="${parts[i]-0}"
        p="${p//[!0-9]/}"
        out+=("${p:-0}")
    done
    printf '%s.%s.%s.%s' "${out[0]}" "${out[1]}" "${out[2]}" "${out[3]}"
}

FILEVER="$(file_version "$VERSION")"

command -v pnpm >/dev/null 2>&1 || die "pnpm not found"
command -v makensis >/dev/null 2>&1 || die "makensis not found (choco install nsis)"

echo "=== packaging windows: version=$VERSION filever=$FILEVER ==="

# 1) 前端构建
cd "$ROOT/assets"
pnpm install
pnpm run build
cd "$ROOT"

# 2) 同步到 Go embed 路径
bash "$ROOT/scripts/stage-embedded-ui.sh"

# 3) Go 构建（cgo + mingw；webview + embed_ui）
mkdir -p "$ROOT/packaging/windows/build"
cd "$ROOT/neo-src"
CGO_ENABLED=1 go build -tags "webview,embed_ui" \
    -ldflags "-s -w -X little-timer/internal/app.Version=$VERSION" \
    -o "$ROOT/packaging/windows/build/LittleTimer.exe" ./cmd/server
cd "$ROOT"

# 4) 许可证 —— MUI 许可证页在**编译期**读它，必须在 makensis 之前就位
cp "$ROOT/LICENSE" "$ROOT/packaging/windows/LICENSE.txt"

# 5) WebView2 evergreen bootstrapper（运行时缺失时由安装器静默补装）
curl -fsSL "https://go.microsoft.com/fwlink/p/?LinkId=2124703" \
    -o "$ROOT/packaging/windows/build/MicrosoftEdgeWebview2Setup.exe"

# 6) NSIS 打包 —— 必须在 packaging/windows/ 内执行，
#    否则 File "build\..." 与 !define MUI_ICON 的相对路径解析不到
cd "$ROOT/packaging/windows"
makensis -DVERSION="$VERSION" -DFILEVER="$FILEVER" \
    -X"OutFile LittleTimer-setup.exe" little-timer.nsi
cd "$ROOT"

# 7) 安装器改名入库
mkdir -p "$ROOT/dist"
mv "$ROOT/packaging/windows/LittleTimer-setup.exe" \
    "$ROOT/dist/little-timer-$VERSION-windows-x86_64-setup.exe"

# 8) 便携包（单文件 zip，省略安装步骤）
rm -f "$ROOT/dist/little-timer-$VERSION-windows-x86_64-portable.zip"
powershell -NoProfile -Command \
    "Compress-Archive -Path 'packaging/windows/build/LittleTimer.exe' -DestinationPath 'dist/little-timer-$VERSION-windows-x86_64-portable.zip'"

# 9) 清理暂存
rm -rf "$ROOT/packaging/windows/build" "$ROOT/packaging/windows/LICENSE.txt"

echo "✅ $ROOT/dist/little-timer-$VERSION-windows-x86_64-setup.exe"
echo "✅ $ROOT/dist/little-timer-$VERSION-windows-x86_64-portable.zip"
