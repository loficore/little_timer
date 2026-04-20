//! SSE 状态序列化与 ClockManager 事件测试
const std = @import("std");
const clock = @import("../core/clock.zig");
const interface = @import("../core/interface.zig");

test "ClockState JSON 序列化 - countdown 模式" {
    const config: interface.ClockTaskConfig = .{
        .countdown = .{
            .duration_seconds = 1500,
            .loop = true,
            .loop_count = 4,
            .loop_interval_seconds = 300,
        },
    };
    var manager = clock.ClockManager.init(config);

    try std.testing.expect(manager.state.COUNTDOWN_MODE.is_paused);
    try std.testing.expect(!manager.state.COUNTDOWN_MODE.is_finished);
    try std.testing.expectEqual(manager.state.COUNTDOWN_MODE.duration_ms, 1500 * 1000);
    try std.testing.expectEqual(manager.state.COUNTDOWN_MODE.remaining_ms, 1500 * 1000);
}

test "ClockState JSON 序列化 - stopwatch 模式" {
    const config: interface.ClockTaskConfig = .{
        .default_mode = .STOPWATCH_MODE,
        .stopwatch = .{ .max_seconds = 3600 },
    };
    var manager = clock.ClockManager.init(config);

    try std.testing.expect(manager.state.STOPWATCH_MODE.is_paused);
    try std.testing.expect(!manager.state.STOPWATCH_MODE.is_finished);
    try std.testing.expectEqual(manager.state.STOPWATCH_MODE.esplased_ms, 0);
    try std.testing.expectEqual(manager.state.STOPWATCH_MODE.max_ms, 3600 * 1000);
}

test "ClockManager tick 事件触发状态变化" {
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
    try std.testing.expect(!manager.state.COUNTDOWN_MODE.is_paused);

    manager.handleEvent(.{ .tick = 1000 });
    try std.testing.expectEqual(manager.state.COUNTDOWN_MODE.remaining_ms, 59000);

    manager.handleEvent(.user_pause_timer);
    try std.testing.expect(manager.state.COUNTDOWN_MODE.is_paused);
}

test "ClockManager in_rest 状态切换" {
    const config: interface.ClockTaskConfig = .{
        .countdown = .{
            .duration_seconds = 10,
            .loop = true,
            .loop_count = 2,
            .loop_interval_seconds = 5,
        },
    };
    var manager = clock.ClockManager.init(config);

    try std.testing.expect(!manager.state.COUNTDOWN_MODE.in_rest);

    manager.handleEvent(.user_start_timer);
    manager.handleEvent(.{ .tick = 10000 });

    try std.testing.expect(manager.state.COUNTDOWN_MODE.in_rest);
    try std.testing.expectEqual(manager.state.COUNTDOWN_MODE.rest_remaining_ms, 5000);

    manager.handleEvent(.{ .tick = 5000 });

    try std.testing.expect(!manager.state.COUNTDOWN_MODE.in_rest);
    try std.testing.expectEqual(manager.state.COUNTDOWN_MODE.remaining_ms, 10000);
}

test "ClockManager loop_remaining 递减" {
    const config: interface.ClockTaskConfig = .{
        .countdown = .{
            .duration_seconds = 5,
            .loop = true,
            .loop_count = 3,
            .loop_interval_seconds = 0,
        },
    };
    var manager = clock.ClockManager.init(config);

    try std.testing.expectEqual(manager.state.COUNTDOWN_MODE.loop_remaining, 3);

    manager.handleEvent(.user_start_timer);
    manager.handleEvent(.{ .tick = 5000 });

    try std.testing.expectEqual(manager.state.COUNTDOWN_MODE.loop_remaining, 2);
}

test "ClockManager loop_completed 标记" {
    const config: interface.ClockTaskConfig = .{
        .countdown = .{
            .duration_seconds = 5,
            .loop = true,
            .loop_count = 2,
            .loop_interval_seconds = 0,
        },
    };
    var manager = clock.ClockManager.init(config);

    try std.testing.expect(!manager.state.COUNTDOWN_MODE.loop_completed);

    manager.handleEvent(.user_start_timer);

    manager.handleEvent(.{ .tick = 5000 });
    try std.testing.expectEqual(manager.state.COUNTDOWN_MODE.loop_remaining, 1);

    manager.handleEvent(.{ .tick = 5000 });
    try std.testing.expectEqual(manager.state.COUNTDOWN_MODE.loop_remaining, 0);
    try std.testing.expect(manager.state.COUNTDOWN_MODE.loop_completed);
}

test "ClockManager isFinished 状态正确" {
    const config: interface.ClockTaskConfig = .{
        .countdown = .{
            .duration_seconds = 5,
            .loop = false,
            .loop_count = 0,
            .loop_interval_seconds = 0,
        },
    };
    var manager = clock.ClockManager.init(config);

    try std.testing.expect(!manager.state.COUNTDOWN_MODE.is_finished);

    manager.handleEvent(.user_start_timer);
    manager.handleEvent(.{ .tick = 5000 });

    try std.testing.expect(manager.state.COUNTDOWN_MODE.is_finished);
    try std.testing.expectEqual(manager.state.COUNTDOWN_MODE.remaining_ms, 0);
}

test "ClockManager reset 后 loop_remaining 恢复" {
    const config: interface.ClockTaskConfig = .{
        .countdown = .{
            .duration_seconds = 5,
            .loop = true,
            .loop_count = 3,
            .loop_interval_seconds = 0,
        },
    };
    var manager = clock.ClockManager.init(config);

    manager.handleEvent(.user_start_timer);
    manager.handleEvent(.{ .tick = 5000 });
    try std.testing.expectEqual(manager.state.COUNTDOWN_MODE.loop_remaining, 2);

    manager.handleEvent(.user_reset_timer);

    try std.testing.expectEqual(manager.state.COUNTDOWN_MODE.loop_remaining, 3);
    try std.testing.expect(!manager.state.COUNTDOWN_MODE.loop_completed);
}

test "ClockManager getRestRemainingTime 返回正确值" {
    const config: interface.ClockTaskConfig = .{
        .countdown = .{
            .duration_seconds = 10,
            .loop = true,
            .loop_count = 1,
            .loop_interval_seconds = 5,
        },
    };
    var manager = clock.ClockManager.init(config);

    manager.handleEvent(.user_start_timer);
    manager.handleEvent(.{ .tick = 10000 });

    try std.testing.expect(manager.state.COUNTDOWN_MODE.in_rest);
    try std.testing.expectEqual(manager.state.COUNTDOWN_MODE.getRestRemainingTime(), 5);
}

test "ClockManager getTimeInfo 返回正确值" {
    const config: interface.ClockTaskConfig = .{
        .countdown = .{
            .duration_seconds = 60,
            .loop = false,
            .loop_count = 0,
            .loop_interval_seconds = 0,
        },
    };
    var manager = clock.ClockManager.init(config);

    try std.testing.expectEqual(manager.state.getTimeInfo(), 60);

    manager.handleEvent(.user_start_timer);
    manager.handleEvent(.{ .tick = 1000 });

    try std.testing.expectEqual(manager.state.getTimeInfo(), 59);
}

test "ClockManager stopwatch getTimeInfo 返回 elapsed" {
    const config: interface.ClockTaskConfig = .{
        .default_mode = .STOPWATCH_MODE,
        .stopwatch = .{ .max_seconds = 3600 },
    };
    var manager = clock.ClockManager.init(config);

    try std.testing.expectEqual(manager.state.getTimeInfo(), 0);

    manager.handleEvent(.user_start_timer);
    manager.handleEvent(.{ .tick = 5000 });

    try std.testing.expectEqual(manager.state.getTimeInfo(), 5);
}

test "ClockManager getMode 返回正确模式" {
    const countdown_config: interface.ClockTaskConfig = .{
        .countdown = .{
            .duration_seconds = 60,
            .loop = false,
            .loop_count = 0,
            .loop_interval_seconds = 0,
        },
    };
    var countdown_manager = clock.ClockManager.init(countdown_config);
    try std.testing.expectEqual(countdown_manager.state.getMode(), .COUNTDOWN_MODE);

    const stopwatch_config: interface.ClockTaskConfig = .{
        .default_mode = .STOPWATCH_MODE,
        .stopwatch = .{ .max_seconds = 3600 },
    };
    var stopwatch_manager = clock.ClockManager.init(stopwatch_config);
    try std.testing.expectEqual(stopwatch_manager.state.getMode(), .STOPWATCH_MODE);
}

test "ClockManager isPaused 检查" {
    const config: interface.ClockTaskConfig = .{
        .countdown = .{
            .duration_seconds = 60,
            .loop = false,
            .loop_count = 0,
            .loop_interval_seconds = 0,
        },
    };
    var manager = clock.ClockManager.init(config);

    try std.testing.expect(manager.state.isPaused());

    manager.handleEvent(.user_start_timer);
    try std.testing.expect(!manager.state.isPaused());

    manager.handleEvent(.user_pause_timer);
    try std.testing.expect(manager.state.isPaused());
}

test "ClockManager isFinished 检查" {
    const config: interface.ClockTaskConfig = .{
        .countdown = .{
            .duration_seconds = 5,
            .loop = false,
            .loop_count = 0,
            .loop_interval_seconds = 0,
        },
    };
    var manager = clock.ClockManager.init(config);

    try std.testing.expect(!manager.state.isFinished());

    manager.handleEvent(.user_start_timer);
    manager.handleEvent(.{ .tick = 5000 });

    try std.testing.expect(manager.state.isFinished());
}

test "ClockManager stopwatch isFinished 检查" {
    const config: interface.ClockTaskConfig = .{
        .default_mode = .STOPWATCH_MODE,
        .stopwatch = .{ .max_seconds = 5 },
    };
    var manager = clock.ClockManager.init(config);

    try std.testing.expect(!manager.state.isFinished());

    manager.handleEvent(.user_start_timer);
    manager.handleEvent(.{ .tick = 5000 });

    try std.testing.expect(manager.state.isFinished());
}

test "ClockManager getElapsedSeconds countdown" {
    const config: interface.ClockTaskConfig = .{
        .countdown = .{
            .duration_seconds = 60,
            .loop = false,
            .loop_count = 0,
            .loop_interval_seconds = 0,
        },
    };
    var manager = clock.ClockManager.init(config);

    try std.testing.expectEqual(manager.state.getElapsedSeconds(), 0);

    manager.handleEvent(.user_start_timer);
    manager.handleEvent(.{ .tick = 30000 });

    try std.testing.expectEqual(manager.state.getElapsedSeconds(), 30);
}

test "ClockManager getRemainingSeconds countdown" {
    const config: interface.ClockTaskConfig = .{
        .countdown = .{
            .duration_seconds = 60,
            .loop = false,
            .loop_count = 0,
            .loop_interval_seconds = 0,
        },
    };
    var manager = clock.ClockManager.init(config);

    try std.testing.expectEqual(manager.state.getRemainingSeconds(), 60);

    manager.handleEvent(.user_start_timer);
    manager.handleEvent(.{ .tick = 30000 });

    try std.testing.expectEqual(manager.state.getRemainingSeconds(), 30);
}

test "ClockManager getCurrentRound" {
    const config: interface.ClockTaskConfig = .{
        .countdown = .{
            .duration_seconds = 5,
            .loop = true,
            .loop_count = 3,
            .loop_interval_seconds = 0,
        },
    };
    var manager = clock.ClockManager.init(config);

    try std.testing.expectEqual(manager.state.getCurrentRound(), 1);

    manager.handleEvent(.user_start_timer);
    manager.handleEvent(.{ .tick = 5000 });

    try std.testing.expectEqual(manager.state.getCurrentRound(), 2);
}
