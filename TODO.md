# Little Timer - 待办清单

## 后端 API 实现

- [x] 实现习惯更新 API (PUT /api/habits/:id)
- [x] 实现习惯删除 API (DELETE /api/habits/:id)
- [x] 实现习惯集更新 API (PUT /api/habit-sets/:id)
- [x] 实现习惯集删除 API (DELETE /api/habit-sets/:id)

## 计时器集成

- [x] SSE 事件携带 habit_id
- [x] 完成时自动记录 session

## 统计功能

- [x] 连续天数（streak）计算
- [x] 完成率饼图

---

## 测试更新

### 后端 (Zig)

- [x] 删除 `src/test/test_settings_presets.zig`
- [x] 修改 `test_settings.zig` - 移除 presets 相关测试
- [x] 修改 `test_clock.zig` - 移除 WORLD_CLOCK_MODE 相关测试
- [x] 修改 `test_boundary_conditions.zig` - 移除 world_clock 配置

### 前端 (TypeScript)

- [x] 修改 `ModeSelector.test.tsx` - 移除世界时钟期望，改为 2 列布局
- [x] 修改 `TimeDisplay.test.tsx` - 更新样式类名
- [x] 修改 `utils.test.ts` - 移除世界时钟格式化测试
- [x] 修复测试环境问题 - mock 图标和 i18n

## 前端 UI 重构

### 阶段 1：API 客户端扩展

- [x] 添加 updateHabitSet() 方法
- [x] 添加 deleteHabitSet() 方法
- [x] 添加 updateHabit() 方法

### 阶段 2：侧边栏组件

- [x] 创建 Sidebar.tsx 组件
- [x] 桌面端显示左侧 240px 侧边栏
- [x] 移动端隐藏，使用底部导航

### 阶段 3：App.tsx 重构

- [x] 移除 home 页面
- [x] 保留 habits、stats、settings 三个页面
- [x] 添加响应式导航切换

### 阶段 4：习惯管理页面

- [x] 创建 HabitsPage.tsx
- [x] 习惯集卡片列表（可展开）
- [x] 习惯列表（可编辑/删除）
- [x] 点击习惯进入计时

### 阶段 5：HabitModal 支持编辑

- [x] 支持 editData 属性
- [x] 创建/编辑模式切换
- [x] 预填充表单数据
