#!/usr/bin/env bash
# Windows 安装器静态门禁：对 packaging/windows/little-timer.nsi 做必备/禁止项断言。
# 本机没有 Windows，装不了也跑不了安装器 —— 结构正确性只能静态卡。
#
# 用法：scripts/check-windows-installer.sh
# 退出码：0 = 全部断言通过；1 = 有失败项（逐条打印）
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
NSI="$ROOT/packaging/windows/little-timer.nsi"

if [ ! -f "$NSI" ]; then
    echo "missing file: $NSI" >&2
    exit 1
fi

fail=0

require() {
    if ! grep -Fq -- "$1" "$NSI"; then
        echo "FAIL: .nsi 缺少必需内容: $1"
        fail=1
    fi
}

forbid() {
    if grep -Fq -- "$1" "$NSI"; then
        echo "FAIL: .nsi 含禁止内容: $1"
        fail=1
    fi
}

# --- 必备项 ---
require 'RequestExecutionLevel user'
require 'InstallDir "$LOCALAPPDATA\Programs\LittleTimer"'
require 'SetShellVarContext current'
require 'SetRegView 64'
require 'WriteUninstaller'
require '{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}'
require 'MUI_UNPAGE_CONFIRM'
require 'LittleTimer.exe'

# --- 禁止项 ---
forbid 'RequestExecutionLevel admin'
forbid 'RMDir /r "$APPDATA'
forbid 'RMDir /r "$PROFILE'
forbid 'Delete "$APPDATA'

# --- 结构项：WebView2 运行时探测必须 64/32 两个注册表视图各查一次 ---
if [ "$(grep -Fc -- 'SetRegView 32' "$NSI" || true)" -lt 1 ]; then
    echo "FAIL: .nsi 未在 32 位注册表视图再查一次 pv（必须两处都为空才补装）"
    fail=1
fi

# --- 结构项：按用户安装要求 .onInit 与 un.onInit 各设一次 SetShellVarContext ---
n_ctx="$(grep -Fc -- 'SetShellVarContext current' "$NSI" || true)"
if [ "$n_ctx" -lt 2 ]; then
    echo "FAIL: SetShellVarContext current 只出现 ${n_ctx} 次（.onInit 与 un.onInit 各需一次）"
    fail=1
fi

if [ "$fail" -ne 0 ]; then
    exit 1
fi

echo "✅ windows installer static checks passed"
