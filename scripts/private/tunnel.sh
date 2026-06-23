#!/usr/bin/env bash
set -eu

# 顺便读取 Justfile 里定义的 ssh_host（这里直接用你配置的 VPS-RackNerd）
SSH_HOST="VPS-RackNerd"

echo "正在建立 SSH 端口隧道 (转发 5173 和 8080 到本地)..."

# 检查是否已经存在相同的隧道，防止重复启动
if pkill -0 -f "ssh -fN -L 5173" 2>/dev/null; then
    echo "隧道已在运行中。"
else
    # 现在改为直接穿透到容器（在 VPS 视角下，127.0.0.1 依然可以，但前提是你 container 映射了端口；如果没有映射，可以用下面的方式）
    ssh -fN -L 5173:127.0.0.1:5173 -L 8080:127.0.0.1:8080 "${SSH_HOST}"
    echo "SSH 隧道建立成功！本地可通过 localhost:5173 和 localhost:8080 访问。"
fi