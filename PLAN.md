# Little Timer 开发计划

## 目标
将计时器应用重构为**习惯养成应用**，计时为基础功能，习惯养成为核心。

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

### Phase 9.1: 壁纸显示逻辑修复（进行中 🔄）

**问题**：选择壁纸后，CSS 默认黑色渐变和用户壁纸叠加在一起，不是"选择哪个就显示哪个"。

**修复方案**：将壁纸从 body 改为应用到 html 元素，直接覆盖 CSS 样式。

- [ ] App.tsx - 壁纸应用到 html 元素而非 body
- [ ] globals.css - 添加 has-wallpaper 类用于禁用默认背景
- [ ] 验证：选择无/渐变/纯色/图片各自正确显示

### Phase 8: UI 美化与动画统一 ⏳

- [ ] 背景渐变层次
- [ ] 微质感按钮样式
- [ ] 卡片阴影层次
- [ ] 时间显示发光效果
- [ ] 统一过渡动画

### Phase 11: 统一 Material You 组件样式

- [ ] 创建统一表单控件样式（.my-input, .my-select）
- [ ] 创建统一徽章样式（.my-badge-*）
- [ ] 创建统一次要按钮样式（.my-btn-secondary）
- [ ] 替换所有 DaisyUI 表单组件
- [ ] 替换所有 DaisyUI 徽章
- [ ] 替换所有 DaisyUI 次要按钮
- [ ] 统一卡片样式

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