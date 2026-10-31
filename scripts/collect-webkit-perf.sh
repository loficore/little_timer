#!/usr/bin/env bash
# 采集 WebKitWebProcess + 主进程的原生栈。前提:调试机上已用 just webview-inspect 起窗口。
# 退出码:0 成功(含符号 WARN);1 前置检查失败;2 采不到数据。
set -euo pipefail

usage() {
    cat <<'EOF'
用法: collect-webkit-perf.sh [--seconds N] [--out DIR]
  --seconds N   采集时长(秒),默认 20
  --out DIR     产物目录,默认 perf-out
EOF
}

DUR=20
OUT=perf-out
while [[ $# -gt 0 ]]; do
    case "$1" in
        --seconds)
            DUR="${2:-}"
            [[ "$DUR" =~ ^[0-9]+$ ]] || { usage; exit 1; }
            shift 2
            ;;
        --out)
            OUT="${2:-}"
            [[ -n "$OUT" ]] || { usage; exit 1; }
            shift 2
            ;;
        *)
            usage
            exit 1
            ;;
    esac
done

if ! command -v perf >/dev/null 2>&1; then
    echo "缺 perf:下一步执行 sudo dnf install perf" >&2
    exit 1
fi

paranoid=$(cat /proc/sys/kernel/perf_event_paranoid)
if (( paranoid > 2 )); then
    echo "perf_event_paranoid=$paranoid(>2)采不了:临时放宽用 sudo sysctl kernel.perf_event_paranoid=2(重启即失效,不建议永久改)" >&2
    exit 1
fi

mapfile -t webkit_pids < <(pgrep -x WebKitWebProcess || true)
if (( ${#webkit_pids[@]} == 0 )); then
    echo "没找到 WebKitWebProcess:先在调试机上跑 just webview-inspect 起窗口,再采" >&2
    exit 1
fi

mapfile -t main_pids < <(pgrep -f 'bin/server serve --webview' || true)
if (( ${#main_pids[@]} == 0 )); then
    echo "没找到主进程(bin/server serve --webview):同样先 just webview-inspect 起窗口" >&2
    exit 1
fi

all_pids=( "${webkit_pids[@]}" "${main_pids[@]}" )
pid_list=$(IFS=,; echo "${all_pids[*]}")

mkdir -p "$OUT"
echo "采集 pid=$pid_list,时长=${DUR}s —— 请在窗口里连点 tab"
if ! perf record -g --call-graph dwarf -p "$pid_list" -o "$OUT/perf.data" -- sleep "$DUR"; then
    echo "perf record 非零退出,采不到数据" >&2
    exit 2
fi

perf report --stdio --max-stack=24 -i "$OUT/perf.data" > "$OUT/report.txt"
head -40 < <(perf report --stdio -g none --percent-limit 1 -i "$OUT/perf.data") > "$OUT/self-top.txt"

samples=$(grep -m1 -oE 'Samples: [0-9]+' "$OUT/report.txt" | awk '{print $2}' || true)
if [[ -z "$samples" || "$samples" -eq 0 ]]; then
    echo "report.txt 里 Samples 为 0,采不到数据" >&2
    exit 2
fi

nonempty=$(grep -c -v '^[[:space:]]*$' "$OUT/report.txt" || true)
unknown=$(grep -c '<unknown>' "$OUT/report.txt" || true)
if (( nonempty > 0 )) && (( unknown * 100 / nonempty > 30 )); then
    echo "WARN: 未符号化比例 $(( unknown * 100 / nonempty ))%,勿写入 baseline;装调试符号: sudo dnf debuginfo-install webkit2gtk4.1 gtk3" >&2
fi

echo "产物: $OUT/perf.data $OUT/report.txt $OUT/self-top.txt"
