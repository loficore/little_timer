//! 主应用单元测试
const std = @import("std");
const interface = @import("../core/interface.zig");
const clock = @import("../core/clock.zig");
const settings_module = @import("../settings/settings_manager.zig");
const error_recovery = @import("../core/utils/error_recovery.zig");

test "MainApplication 结构体字段存在" {
    try std.testing.expectEqual(true, @hasField(std.types, "ClockManager"));
}

test "ClockManager 初始化状态" {
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

test "ClockManager 事件处理 - 启动和暂停" {
    const config: interface.ClockTaskConfig = .{
        .default_mode = .STOPWATCH_MODE,
        .stopwatch = .{ .max_seconds = 3600 },
    };

    var manager = clock.ClockManager.init(config);

    manager.handleEvent(.user_start_timer);
    var state = manager.update();
    try std.testing.expect(!state.isPaused());

    manager.handleEvent(.user_pause_timer);
    state = manager.update();
    try std.testing.expect(state.isPaused());
}

test "ClockManager 事件处理 - 重置" {
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
    manager.handleEvent(.{ .tick = 30000 });

    manager.handleEvent(.user_reset_timer);
    const state = manager.update();

    try std.testing.expectEqual(state.COUNTDOWN_MODE.remaining_ms, 60000);
    try std.testing.expect(state.isPaused());
}

test "ErrorRecoveryManager 初始化" {
    const allocator = std.testing.allocator;
    const manager = error_recovery.ErrorRecoveryManager.init(allocator);

    try std.testing.expectEqual(manager.error_count, 0);
    try std.testing.expect(!manager.is_recovering);
}

test "ErrorRecoveryManager 记录错误" {
    const allocator = std.testing.allocator;
    var manager = error_recovery.ErrorRecoveryManager.init(allocator);
    defer manager.deinit();

    manager.recordError("测试错误", "TEST_ERROR");

    try std.testing.expectEqual(manager.error_count, 1);
}

test "ErrorRecoveryManager 恢复机制" {
    const allocator = std.testing.allocator;
    var manager = error_recovery.ErrorRecoveryManager.init(allocator);
    defer manager.deinit();

    for (0..6) |_| {
        manager.recordError("测试错误", "TEST");
    }

    try std.testing.expect(manager.is_recovering);

    const result = manager.attemptRecovery();
    try std.testing.expect(result);
    try std.testing.expectEqual(manager.recovery_attempts, 1);
}

test "ClockTaskConfig 默认值" {
    const config = interface.ClockTaskConfig{};

    try std.testing.expectEqual(config.default_mode, .COUNTDOWN_MODE);
    try std.testing.expectEqual(config.countdown.duration_seconds, 25 * 60);
}

test "ClockTaskConfig 完整配置" {
    const config: interface.ClockTaskConfig = .{
        .default_mode = .STOPWATCH_MODE,
        .countdown = .{
            .duration_seconds = 1800,
            .loop = true,
            .loop_count = 4,
            .loop_interval_seconds = 300,
        },
        .stopwatch = .{ .max_seconds = 7200 },
    };

    try std.testing.expectEqual(config.default_mode, .STOPWATCH_MODE);
    try std.testing.expectEqual(config.countdown.duration_seconds, 1800);
    try std.testing.expect(config.countdown.loop);
    try std.testing.expectEqual(config.countdown.loop_count, 4);
    try std.testing.expectEqual(config.stopwatch.max_seconds, 7200);
}

test "ClockEvent 变体" {
    const event1: interface.ClockEvent = .user_start_timer;
    try std.testing.expect(event1 == .user_start_timer);

    const event2: interface.ClockEvent = .user_pause_timer;
    try std.testing.expect(event2 == .user_pause_timer);

    const event3: interface.ClockEvent = .user_reset_timer;
    try std.testing.expect(event3 == .user_reset_timer);

    const event4: interface.ClockEvent = .{ .tick = 1000 };
    try std.testing.expect(event4 == .tick);

    const event5: interface.ClockEvent = .{ .user_change_mode = .STOPWATCH_MODE };
    try std.testing.expect(event5 == .user_change_mode);
}

test "ClockState 状态查询" {
    const config: interface.ClockTaskConfig = .{
        .countdown = .{
            .duration_seconds = 60,
            .loop = false,
            .loop_count = 0,
            .loop_interval_seconds = 0,
        },
    };

    const manager = clock.ClockManager.init(config);
    const state = manager.update();

    try std.testing.expect(state.isPaused());
    try std.testing.expect(!state.isFinished());
    try std.testing.expect(!state.inRest());
    try std.testing.expectEqual(state.getLoopRemaining(), 0);
    try std.testing.expectEqual(state.getLoopTotal(), 0);
}

test "ClockState getMode 方法" {
    const config: interface.ClockTaskConfig = .{
        .default_mode = .STOPWATCH_MODE,
        .stopwatch = .{ .max_seconds = 3600 },
    };

    const manager = clock.ClockManager.init(config);
    const state = manager.update();

    const mode = state.getMode();
    try std.testing.expectEqual(mode, .STOPWATCH_MODE);
}

test "ClockState getTimeInfo 方法" {
    const config: interface.ClockTaskConfig = .{
        .countdown = .{
            .duration_seconds = 100,
            .loop = false,
            .loop_count = 0,
            .loop_interval_seconds = 0,
        },
    };

    var manager = clock.ClockManager.init(config);
    manager.handleEvent(.user_start_timer);
    manager.handleEvent(.{ .tick = 30000 });

    const state = manager.update();
    const time_info = state.getTimeInfo();

    try std.testing.expectEqual(time_info, 70);
}

test "ClockState getElapsedSeconds 方法" {
    const config: interface.ClockTaskConfig = .{
        .default_mode = .STOPWATCH_MODE,
        .stopwatch = .{ .max_seconds = 3600 },
    };

    var manager = clock.ClockManager.init(config);
    manager.handleEvent(.user_start_timer);
    manager.handleEvent(.{ .tick = 5000 });

    const state = manager.update();
    const elapsed = state.getElapsedSeconds();

    try std.testing.expectEqual(elapsed, 5);
}

test "ClockState getRemainingSeconds 方法" {
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
    manager.handleEvent(.{ .tick = 30000 });

    const state = manager.update();
    const remaining = state.getRemainingSeconds();

    try std.testing.expectEqual(remaining, 30);
}
