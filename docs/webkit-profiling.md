# WebKit 桌面形态剖析工具链

这套工具回答一个问题:webview 窗口里 `Settings.interaction.frame` 的 215–382 ms 到底花在 WebKit 的哪一侧——JS 执行、页面绘制,还是原生提交。它把[《Linux 打包决策》](linux-packaging.md)「已知问题」7 里「成本出在 WebKitGTK 这一侧」的推断变成有符号的测量,判定输入供 D6(webview 能否作为默认推荐形态)使用。

## 三层各自能看见什么

| 层 | 入口 | 能看见 | 看不见 |
| --- | --- | --- | --- |
| ① WebKit 自带 inspector | `just webview-inspect` | 页面 Timeline、JS Profiler、Memory 堆快照、DOM/CSS 与 paint 计时 | `WebKitWebProcess` 原生绘制管线与主进程 Go 侧的原生栈 |
| ② 系统 `perf` 原生栈 | `scripts/collect-webkit-perf.sh` | `WebKitWebProcess` + 主进程真实的 CPU 花费位置(装好符号后) | JS 语义——只知道落在哪个函数地址,不知道对应哪个组件 |
| ③ 内存分配 | inspector Memory 面板 / massif / `perf --mem-mode` | JS 堆增长;pages-as-heap 粒度下 painter/cairo 侧的峰值与曲线 | 任何帧时间(profiler 本身把进程拖慢 10–50 倍) |

三层各自独立可用;②③的 perf 采集以 ① 的方式起窗口即可,不要求 inspector 服务在线。

## 第一层:WebKit 远程 inspector

```bash
just webview-inspect
# 然后在同机浏览器打开列表页 http://127.0.0.1:2224,选 Little Timer → Inspect
```

两个环境变量各管一半,缺一不可:

- `LT_WEBKIT_DEBUG`(取值 `1` / `true` / `on`)打开 developer extras。上游 `webview.h` 只在 `debug == true` 时才调用 `webkit_settings_set_enable_developer_extras`——不设它,页面右键没有 Inspect Element,列表里也不会有目标。
- `WEBKIT_INSPECTOR_SERVER` 与 `WEBKIT_INSPECTOR_HTTP_SERVER` 分别是**协议 socket** 和 **HTTP 列表页**:实测 webkit2gtk 2.52.1 上 SERVER 口(2222)不是网页,curl 只得 Empty reply、浏览器显示空白页;列表页在 HTTP_SERVER 口(2224)。旧版 WebKit 直接把 SERVER 当 HTTP 前端用,拿不准时两个地址都开一次,看哪个出列表。

**反例对照**(最容易让人对同一现象重复花一小时的组合):

```bash
WEBKIT_INSPECTOR_SERVER=127.0.0.1:2223 just go-dev-webview   # 不带 LT_WEBKIT_DEBUG
```

实测记录(2026-10-31,Fedora_Jiaolong / fc44 / webkit2gtk4.1-2.52.1):

- 正例(两变量齐,`just webview-inspect`):终端出现 `webview: devtools 已开…` 提示行;列表页出现 Little Timer 条目(指向 `http://localhost:5173/?runtime=webview`);窗口内右键 → Inspect Element 可直接弹出 Web Inspector。
- 反例(不带 `LT_WEBKIT_DEBUG`):2223 同样「能连上但回空白」(SERVER socket 照常在听),终端**没有**提示行。判定「devtools 开没开」以**提示行 + 列表页里有没有目标**为准,端口在不在监听不算。

## 第二层:`perf` 采原生栈

```bash
./scripts/collect-webkit-perf.sh [--seconds N] [--out DIR]   # 默认 20 s、perf-out/
```

采集期间在窗口里连点 tab。脚本依次做四项前置检查,每条失败都打印可执行的下一步并以 1 退出,不会自行改系统状态:`perf` 未安装(装:`sudo dnf install perf`)、`perf_event_paranoid` > 2(临时放宽:`sudo sysctl kernel.perf_event_paranoid=2`,重启即失效——不建议永久改内核参数)、没有 `WebKitWebProcess`、没有主进程 `bin/server serve --webview`(后两者:先 `just webview-inspect` 起窗口)。

产物与退出码:

| 产物 / 退出码 | 含义 |
| --- | --- |
| `perf-out/perf.data`、`report.txt`、`self-top.txt` | 原始数据、全栈报告、占比 ≥1% 的自身热点前 40 行 |
| 0 | 采集成功;若报告里 `<unknown>` 行占非空行比例 > 30% 会打 WARN(数据对本进程侧仍可用) |
| 1 | 前置检查失败,未开始采集 |
| 2 | 采不到数据(`perf record` 非零退出,或 `report.txt` 里 Samples 为 0) |

**未符号化的数据不得写入 baseline**(Review Focus 重点):没装 webkit 调试符号时采样会「成功」,但报告里全是 `<unknown>`,据此得出的「成本在 X」完全无意义。脚本检测到就 WARN,处理方式:

```bash
sudo dnf debuginfo-install webkit2gtk4.1 gtk3
./scripts/collect-webkit-perf.sh --seconds 20   # 重跑,<unknown> 占比应明显下降
```

判读分岔:`self-top.txt` 头部落在 `WebCore::RenderLayer` / `paint` 系 → 绘制成本;落在 `JSC` / `execute` 系 → JS 成本。

> 调试机实跑输出(带 debuginfo 前后各一次)按采集纪律贴在本小节,当前待补。

## 第三层:内存分配

两条路并列,回答的问题不同,代价也不同:

**inspector → Memory 面板**(经第一层进入)。只看 JS 堆,适合回答「是不是前端在囤对象」。快照与时间线开销可控,可与帧时间测量同时进行。

**massif 看 painter/cairo 侧**(软件合成路径下每帧全屏缓冲的分配都记在这里):

```bash
valgrind --tool=massif --pages-as-heap=yes --massif-out-file=perf-out/massif.out \
  ./bin/server serve --webview --port 8013
```

只读峰值与增长曲线。**massif 下帧时间不可用**——进程被拖慢 10–50 倍,此时观测到的任何帧间隔都不能写进 baseline 表。更轻的对照:`perf record --mem-mode -g --call-graph dwarf -p <pid> -- sleep 20`,拿到带分配点的采样,开销远小于 valgrind,但同样要求符号齐备。

## Baseline

| 指标 | 数值 | 来源 |
| --- | --- | --- |
| 空闲 rAF 间隔(探针页,无业务代码) | avg 31.9 / median 32.0 / max 33.0 ms | 本机实验 2026-10-30 |
| 装饰层 `backdrop-filter: blur(18px/8px/呼吸动画)` 对间隔的影响 | 无(四阶段均 32 ms) | 本机实验 2026-10-30 |
| `Settings.interaction.frame`(dev / prod / X11 / 关掉 DMABUF 开关) | 215–382 ms,四组同量级 | [linux-packaging.md](linux-packaging.md) 已知问题 7 |
| 浏览器同页面 | 主观流畅(`[CAD]` 数字待补) | `just webview-cadence` 产物在 :5173,浏览器直接开 |
| webview 同页面 2026-10-31 复访 | 主观不卡(无 `[PERF]`/`[CAD]` 数字;「webkitgtk 升到 2.52.1」假设已排除——该机 2026-05-08 就装的 2.52.1,早于卡顿数据;`justfile.local` 仍导出 `WEBKIT_DISABLE_DMABUF_RENDERER := "1"`,即复访跑的仍是软件路径,差异原因待 B/C 两格量化) | 调试机实跑,待量化 |

持续帧率计已正式化:`assets/devprobe/cadence.js` 由 `just webview-cadence` 注入生产构建产物,输出 `[CAD] idle …` / `[CAD] click …` 两种窗口。判读:idle 与 click 都是 ~32 ms → 成本在 Preact 渲染侧;idle ~32 而 click ~200 ms → 成本在 WebKit 的原生提交/绘制,用第二层的 `self-top.txt` 收口。

**自动留档(2026-10-31 补)**:WebKitGTK 默认不把页面 console 转发到终端,`[CAD]` 行只能靠 webview 绑定回传——`LT_WEBKIT_DEBUG=1` 时 `native.go` 注册 `window.ltProfLog`,cadence.js 把同一行送回来写进 stderr;recipe 自身 `tee` 整段输出到 `perf-out/cadence-<时间戳>.log`(gitignore 内),跑完一格交 `grep '\[CAD\]' <日志>` 即可,不需要人抄。绑定只在 debug 开时注册,发布行为不变;若终端一直没有 `[CAD]` 行,降级读数走右键 → Inspect Element 的 Console(右键 → Save All Messages)或浏览器 `http://localhost:5173` 的 devtools。

**矩阵四格**(每格单独跑一次 recipe,Ctrl+C 结束后交对应日志的 `grep '[CAD]'` 结果;Wayland/X11 与 DMABUF 状态逐格标注,禁止跨格平均):

| 格 | 命令 | 说明 |
| --- | --- | --- |
| A/C(当前默认,DMABUF 关) | `just webview-cadence` | `justfile.local` 导出 =1 的机器上 A 与 C 同格 |
| B(DMABUF 开) | `CADENCE_CLEAR_DMABUF=1 just webview-cadence` | recipe 只在自己的子进程里 unset,不改 `justfile.local` |
| D(X11 + DMABUF 开) | `GDK_BACKEND=x11 CADENCE_CLEAR_DMABUF=1 just webview-cadence` | X11 对照,同一次采集不混 Wayland |

浏览器分母:同一 `vite preview` 已在 :5173 服务注入了 cadence 的产物,浏览器打开同址连点 tab,从 devtools console 抄 `[CAD]` 行(或 Save All Messages)。

调试机若 `justfile.local` 里还留着同名的临时 `webview-cadence` recipe,同步本分支后先删掉它——`justfile` 经 `import? "justfile.local"` 合并,重名会让所有 `just` 命令报 recipe redefined。

**怎么算正常(硬阈值)**:webview 的交互帧时间 ≤ 同一页面在浏览器里交互帧时间的 2 倍,才算能当默认推荐形态发;否则 [linux-packaging.md](linux-packaging.md) D6 取「HTTP-only + 浏览器为默认,webview 为可选」。

## 常见误读

- **观测者效应**:massif / `perf --mem-mode` 之下测得的帧时间与空闲时不可比,只能读分配曲线;帧时间一律以无 profiler 附加的 `[CAD]` / `[PERF]` 输出为准。
- **Wayland 与 X11 混采**:合成者不同(`kwin_wayland` vs XWayland),同一个 `perf.data` 里两条路的栈不可平均。一次采集只跑一种后端,并在带日期的记录里写明。
- **拿未符号化数据下结论**:见第二层符号健康检查,`<unknown>` 占比过高的报告只能证明「采到了」,不能证明「成本在哪」。
