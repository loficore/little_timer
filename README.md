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

## 常见问题

**Q：为什么编译失败？**
A：确保已安装 Zig 最新版本。第一次构建会自动编译依赖，较为缓慢。

**Q：编译很慢？**
A：第一次编译会编译 webui 的 C 源码。之后使用缓存，速度会快很多。

**Q：我想了解更多技术细节？**
A：参考 [ARCHITECTURE.md](./ARCHITECTURE.md) 和 [ANDROID_BUILD.md](./ANDROID_BUILD.md)。
