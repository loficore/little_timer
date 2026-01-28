# Little Timer - 开发计划清单

## 后端 (Zig) - 需要完善~~~~

### 核心功能

- [X]  **WebUI通信完整实现** ✅
  - [X]  JS函数绑定：start/pause/reset/tick/mode_change/get_settings（`setupEventHandlers()` 已实现）
  - [X]  后端推送更新到前端（`updateDisplay()` 完整实现，含时间格式化）
  - [X]  事件回调函数完整实现（handleStart/Pause/Reset/ModeChange 等）

### 时钟引擎 (clock.zig)

- [X]  **正计时 (Stopwatch) 模式** ✅

  - [X]  秒表运算逻辑完整实现（tick 方法，含上限检测）
  - [X]  状态管理完整（is_paused, is_finished）
- [X]  **循环倒计时逻辑** ✅

  - [X]  循环次数扣减完整实现
  - [X]  **无限循环模式已修复** - 改为先判断 `loop_count > 0` 才扣减，`loop_count == 0` 时始终进行循环
  - [X]  间隔休息状态管理完整（in_rest, rest_remaining_ms）
  - [X]  loop_completed 标记已实现
- [X]  **状态重置和切换** ✅

  - [X]  `user_change_mode` 事件完整实现
  - [X]  模式切换时的状态初始化完整
  - [X]  `user_change_config` 事件完整实现

### 设置管理 (settings.zig)

- [X]  **设置持久化** ✅

  - [X]  `save()` 和 `load()` 方法完整实现
  - [X]  TOML 序列化和反序列化完整实现
  - [X]  JSON 转换完整实现（`toJson()`, `jsonToSettings()`）
  - [X]  `handleSettingsEvent()` 完整实现
- [ ]  **预设管理** (采用全量覆盖策略)

  - [X]  预设添加功能 (`addPreset()`) ✅ 完整实现
  - [ ]  删除/编辑：通过前端提交“完整预设列表”由后端直接覆盖，暂不单独提供 `removePreset()`/编辑接口
  - [X]  预设列表前端 UI ✅ 已实现
- [X]  **校验** ✅

  - [X]  时区范围校验（-12 到 14）
  - [X]  语言代码校验
  - [X]  预设名称冲突检测

### 应用架构 (app.zig & main.zig)

- [X]  **主程序结构** ✅

  - [X]  主程序入口（main.zig）完整实现
  - [X]  应用初始化和运行完整实现
  - [X]  WebUI 初始化完整实现
  - [X]  全局 app 指针设置（setGlobalApp）
- [X]  **内存管理** ✅

  - [X]  完善资源清理（defer 清理所有分配的内存）✅
    - [X]  WebUIManager.deinit()：释放 html_content + webui_module.clean()
    - [X]  MainApplication.deinit()：调用 webui.deinit()
    - [X]  main.zig：defer 中确保调用 deinit()
    - [X]  settings.zig：JSON 字符串内存管理已实现（正确释放）
    - [X]  webui_windows.zig：handleChangeSettings() 已改用应用 allocator（无泄漏）
  - [ ]  测试长期运行是否有内存泄漏 ⏳ （需要动态运行验证）
- [ ]  **错误处理** (部分完成)

  - [X]  配置文件缺失时的降级方案（已有基础实现）
  - [X]  Settings.toml 加载失败时缺少强制重置为默认值机制（应实现 `resetToDefaults()`）
  - [ ]  webui 初始化失败时的处理
  - [ ]  ErrorRecoveryManager 框架存在但无实际恢复操作（仅有记录，缺少资源清理等真实恢复）
- [X]  **事件处理** ✅ 已完成

  - [X]  时钟事件处理完整实现（clockHandle）
  - [X]  设置事件处理完整实现（settingsHandle，包括 get_settings/change_settings）
  - [X]  mutex 线程锁已实现
  - [X]  状态转移验证已实现（防止幂等性问题）- start/pause 事件添加状态检查
  - [X]  事件去抖机制已实现（100ms阈值）- 使用 EventThrottle 结构体
- [ ]  **测试覆盖** (未完成)

  - [X]  补充更多单元测试
  - [ ]  集成测试：前后端通信流程

### 日志系统 (logger.zig)

- [X]  **日志模块完整实现** ✅
  - [X]  结构化日志完整实现（Logger 结构体）
  - [X]  支持多个日志级别：DEBUG、INFO、WARN、ERROR
  - [X]  支持时间戳格式化（ISO 8601）
  - [X]  支持 emoji 前缀（🐛 ℹ️ ⚠️ ❌）
  - [X]  等级过滤机制实现
  - [X]  全局 logger 实例
  - [ ]  日志到文件功能 ❌ 未实现

---

## 前端 (TypeScript/Preact) - 绝大部分已实现

### 核心交互

- [X]  **与后端的双向通信** (已完成) ✅

  - [X]  基础函数绑定已实现（start/pause/reset/change_mode）✅
  - [X]  设置同步已实现（`get_settings` / `change_settings`）✅
  - [X]  `webuiEvent` 回调结构完整，可处理 update_time/mode/state ✅
  - [X]  添加通信错误处理和重连机制（WebUI 连接管理 + 错误通知/离线提示）✅
- [X]  **实时时间同步** (已完成) ✅

  - [X]  基础 tick 事件框架存在 ✅
  - [X]  时间格式化（秒 → HH:MM:SS）已在前端完成 ✅
  - [X]  实时更新显示的完整流程已实现 ✅

### **模式切换 UI** ✅

- [X]  3 个模式按钮实现完整（倒计时、正计时、世界时钟）✅
- [X]  模式切换时禁用/启用相应配置项 ✅
- [X]  例如：正计时模式下隐藏循环配置 ✅
- [X]  **设置页面功能** (已完成) ✅
  - [X]  BasicSettings.tsx 完整实现 ✅
  - [X]  CountdownSettings.tsx 完整实现 ✅
  - [X]  StopwatchSettings.tsx 完整实现 ✅
  - [X]  与后端保存同步 ✅
  - [X]  从后端加载设置初始值 ✅
  - [X]  数值输入范围校验已包含 ✅
- [ ]  **预设功能** (UI已完成, 后端持久化未实现)
  - [X]  预设列表 UI 组件 ✅
  - [X]  添加/删除/使用预设的交互 ✅ (仅前端状态)
  - [X]  预设切换时更新主界面 ✅ (仅前端状态)
  - [X]  预设列表需要与后端同步 ✅

### 样式和动画

- [ ]  **响应式设计** (未完成)
  - [ ]  测试在不同尺寸窗口下的布局
  - [ ]  移动端样式适配（如果需要）
- [X]  **主题系统** ✅
  - [X]  亮色/暗色/自动主题实现完整
  - [X]  主题切换已在 BasicSettings 中实现
  - [X]  主题设置持久化 ✅
- [X]  **动画优化** ✅
  - [X]  `animate-slideUp`, `animate-fadeIn` 等完整定义（tailwind.config.js）
  - [X]  keyframes 定义完整（fadeIn、slideUp、bounce、glow）
  - [X]  时间数字更新时的平滑过渡 ✅

### 工程化

- [X]  **组件实现** ✅
  - [X]  CheckboxInput、NumberInput、SelectInput、TabPanel、SettingItem 等完整实现
  - [X]  BasicSettings、CountdownSettings、StopwatchSettings 完整实现
  - [X]  表单校验和错误提示 ✅ (已替换为表单内提示)
- [ ]  **国际化** (部分完成)
  - [X]  支持多语言切换 ✅
  - [X]  已定义 Mode 枚举支持多种语言值
  - [ ]  UI 中混有中文硬编码 (例如 "✅ 已完成")，应提取为 i18n 配置 ❌
- [X]  **测试** ✅
  - [X]  单元测试：组件逻辑 ✅
    - [X]  测试框架配置（Vitest + Testing Library）
    - [X]  NumberInput 组件测试
    - [X]  CheckboxInput 组件测试
    - [X]  SelectInput 组件测试
    - [X]  TabPanel 组件测试
    - [X]  工具函数测试（时间格式化）
  - [ ]  集成测试：前后端通信 ❌
- [X]  **性能** ✅
  - [X]  减少不必要的重新渲染 ✅
    - [X]  webuiEvent 回调用 useCallback 包裹
    - [X]  优化 statusMemo 直接返回 state 对象减少内存分配
    - [X]  applyTheme 提前定义避免依赖循环
    - [X]  添加 cleanup 函数清理事件监听器
  - [X]  后端 tick 频率控制已实现（每 10 个 tick 打印一次日志以减少输出）

---

## 跨端问题

- [ ]  **WebUI 库集成**

  - [ ]  在 Windows (MinGW64) 上的编译和运行 ❌
  - [X]  在 Linux 上的编译和运行 ✅ (已支持)
  - [ ]  WebUI 库的动态/静态链接方案最优化 ❌
- [ ]  **打包和发布** (未完成)

  - [ ]  创建可执行文件的打包脚本 ❌
  - [ ]  配置跨平台构建流程 ❌
  - [ ]  生成安装程序（MSI for Windows, DEB/RPM for Linux） ❌

## 文档

- [ ]  **API 文档** (未完成)
  - [ ]  后端 Zig 模块的接口文档 ❌
  - [ ]  WebUI JS 接口列表 ❌
  - [ ]  事件格式规范 ❌
- [ ]  **开发指南** (未完成)
  - [ ]  编译和构建说明（详细步骤） ❌
  - [ ]  调试指南 ❌
  - [ ]  贡献指南 ❌
- [ ]  **用户文档** (未完成)
  - [ ]  使用说明 ❌
  - [ ]  FAQ ❌
  - [ ]  故障排除 ❌

---

## 优先级建议

### 第一阶段（核心功能）✅ 全部完成

1. ✅ 后端 webui 通信接口 - **已完成**
2. ✅ **实现设置同步机制** - **已完成**
3. ✅ 时钟引擎 tick 逻辑 - **已完成**
4. ✅ **实时时间显示** - **已完成**
5. ✅ **从后端加载初始设置** - **已完成**
6. ✅ **主题持久化** - **已完成**

### 第二阶段（功能完善）⚠️ 需要完成

1. ⚠️ **预设功能** - UI 已完成，但需要实现后端持久化
2. ⚠️ **国际化** - 硬编码中文字符串应提取
3. ⚠️ **错误处理** - 增加前端通信重连机制和更友好的错误提示
4. ⚠️ **表单校验** - 使用更完善的校验和提示，代替 alert

### 第三阶段（打磨和发布）

1. 性能优化（渲染、tick 频率）
2. 测试覆盖（单元测试、集成测试）
3. 文档完善（API、开发指南、用户文档）
4. 跨平台打包和发布流程
5. 响应式设计和移动端适配
