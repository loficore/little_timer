# 测试计划

## 测试统计

| 类型 | 测试数 | 状态 |
|------|--------|------|
| Zig 后端单元测试 | ~200 | ✅ |
| 前端单元测试 | 460+ | ✅ |
| 前端 E2E 测试 | 12 | ✅ |
| **总计** | **~672** | ✅ |

---

## 测试文件清单

### Zig 后端测试 (`src/test/`)

1. `test_clock.zig` - ClockManager 核心逻辑
2. `test_app.zig` - MainApplication
3. `test_http_server.zig` - HTTP 服务器
4. `test_storage.zig` - Storage CRUD
5. `test_settings.zig` - Settings 管理
6. `test_boundary_conditions.zig` - 边界条件
7. `test_error_recovery.zig` - 错误恢复
8. `test_migration.zig` - 数据库迁移
9. `test_storage_backup.zig` - 备份功能
10. `test_storage_health.zig` - 健康检查
11. `test_settings_validator.zig` - 设置验证
12. `test_logger.zig` - 日志模块
13. `test_habit_crud.zig` - Habit CRUD
14. `test_http_edge.zig` - HTTP 边界
15. `test_timer_transition.zig` - Timer 状态转换
16. `test_sse_edge.zig` - SSE/ClockManager
17. `test_storage_error.zig` - 存储错误处理
18. `test_migration_edge.zig` - 迁移边界
19. `test_session_edge.zig` - Session 边界

### 前端组件测试 (`assets/src/test/components/`)

1. `BasicSettings.test.tsx`
2. `Button.test.tsx`
3. `CheckboxInput.test.tsx`
4. `ControlPanel.test.tsx`
5. `CountdownSettings.test.tsx`
6. `DropdownSelect.test.tsx`
7. `ErrorNotification.test.tsx`
8. `FormGroup.test.tsx`
9. `FormSection.test.tsx`
10. `HabitPicker.test.tsx`
11. `Header.test.tsx`
12. `ModeSelector.test.tsx`
13. `NumberInput.test.tsx`
14. `PickerNumberInput.test.tsx` ⭐ 新增
15. `SelectInput.test.tsx`
16. `SevenSegmentDisplay.test.tsx`
17. `SettingItem.test.tsx`
18. `Sidebar.test.tsx`
19. `StatusBadge.test.tsx`
20. `TabPanel.test.tsx`
21. `TimeDisplay.test.tsx`
22. `TimerConfig.test.tsx`
23. `TimerControls.test.tsx`
24. `TimerProgress.test.tsx`
25. `WallpaperSelector.test.tsx` ⭐ 新增
26. `WorldClockSettings.test.tsx` ⭐ 新增
27. `StopwatchSettings.test.tsx` ⭐ 新增
28. `HabitModal.test.tsx` ⭐ 新增

### 前端页面测试 (`assets/src/test/`)

1. `App.test.tsx` ⭐ 新增
2. `TimerPage.test.tsx` ⭐ 新增
3. `HabitsPage.test.tsx` ⭐ 新增
4. `SettingsPage.test.tsx` ⭐ 新增

### 前端 Hook 测试 (`assets/src/test/hooks/`)

1. `useHabits.test.ts`
2. `useHabitsExtension.test.ts`
3. `useSSE.test.ts`
4. `useSSEExtension.test.ts`
5. `useSettings.test.ts`
6. `useTimer.test.ts`
7. `useTimerExtension.test.ts`

### 前端工具测试 (`assets/src/test/utils/`)

1. `apiClient.test.ts`
2. `audio.test.ts`
3. `formatters.test.ts`
4. `i18n.test.ts`
5. `logger.test.ts`
6. `constants.test.ts`
7. `sseClient.test.ts`
8. `utils.test.ts`

### 前端集成测试 (`assets/src/test/`)

1. `timer-flow.test.ts`

### 前端 E2E 测试 (`assets/src/test/visual/`)

1. `timer.spec.ts`
2. `habits.spec.ts`
3. `navigation.spec.ts`
4. `settings.spec.ts`
5. `habits_crud.spec.ts`

---

## 待改进项目

### 页面级测试 Mock 优化

App、TimerPage、HabitsPage、SettingsPage 的测试存在 mock 复杂性导致的失败。需要优化 mock 策略或使用更轻量的集成测试方法。

### 待完成项目

- [ ] 设置保存与加载 E2E 测试
- [ ] 设置页面 VRT 快照
- [ ] i18n 完整覆盖测试
