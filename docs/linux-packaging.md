# Linux 打包与 WebKit 依赖变体

定义 little_timer 在 Linux 上的发布产物格式（deb / rpm）、WebKitGTK API 变体划分、构建矩阵与依赖校验方式。面向维护者，是 `docs/development.md` 发布章节的展开，CI 改动实施前的决策依据。

**一眼区**：

- 本轮只做两条通道：**deb → webkit2gtk-4.1**、**rpm → webkit2gtk-4.0（EL9）**。`2 构建目标 × 2 格式 ≈ 4 产物` 是长期形态，本轮不排。
- 现有 `apk` / `exe` 不受影响：Android 用系统 WebView，Windows 用 WebView2，都不经 GTK。
- 本地链路已落地：`just package-linux` 产出 rpm（AlmaLinux 9 容器）与 deb（Ubuntu 22.04 容器），产物在 `dist/`；`release.yml` 已把两种包与 tar.gz 一起挂到 GitHub Release。
- **D5 已定：前端 embed 进二进制**，`-tags "webview,embed_ui"` 的包与 `just package` 的 tar.gz 都自带界面。
- 硬约束：`webkit2gtk-4.0` 与 `webkit2gtk-4.1` 是**不同 SONAME**，不存在「一个二进制通吃」。
- 静态链接不可行，已排除（理由见「为什么分变体」末节）。

## 为什么必须分变体

上游 `github.com/webview/webview_go` 的 cgo 指令写死了 pkg-config 模块名：

```
#cgo linux openbsd freebsd netbsd pkg-config: gtk+-3.0 webkit2gtk-4.0
```

本仓库在 `neo-src/third_party/webview_go` 的 fork 里把它拆成默认 4.0 与
`webkit41` tag 的 4.1 两个文件（见 D3），所以同一套 Go 代码可以按需链接不同
API 代，而不必依赖构建镜像里恰好存在的 `.pc` 名称。

而各发行版保留的 API 代不同。运行时 SONAME 按 API 代划分，互不兼容：

| API 模块 | 运行时 SONAME | GTK / soup |
| --- | --- | --- |
| `webkit2gtk-4.0` | `libwebkit2gtk-4.0.so.37` | gtk3 + libsoup2.4 |
| `webkit2gtk-4.1` | `libwebkit2gtk-4.1.so.0`（待 Fedora 机器上线复核） | gtk3 + libsoup3 |
| `webkitgtk-6.0` | `libwebkitgtk-6.0.so.4` | gtk4 + libsoup3 |

发行版可用性（2026-10-29 对各仓库元数据的查询结果）：

| 目标 | 4.0 | 4.1 | 6.0 |
| --- | --- | --- | --- |
| Ubuntu 22.04 jammy | ✅ main | ✅ universe | ✅ updates |
| Ubuntu 24.04 noble / 26.04 resolute | ❌ 仅 `-doc` | ✅（noble 在 universe） | ✅ |
| Debian 12 bookworm | ✅ | ✅ | ✅ |
| Debian 13 trixie | ❌ 运行时装不到 | ✅ | ✅ |
| CentOS 8 / RHEL 8 | ✅ `webkit2gtk3-devel` | ❌ | ❌ |
| AlmaLinux / Rocky 9 | ✅ `webkit2gtk3-devel` 2.54.1 | ❌ 连运行时都没有 | ❌ |
| Fedora 43+ | ❌ 已删（F42 是最后一版） | ✅ | ✅ |

三条推论：

1. deb 只需一个变体：jammy 与 bookworm 同时有 4.0 和 4.1，所以链 4.1 的 deb 覆盖 Ubuntu ≥ 22.04 + Debian ≥ 12。
2. rpm 必须两变体：EL8/EL9 只有 4.0，Fedora 43+ 只有 4.1。本轮先出 EL9 这一条。
3. 6.0 档暂时给不出：vendor 进模块的 `webview.h` 是 0.11 时代，编译期支持 4.1、不支持 6.0；6.0 需要 webview 0.12+ 的 `WEBVIEW_WEBKITGTK_API` 或改走 gotk4。包命名空间不预留 `-webkit6`。

**glibc 方向性规则**：旧环境构建的二进制可在新环境运行，反之不行。因此构建镜像即最低门槛——rpm 在 EL9（glibc 2.34）编，deb 想覆盖 jammy 就在 22.04（2.35）编。包构建统一用 `gcc`，不走 `justfile` 里的 `zig cc` 兜底：`zig cc` 只压低 libc 符号下限，pkg-config 解析的仍是宿主机 `.pc` 与头文件，反而让门槛来源变模糊。

**静态链接为何不行**：WebKitGTK 上游只发 `.so`，且 `WebKitNetworkProcess` / `WebKitWebProcess` 两个 helper 可执行文件位于 libexec，缺失即崩；`.so` 内编译死了 `/usr/...` 绝对路径；gdk-pixbuf 加载器、IM 模块、GSettings/dconf 后端、GStreamer 插件、libsoup TLS 模块全部运行时 `dlopen`。`-linkmode=external` + 静态 libc / musl 也救不了，因为被 `dlopen` 的厂商 `.so` 仍要目标机器提供。自带 webkit 运行时的 AppImage 路线（Tauri 侧的 linuxdeploy GTK 插件 PR 至今未合）维护成本高且解决不了 glibc 下限，不采用。

## 构建矩阵

| 产物 | 构建镜像 | 链接目标 | 需安装的包 |
| --- | --- | --- | --- |
| deb | `ubuntu:22.04`（glibc 2.35，覆盖 jammy + noble） | `libwebkit2gtk-4.1.so.0` | `build-essential` `pkg-config` `libgtk-3-dev` `libwebkit2gtk-4.1-dev` `libsqlite3-dev` `dpkg-dev` |
| rpm | `almalinux:9` | `libwebkit2gtk-4.0.so.37` | `gtk3-devel` `webkit2gtk3-devel` `sqlite-devel` `pkg-config` `rpm-build` `redhat-rpm-config`（均在 AppStream，已在本机核实） |

WebKit 版本由 build tag 决定（D3）：默认 `-tags "webview,embed_ui"` 链 4.0，deb 用 `-tags "webview,embed_ui,webkit41"` 链 4.1。tag 由本地 fork `neo-src/third_party/webview_go` 的两个 cgo 文件提供，构建时不再复制或覆盖 `.pc` 文件；链接目标因此由命令行显式选择，而不是取决于镜像里恰好存在哪个 `.pc`。

本地入口是 `scripts/package-linux.sh {rpm|deb|all} [version]`（`just package-rpm` / `package-deb` / `package-linux`）。宿主机缺少对应开发包时自动进入上表容器；版本默认取最近 git tag（去掉 `v` 前缀），无 tag 时用当天日期。容器内不读 `proxy.golang.org`：脚本挂载宿主机的 `GOROOT` 与 `GOMODCACHE`，并把模块缓存以 `GOPROXY=file:///host-gomod/cache/download` 暴露给容器。

脚本内的构建步骤：

1. `pnpm run build` + `scripts/stage-embedded-ui.sh`，把 `assets/dist/index.html` 与 `fonts/` 同步到 `neo-src/cmd/server/assets/`；
2. `go build -tags "webview,embed_ui"`（rpm）或 `-tags "webview,embed_ui,webkit41"`（deb），`-ldflags "-s -w -X little-timer/internal/app.Version=<ver>"`；
3. `readelf -d` 断言 `DT_NEEDED` 指向本变体的 webkit SONAME；
4. rpm 交给 rpmbuild 自动生成 `Requires`，deb 用 `dpkg-shlibdeps` 生成 `Depends`，两者再断言包含对应的 `libwebkit2gtk-4.x`。

## 依赖声明方式

包依赖必须由二进制的 `DT_NEEDED` 推导，不能靠手写清单和构建环境各自记忆——否则「包名叫 webkit40、实际链 4.1」迟早发生。选型时优先考虑能自动生成 shlib 依赖的工具链（rpmspec + 发行版依赖生成器、debhelper + `dpkg-shlibdeps`）；若统一改用 `nfpm` 出两格式，其 `depends:` 需人工声明（实施时验证该行为），则必须补一道一致性门禁：

```bash
# 从二进制提真实链接目标
readelf -d usr/bin/little-timer | awk '/NEEDED/ {print $NF}' | tr -d '"' \
  | grep -E '^lib(webkit2gtk|javascriptcoregtk)'

# 与包元数据比对
rpm -qp --requires <pkg>.rpm | grep -i webkit
dpkg-deb -f <pkg>.deb depends
```

CI 断言两者指向同一 API 代，不一致即 fail。

## 命名与互斥

- rpm：`Name: little-timer`，`Release: 1webkit40%{?dist}`，产物 `little-timer-<ver>-1webkit40.el9.x86_64.rpm`。将来 Fedora 变体是同 `Name`、不同 dist tag（`.fc44` + `1webkit41`），由 `dnf` 按版本替换而非并存；再加 `Provides: little-timer(webkit40)` 供脚本筛选。
- deb：包名 `little-timer`，`Version: <ver>-1webkit41`，产物 `little-timer_<ver>-1webkit41_amd64.deb`，`Provides: little-timer-webkit41`。
- tar.gz 通道保持现状（`-tags embed_ui`，不含 webview），作为「两个 webkit 都没有 / 只想跑 headless」时的兜底。包与 tar 包不共享安装路径，不做跨格式 `Conflicts`；但 `/usr/local/bin` 下的手工副本不受包管理器管辖，需在发布说明里提醒。

## 包内容与桌面入口

Linux 上 `defaultHTTPOnly()` 返回 true（`neo-src/internal/cli/args.go`），即不带 `--webview` 时只起 HTTP 服务。按 D6，默认桌面入口 `little-timer-desktop` 起 HTTP 服务后用 `xdg-open` 打开浏览器；webview 只放在 Desktop Action `Webview`（`Exec=little-timer serve --webview`），不默认起窗。

其余约定：

- 数据目录走 `os.UserConfigDir()`，即 `~/.config/little_timer/little_timer.db`；程序是用户级桌面应用，包内**不含** systemd 系统单元、不含提权配置。
- 默认端口 `8080`（`cli.DefaultPort`），dev 期的 8013 与包无关。

### 前端资源的提供方式（D5 已定：embed）

选定 embed 进二进制，实现分布如下：

- `cmd/server/ui_embed.go`（`//go:build embed_ui && !android`）用 `//go:embed all:assets` 把 `neo-src/cmd/server/assets/` 编进二进制；`ui_noembed.go` 在其它构建返回 `nil`，保持原占位页。
- `scripts/stage-embedded-ui.sh` 在 build 前把 `assets/dist/index.html` 与 `fonts/` 同步到该目录；i18n 已由 Vite 内联，脚本只在源目录存在时拷贝 `i18n/`，并清理上一轮残留。
- `internal/http/ui.go` 的 `registerRoot` 负责静态文件与 SPA fallback：命中实际文件就 `http.ServeFileFS`，否则回落到 `index.html`；`/api/*` 与 `/wails/*` 不参与 fallback。
- `/wails/runtime.js` 由同一个 handler 返回桩模块（`Call.ByID` 抛错，`CancellablePromise` / `Create` 为空实现），解决非 Wails 运行时下裸 import 白屏的问题。
- `neo-src/cmd/server/assets/README.md` 保证干净 clone 里 `//go:embed all:assets` 有目录可嵌；`index.html`、`fonts/`、`i18n/` 仍 gitignore。

代价是前端每次改动都要重编 Go，换来单二进制、tar.gz 自洽。当前产物仍是 4.27 MB 的单 HTML（gzip 2.51 MB，字体外置为 `fonts/*.woff2`），包体主要在二进制上。

### 密钥服务依赖

Linux 凭据后端走 Secret Service（D-Bus），且**启动时检测不到密钥服务会直接报错退出**，不做静默降级（口径见 `README.md` 的「凭据安全」段）。因此包必须把 provider 写进依赖元数据，否则会「装得上、跑不起」：

- deb：`Recommends: gnome-keyring`（需要更硬可进 `Depends`）；
- rpm：`Recommends: gnome-keyring`，KDE 环境接受 KWallet 提供的实现。

包的 `postinst` / `%post` 不启动任何 daemon，也不安装 keyring 之外的组件。

### 图标

全仓没有 SVG 源（已核实），可复用的是 `android/app/src/main/res/mipmap-*/ic_launcher.png`，按尺寸铺到 hicolor 各档；`ic_launcher_round.png` 不必进包。

## 待决策项

- **D1 二进制命名**：已定包内 `/usr/bin/little-timer`；桌面入口另装 `/usr/bin/little-timer-desktop` 包装器，`StartupWMClass` 仍指向 `little-timer`。
- **D2 图标源**：是否新增 SVG（视觉规范见 `docs/visual-design-spec.md`，图标属资产改动）。本轮沿用 Android `ic_launcher.png`，未新增 SVG。
- **D3 webkit API 选择机制**：**已落地：fork 补丁 + build tag**。上游 `webview_go` 自 2024 年后基本停维护（issue #50、PR #62 未合），仓库内 `neo-src/third_party/webview_go` 是提交 `6173450` 的本地 fork，只把 Linux/BSD 的 `pkg-config` 拆成 `webkit2gtk_40.go`（默认 4.0）与 `webkit2gtk_41.go`（`-tags webkit41`），Go API 与头文件不变。`go.mod` 用 `replace` 指向该目录；打包、CI 与 `just webview-*` 都通过 tag 选择变体，`.pc` shim 已删除。后续同步上游时按 fork 内 README 重做这处拆分即可。风险点只剩构建镜像必须装有目标 `.pc`（EL9 4.0、22.04 4.1），由 `pkg-config --exists` 前置检查兜住。
- **D4 发布渠道**：**已定：GitHub Release 附件**。`release.yml` 在 tar.gz 之后运行 `just package-linux`，把 `dist/*.rpm` 与 `dist/*.deb` 一并附加；apt/dnf 仓库（签名密钥与额外发布步骤）不在本轮范围。
- **D5 前端资源提供方式**：**已定：embed 进二进制**，实现见「前端资源的提供方式」节；tar.gz 通道因此自带界面。
- **D6 NVIDIA + Wayland 的支持边界**：这类机器上 webview 窗口会白屏（见「已知问题」4）。三个选项：
  1. `.desktop` 的 `Exec` 里无条件注入 `WEBKIT_DISABLE_DMABUF_RENDERER=1`——能保证不白屏；代价按常理是 WebKit 改用共享内存上传每帧、内容合成落 CPU，但 2026-10-31 C 格实测软件路径的 idle/click 都是 ~16 ms（见「已知问题」7 复测段），这笔代价在本机量不出来，非 NVIDIA 用户则无条件付；
  2. 不加变量，把这条组合写进「已知不支持」，NVIDIA 用户用 HTTP-only 模式（`little-timer serve` 无 `--webview`，浏览器打开 `:8080`）——包最简单，但点图标开窗口白屏的观感很差；
  3. 包内附一个检测脚本/说明，让用户自行按驱动加变量——把决定权交出去，代价是多一份要维护的文档。
  **D6 选定:HTTP-only + 浏览器为默认形态,webview 为可选变体**。判定依据是[《WebKit 桌面形态剖析工具链》](webkit-profiling.md)的硬阈值——webview 交互帧时间 ≤ 同一页面在浏览器里交互帧时间的 2 倍。已测四组交互帧时间 215–382 ms（见「已知问题」7）对浏览器侧的主观流畅（空闲 rAF 32 ms）已是 6 倍以上差距;要推翻该判定,浏览器交互帧时间须落在 107–191 ms,与全部观察矛盾——浏览器侧 `[CAD]` 数字只会让分母更精确,不会翻转结论。（2026-10-31 复测部分推翻了这个论证前提:C 格 webview 交互帧实测 ~16 ms,见「已知问题」7 复测段——选定维持不变,但依据从「帧时间超阈值」改为「dmabuf 开关缺失即崩」的稳定性证据。）
  三种写法对应的包内容变化:选项 1（`Exec` 无条件注入开关）默认起窗、非 NVIDIA 用户无条件付共享内存上传的代价;选项 2（HTTP-only 为默认）`.desktop` 的 `Exec` **不带 `--webview`**——`defaultHTTPOnly()` 在 Linux 本就返回 true,约定见「包内容与桌面入口」节——点图标起服务、浏览器打开,webview 变体随包可用但不默认;选项 3（附检测脚本）与前者的包内容相同,多一份要维护的脚本与文档。NVIDIA + Wayland 白屏组合按选项 2 的口径写进「已知不支持」（「已知问题」6）;「启动时判定 Wayland + NVIDIA EGL 再决定是否关 dmabuf 路径」属后续增强,不进本轮包。

## 已知问题

1. ~~发布通道给的 tar.gz 没有界面~~（**已解决，D5 embed**）。`just package` 现在先 `pnpm run build` 再走 `scripts/stage-embedded-ui.sh`，构建 `-tags embed_ui` 的 `bin/little-timer` 并打进 tar.gz；打包脚本出 rpm/deb 时额外带 `webview`。
2. **`ci.yml` 的 linux webview 产物在 EL9 上跑不起来**。该 job 用 `-tags webkit41` 显式链 `libwebkit2gtk-4.1.so.0`，EL8/EL9 与 Debian 11 只有 `.so.37`。本地/发布用的 rpm 已走 AlmaLinux 9 容器，这一条只影响 CI 的裸二进制 artifact；接入 `release.yml` 时按格式分构建镜像，不要复用这个 artifact。
3. **`cmd/server/main_android.go:113` 引用了 `application.BundledAssetFileServer(assets)`，而该函数在仓库里找不到定义**（全仓 grep 只命中调用点）。需单独核实：可能是 Android 构建路径未经过该文件（例如走 gomobile bind），也可能是 APK 通道已经断了。与 Linux 打包无关，但既然顺带发现就记在这里。
4. ~~`assets/dist/index.html` 直接给非 Wails 运行时用会白屏~~（**已解决**）。`internal/http/ui.go` 提供 `/wails/runtime.js` 桩模块，`registerRoot` 同时处理静态文件与 SPA fallback；`ui_test.go` / `ui_embed_test.go` 覆盖根路径、SPA 路由、runtime 桩、API 404 与缺 index 的报错路径。
5. **前端产物仍是 4.27 MB 的单个 HTML（gzip 2.51 MB）**。`viteSingleFile` 把 JS/CSS 与 `i18n/*.toml` 内联，字体外置为 `fonts/*.woff2`（3.1 MB）。embed 之后这部分直接计入二进制体积；按平台拆分内联策略仍是可量收益的后续优化，不在本轮范围。
6. **NVIDIA 专有驱动 + Wayland 下 webview 白屏**。开发机 Fedora（KDE Plasma / Wayland / RTX 4060）上 `just go-dev-webview` 起窗后页面全白，日志里是 `Failed to create GBM buffer of size 800x600: 无效的参数`（EINVAL）。宿主侧该配的都配了——`nvidia_drm.modeset=1`、`nvidia_drm.fbdev=1`、`libnvidia-egl-wayland.so.1` 与 `libnvidia-egl-wayland2.so.1` 在位、`libgbm.so.1` 在位、EGL vendor json 齐全、nouveau 已黑名单——所以这不是配置缺失，是 WebKitGTK 走 dmabuf/GBM 零拷贝时与 NVIDIA 驱动在 modifier 协商上谈不拢，属上游问题。`WEBKIT_DISABLE_DMABUF_RENDERER=1` 可以不白屏（开发环境已用它跑通）。对打包的含义：装了 rpm/deb 的 NVIDIA 用户会撞上同一堵墙，`Exec` 不能无条件注入那个变量，也不能假装这个问题不存在——落地口径见 D6。复测（2026-10-31，同机同组合）：症状从白屏恶化为起窗约 6 s 后 `Gdk-Message: Error 71 (Protocol error) dispatching to Wayland display`，Wayland 连接直接被杀——dmabuf 路径在这台机器上现在是「一点就崩」，不是「只是白屏」；详见 `docs/webkit-profiling.md` 矩阵 B 格。
7. **同一套前端在浏览器里流畅、在 webview 里卡，且与缓冲区分配路径无关**。开发机 Fedora 上的四组对照，`[PERF] Settings.interaction.frame` 全部落在 215–382 ms：dev server、生产产物（`vite preview` 顶替 `:5173`）、`GDK_BACKEND=x11`、以及**去掉** `WEBKIT_DISABLE_DMABUF_RENDERER=1` 的 X11 —— 开关不是这笔开销的来源。同一时间同一 dev server、同一 Go 后端，浏览器（HTTP-only 模式）里主观流畅无卡顿。所以成本出在 WebKitGTK 这一侧（送帧节奏或页面绘制），不是 JS 构建产物、也不是后端网络路径。`apiGetHabitsMs` 在 0 条记录上从 253 → 130 → 125 ms 逐级下降，属冷启动而非固定代理税。拆源工具链已落地（[webkit-profiling.md](webkit-profiling.md) 三层）：空闲 rAF 探针连同装饰层对照都稳定在 32 ms，「WebKit 空闲送帧就这么慢」已排除；剩下「前端渲染侧 vs WebKit 原生提交/绘制」的分岔，由调试机上的 `[CAD]` 持续帧率计（`just webview-cadence`）与 `scripts/collect-webkit-perf.sh` 的 `self-top.txt` 收口。对打包的含义：阈值判定「webview ≥ 2× 浏览器」已由现有四组数字满足，D6 据此选定；`[CAD]`/perf 输出只细化成因表述，不翻转该结论。

**复测（2026-10-31，Fedora_Jiaolong，fc44 / webkit2gtk4.1-2.52.1 / nvidia-615.78.08——三者安装时间都早于 10-29/30 的卡顿数据，换版本假设排除）**：C 格（Wayland + `WEBKIT_DISABLE_DMABUF_RENDERER=1`）`[CAD]` idle n=293，p50 16.1 / p99 16.7 / max 24.7 ms，三条真实 click 窗口 median 全部 16 ms——流畅，与「浏览器主观流畅」同级；B 格（dmabuf 开）起窗 ~6 s 后 Protocol error 71 断连，采不到数（见「已知问题」6）；D 格（X11）webkit cgo 内 SIGSEGV。**215–382 ms 在任何一格都未复现**，成因没有符号化证据，不做定论。对打包的含义修正：原「交互帧超浏览器 2 倍」的判定输入已失效，webview 保持可选变体的依据改为**稳定性**——在 NVIDIA + Wayland 上不开 dmabuf 开关就崩，开了才测得到 16 ms；C 格同时是硬阈值的首个 webview 侧分子（浏览器侧 `[CAD]` 分母仍待补）。数据见 `docs/webkit-profiling.md` Baseline 表与 `perf-out/cadence-20261031-*.log`（不入库）。

## 验收清单（发版前）

本地验证（2026-10-31，产物 `little-timer-2.0.0-1webkit40.el9.x86_64.rpm` / `little-timer_2.0.0-1webkit41_amd64.deb`）：

1. [x] EL9 干净容器内 `rpm -ivh` 成功，`rpm -qp --requires` 含 `libwebkit2gtk-4.0.so.37`，`little-timer serve --webview` 能起窗（Xvfb 下验证窗口创建）。
2. [x] jammy 与 noble 干净容器内 `dpkg -i` 成功，`Depends` 含 `libwebkit2gtk-4.1-0`，起窗正常。
3. [x] 起窗后显示的是真实前端，而不是「Build the frontend …」占位页：`/` 返回 `<title>Little Timer</title>`，SPA 路由回落 `index.html`，`/wails/runtime.js` 返回桩，`/api/*` 仍 404，字体可拉取。
4. [x] 在无任何 webkit 运行时的容器内安装两包均被依赖挡住（rpm 缺 `libwebkit2gtk-4.0.so.37`，deb 缺 `libwebkit2gtk-4.1-0`），不是装完运行时崩。
5. [x] 一致性门禁跑通：`readelf -d` 的 NEEDED 与包元数据同代（rpm 4.0、deb 4.1）；`dpkg-shlibdeps` 生成的 Depends 也含 4.1。
6. [ ] KDE Plasma（Fedora_Jiaolong 环境）下桌面入口图标、`StartupWMClass` 与窗口分类正常。Xvfb 下已确认 `WM_CLASS` 为 `"little-timer", "Little-timer"`、`WM_NAME` 为 `Little Timer`，与 `StartupWMClass=little-timer` 匹配；真实 KDE 桌面待验。
7. [x] 无密钥服务的机器上首启**按预期报错退出并给出启动 daemon 的命令**（exit 1）；装好 `gnome-keyring` 并解锁后，打包二进制能正常提供 UI 与凭据功能。
8. [ ] NVIDIA + Wayland 真机（Fedora_Jiaolong）上确认 D6 口径：默认入口走 HTTP-only + 浏览器；webview 变体在该组合下按「已知不支持」记录，不承诺不白屏。

## 落地顺序

1. [x] 定 D5 并实现 Linux 侧 UI 提供：embed 进二进制，含 runtime 桩与 SPA fallback。
2. [x] 定 D1（`/usr/bin/little-timer`）、D3（fork + build tag）与 D6（HTTP-only 默认、webview 可选）；D2/D4 按上文现状保留。
3. [x] 本地打包链路：`scripts/package-linux.sh` 用 EL9 容器出 rpm（默认 tag，4.0）、Ubuntu 22.04 容器出 deb（`webkit41` tag，4.1）；`just package-rpm` / `package-deb` / `package-linux` 为入口。
4. [x] 依赖一致性门禁：构建脚本内 `readelf` 断言 + `dpkg-shlibdeps` / rpmbuild 自动依赖。待接进 `release.yml`。
5. [x] `release.yml` 已把 deb/rpm 与 tar.gz 一起挂到 GitHub Release；包格式与依赖说明已补进 `README.md` / `docs/development.md`。
