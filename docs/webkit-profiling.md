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
# 然后在同机浏览器打开 http://127.0.0.1:2222,列表里选 Little Timer 目标
```

两个环境变量缺一不可,各管一半:

- `LT_WEBKIT_DEBUG`(取值 `1` / `true` / `on`)打开 developer extras。上游 `webview.h` 只在 `debug == true` 时才调用 `webkit_settings_set_enable_developer_extras`——不设它,页面右键没有 Inspect Element,inspector 服务也不会起。
- `WEBKIT_INSPECTOR_SERVER` 只决定监听地址。设了它而没设 `LT_WEBKIT_DEBUG` 时,服务照样连不上。

**反例对照**(最容易让人对同一现象重复花一小时的组合):

```bash
WEBKIT_INSPECTOR_SERVER=127.0.0.1:2223 just go-dev-webview   # 不带 LT_WEBKIT_DEBUG
```

现象:窗口正常,但 `http://127.0.0.1:2223` 连不上,终端里**没有** `webview: devtools 已开` 那行提示;两个变量都设齐时该行出现,列表里出现 Little Timer 目标,可切到 Timeline / Profiler / Memory 面板。

> 调试机(Fedora_Jiaolong)上的两组带日期实跑输出(正例与反例)按采集纪律贴在本小节,当前待补。

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
| 浏览器同页面 | 主观流畅(`[CAD]` 数字待补) | `just webview-cadence` |

持续帧率计已正式化:`assets/devprobe/cadence.js` 由 `just webview-cadence` 注入生产构建产物,终端输出 `[CAD] idle …` / `[CAD] click …` 两种窗口。判读:idle 与 click 都是 ~32 ms → 成本在 Preact 渲染侧;idle ~32 而 click ~200 ms → 成本在 WebKit 的原生提交/绘制,用第二层的 `self-top.txt` 收口。

**怎么算正常(硬阈值)**:webview 的交互帧时间 ≤ 同一页面在浏览器里交互帧时间的 2 倍,才算能当默认推荐形态发;否则 [linux-packaging.md](linux-packaging.md) D6 取「HTTP-only + 浏览器为默认,webview 为可选」。

## 常见误读

- **观测者效应**:massif / `perf --mem-mode` 之下测得的帧时间与空闲时不可比,只能读分配曲线;帧时间一律以无 profiler 附加的 `[CAD]` / `[PERF]` 输出为准。
- **Wayland 与 X11 混采**:合成者不同(`kwin_wayland` vs XWayland),同一个 `perf.data` 里两条路的栈不可平均。一次采集只跑一种后端,并在带日期的记录里写明。
- **拿未符号化数据下结论**:见第二层符号健康检查,`<unknown>` 占比过高的报告只能证明「采到了」,不能证明「成本在哪」。
