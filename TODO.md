# Little Timer - 待办清单

---

## Phase 1: 工具层统一化

### 1.1 创建统一格式化函数

- [x] 创建 `assets/src/utils/formatters.ts`
- [x] 迁移 `formatDuration` 函数
- [x] 迁移所有文件中的重复定义

### 1.2 创建常量文件

- [x] 创建 `assets/src/utils/constants.ts`
- [x] 提取重复的常量定义

### 1.3 统一 API 客户端

- [x] 创建 `src/utils/apiClientSingleton.ts` - 单例模式
- [x] 替换所有组件中的 `new APIClient()`

---

## Phase 2: 数据层抽象 (Hooks)

### 2.1 创建 useTimer hook

- [x] 迁移 TimerPage 中的计时器逻辑
- [x] 统一状态管理

### 2.2 创建 useHabits hook

- [x] 迁移 HabitsPage 中的数据加载逻辑
- [x] 实现数据缓存

### 2.3 创建 useSSE hook

- [x] 封装 SSE 连接逻辑
- [x] 实现重连机制

### 2.4 创建 useSettings hook

- [x] 迁移 SettingsPage 中的设置管理
- [x] 实现持久化

---

## Phase 3: 组件拆分 (TimerPage)

### 3.1 TimerControls 组件

- [x] 创建 `src/components/TimerControls.tsx`
- [x] 使用 memo 包裹

### 3.2 TimerConfig 组件

- [x] 创建 `src/components/TimerConfig.tsx`
- [x] 使用 memo 包裹

### 3.3 HabitPicker 组件

- [x] 创建 `src/components/HabitPicker.tsx`
- [x] 使用 memo 包裹

### 3.4 TimerProgress 组件

- [x] 创建 `src/components/TimerProgress.tsx`
- [x] 使用 memo 包裹

---

## Phase 4: 性能优化

### 4.1 动态导入

- [x] Stats.tsx 动态导入 apexcharts
- [x] 使用 `import()` 延迟加载

### 4.2 组件优化

- [x] 所有新组件使用 memo 包裹
- [x] 统一格式化函数

### 4.3 类型检查

- [x] 通过 `bun run build:check`
- [x] 通过 `bun run lint`

---

## Phase 4: 性能优化

### 4.1 动态导入

- [x] Stats.tsx 动态导入 apexcharts
- [x] 使用 `import()` 延迟加载

### 4.2 组件优化

- [x] 所有新组件使用 memo 包裹
- [x] 统一格式化函数

### 4.3 SVG 提取

- [x] 提取 Sidebar.tsx 中的内联 SVG 为独立组件

---

## Phase 13: 测试扩展 ✅

### 13.1 后端 HTTP 服务器测试（高优先级）

- [x] test_http_server.zig - ClockTaskConfig、ClockEvent、SettingsEvent 测试
- [x] test_http_server.zig - 端点路由测试
- [x] test_http_server.zig - 请求解析测试
- [x] test_http_server.zig - SSE 事件流测试

### 13.2 前端 Hooks 测试（核心业务）

- [x] test/useTimer.test.ts - 计时器状态管理
- [x] test/useSSE.test.ts - SSE 连接与重连
- [x] test/useSettings.test.ts - 设置管理
- [x] test/useHabits.test.ts - 习惯数据管理

### 13.3 存储层测试（数据验证）

- [x] test_storage.zig - SQLite 初始化、错误类型测试
- [x] test_migration.zig - 数据库迁移测试
- [x] test_http_server.zig - ClockTaskConfig、ClockEvent、SettingsEvent 测试
- [x] test_app.zig - 主应用集成测试
- [x] test_habit_crud.zig - 习惯 CRUD 操作测试（内存数据库）
- [x] test_storage_backup.zig - 备份恢复测试（临时文件系统）
- [x] test_storage_health.zig - 健康检查测试（内存数据库）

### 13.4 前端 API 测试

- [x] test/utils/apiClient.test.ts - HTTP 请求
- [x] test/utils/formatters.test.ts - 格式化函数 (formatDuration 统一版)

### 13.5 组件与页面测试（扩展覆盖）

- [x] test/components/Sidebar.test.tsx
- [x] test/components/TimerControls.test.tsx
- [x] test/components/HabitPicker.test.tsx
- [x] test/components/TimerConfig.test.tsx
- [x] test/components/TimerProgress.test.tsx
- [x] test/components/DropdownSelect.test.tsx
- [x] test/components/SevenSegmentDisplay.test.tsx
- [ ] test/components/WallpaperSelector.test.tsx
- [ ] test/components/HabitModal.test.tsx
- [ ] test/pages/Homepage.test.tsx
- [ ] test/pages/Settings.test.tsx
- [ ] test/pages/TimerPage.test.tsx
- [ ] test/pages/HabitsPage.test.tsx
- [ ] test/pages/Stats.test.tsx

### 13.6 工具函数测试

- [x] test/utils/i18n.test.ts - 国际化

---

## 测试结果

### 前端测试 (232 tests passed)
```
 ✓ src/test/components/FormGroup.test.tsx (11 tests)
 ✓ src/test/components/Button.test.tsx (9 tests)
 ✓ src/test/components/ModeSelector.test.tsx (6 tests)
 ✓ src/test/components/TimerControls.test.tsx (10 tests)
 ✓ src/test/components/CheckboxInput.test.tsx (6 tests)
 ✓ src/test/components/ControlPanel.test.tsx (7 tests)
 ✓ src/test/components/FormSection.test.tsx (6 tests)
 ✓ src/test/components/NumberInput.test.tsx (6 tests)
 ✓ src/test/components/SelectInput.test.tsx (5 tests)
 ✓ src/test/components/StatusBadge.test.tsx (5 tests)
 ✓ src/test/components/TabPanel.test.tsx (4 tests)
 ✓ src/test/components/TimeDisplay.test.tsx (5 tests)
 ✓ src/test/components/Sidebar.test.tsx (4 tests)
 ✓ src/test/components/HabitPicker.test.tsx (6 tests)
 ✓ src/test/components/Header.test.tsx (6 tests)
 ✓ src/test/hooks/useTimer.test.ts (12 tests)
 ✓ src/test/hooks/useSSE.test.ts (5 tests)
 ✓ src/test/hooks/useSettings.test.ts (12 tests)
 ✓ src/test/hooks/useHabits.test.ts (9 tests)
 ✓ src/test/utils/apiClient.test.ts (15 tests)
 ✓ src/test/utils/formatters.test.ts (38 tests)
 ✓ src/test/utils/i18n.test.ts (12 tests)
 ✓ src/test/utils.test.ts (3 tests)
```

### 后端测试 (All passed)
```
zig build test
```

---

## Phase 5: 类型检查与测试

- [x] 运行 `bun run build:check`
- [x] 运行 `bun run lint` 并修复问题
- [x] 运行 `bun run test` 确保测试通过

---

## 已完成功能

- [x] 新增 `GET /api/habits/:id/detail` - 返回习惯详情（含今日进度、streak）
- [x] 习惯详情页 - 显示今日进度、目标完成度、连胜天数
- [x] 计时页集成 - 显示习惯进度条和累计时间
- [x] 桌面通知 - 计时完成时发送系统通知
- [x] 计时页面重构 - 主页为计时页，可选择习惯开始计时

---

## 阶段 9：自定义壁纸功能（解耦全局与习惯壁纸）✅

- [x] SQLite: settings 表已有 wallpaper 字段
- [x] SQLite: habit_sets 表已有 wallpaper 字段  
- [x] SQLite: habits 表已有 wallpaper 字段
- [x] GET /api/settings - 返回包含 wallpaper
- [x] POST /api/settings - 支持更新 wallpaper
- [x] PUT /api/habit-sets/:id - 支持 wallpaper
- [x] PUT /api/habits/:id - 支持 wallpaper
- [x] settings API 已返回 wallpaper
- [x] updateHabitSet 支持 wallpaper
- [x] updateHabit 支持 wallpaper
- [x] 在 BasicSettings 添加全局壁纸选择器
- [x] 保存时通过 POST /api/settings 更新全局壁纸
- [x] 启动时从 GET /api/settings 获取全局 wallpaper
- [x] 每个习惯项显示自己的 wallpaper 作为卡片封面
- [x] TimerPage 移除 wallpaper prop
- [x] StatsPage 移除 wallpaper prop

---

## Bug 修复：壁纸设置无法持久化 ✅

- [x] 后端 settings_manager.zig 解析 wallpaper 字段
- [x] 前端 Settings.tsx 加载 wallpaper 字段

---

## 阶段 12：前端风格问题修复 ✅

### 1. 修复选择习惯弹窗样式

- [x] TimerPage.tsx - 弹窗使用 `bg-base-100` 导致与全局背景不协调
- [x] 修改为毛玻璃风格 `my-surface-card`

### 2. 美化计时选项下拉框

- [x] TimerPage.tsx - 计时模式 `<select>` 使用默认样式
- [x] 应用 `timer-option-control` 已有样式

### 3. 修复 i18n 缺少 stats.title 翻译

- [x] zh.toml 添加 `[stats]` 段落和 `title = "统计"`

### 4. 修复统计页面时间分布图表不显示

- [x] Stats.tsx - 检查 pie chart 渲染逻辑
- [x] 修复后端 query string 解析问题（从 request.params 改为 request.query）

### 1. 创建统一样式（globals.css）

#### 1.1 表单控件样式

- [x] 创建 `.my-input` - 毛玻璃背景、圆角、无边框
- [x] 创建 `.my-select` - 同 input
- [x] 创建 `.my-checkbox` - 自定义 Material You 风格

#### 1.2 徽章样式

- [x] 创建 `.my-badge-running` - 主题色 (#515bd4)
- [x] 创建 `.my-badge-paused` - 中性灰
- [x] 创建 `.my-badge-finished` - 成功绿

#### 1.3 按钮样式

- [x] 创建 `.my-btn-secondary` - 毛玻璃透明背景

#### 1.4 卡片样式

- [x] 统一 `.my-surface-card` 样式

### 2. 替换表单组件

- [x] 修改 `SelectInput.tsx` - 使用 `.my-select`
- [x] 修改 `NumberInput.tsx` - 使用 `.my-input`
- [x] 修改 `TimeInput.tsx` - 使用 `.my-input`
- [x] 修改 `CheckboxInput.tsx` - 使用 `.my-checkbox`
- [x] 修改 `HabitModal.tsx` - 使用 `.my-input`
- [x] 修改 `WallpaperSelector.tsx` - 使用 `.my-input`

### 3. 替换徽章组件

- [x] 修改 `StatusBadge.tsx` - 使用 `.my-badge-*`

### 4. 替换次要按钮

- [x] Homepage.tsx - 习惯卡片按钮使用新样式
- [x] Settings.tsx - 重置按钮使用新样式
- [x] ModeSelector.tsx - 模式选择按钮使用新样式

### 5. 统一卡片样式

- [x] Homepage.tsx - 习惯卡片改用 `my-surface-card`

### 6. 验证测试

- [x] 前端构建检查 `bun run build:check`
- [x] 前端测试 `bun run test`
- [x] 视觉检查：所有组件符合 Material You 风格

---

## 开发模式支持

- [x] `./scripts/dev.sh` - 普通模式（HTTP 服务器）
- [x] `./scripts/dev.sh --webview` - WebView 模式（打开 WebView 窗口）

---

## 阶段 14：前端 UI 样式修复 ✅

### 问题1：自定义时间选择组件

**问题**: 时间选择使用原生 `<input type="number">`，与整体风格不贴合

**任务**:
- [x] 创建自定义数字选择器组件，替代原生 input
- [x] 在 TimeInput.tsx 中使用新组件
- [x] 在 NumberInput.tsx 中使用新组件
- [x] 在 TimerConfig.tsx 中使用新组件
- [x] 在 TimerPage.tsx 倒计时配置中使用新组件
- [x] 在 HabitModal.tsx 目标时长输入中使用新组件

### 问题2：毛玻璃效果缺失

**问题**: 计时页面选择习惯按钮和时间选择容器没有毛玻璃效果

**任务**:
- [x] TimerPage.tsx - 选择习惯按钮增强毛玻璃效果
- [x] TimerPage.tsx - 时间选择容器添加毛玻璃效果
- [x] TimerConfig.tsx - 配置面板添加毛玻璃效果

### 问题3：深色遮罩问题

**问题**: HabitPicker 和 HabitModal 的遮罩层使用深黑色，不符合整体风格

**任务**:
- [x] HabitPicker.tsx - 移除深色遮罩，改用透明/模糊
- [x] HabitModal.tsx - 移除深色遮罩，改用透明/模糊
- [x] TimerPage.tsx - 移除 habit picker 深色遮罩

---

## 阶段 15：弹窗焦点区域背景透明度修复

### 问题描述

**问题**: 习惯选择框 (HabitPicker) 和习惯编辑/新建框 (HabitModal) 的背景太透明，导致在焦点区域内的文字难以识别。

**现状**:
- 弹窗使用 `my-surface-card` 类，背景透明度为 0.66
- 弹窗遮罩层使用 `rgba(0,0,0,0.2)` + `blur(4px)`

### 修复任务

- [x] 在 `globals.css` 中创建 `my-surface-modal` 类（更高不透明度，如 0.85-0.9）
- [x] 更新 `HabitPicker.tsx` 使用新样式
- [x] 更新 `HabitModal.tsx` 使用新样式
- [x] 更新 `TimerPage.tsx` habit picker 使用新样式
- [x] 更新 `HabitsPage.tsx` delete confirm 弹窗使用新样式
- [ ] 验证视觉效果

---

## Phase 16: WebView 退出时资源清理

### 问题描述

当用户在 WebView 窗口中点击原生退出按钮时，程序无法完全退出，后端 HTTP 服务器继续占用端口 8080。

### 实施步骤

#### 16.1 添加退出标志到 MainApplication

- [x] 在 `src/core/app.zig` 的 `MainApplication` 结构体中添加 `should_exit: std.atomic.Value(bool)` 字段
- [x] 修改 `stop()` 方法，在调用 `http_server.stop()` 前设置 `should_exit.store(true, .release)`

#### 16.2 修改 main_entry.zig 退出逻辑

- [x] 在 `win.run()` 返回后保留显式的 `main_app.stop()` 调用
- [x] 确保 `app_thread.join()` 在 `stop()` 之后执行
- [x] 验证资源清理顺序正确

#### 16.3 验证修复

- [x] 运行 `zig build test` 确保无内存泄漏
- [ ] 手动测试：启动 WebView → 点击退出 → 验证进程退出
- [ ] 验证端口 8080 被释放

### 相关代码位置

- `src/core/app.zig` - MainApplication 结构体定义
- `src/core/app.zig` - stop() 方法
- `src/main_entry.zig` - WebView 运行逻辑

### httpz 退出机制说明

- `server.stop()` 会关闭 listener socket
- `accept()` 返回错误，HTTP 服务器线程退出
- `thread.join()` 返回，线程结束

此机制无需超时处理，线程几乎立即退出。