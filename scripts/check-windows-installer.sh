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
# 下面这条是「卸载不删用户数据」红线的唯一守卫，所以不能只匹配字面前缀：
# `RMDir "$APPDATA…`（无 /r）、未加引号等变体都要覆盖。
# 判据：非注释行里同时出现「破坏性操作」与「用户数据位置变量」。
# 用户数据（主库与 secret.db）在 `%AppData%\LittleTimer` 的**文件系统**里，
# 不在注册表 —— 所以 `DeleteRegKey HKCU "Software\Little Timer"`（安装目录记录）
# 与 `DeleteRegKey HKCU "${UNINSTKEY}"`（卸载项）都是正当删除，不在此列。
# `$INSTDIR`（= $LOCALAPPDATA\Programs\LittleTimer）与 `$DESKTOP`/`$SMPROGRAMS`
# 的删除同样正当：它们的字面文本不含这些变量名，不会误报。
USERDATA_RE='\$(APPDATA|PROFILE|LOCALAPPDATA|USERPROFILE)'
DESTRUCTIVE_RE='RMDir|RmDir|Delete|DeleteRegKey|RemoveDirectory'
if hits="$(grep -vE '^[[:space:]]*;' "$NSI" | grep -E -- "$DESTRUCTIVE_RE" | grep -E -- "$USERDATA_RE")" && [ -n "$hits" ]; then
    echo "FAIL: .nsi 里有针对用户数据位置的破坏性操作:"
    printf '%s\n' "$hits" | sed 's/^/  /'
    fail=1
fi

# --- 结构项：WebView2 运行时探测必须 64/32 两个注册表视图各查一次 ---
if [ "$(grep -Fc -- 'SetRegView 32' "$NSI" || true)" -lt 1 ]; then
    echo "FAIL: .nsi 未在 32 位注册表视图再查一次 pv（必须两处都为空才补装）"
    fail=1
fi

# --- 结构项：也必须查每用户（HKCU）位置 ---
# 未提权的 bootstrapper 装的是每用户运行时，只看 HKLM 会让已装 runtime 的
# 机器每次安装都白跑一次 bootstrapper。
if ! grep -Eq 'ReadRegStr \$[0-9]+ HKCU "Software\\Microsoft\\EdgeUpdate\\Clients' "$NSI"; then
    echo "FAIL: .nsi 未探测 HKCU（每用户）下的 WebView2 注册位置"
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
