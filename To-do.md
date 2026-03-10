# Little Timer - 开发计划清单

## 📌 方向调整（2026-01-29）

> 目标：移除 WebUI，改为**服务端-客户端**架构；后端提供 **HTTP + SSE**；前端低频同步 + 本地插值；引入 **SQLite** 作为跨平台持久化。

## 后端 (Zig) - 待办

### 设置管理 (settings.zig)

- [ ]  **预设管理** (采用全量覆盖策略)
  - [ ]  删除/编辑：通过前端提交“完整预设列表”由后端直接覆盖，暂不单独提供 `removePreset()`/编辑接口

### 应用架构 (app.zig & main.zig)

- [ ]  **内存管理**
  - [ ]  测试长期运行是否有内存泄漏 ⏳ （需要动态运行验证）
- [ ]  **错误处理**
  - [ ]  webui 初始化失败时的处理
  - [ ]  ErrorRecoveryManager 框架存在但无实际恢复操作（仅有记录，缺少资源清理等真实恢复）
- [X]  **测试覆盖** ✅ 2026-01-31
  - [X]  修复 SQLite 测试内存问题（performDeepCheck 悬空指针）
  - [X]  修复测试间预设名称冲突问题
  - [X]  添加测试数据库自动清理
  - [ ]  集成测试：前后端通信流程

### 日志系统 (logger.zig)

- [ ]  日志到文件功能 ❌ 未实现

### 架构重构（Server-Client 迁移）

#### 第一阶段：后端 HTTP Server 实现

- [ ] **创建 HTTP Server 模块** (`src/core/http_server.zig`)
  - [ ] 使用 std.http.Server 作为基础
  - [ ] 配置监听端口（默认 8080，可配置）
  - [ ] 实现请求路由分发

- [ ] **实现 REST API 端点**
  - [ ] `GET /api/state` - 获取当前状态（时间、模式、运行状态）
  - [ ] `POST /api/start` - 启动计时
  - [ ] `POST /api/pause` - 暂停计时
  - [ ] `POST /api/reset` - 重置计时
  - [ ] `POST /api/mode` - 切换模式 (body: {mode: "countdown"|"stopwatch"|"world_clock"})
  - [ ] `GET /api/settings` - 获取设置
  - [ ] `POST /api/settings` - 更新设置 (body: JSON)
  - [ ] `GET /api/presets` - 获取预设列表
  - [ ] `POST /api/presets` - 保存预设列表 (body: {presets: [...]})

- [ ] **实现 SSE 推送**
  - [ ] `GET /api/events` - Server-Sent Events 流
  - [ ] 推送事件：time_update, state_update, mode_change, settings_change
  - [ ] 低频校准推送（每 10 秒一次 heartbeat）

- [ ] **移除 WebUI 依赖**
  - [ ] 创建新的 HTTP 服务器管理器替代 WebUIManager
  - [ ] 清理 build.zig 中的 webui 依赖
  - [ ] 更新 main.zig 和 app.zig 初始化流程

#### 第二阶段：前端通信层重构

- [ ] **创建 API 客户端** (`assets/src/utils/apiClient.ts`)
  - [ ] fetch 封装（GET/POST）
  - [ ] 错误处理和重试逻辑

- [ ] **实现 SSE 接收**
  - [ ] EventSource 封装
  - [ ] 自动重连机制
  - [ ] 事件解析和分发

- [ ] **实现本地计时插值**
  - [ ] requestAnimationFrame 本地 tick
  - [ ] 与后端校准（每 10 秒）
  - [ ] 漂移校正算法

- [ ] **断线与重连策略**
  - [ ] 连接状态检测
  - [ ] 自动重连（指数退避）
  - [ ] 离线状态提示 UI

#### 第三阶段：跨端适配

- [ ] **Android 适配**
  - [ ] 后端作为 Android Service 运行
  - [ ] WebView 连接 localhost

- [ ] **桌面端适配**
  - [ ] 系统浏览器打开 localhost
  - [ ] 或使用系统 WebView

### 数据层（SQLite）

- [ ] **引入 SQLite 依赖**
  - [X]  选择 Zig SQLite 包装库（优先 zig-sqlite）
  - [X]  桌面端静态/动态链接方案
  - [X]  Android 端链接方案（系统 sqlite 或静态）
- [ ] **设置与预设持久化迁移**
  - [ ]  SettingsConfig 落表
  - [ ]  Presets CRUD（替代 presets.json）
  - [ ]  迁移/回退策略（保留 TOML 作为冷启动或导入）
  - [ ]  **Presets 迁移到纯列存储**
    - [ ]  将 presets 表的 `config_json` TEXT 列拆分为独立字段
    - [ ]  更新序列化/反序列化逻辑
    - [ ]  保留 JSON 导入/导出作为可选功能
- [X] **SQLite 测试框架** ✅ 2026-01-31 完成
  - [X]  **内存模式测试**（`:memory:` 快速验证 CRUD 逻辑）
  - [X]  **临时文件模式测试**（模拟真实文件操作）
  - [X]  **测试数据 fixtures**（预定义 preset JSON 样本）
  - [X]  **验证 zqlite API 兼容性**（内存模式下的 integrity_check 等）
  - [X]  修复 performDeepCheck 内存泄漏问题
  - [X]  修复测试间状态污染问题
  - [X]  添加 build.zig 自动清理测试数据库

---

## 前端 (TypeScript/Preact) - 待办

### 预设功能

- [ ]  **预设功能** (UI已完成, 后端持久化未实现)
  - [ ]  预设列表需要与后端同步 ✅

### 样式和动画

- [ ]  **响应式设计** (未完成)
  - [ ]  测试在不同尺寸窗口下的布局
  - [ ]  移动端样式适配（如果需要）

### 工程化

- [ ]  **国际化** (部分完成)
  - [ ]  UI 中混有中文硬编码 (例如 "✅ 已完成")，应提取为 i18n 配置 ❌
- [ ]  **测试**
  - [ ]  集成测试：前后端通信 ❌

### 前端代码质量修复

#### 类型安全 & 内存安全 🔴

- [ ] **类型定义完善**
  - [ ] 定义 `WebuiEvent` 接口（Homepage.tsx）
  - [ ] 定义 `SettingsEvent` 接口（Settings.tsx）
  - [ ] 定义配置类型（BasicSettings, CountdownSettings）
  - [ ] 声明全局回调类型（global.d.ts）
- [ ] **内存泄漏修复**
  - [ ] Homepage.tsx 添加 window.webuiEvent 清理
  - [ ] Settings.tsx 添加全局回调清理

#### 错误处理 & 代码质量 🟡

- [ ] **错误处理改进**
  - [ ] AudioContext/Notification 错误添加日志
  - [ ] JSON.parse 添加错误边界
- [ ] **代码重复提取**
  - [ ] 创建 utils/timezone.ts（时区数组）
  - [ ] 创建 utils/theme.ts（主题应用）
- [ ] **性能优化**
  - [ ] Settings 组件添加 useCallback 包裹回调
  - [ ] 审查状态管理，考虑 Context 拆分

#### 国际化 & 清理 🟢

- [ ] **国际化准备**
  - [ ] 标记硬编码中文字符串
- [ ] **代码清理**
  - [ ] 移除 Settings.tsx 中 console.log/console.warn

### 前端迁移（Server-Client）

#### 通信层重构

- [ ]  **替换 WebUI 通信层**
  - [ ]  创建 HTTP API 客户端 (apiClient.ts)
  - [ ]  创建 SSE 事件源封装 (sseClient.ts)
  - [ ]  统一事件分发机制
- [ ]  **前端本地插值与校准**
  - [ ]  本地计时器驱动 UI（requestAnimationFrame / setInterval）
  - [ ]  定期与后端校准（避免漂移）
  - [ ]  实现漂移校正算法
- [ ]  **断线与重连策略**
  - [ ]  SSE 自动重连 + 状态恢复
  - [ ]  离线提示与降级显示
  - [ ]  连接状态 UI 指示器

#### 代码适配

- [ ]  **更新 Homepage.tsx**
  - [ ]  替换 window.webui.call 为 API 调用
  - [ ]  替换 window.webuiEvent 为 SSE 事件监听
  - [ ]  实现本地计时插值逻辑

- [ ]  **更新 Settings.tsx**
  - [ ]  替换设置获取为 HTTP GET
  - [ ]  替换设置保存为 HTTP POST

- [ ]  **类型定义更新**
  - [ ]  定义 ApiResponse 接口
  - [ ]  定义 BackendEvent 接口
  - [ ]  更新全局类型声明

---

## 跨端问题

- [ ]  **服务端-客户端架构跨端适配**
  - [ ]  Android 端启动本地后端服务（JNI/Service）
  - [ ]  Android 前端：WebView 或原生 UI 通过 localhost 连接
  - [ ]  Windows/Linux 端：本地服务 + 浏览器/内嵌视图
- [ ]  **打包和发布**
  - [ ]  创建可执行文件的打包脚本 ❌
  - [ ]  配置跨平台构建流程 ❌
  - [ ]  生成安装程序（MSI for Windows, DEB/RPM for Linux） ❌

## 文档

- [ ]  **API 文档**
  - [ ]  后端 Zig 模块的接口文档 ❌
  - [ ]  WebUI JS 接口列表 ❌
  - [ ]  事件格式规范 ❌
- [ ]  **开发指南**
  - [ ]  编译和构建说明（详细步骤） ❌
  - [ ]  调试指南 ❌
  - [ ]  贡献指南 ❌
- [ ]  **用户文档**
  - [ ]  使用说明 ❌
  - [ ]  FAQ ❌
  - [ ]  故障排除 ❌

---

## 优先级建议

### 第零阶段（架构迁移）⚠️ 进行中

#### 后端实现
1. ⚠️ **创建 HTTP Server 模块** - 基于 std.http.Server
2. ⚠️ **实现 REST API** - /api/state, /api/start, /api/pause, /api/reset, /api/mode, /api/settings, /api/presets
3. ⚠️ **实现 SSE 推送** - /api/events 事件流
4. ⚠️ **移除 WebUI 依赖** - 替换 WebUIManager 为 HttpServerManager

#### 前端实现
5. ⚠️ **创建 API 客户端** - fetch 封装 + 错误处理
6. ⚠️ **创建 SSE 客户端** - EventSource 封装 + 自动重连
7. ⚠️ **本地计时插值** - requestAnimationFrame + 校准
8. ⚠️ **更新 Homepage/Settings** - 替换 WebUI 调用

#### 跨端适配
9. ⚠️ **Android Service** - 后端作为后台服务
10. ⚠️ **WebView 集成** - Android 端连接 localhost

### 第二阶段（功能完善）⚠️ 需要完成

1. ⚠️ **预设功能** - 后端持久化框架已完成（SQLite CRUD），待完整功能
2. ⚠️ **国际化** - 硬编码中文字符串应提取
3. ⚠️ **错误处理** - 增加前端通信重连机制和更友好的错误提示
4. ⚠️ **表单校验** - 使用更完善的校验和提示，代替 alert

### 第三阶段（打磨和发布）

1. 性能优化（渲染、tick 频率）
2. 测试覆盖（单元测试、集成测试）
3. 文档完善（API、开发指南、用户文档）
4. 跨平台打包和发布流程
5. 响应式设计和移动端适配
