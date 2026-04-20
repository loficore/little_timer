## [1.0.0] - 2026-04-20

### ✨ Features

- 添加基于 GTK4 的 Zig 图形界面定时器应用 (b589a88)

- 为 Windows 平台添加 GTK4 构建支持 (0c8fbca)

- 实现基础时钟功能与应用架构重构 (8e685bc)

- 实现番茄钟核心功能与 GTK UI 集成 (bc62e1f)

- 改进构建系统并完善项目文档 (08126dd)

- 重构时钟模块接口并实现秒表功能 (e8aaf0b)

- 添加 WebUI 测试可执行程序以验证界面集成 (049fb76)

- 添加 WebUI 支持并重构应用架构以支持多 UI 后端 (a6da88a)

- 完善 WebUI 倒计时器功能并增强日志与稳定性 (922f1d0)

- 重构前端为 React + TypeScript 并增强后端功能 (f4e3f0a)

- 重构前端架构并实现多语言支持的计时器应用 (6acbcec)

- 实现 WebUI 连接管理、错误通知和离线模式指示器 (e70f31f)

- 完善错误恢复机制与前端世界时钟，增强系统健壮性 (f7911c8)

- 添加多个 UI 组件并集成到主页和设置页面，同时引入完整的测试套件 (b4f9f6d)

- 实现预设管理模块与设置验证体系，支持动态预设增删查改及持久化 (9ef0a2c)

### 🐛 Bug Fixes

- 修正 .gitignore 中 zig-out 目录的忽略规则 (81d6c42)

### 🎉 Initial

- Initial commit (cff8a0f)

### ⚙️ Miscellaneous Tasks

- 从版本控制中移除 zig-out 目录下的二进制文件并修正 .gitignore 规则 (54a9bdc)
