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
  - [ ]  预设列表前端 UI ❌ 缺失
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

  - [ ]  补充更多单元测试
  - [ ]  集成测试：前后端通信流程

### 日志系统 (logger.zig)

- [X]  **日志模块完整实现** ✅
  - [X]  结构化日志完整实现（Logger 结构体）
  - [X]  支持多个日志级别：DEBUG、INFO、WARN、ERROR
  - [X]  支持时间戳格式化（HH:MM:SS）
  - [X]  支持 emoji 前缀（🐛 ℹ️ ⚠️ ❌）
  - [X]  等级过滤机制实现
  - [X]  全局 logger 实例
  - [ ]  日志到文件功能 ❌ 未实现

---

## 前端 (TypeScript/Preact) - 部分完成

### 核心交互

- [ ]  **与后端的双向通信** (部分完成)

  - [X]  基础函数绑定已实现（start/pause/reset/change_mode）✅
  - [ ]  设置同步尚未实现（load_settings/save_settings 被注释）❌
  - [ ]  `webuiEvent` 回调结构存在但事件格式有待验证
  - [ ]  添加通信错误处理和重连机制 ❌
- [ ]  **实时时间同步** (部分实现)

  - [X]  基础 tick 事件框架存在
  - [ ]  时间格式化（秒 → HH:MM:SS）需要在前端完成
  - [ ]  实时更新显示的完整流程待测试

### **模式切换 UI** ✅

- [X]  3 个模式按钮实现完整（倒计时、正计时、世界时钟）
- [ ]  模式切换时禁用/启用相应配置项 ❌
- [ ]  例如：正计时模式下隐藏循环配置 ❌
- [ ]  **设置页面功能** (部分实现)
  - [X]  BasicSettings.tsx 完整实现 ✅
  - [X]  CountdownSettings.tsx 完整实现 ✅
  - [X]  StopwatchSettings.tsx 完整实现 ✅
  - [ ]  与后端保存同步（注释状态）❌
  - [ ]  从后端加载设置初始值 ❌
  - [X]  数值输入范围校验已包含
- [ ]  **预设功能** (完全未实现)
  - [ ]  预设列表 UI 组件 ❌
  - [ ]  添加/删除/使用预设的交互 ❌
  - [ ]  预设切换时更新主界面 ❌

### 样式和动画

- [ ]  **响应式设计** (未完成)
  - [ ]  测试在不同尺寸窗口下的布局
  - [ ]  移动端样式适配（如果需要）
- [X]  **主题系统** ✅
  - [X]  亮色/暗色主题实现完整（CSS 变量 + light-mode 类）
  - [X]  主题切换已在 BasicSettings 中实现
  - [ ]  主题设置持久化 ❌ (前后端同步问题)
- [X]  **动画优化** ✅
  - [X]  `animate-slideUp`, `animate-fadeIn`, `animate-slideIn` 等完整定义（tailwind.config.js）
  - [X]  keyframes 定义完整（fadeIn、slideUp、bounce、glow）
  - [ ]  时间数字更新时的平滑过渡 ❌

### 工程化

- [X]  **组件实现** ✅
  - [X]  CheckboxInput、NumberInput、SelectInput、TabPanel、SettingItem 等完整实现
  - [X]  BasicSettings、CountdownSettings、StopwatchSettings 完整实现
  - [ ]  表单校验和错误提示 ❌
- [ ]  **国际化** (未完成)
  - [ ]  UI 中混有中文硬编码，应提取为 i18n 配置 ❌
  - [ ]  支持多语言切换 ❌
  - [X]  已定义 Mode 枚举支持多种语言值
- [ ]  **测试** (未完成)
  - [ ]  单元测试：组件逻辑 ❌
  - [ ]  集成测试：前后端通信 ❌
- [ ]  **性能** (未完成)
  - [ ]  减少不必要的重新渲染 ❌
  - [X]  后端 tick 频率控制已实现（每 10 个 tick 打印一次日志以减少输出）

---

## 跨端问题

- [ ]  **WebUI 库集成**

  - [ ]  在 Windows ( (部分实现)
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

### 第一阶段（核心功能）⚠️ 需要完成

1. ✅ 后端 webui 通信接口 - **已完成**
2. ⚠️ **实现设置同步机制** - 前后端通信需要激活（Settings.tsx 中的 load/save 注释）
3. ✅ 时钟引擎 tick 逻辑 - **已完成**
4. ⚠️ **实时时间显示** - 前端需要处理 tick 更新和时间格式化

### 第二阶段（功能完善）

1. ⚠️ **预设功能** - 删除、编辑和前端 UI 完全缺失
2. ⚠️ **主题持久化** - 选择了但未保存到后端
3. ⚠️ **从后端加载初始设置** - Settings 页面需要初始化数据
4. ⚠️ **国际化** - 硬编码中文字符串应提取

### 第三阶段（打磨和发布）

1. 错误处理和边界情况处理
2. 性能优化（渲染、tick 频率）
3. 测试覆盖（单元测试、集成测试）
4. 文档完善（API、开发指南、用户文档）
5. 跨平台打包和发布流程
