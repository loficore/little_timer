# Little Timer - 待办清单

## 阶段 6：习惯详情与计时集成 ✅

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

## Bug 修复：壁纸显示逻辑混乱（叠加问题）✅

- [x] App.tsx - 壁纸应用到 html 元素而非 body
- [x] 验证：选择无/渐变/纯色/图片各自正确显示
- [x] 验证：刷新页面壁纸持久显示

---

## 阶段 10：UI 美化与动画统一 ✅

- [x] globals.css 添加背景渐变层
- [x] globals.css 添加微质感按钮样式
- [x] globals.css 统一卡片阴影
- [x] TimeDisplay 增加运行发光效果
- [x] Homepage 增加页面过渡动画
- [x] 创建统一的动画类（slideUp, fadeIn, scale）

---

## 阶段 11：统一 Material You 组件样式

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
- [ ] 视觉检查：所有组件符合 Material You 风格