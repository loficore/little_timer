//! HTTP 服务器单元测试
const std = @import("std");
const interface = @import("../core/interface.zig");
const clock = @import("../core/clock.zig");
const settings_module = @import("../settings/settings_manager.zig");

test "ClockTaskConfig 默认配置" {
    const config = interface.ClockTaskConfig{};

    try std.testing.expectEqual(config.default_mode, .COUNTDOWN_MODE);
    try std.testing.expectEqual(config.countdown.duration_seconds, 25 * 60);
    try std.testing.expectEqual(config.countdown.loop, false);
}

test "ClockTaskConfig 倒计时配置" {
    const config: interface.ClockTaskConfig = .{
        .default_mode = .COUNTDOWN_MODE,
        .countdown = .{
            .duration_seconds = 1500,
            .loop = true,
            .loop_count = 4,
            .loop_interval_seconds = 300,
        },
        .stopwatch = .{ .max_seconds = 3600 },
    };

    try std.testing.expectEqual(config.countdown.duration_seconds, 1500);
    try std.testing.expect(config.countdown.loop);
    try std.testing.expectEqual(config.countdown.loop_count, 4);
    try std.testing.expectEqual(config.countdown.loop_interval_seconds, 300);
}

test "ClockTaskConfig 正计时配置" {
    const config: interface.ClockTaskConfig = .{
        .default_mode = .STOPWATCH_MODE,
        .countdown = .{ .duration_seconds = 25 * 60, .loop = false, .loop_count = 0, .loop_interval_seconds = 0 },
        .stopwatch = .{ .max_seconds = 7200 },
    };

    try std.testing.expectEqual(config.default_mode, .STOPWATCH_MODE);
    try std.testing.expectEqual(config.stopwatch.max_seconds, 7200);
}

test "ClockEvent 变体标签" {
    const event1: interface.ClockEvent = .user_start_timer;
    try std.testing.expectEqual(event1, .user_start_timer);

    const event2: interface.ClockEvent = .{ .tick = 1000 };
    try std.testing.expect(event2 == .tick);

    const event3: interface.ClockEvent = .{ .user_change_mode = .STOPWATCH_MODE };
    try std.testing.expect(event3 == .user_change_mode);
}

test "SettingsEvent 变体标签" {
    const event1: interface.SettingsEvent = .get_settings;
    try std.testing.expectEqual(event1, .get_settings);

    var buf: [10]u8 = .{0};
    const event2: interface.SettingsEvent = .{ .change_settings = buf[0..0 :0] };
    try std.testing.expect(event2 == .change_settings);
}

test "ModeEnumT 枚举" {
    try std.testing.expectEqual(@as(interface.ModeEnumT, .COUNTDOWN_MODE), .COUNTDOWN_MODE);
    try std.testing.expectEqual(@as(interface.ModeEnumT, .STOPWATCH_MODE), .STOPWATCH_MODE);
}

test "ClockManager 初始化后状态" {
    const config: interface.ClockTaskConfig = .{
        .countdown = .{
            .duration_seconds = 1500,
            .loop = false,
            .loop_count = 0,
            .loop_interval_seconds = 0,
        },
    };

    const manager = clock.ClockManager.init(config);

    const state = manager.update();
    try std.testing.expect(state.isPaused());
    try std.testing.expect(!state.isFinished());
}

test "ClockManager handleEvent 用户事件" {
    const config: interface.ClockTaskConfig = .{
        .countdown = .{
            .duration_seconds = 60,
            .loop = false,
            .loop_count = 0,
            .loop_interval_seconds = 0,
        },
    };

    var manager = clock.ClockManager.init(config);

    manager.handleEvent(.user_start_timer);
    const state1 = manager.update();
    try std.testing.expect(!state1.isPaused());

    manager.handleEvent(.user_pause_timer);
    const state2 = manager.update();
    try std.testing.expect(state2.isPaused());
}

test "ClockManager tick 事件" {
    const config: interface.ClockTaskConfig = .{
        .default_mode = .STOPWATCH_MODE,
        .stopwatch = .{ .max_seconds = 3600 },
    };

    var manager = clock.ClockManager.init(config);

    manager.handleEvent(.user_start_timer);
    manager.handleEvent(.{ .tick = 5000 });

    const state = manager.update();
    try std.testing.expectEqual(state.STOPWATCH_MODE.esplased_ms, 5000);
}

test "SettingsConfig 默认值" {
    const config = interface.SettingsConfig{};

    try std.testing.expectEqual(config.basic.timezone, 8);
    try std.testing.expectEqual(config.basic.default_mode, .countdown);
    try std.testing.expectEqual(config.clock_defaults.countdown.duration_seconds, 25 * 60);
}

test "SettingsEvent 解析" {
    var buf: [100]u8 = undefined;
    const json = "{\"timezone\":8,\"language\":\"EN\"}";
    @memcpy(buf[0..json.len], json);
    buf[json.len] = 0;

    const event: interface.SettingsEvent = .{ .change_settings = buf[0..json.len :0] };
    try std.testing.expect(event == .change_settings);
}
