# Little Timer - 开发计划清单

## 📌 方向调整（2026-03-23）

> 目标：从纯计时器转型为 **习惯追踪应用（Habit Tracker）**，核心是记录每日目标完成情况，类似番茄TODO。

---

## 第一阶段：数据模型重构

### 后端 - SQLite 表结构

- [ ] **创建 habit_sets 表**
  - id, name, description, color, created_at

- [ ] **创建 habits 表**
  - id, set_id, name, goal_seconds, goal_count, color, created_at

- [ ] **创建 sessions 表**
  - id, habit_id, duration_seconds, count, started_at, date

- [ ] **删除 presets 相关代码**
  - 删除 presets 表
  - 删除预设 API 端点

### 前端 - 数据类型

- [ ] 创建 `types/habit.ts` - Habit, HabitSet, Session 类型
- [ ] 更新 `apiClient.ts` - 添加习惯相关 API 方法

---

## 第二阶段：后端 API

### 习惯集 API

- [ ] `GET /api/habit-sets` - 获取所有习惯集
- [ ] `POST /api/habit-sets` - 创建习惯集
- [ ] `PUT /api/habit-sets/:id` - 更新习惯集
- [ ] `DELETE /api/habit-sets/:id` - 删除习惯集

### 习惯 API

- [ ] `GET /api/habits` - 获取所有习惯（可按 set_id 过滤）
- [ ] `POST /api/habits` - 创建习惯
- [ ] `PUT /api/habits/:id` - 更新习惯
- [ ] `DELETE /api/habits/:id` - 删除习惯

### 记录 API

- [ ] `POST /api/sessions` - 记录专注 session
- [ ] `GET /api/sessions` - 获取记录（支持日期范围过滤）
- [ ] `GET /api/stats` - 获取统计数据

### 计时器 API（复用现有）

- [ ] `POST /api/start` - 启动计时（新增 habit_id 参数）
- [ ] `POST /api/pause` - 暂停计时
- [ ] `POST /api/reset` - 重置计时

---

## 第三阶段：前端 - 习惯管理

### 依赖安装

- [ ] 安装 ApexCharts (`npm install apexcharts`)
- [ ] 安装 Preact 适配 (`npm install preact/compat`)

### 首页重构

- [ ] **习惯集列表组件** - HabitSetList
- [ ] **习惯卡片组件** - HabitCard（含进度条）
- [ ] **今日进度摘要** - 顶部统计卡片
- [ ] **新建/编辑弹窗** - Modal 组件
- [ ] **首页路由** - `/` 显示习惯列表

---

## 第四阶段：计时器集成

### 习惯选择界面

- [ ] **习惯选择器** - 计时前选择要记录的习惯
- [ ] 快捷选择今日未完成习惯

### 计时器页面

- [ ] **显示当前习惯** - 顶部显示正在计时的习惯
- [ ] **完成记录** - 计时结束自动创建 session

---

## 第五阶段：统计页面

### 图表组件

- [ ] **时间范围选择器** - 今天/本周/本月/自定义
- [ ] **饼图** - 各习惯时间分布
- [ ] **柱状图** - 每日专注时长趋势
- [ ] **统计卡片** - 连续天数、总时长、完成率

### 统计 API

- [ ] `GET /api/stats/daily` - 按日统计
- [ ] `GET /api/stats/weekly` - 按周统计
- [ ] `GET /api/stats/monthly` - 按月统计

---

## 第六阶段：UI/UX 优化

### 现代化设计

- [ ] **卡片式布局** - 习惯卡片带阴影和圆角
- [ ] **渐变色点缀** - 按钮、进度条使用品牌色
- [ ] **空白状态** - 无习惯时显示引导
- [ ] **动画过渡** - 完成、删除时有流畅动画
- [ ] **暗色主题适配** - 图表、组件适配暗色

### 响应式

- [ ] 移动端布局适配
- [ ] 触摸友好按钮尺寸

---

## 第七阶段：清理与发布

- [ ] 移除旧 presets 相关代码
- [ ] 清理旧计时模式相关代码（如 stopwatch, world_clock）
- [ ] 测试覆盖
- [ ] 文档完善

---

## 附录：数据结构

### HabitSet（习惯集）
```
id: number
name: string        // "早起习惯包"
description: string // 可选描述
color: string       // 主题色
created_at: date
```

### Habit（习惯）
```
id: number
set_id: number      // 所属习惯集
name: string        // "每天学习"
goal_seconds: number // 目标时间（秒），0 表示不设限
goal_count: number  // 目标次数，0 表示不设限
color: string       // 展示颜色
created_at: date
```

### Session（记录）
```
id: number
habit_id: number
duration_seconds: number // 本次专注时长
count: number          // 本次完成的次数
started_at: datetime
date: string          // YYYY-MM-DD
```

---

## 验收标准

1. 可以创建习惯集和习惯
2. 可以开始计时并关联习惯
3. 计时完成后自动记录 session
4. 可以在统计页查看时间分布和趋势
5. UI 现代化，符合暗色主题
