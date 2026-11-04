#!/usr/bin/env bash
# Windows 安装器冒烟测试：静默安装 → 跑 version → 静默卸载 → 断言
#   - 安装目录已消失
#   - %AppData%\LittleTimer 仍在（卸载不得删用户数据）
#
#   scripts/smoke-windows-installer.sh <setup.exe> <version>
#
# 只能在 Windows（Git Bash）上跑。

# Git Bash(MSYS2) 默认会把以 / 开头的参数当成 POSIX 路径改写：
#   /S → C:\Program Files\Git\S
# 安装器收不到静默开关就会弹 GUI 等输入 —— CI 上没有窗口，
# 表现为整个 job 挂到 timeout 才被取消（实测挂满 34 分钟）。
# 关掉参数路径改写，/S 才能原样送到安装器。
export MSYS2_ARG_CONV_EXCL='*'

set -euo pipefail

SETUP="${1:?usage: smoke-windows-installer.sh <setup.exe> <version>}"
VERSION="${2:?usage: smoke-windows-installer.sh <setup.exe> <version>}"

command -v cygpath >/dev/null 2>&1 || {
    echo "error: cygpath not found — this script only runs in Git Bash on Windows" >&2
    exit 1
}

# Git Bash 里 $LOCALAPPDATA / $APPDATA 是带反斜杠的 Windows 路径，先转 POSIX
inst="$(cygpath -u "$LOCALAPPDATA")/Programs/LittleTimer"
appdata="$(cygpath -u "$APPDATA")/LittleTimer"

# 装/卸都可能因为等待输入或补装运行时而无限期挂着 —— 加超时兜底，
# 失败要能立刻报出来，而不是把整个 job 的时间预算耗光。
run_bounded() {
    local secs="$1"; shift
    if command -v timeout >/dev/null 2>&1; then
        timeout "$secs" "$@"
    else
        "$@"
    fi
}

echo "install dir : $inst"
echo "appdata dir : $appdata"

echo "=== smoke: install ($SETUP) ==="
rc=0
run_bounded 600 "$SETUP" /S || rc=$?
if [ "$rc" -ne 0 ]; then
    echo "error: silent install failed or timed out (exit $rc)" >&2
    exit 1
fi

for _ in $(seq 1 60); do
    if [ -f "$inst/LittleTimer.exe" ]; then break; fi
    sleep 1
done
if [ ! -f "$inst/LittleTimer.exe" ]; then
    echo "error: $inst/LittleTimer.exe did not appear within 60s" >&2
    exit 1
fi

echo "=== smoke: version ==="
out="$(run_bounded 60 "$inst/LittleTimer.exe" version)"
echo "$out"
# `version` 的第一行固定是 "little-timer <ver>"。整行精确匹配 —— 子串匹配的话
# "2.1.1" 会被 "12.1.1" 误判为通过。
if ! printf '%s\n' "$out" | grep -Fxq -- "little-timer $VERSION"; then
    echo "error: version output does not contain 'little-timer $VERSION'" >&2
    exit 1
fi

echo "=== smoke: uninstall ==="
rc=0
run_bounded 600 "$inst/Uninstall.exe" /S || rc=$?
if [ "$rc" -ne 0 ]; then
    echo "error: silent uninstall failed or timed out (exit $rc)" >&2
    exit 1
fi

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
