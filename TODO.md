# 删除连胜功能 - 执行清单

## Phase 1: 删除 HTTP 路由和处理器

- [x] 1.1 删除 `std_server.zig` 中 `handleGetHabitStreak` 函数
- [x] 1.2 从 `handleRequest` 中删除 `/api/habits/*/streak` 路由匹配
- [x] 1.3 从 `handleGetHabitDetail` 移除 streak 相关代码

## Phase 2: 删除 CRUD 实现

- [x] 2.1 删除 `habit_crud.zig` 中 `getHabitStreak()` 函数

## Phase 3: 更新测试

- [x] 3.1 删除 `test_http_edge.zig` 中 streak 相关的 parsePathIdWithSuffix 测试
- [x] 3.2 删除 `test_habit_crud.zig` 中 "记录 CRUD - 获取习惯连胜" 测试
- [x] 3.3 删除 `test_storage_error.zig` 中 streak 相关代码

## Phase 4: 修复审查中发现的问题

### 严重问题

- [x] 4.1 删除 `std_server.zig` 调试打印（2处 std.debug.print）
- [x] 4.2 修复 `std_server.zig` 错误路径内存问题
- [x] 4.3 修复 `settings_manager.zig` 内存双重释放问题

### 未完成

- [ ] 4.4 `app.zig:240-242` 静默吞错误问题（暂未修改）
- [ ] 4.5 拆分 `std_server.zig` 单块文件（需更大重构）

## 已完成总结

1. ✅ 删除所有 streak 相关代码（HTTP路由、CRUD、测试）
2. ✅ 删除调试打印语句
3. ✅ 修复内存双重释放问题

**验证**: `zig build test` 通过

## 验证

- [ ] 运行 `zig build test` 确保无编译错误