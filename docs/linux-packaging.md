# Linux 打包与 WebKit 依赖变体

定义 little_timer 在 Linux 上的发布产物格式（deb / rpm）、WebKitGTK API 变体划分、构建矩阵与依赖校验方式。面向维护者，是 `docs/development.md` 发布章节的展开，CI 改动实施前的决策依据。

**一眼区**：

- 本轮只做两条通道：**deb → webkit2gtk-4.1**、**rpm → webkit2gtk-4.0（EL9）**。`2 构建目标 × 2 格式 ≈ 4 产物` 是长期形态，本轮不排。
- 现有 `apk` / `exe` 不受影响：Android 用系统 WebView，Windows 用 WebView2，都不经 GTK。
- 当前仓库**尚无** deb/rpm 产出：发布通道只有 `just package` 的 tar.gz 与 `scripts/build-android.sh` 的 APK，deb/rpm 属新建。
- **前置缺口**：Linux 侧没有前端静态服务路径，`GET /` 返回占位文案（`internal/http/router.go:90-95`）。补齐这项之前，桌面包打出去只会显示占位页。
- 硬约束：`webkit2gtk-4.0` 与 `webkit2gtk-4.1` 是**不同 SONAME**，不存在「一个二进制通吃」。
- 静态链接不可行，已排除（理由见「为什么分变体」末节）。

## 为什么必须分变体

`github.com/webview/webview_go` 的 cgo 指令写死了 pkg-config 模块名：

```
#cgo linux openbsd freebsd netbsd pkg-config: gtk+-3.0 webkit2gtk-4.0
```

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
| deb | `ubuntu:24.04`（若要覆盖 22.04 改 `ubuntu:22.04`） | `libwebkit2gtk-4.1.so.0` | `libgtk-3-dev` `libwebkit2gtk-4.1-dev`（universe，需先 enable） `libsqlite3-dev` `pkg-config`，另加 4.0 → 4.1 的 `.pc` shim |
| rpm | `almalinux:9` | `libwebkit2gtk-4.0.so.37` | `gtk3-devel` `webkit2gtk3-devel` `sqlite-devel` `pkg-config`（均在 AppStream，已在本机核实） |

`.pc` shim 沿用 `.github/workflows/ci.yml:347-352` 的既有做法（把 `webkit2gtk-4.1.pc` 复制成 `webkit2gtk-4.0.pc`），不引入新机制。EL9 侧不需要 shim，仓库里就是真 4.0。

构建命令与 CI 的 linux job 一致：

```bash
go build -tags "webview,embed_ui" -o bin/server ./cmd/server   # 在 neo-src 下
```

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

Linux 上 `defaultHTTPOnly()` 返回 true（`neo-src/internal/cli/args.go`），即不带 `--webview` 时只起 HTTP 服务。因此 webview 变体的桌面入口必须在 `Exec` 里显式传 `--webview`，否则点击图标只会静默开一个后台服务。

其余约定：

- 数据目录走 `os.UserConfigDir()`，即 `~/.config/little_timer/little_timer.db`；程序是用户级桌面应用，包内**不含** systemd 系统单元、不含提权配置。
- 默认端口 `8080`（`cli.DefaultPort`），dev 期的 8013 与包无关。

### 前端资源的提供方式（打包的前置条件）

Linux 侧目前没有任何把构建好的前端交到客户端手里的路径：

- `//go:embed all:assets` 只出现在 `cmd/server/main_android.go`，Linux 路径不读 `assets/dist`；
- `embed_ui` 这个 tag 只切换 webview 窗口的目标 URL（`url_dev.go` 指向 Vite `:5173`，`url_embed.go` 指向 `:8080`），本身不 embed 任何文件；
- `:8080` 的根路由由 `registerRoot` 提供，返回一句「Build the frontend …」的占位 HTML。

所以 `-tags "webview,embed_ui"` 编出的 Linux 桌面二进制，窗口里显示的是占位页；`just package` 出的 tar.gz 则是纯 API 服务、没有界面。打包之前要先定 UI 的落地方式（D5），它直接决定包内容：

| 方式 | 包内容 | 代价 |
| --- | --- | --- |
| Go server 内 embed `assets/dist`（把 Android 那条路铺到 Linux） | 只带单二进制 | 前端每次改动都要重编 Go；tar.gz 自洽 |
| server 从磁盘读 `/usr/share/little-timer/dist`（含 `i18n/*.toml`） | 需带资源目录；`just package` 的 stage 目前只拷 `bin/server`，要一并补 | 资源与二进制分离，升级时须保证同版本 |

### 密钥服务依赖

Linux 凭据后端走 Secret Service（D-Bus），且**启动时检测不到密钥服务会直接报错退出**，不做静默降级（口径见 `README.md` 的「凭据安全」段）。因此包必须把 provider 写进依赖元数据，否则会「装得上、跑不起」：

- deb：`Recommends: gnome-keyring`（需要更硬可进 `Depends`）；
- rpm：`Recommends: gnome-keyring`，KDE 环境接受 KWallet 提供的实现。

包的 `postinst` / `%post` 不启动任何 daemon，也不安装 keyring 之外的组件。

### 图标

全仓没有 SVG 源（已核实），可复用的是 `android/app/src/main/res/mipmap-*/ic_launcher.png`，按尺寸铺到 hicolor 各档；`ic_launcher_round.png` 不必进包。

## 待决策项

- **D1 二进制命名**：现产物名 `server`，包内路径按 `usr/bin/little-timer` 更符合 FHS，但会打断现有脚本与用户习惯。
- **D2 图标源**：是否新增 SVG（视觉规范见 `docs/visual-design-spec.md`，图标属资产改动）。
- **D3 webkit API 选择机制**：短期沿用 `.pc` shim（改动最小、与 CI 同源），中期换成 fork 补丁 + 自有 build tag，让链接目标出现在命令行而非取决于镜像里恰好存在哪个 `.pc`。上游 `webview_go` 自 2024 年后基本停维护（issue #50、PR #62 未合），已有 4.1 补丁 fork（`intrinsicgray/webview_go`、`GopeedLab/webview_go`，2026-09 有提交）。风险点：在同时提供 4.0 与 4.1 `.pc` 的镜像里，shim 的 `cp` 会覆盖真 4.0.pc，静默改变链接目标——所以构建镜像必须锁定且门禁必须存在。
- **D4 发布渠道**：本轮挂 GitHub Release 附件（与 tar.gz 并列），或搭 apt/dnf 仓库（需签名密钥与额外发布步骤）。
- **D5 前端资源提供方式**：embed 进二进制，还是 server 从 `/usr/share/little-timer/dist` 读磁盘。优先级最高——它决定包内容，也决定 tar.gz 通道能否自带界面（现状是不带，见「包内容」节）。
- **D6 NVIDIA + Wayland 的支持边界**：这类机器上 webview 窗口会白屏（见「已知问题」4）。三个选项：
  1. `.desktop` 的 `Exec` 里无条件注入 `WEBKIT_DISABLE_DMABUF_RENDERER=1`——能保证不白屏；代价按常理是 WebKit 改用共享内存上传每帧、内容合成落 CPU，但本机实测两条分配路径的交互帧时间同为 224–275 ms（见「已知问题」7），这笔钱到底有多大还没量出来，非 NVIDIA 用户则无条件付；
  2. 不加变量，把这条组合写进「已知不支持」，NVIDIA 用户用 HTTP-only 模式（`little-timer serve` 无 `--webview`，浏览器打开 `:8080`）——包最简单，但点图标开窗口白屏的观感很差；
  3. 包内附一个检测脚本/说明，让用户自行按驱动加变量——把决定权交出去，代价是多一份要维护的文档。
  **D6 选定:HTTP-only + 浏览器为默认形态,webview 为可选变体**。判定依据是[《WebKit 桌面形态剖析工具链》](webkit-profiling.md)的硬阈值——webview 交互帧时间 ≤ 同一页面在浏览器里交互帧时间的 2 倍。已测四组交互帧时间 215–382 ms（见「已知问题」7）对浏览器侧的主观流畅（空闲 rAF 32 ms）已是 6 倍以上差距;要推翻该判定,浏览器交互帧时间须落在 107–191 ms,与全部观察矛盾——浏览器侧 `[CAD]` 数字只会让分母更精确,不会翻转结论。
  三种写法对应的包内容变化:选项 1（`Exec` 无条件注入开关）默认起窗、非 NVIDIA 用户无条件付共享内存上传的代价;选项 2（HTTP-only 为默认）`.desktop` 的 `Exec` **不带 `--webview`**——`defaultHTTPOnly()` 在 Linux 本就返回 true,约定见「包内容与桌面入口」节——点图标起服务、浏览器打开,webview 变体随包可用但不默认;选项 3（附检测脚本）与前者的包内容相同,多一份要维护的脚本与文档。NVIDIA + Wayland 白屏组合按选项 2 的口径写进「已知不支持」（「已知问题」6）;「启动时判定 Wayland + NVIDIA EGL 再决定是否关 dmabuf 路径」属后续增强,不进本轮包。

## 已知问题

1. **发布通道给的 tar.gz 没有界面**。`just package` 只 build `-tags embed_ui`（不含 `webview`）且 stage 里只有 `bin/server`，而 Linux 侧无静态资源服务，用户拿到的是纯 API 服务，需自行跑 Vite dev server 才能看 UI。D5 未定之前，deb/rpm 会继承同一个缺口。
2. **`ci.yml` 的 linux webview 产物在 EL9 上跑不起来**。该 job 在 ubuntu-latest 上经 shim 后链的是 `libwebkit2gtk-4.1.so.0`，EL8/EL9 与 Debian 11 只有 `.so.37`。目前尚未影响已发布产物（`release.yml` 只附 tar.gz），但 webview 二进制一旦发出去就会暴露。修法即本文件的 D3 与依赖门禁。
3. **`cmd/server/main_android.go:113` 引用了 `application.BundledAssetFileServer(assets)`，而该函数在仓库里找不到定义**（全仓 grep 只命中调用点）。需单独核实：可能是 Android 构建路径未经过该文件（例如走 gomobile bind），也可能是 APK 通道已经断了。与 Linux 打包无关，但既然顺带发现就记在这里。
4. **`assets/dist/index.html` 直接给非 Wails 运行时用会白屏，D5 两条路都踩**。`vite.config.ts` 里 `rollupOptions.external: ['/wails/runtime.js']` 让产物保留一条裸 `from"/wails/runtime.js"`（已在现成 dist 里核实），这个 URL 只由 Android/Wails 运行时注入；dev server 靠 `wailsRuntimeStub` 插件的 `resolveId` 拦掉，所以 `just go-dev` / `go-dev-webview` 正常。后果是：embed 进二进制或从 `/usr/share/little-timer/dist` 读磁盘，只要不额外提供 `/wails/runtime.js`，窗口就只有空背景。D5 实现时必须连带处理这条（补桩，或让非 Android 路径不 import 那些 binding 模块）。
5. **前端产物是 4.27 MB 的单个 HTML（gzip 2.51 MB）**。`assetsInlineLimit: Number.MAX_SAFE_INTEGER` + `viteSingleFile` 把字体与 `i18n/*.toml` 一并 base64 内联——这是 Android 单文件策略，Linux 侧沿用的话，首帧要先解码这 4 MB。选 embed 时它同时决定二进制体积，选读磁盘时决定装包后要落多大文件；按平台拆开内联策略是一项能量出收益的改动，属 D5 的实现细节。
6. **NVIDIA 专有驱动 + Wayland 下 webview 白屏**。开发机 Fedora（KDE Plasma / Wayland / RTX 4060）上 `just go-dev-webview` 起窗后页面全白，日志里是 `Failed to create GBM buffer of size 800x600: 无效的参数`（EINVAL）。宿主侧该配的都配了——`nvidia_drm.modeset=1`、`nvidia_drm.fbdev=1`、`libnvidia-egl-wayland.so.1` 与 `libnvidia-egl-wayland2.so.1` 在位、`libgbm.so.1` 在位、EGL vendor json 齐全、nouveau 已黑名单——所以这不是配置缺失，是 WebKitGTK 走 dmabuf/GBM 零拷贝时与 NVIDIA 驱动在 modifier 协商上谈不拢，属上游问题。`WEBKIT_DISABLE_DMABUF_RENDERER=1` 可以不白屏（开发环境已用它跑通）。对打包的含义：装了 rpm/deb 的 NVIDIA 用户会撞上同一堵墙，`Exec` 不能无条件注入那个变量，也不能假装这个问题不存在——落地口径见 D6。
7. **同一套前端在浏览器里流畅、在 webview 里卡，且与缓冲区分配路径无关**。开发机 Fedora 上的四组对照，`[PERF] Settings.interaction.frame` 全部落在 215–382 ms：dev server、生产产物（`vite preview` 顶替 `:5173`）、`GDK_BACKEND=x11`、以及**去掉** `WEBKIT_DISABLE_DMABUF_RENDERER=1` 的 X11 —— 开关不是这笔开销的来源。同一时间同一 dev server、同一 Go 后端，浏览器（HTTP-only 模式）里主观流畅无卡顿。所以成本出在 WebKitGTK 这一侧（送帧节奏或页面绘制），不是 JS 构建产物、也不是后端网络路径。`apiGetHabitsMs` 在 0 条记录上从 253 → 130 → 125 ms 逐级下降，属冷启动而非固定代理税。拆源工具链已落地（[webkit-profiling.md](webkit-profiling.md) 三层）：空闲 rAF 探针连同装饰层对照都稳定在 32 ms，「WebKit 空闲送帧就这么慢」已排除；剩下「前端渲染侧 vs WebKit 原生提交/绘制」的分岔，由调试机上的 `[CAD]` 持续帧率计（`just webview-cadence`）与 `scripts/collect-webkit-perf.sh` 的 `self-top.txt` 收口。对打包的含义：阈值判定「webview ≥ 2× 浏览器」已由现有四组数字满足，D6 据此选定；`[CAD]`/perf 输出只细化成因表述，不翻转该结论。

## 验收清单（发版前）

1. EL9 干净容器内 `rpm -ivh` 成功，`rpm -qp --requires` 含 `libwebkit2gtk-4.0.so.37`，`little-timer serve --webview` 能起窗。
2. jammy 与 noble 干净容器内 `dpkg -i` 成功，depends 含 `libwebkit2gtk-4.1-0`，起窗正常。
3. 起窗后显示的是真实前端，而不是「Build the frontend …」占位页（D5 的落地校验）。
4. 在无任何 webkit 运行时的容器内安装两包均应失败（依赖挡住），而不是装完运行时崩。
5. 一致性门禁跑通：二进制 NEEDED 与包元数据同代。
6. KDE Plasma（Fedora_Jiaolong 环境）下桌面入口图标、`StartupWMClass` 与窗口分类正常。
7. 无密钥服务的机器上首启**按预期报错退出并给出启动 daemon 的命令**（不是静默降级），装好 `gnome-keyring` 后凭据功能正常。
8. NVIDIA + Wayland 真机（Fedora_Jiaolong）上桌面入口起窗后**既不白屏、交互帧时间也可接受**——只「能出画面」不够，按 D6 的决定验收：走开关则记录该组合下的帧时间，走「不支持」则确认文档写明改用 HTTP-only。

## 落地顺序

1. 定 D5 并实现 Linux 侧 UI 提供（embed 或读磁盘）——不先做这步，打出来的包只有 API，没有界面。
2. 定 D1–D4 与 D6（D3 决定后续 CI 改动形态，D6 决定 webview 变体能否作为默认推荐形态）。D6 的判定输入来自 `docs/webkit-profiling.md` 的 baseline，不要在没有符号化的 perf 数据上下结论。
3. 加打包 job：EL9 容器出 rpm，Ubuntu 容器出 deb，共用同一 `go build -tags "webview,embed_ui"` 步骤。
4. 加依赖一致性门禁，接进 `release.yml`。
5. 全绿后发新版，`README.md` / `docs/development.md` 的发布章节补上包格式与依赖说明。
