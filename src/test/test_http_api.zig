//! HTTP API 端点测试
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

test "ClockEvent tick 解析" {
    const event: interface.ClockEvent = .{ .tick = 500 };
    try std.testing.expectEqual(event.tick, 500);
}

test "ClockEvent mode 切换" {
    const event: interface.ClockEvent = .{ .user_change_mode = .STOPWATCH_MODE };
    try std.testing.expectEqual(event.user_change_mode, .STOPWATCH_MODE);
}

test "ClockEvent config 切换" {
    const new_config: interface.ClockTaskConfig = .{
        .countdown = .{ .duration_seconds = 1800 },
    };
    const event: interface.ClockEvent = .{ .user_change_config = new_config };
    try std.testing.expectEqual(event.user_change_config.countdown.duration_seconds, 1800);
}

test "SettingsEvent 变体标签" {
    const event1: interface.SettingsEvent = .get_settings;
    try std.testing.expectEqual(event1, .get_settings);

    var buf: [10]u8 = .{0};
    const event2: interface.SettingsEvent = .{ .change_settings = buf[0..0 :0] };
    try std.testing.expect(event2 == .change_settings);
}

test "SettingsConfig 默认值" {
    const config = interface.SettingsConfig{};
    try std.testing.expectEqual(config.basic.timezone, 8);
    try std.testing.expectEqualStrings(config.basic.language, "ZH");
    try std.testing.expectEqual(config.basic.default_mode, .countdown);
    try std.testing.expectEqualStrings(config.basic.theme_mode, "dark");
}

test "TimerState 响应结构" {
    const timer_state = interface.TimerStateResponse{
        .session_id = null,
        .is_finished = false,
        .is_running = false,
        .is_paused = false,
        .habit_id = null,
        .mode = "countdown",
        .elapsed_seconds = 0,
        .remaining_seconds = 1500,
        .in_rest = false,
    };

    try std.testing.expectEqual(timer_state.session_id, null);
    try std.testing.expect(!timer_state.is_finished);
    try std.testing.expect(!timer_state.is_running);
    try std.testing.expectEqualStrings(timer_state.mode, "countdown");
}

test "TimerState 完整状态" {
    const timer_state = interface.TimerStateResponse{
        .session_id = 1,
        .is_finished = false,
        .is_running = true,
        .is_paused = false,
        .habit_id = 5,
        .mode = "stopwatch",
        .elapsed_seconds = 300,
        .remaining_seconds = 0,
        .in_rest = false,
    };

    try std.testing.expectEqual(timer_state.session_id, 1);
    try std.testing.expect(timer_state.is_running);
    try std.testing.expectEqual(timer_state.habit_id, 5);
    try std.testing.expectEqual(timer_state.elapsed_seconds, 300);
}

test "TimerState rest 状态" {
    const timer_state = interface.TimerStateResponse{
        .session_id = 1,
        .is_finished = false,
        .is_running = true,
        .is_paused = false,
        .habit_id = 5,
        .mode = "countdown",
        .elapsed_seconds = 1500,
        .remaining_seconds = 300,
        .in_rest = true,
    };

    try std.testing.expect(timer_state.in_rest);
    try std.testing.expectEqual(timer_state.remaining_seconds, 300);
}

test "API 响应包装" {
    const response = interface.ApiResponse(u32){
        .success = true,
        .data = 42,
    };

    try std.testing.expect(response.success);
    try std.testing.expectEqual(response.data, 42);
}

test "API 响应错误" {
    const response = interface.ApiResponse([]const u8){
        .success = false,
        .data = "error message",
    };

    try std.testing.expect(!response.success);
    try std.testing.expectEqualStrings(response.data, "error message");
}

test "StartTimerRequest 请求体解析" {
    const request: interface.StartTimerRequest = .{
        .habit_id = 1,
        .mode = "countdown",
        .work_duration = 1500,
        .rest_duration = 300,
        .loop_count = 4,
    };

    try std.testing.expectEqual(request.habit_id, 1);
    try std.testing.expectEqualStrings(request.mode, "countdown");
    try std.testing.expectEqual(request.work_duration, 1500);
}

test "StartTimerRequest 可选字段" {
    const request: interface.StartTimerRequest = .{
        .habit_id = null,
        .mode = null,
        .work_duration = null,
        .rest_duration = null,
        .loop_count = null,
    };

    try std.testing.expectEqual(request.habit_id, null);
    try std.testing.expectEqual(request.mode, null);
}

test "UpdateSettingsRequest 设置更新" {
    const request: interface.UpdateSettingsRequest = .{
        .basic = .{
            .timezone = 12,
            .language = "EN",
            .default_mode = .stopwatch,
            .theme_mode = "light",
        },
    };

    try std.testing.expectEqual(request.basic.?.timezone, 12);
    try std.testing.expectEqualStrings(request.basic.?.language, "EN");
}

test "CreateHabitSetRequest 创建习惯集" {
    const request: interface.CreateHabitSetRequest = .{
        .name = "学习",
        .description = "学习习惯",
        .color = "#6366f1",
    };

    try std.testing.expectEqualStrings(request.name, "学习");
    try std.testing.expectEqualStrings(request.description, "学习习惯");
    try std.testing.expectEqualStrings(request.color, "#6366f1");
}

test "CreateHabitRequest 创建习惯" {
    const request: interface.CreateHabitRequest = .{
        .set_id = 1,
        .name = "背单词",
        .goal_seconds = 1800,
        .color = "#22c55e",
    };

    try std.testing.expectEqual(request.set_id, 1);
    try std.testing.expectEqualStrings(request.name, "背单词");
    try std.testing.expectEqual(request.goal_seconds, 1800);
}

test "UnlockResult HTTP 响应格式" {
    const result: interface.UnlockResult = .{
        .success = true,
        .locked_until = 0,
    };

    try std.testing.expect(result.success == true);
    try std.testing.expect(result.locked_until == 0);
}

test "UnlockResult 锁定状态" {
    const now = std.time.timestamp();
    const result: interface.UnlockResult = .{
        .success = false,
        .locked_until = now + 300,
    };

    try std.testing.expect(result.success == false);
    try std.testing.expect(result.locked_until > now);
}

test "MasterPasswordStatus 默认状态" {
    const status: interface.MasterPasswordStatus = .{};

    try std.testing.expect(status.has_password == false);
    try std.testing.expect(status.unlocked == false);
    try std.testing.expect(status.locked_until == 0);
    try std.testing.expect(status.unlock_time == 0);
}

test "MasterPasswordStatus 已设置主密码" {
    const now = std.time.timestamp();
    const status: interface.MasterPasswordStatus = .{
        .has_password = true,
        .unlocked = true,
        .locked_until = 0,
        .unlock_time = now,
    };

    try std.testing.expect(status.has_password == true);
    try std.testing.expect(status.unlocked == true);
    try std.testing.expect(status.unlock_time == now);
}

test "MasterPasswordStatus 锁定状态" {
    const now = std.time.timestamp();
    const status: interface.MasterPasswordStatus = .{
        .has_password = true,
        .unlocked = false,
        .locked_until = now + 300,
        .unlock_time = now - 100,
    };

    try std.testing.expect(status.has_password == true);
    try std.testing.expect(status.unlocked == false);
    try std.testing.expect(status.locked_until > now);
}