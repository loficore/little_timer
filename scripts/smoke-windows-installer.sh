#!/usr/bin/env bash
# Windows 安装器冒烟测试：静默安装 → 跑 version → 静默卸载 → 断言
#   - 安装目录已消失
#   - %AppData%\LittleTimer 仍在（卸载不得删用户数据）
#
#   scripts/smoke-windows-installer.sh <setup.exe> <version>
#
# 只能在 Windows（Git Bash）上跑。
set -euo pipefail

SETUP="${1:?usage: smoke-windows-installer.sh <setup.exe> <version>}"
VERSION="${2:?usage: smoke-windows-installer.sh <setup.exe> <version>}"

# Git Bash 里 $LOCALAPPDATA / $APPDATA 是带反斜杠的 Windows 路径，先转 POSIX
inst="$(cygpath -u "$LOCALAPPDATA")/Programs/LittleTimer"
appdata="$(cygpath -u "$APPDATA")/LittleTimer"

echo "=== smoke: install ($SETUP) ==="
"$SETUP" /S

for _ in $(seq 1 60); do
    if [ -f "$inst/LittleTimer.exe" ]; then break; fi
    sleep 1
done
if [ ! -f "$inst/LittleTimer.exe" ]; then
    echo "error: $inst/LittleTimer.exe did not appear within 60s" >&2
    exit 1
fi

echo "=== smoke: version ==="
out="$("$inst/LittleTimer.exe" version)"
echo "$out"
if ! printf '%s\n' "$out" | grep -q -- "$VERSION"; then
    echo "error: version output does not contain '$VERSION'" >&2
    exit 1
fi

echo "=== smoke: uninstall ==="
"$inst/Uninstall.exe" /S

for _ in $(seq 1 60); do
    if [ ! -f "$inst/LittleTimer.exe" ]; then break; fi
    sleep 1
done
if [ -f "$inst/LittleTimer.exe" ]; then
    echo "error: $inst/LittleTimer.exe still exists after uninstall" >&2
    exit 1
fi

# 第 4 步跑 version 时 defaultDBPath() 会 MkdirAll 出该目录；
# 卸载器若把它一起删掉，主库与 secret.db 就没了。
echo "=== smoke: user data must survive ==="
if [ ! -d "$appdata" ]; then
    echo "error: uninstaller deleted user data: $appdata" >&2
    exit 1
fi

echo "✅ smoke passed"
