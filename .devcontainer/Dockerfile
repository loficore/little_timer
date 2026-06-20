FROM ubuntu:22.04

# 1. 系统依赖：只保留 Ubuntu 22.04 官方原生自带依赖
RUN apt-get update && DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \
    ca-certificates curl xz-utils build-essential pkg-config git zsh gnupg \
    libgtk-4-dev libwebkitgtk-6.0-dev \
    libgtk-3-dev libwebkit2gtk-4.1-dev \
    libwebkit2gtk-4.0-dev \
    libsecret-1-dev libglib2.0-dev \
    && apt-get clean && rm -rf /var/lib/apt/lists/*

# 💡 强行将构建环境挂上你的本地 Clash/Xray 代理端口（借用 Docker 宿主机默认网关 172.17.0.1）
# 如果你的 Clash 没开“允许局域网连接（Allow LAN）”，请记得在 Clash 界面勾选一下！
ARG http_proxy=http://172.17.0.1:7897
ARG https_proxy=http://172.17.0.1:7897

# 💡 拆分层 [A]：独立安装 Zig 0.15.2
RUN ARCH=$(uname -m) && \
    if [ "$ARCH" = "x86_64" ]; then ZIG_ARCH="x86_64-linux"; \
    elif [ "$ARCH" = "aarch64" ]; then ZIG_ARCH="aarch64-linux"; \
    else echo "Unsupported arch: $ARCH" && exit 1; fi && \
    curl -fsSL "https://ziglang.org/download/0.15.2/zig-${ZIG_ARCH}-0.15.2.tar.xz" -o /tmp/zig.tar.xz && \
    tar -xf /tmp/zig.tar.xz -C /usr/local/share && \
    ln -s /usr/local/share/zig-${ZIG_ARCH}-0.15.2/zig /usr/local/bin/zig && \
    rm /tmp/zig.tar.xz

# 💡 拆分层 [B]：独立安装 eza
RUN ARCH=$(uname -m) && \
    if [ "$ARCH" = "x86_64" ]; then EZA_ARCH="x86_64-unknown-linux-gnu"; \
    else EZA_ARCH="aarch64-unknown-linux-gnu"; fi && \
    curl -fsSL "https://github.com/eza-community/eza/releases/download/v0.23.4/eza_${EZA_ARCH}.tar.gz" -o /tmp/eza.tar.gz && \
    tar -xf /tmp/eza.tar.gz -C /usr/local/bin ./eza && \
    chmod +x /usr/local/bin/eza && \
    rm /tmp/eza.tar.gz

# 💡 拆分层 [C]：独立安装 fastfetch 2.64.2
RUN ARCH=$(uname -m) && \
    if [ "$ARCH" = "x86_64" ]; then FF_ARCH="linux-amd64"; \
    else FF_ARCH="linux-aarch64"; fi && \
    curl -fsSL "https://github.com/fastfetch-cli/fastfetch/releases/download/2.64.2/fastfetch-${FF_ARCH}.tar.gz" -o /tmp/ff.tar.gz && \
    tar -xf /tmp/ff.tar.gz -C /usr/local/bin --strip-components=3 "fastfetch-${FF_ARCH}/usr/bin/fastfetch" && \
    chmod +x /usr/local/bin/fastfetch && \
    rm -f /tmp/ff.tar.gz

# 💡 拆分层 [D]：Node.js 22 LTS + pnpm
RUN curl -fsSL https://deb.nodesource.com/setup_22.x | bash - && \
    apt-get install -y nodejs && \
    corepack enable pnpm && \
    apt-get clean && rm -rf /var/lib/apt/lists/*

# 清除后续容器运行时的代理环境变量，防止干扰容器内网络
ARG http_proxy=
ARG https_proxy=

# 3. 创建约定用户并将其默认 Shell 设为 Zsh
RUN groupadd -g 1000 vscode && \
    useradd -m -u 1000 -g vscode -s /bin/zsh vscode

# --- 🎯 Zsh 环境配置 ---
USER vscode
ENV HOME=/home/vscode
WORKDIR /home/vscode

# 4. 自动化安装 Oh My Zsh 及其核心高亮/建议插件（同样吃上面指定的代理）
RUN sh -c "$(curl -fsSL https://raw.githubusercontent.com/ohmyzsh/ohmyzsh/master/tools/install.sh)" "" --unattended && \
    git clone --depth=1 https://github.com/romkatv/powerlevel10k.git ${HOME}/.oh-my-zsh/custom/themes/powerlevel10k && \
    git clone --depth=1 https://github.com/zsh-users/zsh-autosuggestions ${HOME}/.oh-my-zsh/custom/plugins/zsh-autosuggestions && \
    git clone --depth=1 https://github.com/zsh-users/zsh-syntax-highlighting.git ${HOME}/.oh-my-zsh/custom/plugins/zsh-syntax-highlighting && \
    git clone --depth=1 https://github.com/romkatv/zsh-defer.git ${HOME}/.oh-my-zsh/custom/plugins/zsh-defer

# 5. 将裁剪改良后的 .zshrc 写入容器用户家目录
RUN echo 'if [[ -r "${XDG_CACHE_HOME:-$HOME/.cache}/p10k-instant-prompt-${(%):-%n}.zsh" ]]; then \n\
  source "${XDG_CACHE_HOME:-$HOME/.cache}/p10k-instant-prompt-${(%):-%n}.zsh" \n\
fi \n\
\n\
typeset -g POWERLEVEL9K_INSTANT_PROMPT=quiet \n\
export ZSH="$HOME/.oh-my-zsh" \n\
\n\
# --- 路径配置 --- \n\
path_list=( \n\
    "$HOME/.local/bin" \n\
    "/usr/local/bin" \n\
) \n\
for p in $path_list; do \n\
    [[ -d "$p" ]] && export PATH="$p:$PATH" \n\
done \n\
\n\
# --- OMZ 配置 --- \n\
ZSH_THEME="powerlevel10k/powerlevel10k" \n\
plugins=(git z extract zsh-defer zsh-autosuggestions zsh-syntax-highlighting) \n\
source $ZSH/oh-my-zsh.sh \n\
\n\
# --- 容器专属裁剪别名 --- \n\
alias c="clear" \n\
alias sz="source ~/.zshrc" \n\
alias jj="nocorrect jj" \n\
\n\
# eza 别名映射 \n\
alias ls="eza --icons --group-directories-first" \n\
alias la="eza -a --icons --group-directories-first" \n\
alias ll="eza -l --icons --group-directories-first --git" \n\
alias lla="eza -la --icons --group-directories-first --git" \n\
alias lt="eza -T --level=2 --icons --group-directories-first" \n\
\n\
# --- 环境配置 --- \n\
export PNPM_HOME="$HOME/.local/share/pnpm" \n\
[[ -d "$PNPM_HOME" ]] && export PATH="$PNPM_HOME:$PATH" \n\
export PATH="$PNPM_HOME/bin:$PATH" \n\
\n\
# 容器启动终端展示 \n\
fastfetch \n\
' > ${HOME}/.zshrc

# 6. 免交互的 p10k 配置文件
RUN echo 'typeset -g POWERLEVEL9K_LEFT_PROMPT_ELEMENTS=(dir vcs)' > ${HOME}/.p10k.zsh

WORKDIR /workspace