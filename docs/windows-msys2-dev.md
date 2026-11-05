# Windows / msys2 本地开发

面向**在 Windows 上开发 Little Timer** 的人。目标：在 msys2 里能编、能跑、能调，
HTTP-only 与原生 WebView 两种形态都可用。

## 本文的位置

- Windows **发行产物**（NSIS 安装器 / 便携包）由 CI 的 `windows-installer` job 产出，
  脚本是 `scripts/package-windows.sh`。那是**打包**，不是开发环境。
- `docs/development.md` 的「Windows 开发机运行指引」只覆盖 DPAPI 凭据与 CI 测试，
  不含任何本地开发流程。
- 本文补上「Windows 上从源码开发」这条路径。

## 前置

- **msys2**（装在哪都行，脚本里按实际路径改）
- 工具链，三选一：

  | 方案 | 怎么装 | 备注 |
  | --- | --- | --- |
  | msys2 自带 mingw-w64（推荐） | `pacman -S mingw-w64-ucrt-x86_64-gcc` | 无需任何额外 flag，本文其余部分按它写 |
  | mise 管理的 zig | `mise use zig@0.16.0` | 需要额外两组环境变量，见下 |
  | 其它 gcc 风格工具链 | — | 需自带 `gcc` + `g++` |

- Go 1.25+、Node 20+、pnpm

## cgo 环境变量

`go-sqlite3` 是 cgo-only 驱动 —— **没有 C 编译器就编不出来**。WebView 形态还额外
需要 **C++ 编译器**：`webview_go` 里含 `webview.cc`。

| 变量 | msys2 mingw | zig |
| --- | --- | --- |
| `CGO_ENABLED` | `1` | `1` |
| `CC` | `gcc` | `zig cc` |
| `CXX` | `g++`（默认即可） | **`zig c++`** |

用 zig 时 `CXX` 必须显式给：cgo 的 C++ 编译器默认是 `g++`，不设就会报
`exec: "g++": executable file not found in %PATH%`。

### zig 的第二个坑：UBSan

zig 的 clang 默认开 UBSan（zig 的 Debug 安全默认），会生成
`__ubsan_handle_*` 调用；**Go 的链接器不提供 ubsan runtime**，于是链接期报
`undefined symbol: __ubsan_handle_add_overflow` 之类。关掉即可：

```sh
export CGO_CFLAGS="-fno-sanitize=undefined"
export CGO_CXXFLAGS="-fno-sanitize=undefined"
```

用 msys2 的 mingw gcc 的话这条不适用（gcc 默认不开）。

## EventToken.h：不需要装 Windows SDK

`WebView2.h` 里写着 `#include "EventToken.h"`，而那个头**只随 Windows SDK 发布**，
mingw-w64 各发行版都不带。缺了它，编译会停在：

```
fatal error: 'EventToken.h' file not found
```

仓库已经 vendor 了一份兼容头：
`neo-src/third_party/webview_go/libs/mswebview2/include/EventToken.h`
（取自上游 `webview/webview` 的 `compatibility/mingw/include/`，同为 MIT）。
**所以 msys2 不需要安装 Windows SDK。**

CI 的 `windows-msys2` job 专门守这一条：它只在 msys2 + mingw-w64 里编译，
一旦有人删掉这个兼容头，job 立刻红。

## 缓存放哪

Go 的 module / build 缓存默认落 `%USERPROFILE%`（C 盘）；pnpm 的 store 也默认在
C 盘。C 盘紧张就指到别处：

```sh
export GOPATH="D:/dev/go"
export GOMODCACHE="D:/dev/go/pkg/mod"
export GOCACHE="D:/dev/go-build"
export GOTMPDIR="D:/dev/tmp"
export TMPDIR=/d/dev/tmp
export TEMP='D:\dev\tmp'
export TMP='D:\dev\tmp'
```

pnpm 的 store 单独指：

```sh
pnpm install --store-dir D:/dev/pnpm-store
```

## 两种运行形态

两种形态**抢同一个 5173 / 8013**，同一时刻只能跑一个。

### A. HTTP-only（后端 + 浏览器）

```sh
cd assets && pnpm install && pnpm run dev          # vite dev server :5173
cd neo-src && go build -o bin/server.exe ./cmd/server
./bin/server.exe serve --http-only --port 8013     # Go API :8013
```

浏览器开 `http://localhost:5173`；`/api` 由 vite 代理到 8013
（`assets/vite.config.ts`，默认 target 就是 8013）。

后端**不需要** `-tags webview`，也不需要 WebView2 运行时。

### B. 原生 WebView（WebView2）

```sh
cd neo-src && go build -tags webview -o bin/webview.exe ./cmd/server
./bin/webview.exe serve --webview --port 8013
```

窗口 URL 在 dev 模式下**硬编码**为 `http://localhost:5173/?runtime=webview`
（`internal/webview/url_dev.go`）—— 所以**必须同时开着 vite**，否则窗口是白的。

> **会话限制（最容易踩的一条）**
> webview 窗口只在**交互式桌面会话**里可见。经 SSH 起的进程跑在 Session 0
> （服务会话），而 WebView2 需要交互式 window station，会直接以
> `Exception 0xc0000005` 崩掉。**必须在 RDP / 本地桌面的终端里启动**，
> 不能靠 SSH 后台起。

想跑一个不依赖 vite 的独立二进制，用内嵌前端：

```sh
go build -tags "webview,embed_ui" -o bin/little-timer.exe ./cmd/server
```

（先在 `assets/` 跑 `pnpm run build` + `scripts/stage-embedded-ui.sh`，
和 `scripts/package-windows.sh` 走同一条路。）

## 启动脚本

下面这套是实际在 msys2 上跑通的形态，可直接抄。`ROOT` 按实际路径改。

### `env.sh` —— 工具链接线

```sh
# 工具链由 mise 提供（与交互式 zsh 里 `eval "$(mise activate zsh)"` 同一套）。
# SSH 的非交互 shell 不读 .zshrc，所以这里显式接线。
export PATH="/usr/local/bin/mise/bin:$PATH"
eval "$(mise activate bash)"

# cgo：go-sqlite3 是 cgo-only；C 与 C++ 编译器都交给 zig
export CC="zig cc"
export CXX="zig c++"
export CGO_ENABLED=1
# zig 的 clang 默认开 UBSan，Go 的链接器不提供 ubsan runtime → 关掉
export CGO_CFLAGS="-fno-sanitize=undefined"
# WebView2.h 需要 EventToken.h；仓库已 vendor，无需 Windows SDK
export CGO_CXXFLAGS="-fno-sanitize=undefined"

# 缓存全部钉到 D:（C 盘紧张时）
export GOPATH="D:/dev/go"
export GOMODCACHE="D:/dev/go/pkg/mod"
export GOCACHE="D:/dev/go-build"
export GOTMPDIR="D:/dev/tmp"
export TMPDIR=/d/dev/tmp
export TEMP='D:\dev\tmp'
export TMP='D:\dev\tmp'
export GOPROXY="https://goproxy.cn,direct"
export GOSUMDB="sum.golang.google.cn"

# 后端端口，与 vite proxy 默认 target 对齐
export BACKEND_PORT=8013
```

用 msys2 自带 mingw gcc 的话，把 `CC` / `CXX` / `CGO_CFLAGS` / `CGO_CXXFLAGS`
四行换成（或直接删掉）：

```sh
export CC="gcc"
export CXX="g++"
```

### `run-http.sh` —— HTTP-only

```sh
#!/usr/bin/env bash
# HTTP-only：vite dev server(:5173) + Go 后端(:8013)，Ctrl+C 一起停
set -u
source "$(dirname "$0")/env.sh"
ROOT=/d/dev/little_timer
cleanup() { kill "$VITE_PID" "$GO_PID" 2>/dev/null; }
trap cleanup INT TERM EXIT

( cd "$ROOT/assets"  && exec pnpm run dev -- --strictPort ) & VITE_PID=$!
( cd "$ROOT/neo-src" && exec ./bin/server.exe serve --http-only --port "$BACKEND_PORT" ) & GO_PID=$!

echo "前端: http://localhost:5173"
echo "API : http://localhost:$BACKEND_PORT"
wait
```

### `run-webview.sh` —— 原生 WebView

```sh
#!/usr/bin/env bash
# WebView：先起 vite(:5173)，再起原生窗口（Windows 走 WebView2）
# 注意：窗口只出现在交互桌面会话里；SSH(Session 0) 起会 0xc0000005 崩掉。
set -u
source "$(dirname "$0")/env.sh"
ROOT=/d/dev/little_timer
cleanup() { kill "$VITE_PID" "$GO_PID" 2>/dev/null; }
trap cleanup INT TERM EXIT

( cd "$ROOT/assets" && exec pnpm run dev -- --strictPort ) & VITE_PID=$!
sleep 3
( cd "$ROOT/neo-src" && exec ./bin/webview.exe serve --webview --port "$BACKEND_PORT" ) & GO_PID=$!
wait
```

### 桌面上双击启动

```bat
@echo off
"D:\msys64\usr\bin\bash.exe" -l -c "exec /d/dev/little-timer-win/run-webview.sh"
pause
```

用 `.cmd` 而不是 SSH，是因为会话限制（见上）。`pause` 让窗口不立刻消失，
方便看日志；也可以删掉。

## just 配方在 msys2 上也能用

`just go-dev` / `just go-dev-webview` 的端口探测已改成不依赖 `ss`
（msys2 没有 iproute2），Linux 侧行为不变。前提是本机有 `just`，
且 `justfile` 顶部的 `export CC` 能找到你的 C 编译器。

## CI 门禁

`windows-msys2` job（`.github/workflows/ci.yml`）在真实 msys2 + mingw-w64 里
编译 HTTP-only 与 WebView 两个目标，且**不依赖任何 Windows SDK**。
它是「msys2 能开发」这件事唯一的自动化证据 —— 加新改动后请等它绿。

## 已知问题

- `just go-dev*` 里 vite 的端口：webview 窗口的 URL 硬编码 5173，而 vite 端口
  被占时会自己让到 5174 —— 那样窗口会加载错的东西。用上面的脚本时加了
  `--strictPort`（占端口就直接失败，而不是静默跑歪）。
- `justfile` 的 `export CC` 优先选 `zig cc`；若你的 PATH 上有 zig 而又想用
  mingw gcc，`CC` 需要显式覆盖。
