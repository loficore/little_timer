# Little Timer 重构计划 - 习惯养成为核心

## 目标
将计时器应用重构为**习惯养成应用**，计时为基础功能，习惯养成为核心。

## 确认的配置
- 默认轮次：0（不需要轮次）
- 番茄钟时长：25 分钟
- 休息时长：5 分钟

---

## Phase 1: 数据与后端 ✅ 已完成

### 1.1 删除 WorldClock 模式（后端）✅
- [x] 删除 WorldClock 相关代码 `src/core/clock.zig`
- [x] 删除 WorldClock 设置相关代码
- [x] 删除 WorldClock API 路由

### 1.2 添加轮次/休息逻辑（后端）✅
- [x] 扩展 TimerState 添加 `rounds`, `rounds_completed`, `rest_seconds`, `is_resting`（已内置在 countdown 逻辑中）
- [x] 添加 `/api/start` 支持 habit_id 参数
- [x] 添加 `/api/timer/rest` - 开始休息

### 1.3 完善 habit CRUD API ✅
- [x] 后端已有基本 CRUD（待完善 PUT）

---

## Phase 2: 前端核心 ✅ 已完成

### 2.1 重构 App.tsx - 路由状态 + 底部导航 ✅
- [x] 添加页面状态：`home` | `habits` | `timer` | `stats` | `settings`
- [x] 添加选中状态：`selectedSetId`, `selectedHabitId`
- [x] 实现底部导航栏（首页/统计/设置）

### 2.2 新建 HabitListPage - 习惯集+习惯列表 ✅
- [x] 显示习惯集卡片列表
- [x] 点击习惯集 → 进入习惯列表
- [x] 点击习惯 → 进入计时页
- [x] 按钮 → 创建新习惯集（待完善弹窗）

### 2.3 新建 TimerPage - 习惯计时（含休息）✅
- [x] 显示当前习惯名称、颜色
- [x] 计时器（正计时/倒计时）
- [ ] 完成时自动记录 session
- [x] 休息按钮（5分钟倒计时）
- [ ] SSE 接收 habit_id 同步

### 2.4 改造 ModeSelector - 只保留正/倒计时 ✅
- [x] 删除 WorldClock 选项
- [x] 默认倒计时 25 分钟

### 2.5 改造 Header ✅
- [x] 支持返回按钮（返回上一级）
- [x] 计时页显示当前习惯名称

---

## Phase 3: 管理与统计

### 3.1 新建 HabitManagePage - 创建/编辑习惯 ✅（弹窗实现）
- [x] 创建习惯集表单（弹窗）
- [x] 创建习惯表单（名称、目标时间/次数、颜色）
- [ ] 编辑/删除习惯（待完善）
- [ ] 编辑/删除习惯集（待完善）

### 3.2 改造 StatsPage - 按习惯查看 ✅
- [x] 按习惯筛选统计
- [ ] 连续天数（streak）计算
- [ ] 完成率饼图

### 3.3 改造 SettingsPage ✅
- [x] 保留主题、语言
- [x] 保留番茄钟时长、休息时长设置

---

## Phase 4: 优化

### 4.1 SSE 事件增加 habit_id ⏳ 待开始
- [ ] 后端 SSE 事件携带 habit_id

### 4.2 移动端底部导航适配 ✅
- [x] 适配不同屏幕尺寸（btm-nav-md）
- [x] 隐藏/显示逻辑

### 4.3 动画与交互优化 ✅
- [x] 页面切换动画（已有基础）
- [x] 完成时的庆祝动画（已有音效）

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
                    │
                    v
习惯列表：
┌─────────────────────┐
│ < 返回   学习习惯集  │
├─────────────────────┤
│ 🔵 背单词    25分钟 │
│    今日 15/25 分钟  │
├─────────────────────┤
│ 🟢 听写    20分钟   │
│    今日 0/20 分钟  │
└─────────────────────┘
[+] 添加习惯

                    │
                    v
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

## API 端点设计

| 方法 | 路径 | 描述 |
|------|------|------|
| GET | `/api/habit-sets` | 获取习惯集列表 |
| POST | `/api/habit-sets` | 创建习惯集 |
| PUT | `/api/habit-sets/:id` | 更新习惯集 |
| DELETE | `/api/habit-sets/:id` | 删除习惯集 |
| GET | `/api/habits` | 获取习惯列表（可按 set_id 筛选） |
| POST | `/api/habits` | 创建习惯 |
| PUT | `/api/habits/:id` | 更新习惯 |
| DELETE | `/api/habits/:id` | 删除习惯 |
| GET | `/api/sessions` | 获取打卡记录 |
| POST | `/api/sessions` | 创建打卡记录 |
| POST | `/api/start` | 开始计时（带 habit_id） |
| POST | `/api/pause` | 暂停计时 |
| POST | `/api/reset` | 重置计时 |
| POST | `/api/timer/rest` | 开始休息（5分钟） |

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
    goal_count INTEGER NOT NULL DEFAULT 1,
    color TEXT NOT NULL,
    created_at INTEGER NOT NULL,
    FOREIGN KEY (set_id) REFERENCES habit_sets(id)
);

-- 打卡记录
CREATE TABLE sessions (
    id INTEGER PRIMARY KEY,
    habit_id INTEGER NOT NULL,
    duration_seconds INTEGER NOT NULL,
    count INTEGER NOT NULL DEFAULT 1,
    date TEXT NOT NULL,
    created_at INTEGER NOT NULL,
    FOREIGN KEY (habit_id) REFERENCES habits(id)
);
```

---

## 备注
- 现有 presets 数据不需要迁移
- 计时模式只保留正计时和倒计时
- 轮次默认 0（简单模式）
- 已完成：Zig 后端构建成功、前端 TypeScript 检查通过、前端构建成功
