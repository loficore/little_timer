# Little Timer 开发计划

## 目标

将计时器应用重构为**习惯养成应用**，计时为基础功能，习惯养成为核心。

### 性能优化目标 (Vercel React Best Practices)

按照 Vercel React Best Practices 优化前端代码，提升性能、可维护性和模块化程度。

#### 问题分析

1. **代码重复**
   - `formatDuration` 在 Homepage.tsx, TimerPage.tsx, HabitsPage.tsx, Stats.tsx 中重复定义
   - API 客户端在每个组件中独立实例化

2. **重新渲染问题** (rerender-*)
   - 大量内联函数和对象作为 props
   - 缺少 memo/useCallback 优化
   - 组件内定义组件 (rerender-no-inline-components)

3. **Bundle Size 问题** (bundle-*)
   - 所有组件一次性加载
   - apexcharts 库全量导入

4. **数据获取问题** (client-*)
   - 每个组件独立创建 APIClient 实例
   - 没有请求去重和缓存

5. **模块化不足**
   - `TimerPage.tsx` 超过 700 行，职责过多

---

## 核心概念

### 习惯目标
**习惯目标 = goal_seconds（累计时长）**
- 例如：每周背单词目标 180 分钟
- 只要启动计时并关联这个习惯，计时时长就计入进度
- 进度 = 今天累计时长 / 目标时长

### 计时模式
- **正计时**: 累计计时，直到手动停止
- **倒计时**: 设定时长，计时完成后停止
- 两种模式都为习惯进度服务，而非独立功能

---

## 已完成功能

### Phase 1: 数据与后端 ✅

- [x] 删除 WorldClock 模式
- [x] 添加轮次/休息逻辑
- [x] 完善 habit CRUD API
- [x] 废弃 presets 相关代码
- [x] 废弃 goal_count 字段

### Phase 2: 前端核心 ✅

- [x] 底部导航栏（首页/统计/设置）
- [x] 习惯集卡片列表
- [x] 创建习惯集/习惯弹窗（支持小时+分钟输入）
- [x] TimerPage（含休息功能）
- [x] 移除 180 分钟上限（最大 9999 小时）

### Phase 3: 测试更新 ✅

- [x] 删除已废弃的 presets 测试文件
- [x] 更新 clock 测试（移除世界时钟）
- [x] 更新 settings 测试（移除 presets）
- [x] 更新前端组件测试（移除世界时钟相关期望）

### Phase 4: 习惯详情与计时集成 ✅

- [x] 实现习惯详情 API (GET /api/habits/:id/detail)
- [x] 计时页显示习惯进度条
- [x] 桌面通知（计时完成时）
- [x] 主页重构为计时页（TimerPage）

### Phase 7: 自定义壁纸功能 ✅

- [x] 全局壁纸设置（颜色/渐变/图片 URL）- SQLite settings.wallpaper
- [x] 习惯集壁纸 - habit_sets.wallpaper
- [x] 习惯壁纸 - habits.wallpaper
- [x] 后端 API 已支持（GET/POST /api/settings, PUT /api/habit-sets/:id, PUT /api/habits/:id）
- [x] 前端独立设置页面
- [x] 全局壁纸和习惯壁纸解耦（已完成，但有 bug 已修复）
- [x] 背景渲染优先级：习惯卡片封面 > 全局壁纸 > 默认

> **Bug 修复**：settings_manager.zig 未解析 wallpaper 字段，Settings.tsx 未加载 wallpaper - 已修复

---

## 实施阶段

### Phase 1: 工具层统一化 ⏳

- [ ] 创建 `src/utils/formatters.ts` - 统一格式化函数 (formatDuration 等)
- [ ] 创建 `src/utils/constants.ts` - 统一常量定义
- [ ] 统一 API 客户端实例化模式 (单例模式)

### Phase 2: 数据层抽象 (Hooks) ⏳

- [ ] 创建 `src/hooks/useTimer.ts` - 计时器状态管理
- [ ] 创建 `src/hooks/useHabits.ts` - 习惯数据管理
- [ ] 创建 `src/hooks/useSSE.ts` - SSE 连接管理
- [ ] 创建 `src/hooks/useSettings.ts` - 设置管理

### Phase 3: 组件拆分 (TimerPage) ⏳

- [ ] 拆分出 `TimerControls.tsx` - 控制按钮组件
- [ ] 拆分出 `TimerConfig.tsx` - 计时配置面板
- [ ] 拆分出 `HabitPicker.tsx` - 习惯选择器
- [ ] 拆分出 `TimerProgress.tsx` - 进度显示组件

### Phase 4: 性能优化 ⏳

- [ ] 全局 memo 策略 - 确保所有组件使用 memo 包裹
- [ ] 提取内联 SVG 为独立组件 (Sidebar.tsx icons)
- [ ] 动态导入 apexcharts (bundle-dynamic-imports)
- [ ] 优化状态派生 - 减少不必要的 useEffect

### Phase 5: 目录结构重组 ⏳

```
src/
├── pages/           # 页面组件
├── components/      # 通用组件
├── hooks/           # 自定义 hooks
├── utils/           # 工具函数
└── types/           # 类型定义
```

---

## 验收标准

1. [ ] 消除所有 `formatDuration` 重复定义
2. [ ] TimerPage 组件代码行数控制在 300 行以内
3. [ ] 所有 API 调用通过 hooks 管理
4. [ ] 动态导入 apexcharts
5. [ ] 通过 `bun run build:check` 类型检查
6. [ ] 通过 `bun run lint` 代码规范检查

### 12.1 修复选择习惯弹窗样式

**问题**: 点击"选择习惯"后弹窗背景与全局背景不协调，可能导致页面变红

**修复**: TimerPage.tsx 的 showHabitPicker 弹窗改为毛玻璃风格

- [x] 修改弹窗 className 使用 `my-surface-card`

### 12.2 美化计时选项下拉框

**问题**: 计时模式 `<select>` 使用默认样式，与设计风格不统一

**修复**: 应用已有的 `timer-option-control` 样式类

- [x] TimerPage.tsx - 验证 `<select>` 使用正确样式类

### 12.3 修复 i18n 缺少翻译

**问题**: 统计页面顶栏显示 "stats.title" 字面量，而非"统计"

**修复**: 在 i18n/zh.toml 添加缺失的翻译

- [x] 添加 `[stats]` 段落和 `title = "统计"`

### 12.4 修复统计页面时间分布图表

**问题**: 统计页面时间分布图表区域空白

**修复**: 检查 Stats.tsx 中 pie chart 渲染逻辑

- [x] 后端 handleGetSessions 使用 `request.params` 获取 query string，改为使用 `request.query()` 方法
- [x] 前端添加 `habits.length > 0` 条件检查

- [ ] 背景渐变层次
- [ ] 微质感按钮样式
- [ ] 卡片阴影层次
- [ ] 时间显示发光效果
- [ ] 统一过渡动画

## Phase 11: 统一 Material You 组件样式

- [ ] 创建统一表单控件样式（.my-input, .my-select）
- [ ] 创建统一徽章样式（.my-badge-*）
- [ ] 创建统一次要按钮样式（.my-btn-secondary）
- [ ] 替换所有 DaisyUI 表单组件
- [ ] 替换所有 DaisyUI 徽章
- [ ] 替换所有 DaisyUI 次要按钮
- [ ] 统一卡片样式

---

## Phase 13: 前端 UI 样式修复 ✅

### 问题1：自定义时间选择组件

- [x] 创建自定义数字选择器组件，替代原生 input
- [x] 在 TimeInput.tsx、NumberInput.tsx、TimerConfig.tsx、TimerPage.tsx、HabitModal.tsx 中使用

### 问题2：毛玻璃效果缺失

- [x] TimerPage - 选择习惯按钮增强毛玻璃效果
- [x] TimerPage - 时间选择容器添加毛玻璃效果
- [x] TimerConfig - 配置面板添加毛玻璃效果

### 问题3：深色遮罩问题

- [x] HabitPicker 移除深色遮罩
- [x] HabitModal 移除深色遮罩  
- [x] TimerPage habit picker 移除深色遮罩

---

## Phase 15: 弹窗焦点区域背景透明度修复

### 15.1 问题描述

**问题**: 习惯选择框 (HabitPicker) 和习惯编辑/新建框 (HabitModal) 的背景太透明，导致在焦点区域内的文字难以识别。

**现状**:
- 弹窗使用 `my-surface-card` 类，背景透明度为 0.66
- 弹窗遮罩层使用 `rgba(0,0,0,0.2)` + `blur(4px)`

### 15.2 修复方案

- [x] 在 `globals.css` 中创建 `my-surface-modal` 类（更高不透明度，如 0.85-0.9）
- [x] 更新 `HabitPicker.tsx` 使用新样式
- [x] 更新 `HabitModal.tsx` 使用新样式
- [x] 更新 `TimerPage.tsx` habit picker 使用新样式
- [x] 更新 `HabitsPage.tsx` delete confirm 弹窗使用新样式
- [x] 验证视觉效果

---

## Phase 12: 测试扩展 ✅

### 后端测试 (12/12 模块覆盖) ✅

| 模块 | 文件 | 状态 |
|------|------|------|
| 时钟 | `test_clock.zig` | ✅ 完成 |
| 设置 | `test_settings.zig` | ✅ 完成 |
| 错误恢复 | `test_error_recovery.zig` | ✅ 完成 |
| 日志 | `test_logger.zig` | ✅ 完成 |
| 设置校验 | `test_settings_validator.zig` | ✅ 完成 |
| 边界条件 | `test_boundary_conditions.zig` | ✅ 完成 |
| HTTP 服务器 | `test_http_server.zig` | ✅ 完成 |
| 存储层 | `test_storage.zig` | ✅ 完成 |
| 主应用 | `test_app.zig` | ✅ 完成 |
| 数据迁移 | `test_migration.zig` | ✅ 完成 |
| CRUD | `habit_crud.zig` | ✅ 完成 |
| 备份恢复 | `storage_backup.zig` | ✅ 完成 |
| 健康检查 | `storage_health.zig` | ✅ 完成 |
| 预设管理 | `settings_presets.zig` | ❌ 已废弃 |

### 前端测试补充计划

#### 已覆盖 (23/30+)

| 类型 | 模块 | 状态 |
|------|------|------|
| 组件 | Button, Header, TimeDisplay, TabPanel, SelectInput, NumberInput, CheckboxInput, FormGroup, FormSection, ModeSelector, StatusBadge, ControlPanel, Sidebar, TimerControls, HabitPicker | ✅ 完成 |
| 工具 | formatDuration (统一版), apiClient, formatters, i18n | ✅ 完成 |
| Hooks | useTimer, useSSE, useSettings, useHabits | ✅ 完成 |

#### 待补充

| 优先级 | 类型 | 模块 |
|--------|------|------|
| 中 | 组件 | TimerConfig, TimerProgress, DropdownSelect, SevenSegmentDisplay |
| 低 | 组件 | WallpaperSelector, HabitModal, TimeInput, BasicSettings, ErrorNotification, CountdownSettings, SettingItem 等 |
| 低 | 页面 | Homepage, Settings, TimerPage, HabitsPage, Stats |
| 低 | 工具 | audio.ts, share.ts |

### Phase 5: 倒计时模式 ⏳

- [ ] 添加模式选择（正计时/倒计时）
- [ ] 倒计时参数设置（工作时间、休息时间、重复次数）
- [ ] 前端倒计时逻辑实现
- [ ] 轮次和休息逻辑

### Phase 6: 数据导出/备份 ⏳

- [ ] 导出 API (GET /api/export)
- [ ] 导入 API (POST /api/import)
- [ ] 备份 API (POST /api/backup)
- [ ] 前端设置页集成

---

## Phase 16: WebView 退出时资源清理 ✅

### 问题描述

当用户在 WebView 窗口中点击原生退出按钮时，程序无法完全退出，后端 HTTP 服务器继续占用端口 8080。

### 解决方案

使用 `std.atomic.Bool` 作为退出标志，主线程在 `webview.run()` 返回后检查标志并清理资源。

### 已完成

- [x] 在 `src/core/app.zig` 的 `MainApplication` 结构体中添加 `should_exit: std.atomic.Bool` 字段
- [x] 修改 `stop()` 方法设置退出标志
- [x] 在 `win.run()` 返回后显式调用 `stop()` + `join()`
- [x] `zig build test` 通过

### 执行流程

```
用户点击退出
    ↓
WebView 窗口关闭
    ↓
win.run() 返回
    ↓
main_app.stop() → 设置 should_exit + 停止 HTTP 服务器
    ↓
app_thread.join() → 等待 HTTP 线程结束
    ↓
main_app.deinit() → 清理所有资源
    ↓
allocator.destroy(main_app)
    ↓
进程退出 ✓
```

### 验收标准

- [ ] 点击 WebView 原生退出按钮后，进程完全退出
- [ ] 端口 8080 被释放，可重新启动
- [ ] 无内存泄漏（通过 GPA 检测）
- [ ] `zig build test` 通过

---

## 数据库表

```sql
-- 习惯集
CREATE TABLE habit_sets (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT,
    color TEXT NOT NULL,
    created_at INTEGER NOT NULL
);

-- 习惯
CREATE TABLE habits (
    id INTEGER PRIMARY KEY,
    set_id INTEGER NOT NULL,
    name TEXT NOT NULL,
    goal_seconds INTEGER NOT NULL DEFAULT 1500,
    color TEXT NOT NULL,
    created_at INTEGER NOT NULL,
    FOREIGN KEY (set_id) REFERENCES habit_sets(id)
);

-- 打卡记录
CREATE TABLE sessions (
    id INTEGER PRIMARY KEY,
    habit_id INTEGER NOT NULL,
    duration_seconds INTEGER NOT NULL,
    date TEXT NOT NULL,
    created_at INTEGER NOT NULL,
    FOREIGN KEY (habit_id) REFERENCES habits(id)
);
```

---

## API 端点

| 方法 | 路径 | 状态 |
|------|------|------|
| GET | `/api/habit-sets` | ✅ |
| POST | `/api/habit-sets` | ✅ |
| PUT | `/api/habit-sets/:id` | ✅ |
| DELETE | `/api/habit-sets/:id` | ✅ |
| GET | `/api/habits` | ✅ |
| POST | `/api/habits` | ✅ |
| PUT | `/api/habits/:id` | ✅ |
| DELETE | `/api/habits/:id` | ✅ |
| GET | `/api/habits/:id/detail` | ✅ |
| GET | `/api/habits/:id/streak` | ✅ |
| GET | `/api/sessions` | ✅ |
| POST | `/api/sessions` | ✅ |
| POST | `/api/start` | ✅ |
| POST | `/api/pause` | ✅ |
| POST | `/api/reset` | ✅ |
| POST | `/api/timer/rest` | ✅ |
| GET | `/api/export` | ⏳ |
| POST | `/api/import` | ⏳ |
| POST | `/api/backup` | ⏳ |

---

## 页面流程图

```
┌─────────────────────────────────────────────────────────────┐
│                      底部导航栏                             │
│   [首页]                    [统计]                [设置]   │
└─────────────────────────────────────────────────────────────┘

首页：
┌─────────────────────┐    ┌─────────────────────┐
│ 🎯 学习习惯集      > │    │ 💪 运动习惯集      > │
│   3 个习惯          │    │   2 个习惯          │
└─────────────────────┘    └─────────────────────┘

习惯列表：
┌─────────────────────┐
│ < 返回   学习习惯集  │
├─────────────────────┤
│ 🔵 背单词    2h 30m  │
│    今日 15/180 分钟  │
├─────────────────────┤
│ 🟢 听写    1h        │
│    今日 0/60 分钟   │
└─────────────────────┘
[+] 添加习惯

计时页：
┌─────────────────────┐
│ < 返回     🔵 背单词│
│                     │
│      00:15:32       │
│                     │
│   [暂停]  [重置]    │
│                     │
│  休息: [5:00]       │
└─────────────────────┘

统计页：
┌─────────────────────┐
│ 今日   本周   本月  │
├─────────────────────┤
│ 总专注: 2h 30m      │
│ 完成次数: 5         │
├─────────────────────┤
│ [时间分布饼图]      │
├─────────────────────┤
│ [每日趋势柱状图]    │
└─────────────────────┘
```

---

## 技术架构

### 后端 (Zig)
- HTTP 服务器 (httpz)
- SQLite 持久化 (zqlite)
- SSE 实时推送

### 前端 (Preact + TypeScript)
- 组件化架构
- Tailwind CSS 样式
- Vite 构建

---

## 配置参数

- 默认轮次：0（不需要轮次）
- 番茄钟时长：25 分钟
- 休息时长：5 分钟
- 计时模式：只保留正计时和倒计时
- 目标时长上限：9999 小时

---

## 参考项目

- 番茄TODO类应用的功能设计
- 习惯追踪的核心指标：累计时长、连续天数、完成率