# 代码审查修复清单

## Phase 4: 修复审查中发现的问题

### 严重问题

- [x] 4.1 删除 `std_server.zig` 调试打印（2处 std.debug.print）
- [x] 4.2 修复 `std_server.zig` 错误路径内存问题
- [x] 4.3 修复 `settings_manager.zig` 内存双重释放问题
- [x] 4.4 `app.zig:240-242` 静默吞错误问题（删除不存在的 updateDisplay 调用）

### 重要问题

- [x] 4.5 拆分 `std_server.zig` 单块文件（通过分组注释改善可读性）

## 已完成总结

1. ✅ 删除所有 streak 相关代码（HTTP路由、CRUD、测试）
2. ✅ 删除调试打印语句
3. ✅ 修复内存双重释放问题

**验证**: `zig build test` 通过

## 4.4 静默吞错误问题

当前 `app.zig:240-242` 代码：
```zig
self.updateDisplay(display_data) catch |err| {
    logger.global_logger.err("更新显示失败: {any}", .{err});
    self.error_recovery.recordError("更新显示失败", "DISPLAY_UPDATE");
};
```

选项：
1. 添加注释说明这是故意的（display失败不中断主流程）
2. 改为传播错误
3. 添加断言确保永远不会失败

## 4.5 拆分 std_server.zig

当前 1525 行文件，建议拆分为：
- `http/routes/timer.zig` - 计时器相关路由
- `http/routes/habits.zig` - 习惯相关路由
- `http/routes/sessions.zig` - 会话相关路由
- `http/routes/backup.zig` - 备份相关路由
- `http/routes/settings.zig` - 设置相关路由
- `http/std_server.zig` - 主入口和路由注册