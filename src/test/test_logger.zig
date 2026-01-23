//! 日志系统模块单元测试
const std = @import("std");
const logger = @import("../logger.zig");

// ============ 日志等级测试 ============

test "LogLevel 字符串转换 - DEBUG" {
    const level = logger.LogLevel.DEBUG;
    try std.testing.expect(std.mem.eql(u8, level.toString(), "DEBUG"));
}

test "LogLevel 字符串转换 - INFO" {
    const level = logger.LogLevel.INFO;
    try std.testing.expect(std.mem.eql(u8, level.toString(), "INFO"));
}

test "LogLevel 字符串转换 - WARN" {
    const level = logger.LogLevel.WARN;
    try std.testing.expect(std.mem.eql(u8, level.toString(), "WARN"));
}

test "LogLevel 字符串转换 - ERROR" {
    const level = logger.LogLevel.ERROR;
    try std.testing.expect(std.mem.eql(u8, level.toString(), "ERROR"));
}

test "LogLevel emoji 表示 - DEBUG" {
    const level = logger.LogLevel.DEBUG;
    try std.testing.expect(std.mem.eql(u8, level.emoji(), "🐛"));
}

test "LogLevel emoji 表示 - INFO" {
    const level = logger.LogLevel.INFO;
    try std.testing.expect(std.mem.eql(u8, level.emoji(), "ℹ️"));
}

test "LogLevel emoji 表示 - WARN" {
    const level = logger.LogLevel.WARN;
    try std.testing.expect(std.mem.eql(u8, level.emoji(), "⚠️"));
}

test "LogLevel emoji 表示 - ERROR" {
    const level = logger.LogLevel.ERROR;
    try std.testing.expect(std.mem.eql(u8, level.emoji(), "❌"));
}

// ============ 字符串解析测试 ============

test "LogLevel fromString - DEBUG (大写)" {
    const level = logger.LogLevel.fromString("DEBUG");
    try std.testing.expect(level.? == .DEBUG);
}

test "LogLevel fromString - INFO (大写)" {
    const level = logger.LogLevel.fromString("INFO");
    try std.testing.expect(level.? == .INFO);
}

test "LogLevel fromString - WARN (大写)" {
    const level = logger.LogLevel.fromString("WARN");
    try std.testing.expect(level.? == .WARN);
}

test "LogLevel fromString - ERROR (大写)" {
    const level = logger.LogLevel.fromString("ERROR");
    try std.testing.expect(level.? == .ERROR);
}

test "LogLevel fromString - debug (小写)" {
    const level = logger.LogLevel.fromString("debug");
    try std.testing.expect(level.? == .DEBUG);
}

test "LogLevel fromString - info (小写)" {
    const level = logger.LogLevel.fromString("info");
    try std.testing.expect(level.? == .INFO);
}

test "LogLevel fromString - warn (小写)" {
    const level = logger.LogLevel.fromString("warn");
    try std.testing.expect(level.? == .WARN);
}

test "LogLevel fromString - error (小写)" {
    const level = logger.LogLevel.fromString("error");
    try std.testing.expect(level.? == .ERROR);
}

test "LogLevel fromString - 无效字符串返回 null" {
    const level = logger.LogLevel.fromString("INVALID");
    try std.testing.expect(level == null);
}

test "LogLevel fromString - 空字符串返回 null" {
    const level = logger.LogLevel.fromString("");
    try std.testing.expect(level == null);
}

test "LogLevel fromString - 空格处理" {
    const level1 = logger.LogLevel.fromString("  DEBUG  ");
    try std.testing.expect(level1.? == .DEBUG);

    const level2 = logger.LogLevel.fromString("  INFO  ");
    try std.testing.expect(level2.? == .INFO);
}

// ============ Logger 实例测试 ============

test "Logger 默认配置" {
    const test_logger = logger.Logger{};

    try std.testing.expectEqual(test_logger.current_level, .INFO);
    try std.testing.expect(test_logger.enable_timestamp);
}

test "Logger 自定义初始化" {
    const test_logger = logger.Logger{
        .current_level = .DEBUG,
        .enable_timestamp = false,
    };

    try std.testing.expectEqual(test_logger.current_level, .DEBUG);
    try std.testing.expect(!test_logger.enable_timestamp);
}

test "setLevel 改变日志等级" {
    var test_logger = logger.Logger{
        .current_level = .INFO,
        .enable_timestamp = false,
    };

    test_logger.setLevel(.DEBUG);
    try std.testing.expectEqual(test_logger.current_level, .DEBUG);

    test_logger.setLevel(.ERROR);
    try std.testing.expectEqual(test_logger.current_level, .ERROR);
}

test "setTimestamp 改变时间戳设置" {
    var test_logger = logger.Logger{
        .current_level = .INFO,
        .enable_timestamp = true,
    };

    test_logger.setTimestamp(false);
    try std.testing.expect(!test_logger.enable_timestamp);

    test_logger.setTimestamp(true);
    try std.testing.expect(test_logger.enable_timestamp);
}

// ============ 日志等级过滤测试 ============

test "日志等级顺序 DEBUG < INFO < WARN < ERROR" {
    const debug_val = @intFromEnum(logger.LogLevel.DEBUG);
    const info_val = @intFromEnum(logger.LogLevel.INFO);
    const warn_val = @intFromEnum(logger.LogLevel.WARN);
    const error_val = @intFromEnum(logger.LogLevel.ERROR);

    try std.testing.expect(debug_val < info_val);
    try std.testing.expect(info_val < warn_val);
    try std.testing.expect(warn_val < error_val);
}

test "INFO 级别过滤 - 允许 INFO 及以上" {
    const test_logger = logger.Logger{
        .current_level = .INFO,
        .enable_timestamp = false,
    };

    // DEBUG 不应该输出（<INFO）
    const debug_level = @intFromEnum(logger.LogLevel.DEBUG);
    const info_level = @intFromEnum(test_logger.current_level);
    try std.testing.expect(debug_level < info_level);

    // INFO 应该输出（=INFO）
    try std.testing.expect(info_level >= info_level);

    // WARN 应该输出（>INFO）
    const warn_level = @intFromEnum(logger.LogLevel.WARN);
    try std.testing.expect(warn_level >= info_level);
}

test "WARN 级别过滤 - 允许 WARN 及以上" {
    const test_logger = logger.Logger{
        .current_level = .WARN,
        .enable_timestamp = false,
    };

    const warn_level = @intFromEnum(logger.LogLevel.WARN);
    const error_level = @intFromEnum(logger.LogLevel.ERROR);
    const info_level = @intFromEnum(logger.LogLevel.INFO);

    // INFO 不应该输出（<WARN）
    try std.testing.expect(info_level < @intFromEnum(test_logger.current_level));

    // WARN 应该输出
    try std.testing.expect(warn_level >= @intFromEnum(test_logger.current_level));

    // ERROR 应该输出
    try std.testing.expect(error_level >= @intFromEnum(test_logger.current_level));
}

test "DEBUG 级别过滤 - 允许所有级别" {
    const test_logger = logger.Logger{
        .current_level = .DEBUG,
        .enable_timestamp = false,
    };

    const debug_level = @intFromEnum(logger.LogLevel.DEBUG);
    const info_level = @intFromEnum(logger.LogLevel.INFO);
    const warn_level = @intFromEnum(logger.LogLevel.WARN);
    const error_level = @intFromEnum(logger.LogLevel.ERROR);

    try std.testing.expect(debug_level >= @intFromEnum(test_logger.current_level));
    try std.testing.expect(info_level >= @intFromEnum(test_logger.current_level));
    try std.testing.expect(warn_level >= @intFromEnum(test_logger.current_level));
    try std.testing.expect(error_level >= @intFromEnum(test_logger.current_level));
}

test "ERROR 级别过滤 - 仅允许 ERROR" {
    const test_logger = logger.Logger{
        .current_level = .ERROR,
        .enable_timestamp = false,
    };

    const current = @intFromEnum(test_logger.current_level);
    const debug = @intFromEnum(logger.LogLevel.DEBUG);
    const info = @intFromEnum(logger.LogLevel.INFO);
    const warn = @intFromEnum(logger.LogLevel.WARN);
    const error_level = @intFromEnum(logger.LogLevel.ERROR);

    // 只有 ERROR 通过
    try std.testing.expect(debug < current);
    try std.testing.expect(info < current);
    try std.testing.expect(warn < current);
    try std.testing.expect(error_level >= current);
}

// ============ 全局 logger 测试 ============

test "全局 logger 实例存在" {
    try std.testing.expect(true); // 验证全局 logger 可访问
    try std.testing.expectEqual(logger.global_logger.current_level, .INFO);
}

test "全局 logger 初始启用时间戳" {
    try std.testing.expect(logger.global_logger.enable_timestamp);
}

// ============ 时间戳格式化测试 ============

test "时间戳格式化 - 禁用时返回空字符串" {
    const test_logger = logger.Logger{
        .current_level = .INFO,
        .enable_timestamp = false,
    };

    var buffer: [32]u8 = undefined;
    const timestamp = test_logger.formatTimestamp(&buffer);

    try std.testing.expectEqual(timestamp.len, 0);
}

test "时间戳格式化 - 启用时包含方括号" {
    const test_logger = logger.Logger{
        .current_level = .INFO,
        .enable_timestamp = true,
    };

    var buffer: [32]u8 = undefined;
    const timestamp = test_logger.formatTimestamp(&buffer);

    try std.testing.expect(std.mem.containsAtLeast(u8, timestamp, 1, "["));
    try std.testing.expect(std.mem.containsAtLeast(u8, timestamp, 1, "]"));
}

test "时间戳格式化 - 缓冲区溢出保护" {
    const test_logger = logger.Logger{
        .current_level = .INFO,
        .enable_timestamp = true,
    };

    var buffer: [10]u8 = undefined; // 太小的缓冲区
    const timestamp = test_logger.formatTimestamp(&buffer);

    // 应该返回空字符串或有效内容，不会越界
    try std.testing.expect(timestamp.len <= buffer.len);
}

// ============ 日志方法签名测试 ============

test "Logger 包含 debug 方法" {
    const test_logger = logger.Logger{
        .current_level = .DEBUG,
        .enable_timestamp = false,
    };

    // 验证方法可以调用（不会编译错误）
    test_logger.debug("测试 debug 消息: {}", .{42});
}

test "Logger 包含 info 方法" {
    const test_logger = logger.Logger{
        .current_level = .DEBUG,
        .enable_timestamp = false,
    };

    test_logger.info("测试 info 消息: {s}", .{"hello"});
}

test "Logger 包含 warn 方法" {
    const test_logger = logger.Logger{
        .current_level = .DEBUG,
        .enable_timestamp = false,
    };

    test_logger.warn("测试 warn 消息", .{});
}

test "Logger 包含 err 方法" {
    const test_logger = logger.Logger{
        .current_level = .DEBUG,
        .enable_timestamp = false,
    };

    test_logger.err("测试 error 消息: {d}", .{123});
}

// ============ LogLevel 枚举值测试 ============

test "LogLevel 枚举值范围" {
    const debug_val = @intFromEnum(logger.LogLevel.DEBUG);
    const error_val = @intFromEnum(logger.LogLevel.ERROR);

    try std.testing.expectEqual(debug_val, 0);
    try std.testing.expectEqual(error_val, 3);
}

test "LogLevel 值递增" {
    const debug_val = @intFromEnum(logger.LogLevel.DEBUG);
    const info_val = @intFromEnum(logger.LogLevel.INFO);
    const warn_val = @intFromEnum(logger.LogLevel.WARN);
    const error_val = @intFromEnum(logger.LogLevel.ERROR);

    try std.testing.expect(debug_val < info_val);
    try std.testing.expect(info_val < warn_val);
    try std.testing.expect(warn_val < error_val);
}

// ============ 边界条件测试 ============

test "空字符串日志消息" {
    const test_logger = logger.Logger{
        .current_level = .INFO,
        .enable_timestamp = false,
    };

    test_logger.info("", .{});
}

test "长日志消息" {
    const test_logger = logger.Logger{
        .current_level = .INFO,
        .enable_timestamp = false,
    };

    const long_msg = "这是一条很长的日志消息，包含大量信息和上下文，用于测试日志系统是否能正确处理长字符串";
    test_logger.info("{s}", .{long_msg});
}

test "特殊字符在日志消息中" {
    const test_logger = logger.Logger{
        .current_level = .INFO,
        .enable_timestamp = false,
    };

    test_logger.info("特殊字符: !@#$%^&*()[]{{}} 🚀", .{});
}

test "多个占位符" {
    const test_logger = logger.Logger{
        .current_level = .INFO,
        .enable_timestamp = false,
    };

    test_logger.info("值1: {d}, 值2: {d}, 值3: {s}", .{ 42, 3, "test" });
}
