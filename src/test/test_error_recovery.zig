//! 错误恢复模块单元测试
const std = @import("std");
const error_recovery = @import("../core/utils/error_recovery.zig");

// ============ 初始化和基本状态测试 ============

test "ErrorRecoveryManager 初始化" {
    const allocator = std.testing.allocator;
    const manager = error_recovery.ErrorRecoveryManager.init(allocator);

    try std.testing.expectEqual(manager.error_count, 0);
    try std.testing.expect(!manager.is_recovering);
    try std.testing.expectEqual(manager.recovery_attempts, 0);
    try std.testing.expectEqual(manager.last_error_time, 0);
}

// ============ 错误记录测试 ============

test "记录单个错误" {
    const allocator = std.testing.allocator;
    var manager = error_recovery.ErrorRecoveryManager.init(allocator);
    defer manager.deinit();

    manager.recordError("测试错误", "TEST_ERROR");

    try std.testing.expectEqual(manager.error_count, 1);
    try std.testing.expect(!manager.is_recovering); // 单个错误不触发恢复
    try std.testing.expect(manager.last_error_time > 0);
}

test "记录多个错误" {
    const allocator = std.testing.allocator;
    var manager = error_recovery.ErrorRecoveryManager.init(allocator);
    defer manager.deinit();

    manager.recordError("错误1", "TEST_ERROR_1");
    manager.recordError("错误2", "TEST_ERROR_2");
    manager.recordError("错误3", "TEST_ERROR_3");

    try std.testing.expectEqual(manager.error_count, 3);
}

test "记录错误超过阈值触发恢复" {
    const allocator = std.testing.allocator;
    var manager = error_recovery.ErrorRecoveryManager.init(allocator);
    defer manager.deinit();

    // 记录 6 个错误（超过阈值 5）
    for (0..6) |i| {
        var buf: [50]u8 = undefined;
        const msg = std.fmt.bufPrint(&buf, "错误 {d}", .{i}) catch unreachable;
        manager.recordError(msg, "TEST_ERROR");
    }

    try std.testing.expectEqual(manager.error_count, 6);
    try std.testing.expect(manager.is_recovering); // 应该触发恢复
}

test "错误消息截断保护（超长消息）" {
    const allocator = std.testing.allocator;
    var manager = error_recovery.ErrorRecoveryManager.init(allocator);
    defer manager.deinit();

    // 创建一个超长的错误消息（>256字节）
    var long_msg: [500]u8 = undefined;
    @memset(&long_msg, 'A');

    manager.recordError(&long_msg, "TEST_ERROR");

    // 验证消息被正确截断（不应崩溃）
    try std.testing.expectEqual(manager.error_count, 1);
    // last_error_message 应该被截断到最大长度
    try std.testing.expect(manager.last_error_message[255] == 0); // 确保有哨兵
}

test "错误类型分类存储" {
    const allocator = std.testing.allocator;
    var manager = error_recovery.ErrorRecoveryManager.init(allocator);
    defer manager.deinit();

    manager.recordError("显示更新错误", "DISPLAY_UPDATE");
    manager.recordError("时钟事件错误", "CLOCK_EVENT");
    manager.recordError("设置保存错误", "SETTINGS_SAVE");

    try std.testing.expectEqual(manager.error_count, 3);
}

// ============ 恢复机制测试 ============

test "首次恢复尝试" {
    const allocator = std.testing.allocator;
    var manager = error_recovery.ErrorRecoveryManager.init(allocator);
    defer manager.deinit();

    // 触发恢复模式
    for (0..6) |_| {
        manager.recordError("测试错误", "TEST");
    }

    try std.testing.expect(manager.is_recovering);

    const result = manager.attemptRecovery();
    try std.testing.expect(result); // 第一次尝试应该成功
    try std.testing.expectEqual(manager.recovery_attempts, 1);
}

test "恢复尝试指数退避" {
    const allocator = std.testing.allocator;
    var manager = error_recovery.ErrorRecoveryManager.init(allocator);
    defer manager.deinit();

    // 触发恢复模式
    for (0..6) |_| {
        manager.recordError("测试错误", "TEST");
    }

    // 第一次尝试
    _ = manager.attemptRecovery();
    try std.testing.expectEqual(manager.recovery_attempts, 1);

    // 第二次尝试
    _ = manager.attemptRecovery();
    try std.testing.expectEqual(manager.recovery_attempts, 2);

    // 第三次尝试
    _ = manager.attemptRecovery();
    try std.testing.expectEqual(manager.recovery_attempts, 3);
}

test "恢复尝试超过最大次数失败" {
    const allocator = std.testing.allocator;
    var manager = error_recovery.ErrorRecoveryManager.init(allocator);
    defer manager.deinit();

    // 触发恢复模式
    for (0..6) |_| {
        manager.recordError("测试错误", "TEST");
    }

    // 尝试恢复 4 次（超过最大 3 次）
    _ = manager.attemptRecovery(); // 尝试 1
    _ = manager.attemptRecovery(); // 尝试 2
    _ = manager.attemptRecovery(); // 尝试 3
    const result = manager.attemptRecovery(); // 尝试 4（应该失败）

    try std.testing.expect(!result); // 第四次应该失败
    try std.testing.expect(!manager.is_recovering); // 退出恢复模式
    try std.testing.expectEqual(manager.error_count, 0); // 错误计数应该被重置
}

test "恢复成功后重置状态" {
    const allocator = std.testing.allocator;
    var manager = error_recovery.ErrorRecoveryManager.init(allocator);
    defer manager.deinit();

    // 触发恢复模式
    for (0..6) |_| {
        manager.recordError("测试错误", "TEST");
    }

    try std.testing.expect(manager.is_recovering);

    // 标记恢复成功
    manager.markRecoverySuccess();

    try std.testing.expect(!manager.is_recovering);
    try std.testing.expectEqual(manager.recovery_attempts, 0);
    try std.testing.expectEqual(manager.error_count, 0);
}

test "未处于恢复状态时尝试恢复直接返回成功" {
    const allocator = std.testing.allocator;
    var manager = error_recovery.ErrorRecoveryManager.init(allocator);
    defer manager.deinit();

    // 未触发恢复模式
    const result = manager.attemptRecovery();

    try std.testing.expect(result); // 应该返回成功
    try std.testing.expectEqual(manager.recovery_attempts, 0); // 尝试次数不变
}

// ============ 错误统计测试 ============

test "getErrorStats 返回正确统计" {
    const allocator = std.testing.allocator;
    var manager = error_recovery.ErrorRecoveryManager.init(allocator);
    defer manager.deinit();

    manager.recordError("错误1", "TYPE_A");
    manager.recordError("错误2", "TYPE_B");
    manager.recordError("错误3", "TYPE_A");

    const stats = manager.getErrorStats();

    try std.testing.expectEqual(stats.count, 3);
    try std.testing.expect(!stats.is_recovering);
}

test "getErrorStats 在恢复状态下" {
    const allocator = std.testing.allocator;
    var manager = error_recovery.ErrorRecoveryManager.init(allocator);
    defer manager.deinit();

    // 触发恢复模式
    for (0..6) |_| {
        manager.recordError("测试错误", "TEST");
    }

    const stats = manager.getErrorStats();

    try std.testing.expectEqual(stats.count, 6);
    try std.testing.expect(stats.is_recovering);
}

// ============ 清理和重置测试 ============

test "clearErrors 清空错误计数" {
    const allocator = std.testing.allocator;
    var manager = error_recovery.ErrorRecoveryManager.init(allocator);
    defer manager.deinit();

    manager.recordError("错误1", "TEST");
    manager.recordError("错误2", "TEST");

    try std.testing.expectEqual(manager.error_count, 2);

    manager.clearErrors();

    try std.testing.expectEqual(manager.error_count, 0);
    try std.testing.expectEqual(manager.recovery_attempts, 0);
    try std.testing.expect(!manager.is_recovering);
}

test "deinit 正常清理资源" {
    const allocator = std.testing.allocator;
    var manager = error_recovery.ErrorRecoveryManager.init(allocator);

    manager.recordError("测试错误", "TEST");

    // 调用 deinit 不应崩溃
    manager.deinit();
}

// ============ 边界条件测试 ============

test "空错误消息" {
    const allocator = std.testing.allocator;
    var manager = error_recovery.ErrorRecoveryManager.init(allocator);
    defer manager.deinit();

    manager.recordError("", "EMPTY_ERROR");

    try std.testing.expectEqual(manager.error_count, 1);
    try std.testing.expectEqual(manager.last_error_message[0], 0); // 应该是空字符串
}

test "空错误类型" {
    const allocator = std.testing.allocator;
    var manager = error_recovery.ErrorRecoveryManager.init(allocator);
    defer manager.deinit();

    manager.recordError("测试错误", "");

    try std.testing.expectEqual(manager.error_count, 1);
}

test "连续快速记录错误" {
    const allocator = std.testing.allocator;
    var manager = error_recovery.ErrorRecoveryManager.init(allocator);
    defer manager.deinit();

    // 快速连续记录 10 个错误
    for (0..10) |i| {
        var buf: [50]u8 = undefined;
        const msg = std.fmt.bufPrint(&buf, "快速错误 {d}", .{i}) catch unreachable;
        manager.recordError(msg, "RAPID_ERROR");
    }

    try std.testing.expectEqual(manager.error_count, 10);
    try std.testing.expect(manager.is_recovering); // 应该触发恢复
}

test "错误时间戳单调递增" {
    const allocator = std.testing.allocator;
    var manager = error_recovery.ErrorRecoveryManager.init(allocator);
    defer manager.deinit();

    manager.recordError("错误1", "TEST");
    const time1 = manager.last_error_time;

    // 由于时间戳精度很高，直接记录第二个错误
    // 时间戳应该保持单调递增（即使是几乎同时发生的事件）
    manager.recordError("错误2", "TEST");
    const time2 = manager.last_error_time;

    try std.testing.expect(time2 >= time1);
}

// ============ 并发安全测试（可选，如果需要线程安全）============

test "单线程环境下的多次操作" {
    const allocator = std.testing.allocator;
    var manager = error_recovery.ErrorRecoveryManager.init(allocator);
    defer manager.deinit();

    // 模拟复杂的操作序列
    manager.recordError("错误1", "TYPE_A");
    _ = manager.attemptRecovery();
    manager.recordError("错误2", "TYPE_B");
    manager.clearErrors();
    manager.recordError("错误3", "TYPE_C");

    try std.testing.expectEqual(manager.error_count, 1); // 清空后只剩最后一个
}

// ============ 恢复策略测试 ============

test "恢复失败后重新触发恢复" {
    const allocator = std.testing.allocator;
    var manager = error_recovery.ErrorRecoveryManager.init(allocator);
    defer manager.deinit();

    // 第一轮：触发恢复并失败
    for (0..6) |_| {
        manager.recordError("测试错误", "TEST");
    }

    _ = manager.attemptRecovery(); // 尝试 1
    _ = manager.attemptRecovery(); // 尝试 2
    _ = manager.attemptRecovery(); // 尝试 3
    const result = manager.attemptRecovery(); // 尝试 4（失败）

    try std.testing.expect(!result);
    try std.testing.expect(!manager.is_recovering);

    // 第二轮：重新触发恢复
    for (0..6) |_| {
        manager.recordError("新错误", "TEST");
    }

    try std.testing.expect(manager.is_recovering); // 应该重新进入恢复模式
    try std.testing.expectEqual(manager.recovery_attempts, 0); // 重置尝试次数
}

test "记录错误但未超过阈值不触发恢复" {
    const allocator = std.testing.allocator;
    var manager = error_recovery.ErrorRecoveryManager.init(allocator);
    defer manager.deinit();

    // 记录 5 个错误（正好等于阈值）
    for (0..5) |_| {
        manager.recordError("测试错误", "TEST");
    }

    try std.testing.expectEqual(manager.error_count, 5);
    try std.testing.expect(!manager.is_recovering); // 等于阈值不触发
}

test "getLastErrorMessage 返回正确的错误消息" {
    const allocator = std.testing.allocator;
    var manager = error_recovery.ErrorRecoveryManager.init(allocator);
    defer manager.deinit();

    const test_msg = "这是测试错误消息";
    manager.recordError(test_msg, "TEST");

    const last_msg = manager.getLastErrorMessage();

    try std.testing.expect(std.mem.eql(u8, last_msg, test_msg));
}

test "isRecovering 状态查询" {
    const allocator = std.testing.allocator;
    var manager = error_recovery.ErrorRecoveryManager.init(allocator);
    defer manager.deinit();

    try std.testing.expect(!manager.isRecovering());

    // 触发恢复
    for (0..6) |_| {
        manager.recordError("测试错误", "TEST");
    }

    try std.testing.expect(manager.isRecovering());

    // 标记恢复成功
    manager.markRecoverySuccess();

    try std.testing.expect(!manager.isRecovering());
}

test "getRecoveryAttempts 返回正确的尝试次数" {
    const allocator = std.testing.allocator;
    var manager = error_recovery.ErrorRecoveryManager.init(allocator);
    defer manager.deinit();

    try std.testing.expectEqual(manager.getRecoveryAttempts(), 0);

    // 触发恢复
    for (0..6) |_| {
        manager.recordError("测试错误", "TEST");
    }

    _ = manager.attemptRecovery();
    try std.testing.expectEqual(manager.getRecoveryAttempts(), 1);

    _ = manager.attemptRecovery();
    try std.testing.expectEqual(manager.getRecoveryAttempts(), 2);
}
