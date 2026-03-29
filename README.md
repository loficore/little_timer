# Little Timer

Little Timer 是一个基于 Zig 和 WebUI 开发的跨平台定时器应用，支持倒计时、正计时和世界时钟功能。

## 项目特点

- 🎯 **跨平台**：支持 Linux 和 Windows（Android 支持计划中）
- ⚡ **高性能**：使用 Zig 编写的高效后端
- 🎨 **现代 UI**：基于 Preact + Tailwind CSS 的响应式界面
- 🔄 **模块化架构**：清晰的前后端分离设计
- 📱 **移动友好**：支持触摸操作和移动端适配

## 开源协议

本项目采用 [Apache License 2.0](./LICENSE) 协议，请遵照协议使用。

## 快速开始

### 桌面端（Linux / WSL / Windows）

Windows建议使用Mingw64,防止出现奇怪的依赖缺失问题.

```bash
zig build run
```

Zig 会自动下载依赖、编译 C 库并运行应用。

### Android

⚠️ **当前状态**：Android 支持目前在开发中，暂不可用。我们计划在未来版本中提供完整的 Android 支持。

## 依赖与环境要求

- **Zig**：建议使用0.15.2（用于构建后端与依赖）
- **Node.js + bun**：用于前端开发与构建（前端代码位于 assets/），建议使用最新版 Bun
- **系统库**：WebUI 运行时依赖（Linux/Windows 请确保系统环境可正常加载 WebUI 相关动态库）

> 若你只运行后端 `zig build run`，前端已构建产物可直接使用；需要修改 UI 时请看下方“前端开发与构建流程”。

## 前端开发与构建流程

进入前端目录并安装依赖：

```bash
cd assets
bun install
```

本地开发（HMR）：

```bash
bun run dev
```

生产构建（输出到 assets/dist）：

```bash
bun run build
```

代码检查：

```bash
bun run lint
```

## 配置说明

运行时配置文件：

- **settings.toml**：应用启动时读取，位于项目根目录
- **presets.json**：计时器预设持久化文件，与 settings.toml 同目录

常用字段（摘录）：

- **[basic]**
  - `timezone`：时区（范围 -12 ~ 14）
  - `language`：语言代码（如 "ZH"、"EN"）
  - `default_mode`：默认模式（countdown / stopwatch / world_clock）
  - `theme_mode`：主题（dark / light / auto）
- **[clock_defaults.countdown]**
  - `duration_seconds`：倒计时总秒数
  - `loop`：是否循环
  - `loop_count`：循环次数（0 表示无限循环）
  - `loop_interval_seconds`：循环间隔休息秒数
- **[clock_defaults.stopwatch]**
  - `max_seconds`：正计时上限秒数
- **[logging]**
  - `level`：日志等级（DEBUG/INFO/WARN/ERROR）
  - `enable_timestamp`：日志时间戳开关
  - `tick_interval_ms`：Tick 间隔（100 ~ 5000ms）

如需完整默认值，请查看 [settings.toml](settings.toml)。

## 关于工具使用

```bash
# 获取最新版本的变更内容并存入临时文件
git-cliff --latest --strip header > temp_note.md

# 创建 GitHub Release
gh release create v0.1.0 -F temp_note.md --title "v0.1.0 First Release"
```

## 常见问题

**Q：为什么编译失败？**
A：确保已安装 Zig 最新版本。第一次构建会自动编译依赖，较为缓慢。

**Q：编译很慢？**
A：第一次编译会编译 webui 的 C 源码。之后使用缓存，速度会快很多。

**Q：我想了解更多技术细节？**
A：参考 [ARCHITECTURE.md](./ARCHITECTURE.md) 和 [ANDROID_BUILD.md](./ANDROID_BUILD.md)。
