//! HTTP Server 边界测试
const std = @import("std");
const http = std.http;
const interface = @import("../core/interface.zig");
const clock = @import("../core/clock.zig");

test "parsePathId 有效路径" {
    const result = parsePathId("/api/habits/123", "/api/habits/");
    try std.testing.expectEqual(@as(i64, 123), result);
}

test "parsePathId 无效路径前缀" {
    const result = parsePathId("/api/wrong/123", "/api/habits/");
    try std.testing.expectError(error.InvalidPath, result);
}

test "parsePathId 空 ID" {
    const result = parsePathId("/api/habits/", "/api/habits/");
    try std.testing.expectError(error.InvalidPath, result);
}

test "parsePathId 非数字 ID" {
    const result = parsePathId("/api/habits/abc", "/api/habits/");
    try std.testing.expectError(std.fmt.ParseIntError.InvalidFormat, result);
}

test "parsePathIdWithSuffix 有效路径" {
    const result = parsePathIdWithSuffix("/api/habits/123/streak", "/api/habits/", "/streak");
    try std.testing.expectEqual(@as(i64, 123), result);
}

test "parsePathIdWithSuffix 无效前缀" {
    const result = parsePathIdWithSuffix("/api/wrong/123/streak", "/api/habits/", "/streak");
    try std.testing.expectError(error.InvalidPath, result);
}

test "parsePathIdWithSuffix 无效后缀" {
    const result = parsePathIdWithSuffix("/api/habits/123/invalid", "/api/habits/", "/streak");
    try std.testing.expectError(error.InvalidPath, result);
}

test "parsePathIdWithSuffix 路径太短" {
    const result = parsePathIdWithSuffix("/api/habits/", "/api/habits/", "/streak");
    try std.testing.expectError(error.InvalidPath, result);
}

test "ModeEnumT 枚举值" {
    try std.testing.expectEqual(@as(interface.ModeEnumT, .COUNTDOWN_MODE), .COUNTDOWN_MODE);
    try std.testing.expectEqual(@as(interface.ModeEnumT, .STOPWATCH_MODE), .STOPWATCH_MODE);
}

test "ClockTaskConfig 默认值" {
    const config = interface.ClockTaskConfig{};

    try std.testing.expectEqual(config.default_mode, .COUNTDOWN_MODE);
    try std.testing.expectEqual(config.countdown.duration_seconds, 25 * 60);
    try std.testing.expectEqual(config.countdown.loop, false);
    try std.testing.expectEqual(config.countdown.loop_count, 0);
    try std.testing.expectEqual(config.countdown.loop_interval_seconds, 0);
    try std.testing.expectEqual(config.stopwatch.max_seconds, 24 * 60 * 60);
}

test "ClockTaskConfig 自定义值" {
    const config: interface.ClockTaskConfig = .{
        .default_mode = .STOPWATCH_MODE,
        .countdown = .{
            .duration_seconds = 1500,
            .loop = true,
            .loop_count = 4,
            .loop_interval_seconds = 300,
        },
        .stopwatch = .{
            .max_seconds = 7200,
        },
    };

    try std.testing.expectEqual(config.default_mode, .STOPWATCH_MODE);
    try std.testing.expectEqual(config.countdown.duration_seconds, 1500);
    try std.testing.expect(config.countdown.loop);
    try std.testing.expectEqual(config.countdown.loop_count, 4);
    try std.testing.expectEqual(config.countdown.loop_interval_seconds, 300);
    try std.testing.expectEqual(config.stopwatch.max_seconds, 7200);
}

test "ClockEvent 变体" {
    const tick_event: interface.ClockEvent = .{ .tick = 1000 };
    try std.testing.expect(tick_event == .tick);
    try std.testing.expectEqual(tick_event.tick, 1000);

    const start_event: interface.ClockEvent = .user_start_timer;
    try std.testing.expect(start_event == .user_start_timer);

    const pause_event: interface.ClockEvent = .user_pause_timer;
    try std.testing.expect(pause_event == .user_pause_timer);

    const reset_event: interface.ClockEvent = .user_reset_timer;
    try std.testing.expect(reset_event == .user_reset_timer);

    const finish_event: interface.ClockEvent = .user_finish_timer;
    try std.testing.expect(finish_event == .user_finish_timer);

    const mode_event: interface.ClockEvent = .{ .user_change_mode = .STOPWATCH_MODE };
    try std.testing.expect(mode_event == .user_change_mode);
}

test "SettingsConfig 默认值" {
    const config = interface.SettingsConfig{};

    try std.testing.expectEqual(config.basic.timezone, 8);
    try std.testing.expectEqualStrings(config.basic.language, "ZH");
    try std.testing.expectEqual(config.basic.default_mode, .countdown);
    try std.testing.expectEqualStrings(config.basic.theme_mode, "dark");
    try std.testing.expectEqualStrings(config.basic.wallpaper, "");
}

test "TimerPreset 结构体" {
    const preset: interface.TimerPreset = .{
        .name = "番茄钟",
        .mode = .COUNTDOWN_MODE,
        .config = .{
            .countdown = .{
                .duration_seconds = 1500,
                .loop = false,
                .loop_count = 0,
                .loop_interval_seconds = 0,
            },
        },
    };

    try std.testing.expectEqualStrings(preset.name, "番茄钟");
    try std.testing.expectEqual(preset.mode, .COUNTDOWN_MODE);
    try std.testing.expectEqual(preset.config.countdown.duration_seconds, 1500);
}

test "http Method 枚举值" {
    try std.testing.expectEqual(@as(http.Method, .GET), .GET);
    try std.testing.expectEqual(@as(http.Method, .POST), .POST);
    try std.testing.expectEqual(@as(http.Method, .PUT), .PUT);
    try std.testing.expectEqual(@as(http.Method, .DELETE), .DELETE);
}
