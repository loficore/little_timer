# Little Timer - 开发计划清单

## 📌 方向调整（2026-01-29）

> 目标：移除 WebUI，改为**服务端-客户端**架构；后端提供 **HTTP + SSE**；前端低频同步 + 本地插值；引入 **SQLite** 作为跨平台持久化。

## 后端 (Zig) - 待办

### 设置管理 (settings.zig)

- [ ]  **预设管理** (采用全量覆盖策略)
  - [ ]  删除/编辑：通过前端提交"完整预设列表"由后端直接覆盖，暂不单独提供 `removePreset()`/编辑接口

### 应用架构 (app.zig & main.zig)

- [ ]  **内存管理**
  - [ ]  测试长期运行是否有内存泄漏 ⏳ （需要动态运行验证）
- [ ]  **错误处理**
  - [X]  webui 初始化已移除（已迁移到 HTTP）
  - [ ]  ErrorRecoveryManager 框架存在但无实际恢复操作（仅有记录，缺少资源清理等真实恢复）
- [X]  **测试覆盖** ✅ 2026-01-31
  - [X]  修复 SQLite 测试内存问题（performDeepCheck 悬空指针）
  - [X]  修复测试间预设名称冲突问题
  - [X]  添加测试数据库自动清理
- [ ]  集成测试：前后端通信流程

### 日志系统 (logger.zig)

- [ ]  日志到文件功能 ❌ 未实现

### 架构重构（Server-Client 迁移）

#### 第一阶段：后端 HTTP Server 实现 ✅ 已完成

- [X] **创建 HTTP Server 模块** (`src/core/http/http_server.zig`)
  - [X]  使用 httpz 作为基础（而非 std.http.Server）
  - [X]  配置监听端口（默认 8080）
  - [X]  实现请求路由分发

- [X] **实现 REST API 端点**
  - [X]  `GET /api/state` - 获取当前状态
  - [X]  `POST /api/start` - 启动计时
  - [X]  `POST /api/pause` - 暂停计时
  - [X]  `POST /api/reset` - 重置计时
  - [X]  `POST /api/mode` - 切换模式
  - [X]  `GET /api/settings` - 获取设置
  - [X]  `POST /api/settings` - 更新设置
  - [X]  `GET /api/presets` - 获取预设列表
  - [X]  `POST /api/presets` - 简化实现：通过 /api/settings 一起更新

- [X] **实现 SSE 推送**
  - [X]  `GET /api/events` - Server-Sent Events 流
  - [X]  每秒推送当前状态
  - [ ]  低频校准推送（每 10 秒一次 heartbeat）

- [X] **移除 WebUI 依赖**
  - [X]  创建 HttpServerManager 替代 WebUIManager
  - [ ]  清理 build.zig 中的 webui 依赖
  - [X]  更新 main.zig 和 app.zig 初始化流程

#### 第二阶段：前端通信层重构 ✅ 已完成

- [X]  **创建 API 客户端** (`assets/src/utils/apiClient.ts`)
  - [X]  fetch 封装（GET/POST）
  - [X]  错误处理

- [X]  **实现 SSE 接收** (`assets/src/utils/sseClient.ts`)
  - [X]  EventSource 封装
  - [X]  自动重连机制（指数退避）
  - [X]  事件解析和分发

- [X]  **实现本地计时插值**
  - [X]  setInterval 本地 tick（WorldClock 模式）
  - [X]  SSE 接收后端状态

- [X]  **断线与重连策略**
  - [X]  SSE 自动重连
  - [ ]  离线状态提示 UI

#### 第三阶段：跨端适配

- [ ]  **Android 适配**
  - [ ]  后端作为 Android Service 运行
  - [ ]  WebView 连接 localhost

- [ ]  **桌面端适配**
  - [ ]  系统浏览器打开 localhost
  - [ ]  或使用系统 WebView

### 数据层（SQLite）

- [X]  **引入 SQLite 依赖**
  - [X]  选择 Zig SQLite 包装库（zqlite）
  - [X]  桌面端静态/动态链接方案
  - [X]  Android 端链接方案
- [X]  **设置与预设持久化** ✅ 已完成
  - [X]  SettingsConfig 落表
  - [X]  Presets CRUD（替代 presets.json）
- [ ]  **Presets 迁移到纯列存储**
  - [ ]  将 presets 表的 `config_json` TEXT 列拆分为独立字段
  - [ ]  更新序列化/反序列化逻辑
  - [ ]  保留 JSON 导入/导出作为可选功能
- [X]  **SQLite 测试框架** ✅ 2026-01-31 完成
  - [X]  **内存模式测试**
  - [X]  **临时文件模式测试**
  - [X]  **测试数据 fixtures**
  - [X]  **验证 zqlite API 兼容性**
  - [X]  修复 performDeepCheck 内存泄漏问题
  - [X]  修复测试间状态污染问题
  - [X]  添加 build.zig 自动清理测试数据库

---

## 前端 (TypeScript/Preact) - 待办

### 预设功能

- [X]  **预设功能** - 后端持久化已完成 ✅

### 样式和动画

- [ ]  **响应式设计** (未完成)
  - [ ]  测试在不同尺寸窗口下的布局
  - [ ]  移动端样式适配

### 工程化

- [X]  **测试** - 前端与后端通信 ✅
- [ ]  **国际化** (部分完成)
  - [ ]  UI 中混有中文硬编码

### 前端代码质量修复

#### 类型安全 & 内存安全

- [X]  **类型定义完善** - TimerState, Settings 接口
- [X]  **内存泄漏修复** - 已移除 WebUI 回调

#### 错误处理 & 代码质量

- [X]  **错误处理改进** - AudioContext/Notification 错误处理
- [ ]  **代码重复提取**
  - [ ]  创建 utils/timezone.ts
  - [ ]  创建 utils/theme.ts
- [ ]  **性能优化**
  - [ ]  Settings 组件添加 useCallback 包裹回调
  - [ ]  审查状态管理，考虑 Context 拆分

#### 国际化 & 清理

- [ ]  **国际化准备**
  - [ ]  标记硬编码中文字符串
- [ ]  **代码清理**

### 前端迁移（Server-Client） ✅ 已完成

#### 通信层重构

- [X]  **替换 WebUI 通信层**
  - [X]  HTTP API 客户端 (apiClient.ts)
  - [X]  SSE 事件源封装 (sseClient.ts)
  - [X]  统一事件分发机制

- [X]  **前端本地插值与校准**
  - [X]  SSE 接收后端状态
  - [X]  WorldClock 模式本地 tick

- [X]  **断线与重连策略**
  - [X]  SSE 自动重连 + 指数退避
  - [ ]  离线提示与降级显示
  - [ ]  连接状态 UI 指示器

#### 代码适配

- [X]  **更新 Homepage.tsx** - 替换为 API 调用
- [X]  **更新 Settings.tsx** - 替换为 HTTP API
- [X]  **类型定义更新** - TimerState, Settings 接口

---

## 跨端问题

- [ ]  **服务端-客户端架构跨端适配**
  - [ ]  Android 端启动本地后端服务
  - [ ]  Android 前端：WebView 连接 localhost
  - [ ]  Windows/Linux 端：本地服务 + 浏览器/内嵌视图
- [ ]  **打包和发布**
  - [ ]  创建可执行文件的打包脚本
  - [ ]  配置跨平台构建流程
  - [ ]  生成安装程序

## 文档

- [ ]  **API 文档**
  - [ ]  HTTP API 端点列表
  - [ ]  事件格式规范
- [ ]  **开发指南**
  - [ ]  编译和构建说明
  - [ ]  调试指南
- [ ]  **用户文档**
  - [ ]  使用说明
  - [ ]  FAQ
  - [ ]  故障排除

---

## 优先级建议

### 第一阶段（架构迁移）✅ 已完成

1. ✅ **创建 HTTP Server 模块** - httpz
2. ✅ **实现 REST API** - 全部端点
3. ✅ **实现 SSE 推送** - /api/events
4. ✅ **移除 WebUI 依赖** - HttpServerManager

### 第二阶段（前端重构）✅ 已完成

5. ✅ **创建 API 客户端** - apiClient.ts
6. ✅ **创建 SSE 客户端** - sseClient.ts
7. ✅ **更新 Homepage/Settings** - 替换 WebUI 调用

### 第三阶段（跨端适配）⏳ 待进行

8. ⏳ **Android Service** - 后端作为后台服务
9. ⏳ **WebView 集成** - Android 端连接 localhost

### 第四阶段（功能完善）

1. ⏳ **日志到文件** - logger.zig 文件输出
2. ⏳ **国际化** - 硬编码中文字符串提取
3. ⏳ **离线 UI 提示** - 连接状态指示器
4. ⏳ **预设列存储** - 优化 SQLite 表结构

### 第五阶段（打磨和发布）

1. 性能优化（渲染、tick 频率）
2. 测试覆盖（单元测试、集成测试）
3. 文档完善
4. 跨平台打包和发布流程
5. 响应式设计和移动端适配
