//! Timer 状态转换综合测试
const std = @import("std");
const clock = @import("../core/clock.zig");
const interface = @import("../core/interface.zig");

test "运行中切换 countdown → stopwatch" {
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
    manager.handleEvent(.{ .tick = 10000 });

    try std.testing.expect(!manager.state.COUNTDOWN_MODE.is_paused);
    try std.testing.expectEqual(manager.state.COUNTDOWN_MODE.remaining_ms, 50000);

    manager.handleEvent(.{ .user_change_mode = .STOPWATCH_MODE });

    try std.testing.expect(manager.state.STOPWATCH_MODE.esplased_ms == 0);
    try std.testing.expect(manager.state.STOPWATCH_MODE.is_paused);
    try std.testing.expect(!manager.state.STOPWATCH_MODE.is_finished);
}

test "运行中切换 stopwatch → countdown" {
    const config: interface.ClockTaskConfig = .{
        .default_mode = .STOPWATCH_MODE,
        .stopwatch = .{ .max_seconds = 3600 },
    };
    var manager = clock.ClockManager.init(config);

    manager.handleEvent(.user_start_timer);
    manager.handleEvent(.{ .tick = 5000 });

    try std.testing.expectEqual(manager.state.STOPWATCH_MODE.esplased_ms, 5000);

    manager.handleEvent(.{ .user_change_mode = .COUNTDOWN_MODE });

    try std.testing.expect(manager.state.COUNTDOWN_MODE.remaining_ms == 25 * 60 * 1000);
    try std.testing.expect(manager.state.COUNTDOWN_MODE.is_paused);
}

test "运行中修改 countdown 配置" {
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
    manager.handleEvent(.{ .tick = 10000 });

    try std.testing.expectEqual(manager.state.COUNTDOWN_MODE.remaining_ms, 50000);

    const new_config: interface.ClockTaskConfig = .{
        .default_mode = .COUNTDOWN_MODE,
        .countdown = .{
            .duration_seconds = 120,
            .loop = false,
            .loop_count = 0,
            .loop_interval_seconds = 0,
        },
    };
    manager.handleEvent(.{ .user_change_config = new_config });

    try std.testing.expectEqual(manager.state.COUNTDOWN_MODE.duration_ms, 120 * 1000);
    try std.testing.expectEqual(manager.state.COUNTDOWN_MODE.remaining_ms, 120 * 1000);
}

test "reset 打断 rest period" {
    const config: interface.ClockTaskConfig = .{
        .countdown = .{
            .duration_seconds = 60,
            .loop = true,
            .loop_count = 2,
            .loop_interval_seconds = 10,
        },
    };
    var manager = clock.ClockManager.init(config);

    manager.handleEvent(.user_start_timer);
    manager.handleEvent(.{ .tick = 60000 });

    try std.testing.expect(manager.state.COUNTDOWN_MODE.in_rest);
    try std.testing.expect(!manager.state.COUNTDOWN_MODE.is_finished);

    manager.handleEvent(.user_reset_timer);

    try std.testing.expect(!manager.state.COUNTDOWN_MODE.in_rest);
    try std.testing.expectEqual(manager.state.COUNTDOWN_MODE.rest_remaining_ms, 0);
    try std.testing.expect(manager.state.COUNTDOWN_MODE.is_paused);
    try std.testing.expect(!manager.state.COUNTDOWN_MODE.is_finished);
}

test "pause → change mode → resume" {
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
    manager.handleEvent(.{ .tick = 10000 });
    manager.handleEvent(.user_pause_timer);

    try std.testing.expect(manager.state.COUNTDOWN_MODE.is_paused);
    try std.testing.expectEqual(manager.state.COUNTDOWN_MODE.remaining_ms, 50000);

    manager.handleEvent(.{ .user_change_mode = .STOPWATCH_MODE });

    try std.testing.expect(manager.state.STOPWATCH_MODE.is_paused);

    manager.handleEvent(.user_start_timer);
    manager.handleEvent(.{ .tick = 5000 });

    try std.testing.expectEqual(manager.state.STOPWATCH_MODE.esplased_ms, 5000);
}

test "tick 正好在 0ms 结束" {
    const config: interface.ClockTaskConfig = .{
        .countdown = .{
            .duration_seconds = 10,
            .loop = false,
            .loop_count = 0,
            .loop_interval_seconds = 0,
        },
    };
    var manager = clock.ClockManager.init(config);

    manager.handleEvent(.user_start_timer);

    try std.testing.expect(!manager.state.COUNTDOWN_MODE.is_finished);

    manager.handleEvent(.{ .tick = 10000 });

    try std.testing.expect(manager.state.COUNTDOWN_MODE.is_finished);
    try std.testing.expectEqual(manager.state.COUNTDOWN_MODE.remaining_ms, 0);
}

test "tick 超过剩余时间（不应变成负数）" {
    const config: interface.ClockTaskConfig = .{
        .countdown = .{
            .duration_seconds = 10,
            .loop = false,
            .loop_count = 0,
            .loop_interval_seconds = 0,
        },
    };
    var manager = clock.ClockManager.init(config);

    manager.handleEvent(.user_start_timer);
    manager.handleEvent(.{ .tick = 5000 });

    try std.testing.expectEqual(manager.state.COUNTDOWN_MODE.remaining_ms, 5000);

    manager.handleEvent(.{ .tick = 10000 });

    try std.testing.expect(manager.state.COUNTDOWN_MODE.is_finished);
    try std.testing.expectEqual(manager.state.COUNTDOWN_MODE.remaining_ms, 0);
}

test "stopwatch tick 正好达到 max" {
    const config: interface.ClockTaskConfig = .{
        .default_mode = .STOPWATCH_MODE,
        .stopwatch = .{ .max_seconds = 60 },
    };
    var manager = clock.ClockManager.init(config);

    manager.handleEvent(.user_start_timer);
    manager.handleEvent(.{ .tick = 60000 });

    try std.testing.expect(manager.state.STOPWATCH_MODE.is_finished);
    try std.testing.expectEqual(manager.state.STOPWATCH_MODE.esplased_ms, 60000);
}

test "stopwatch tick 超过 max（不应超过）" {
    const config: interface.ClockTaskConfig = .{
        .default_mode = .STOPWATCH_MODE,
        .stopwatch = .{ .max_seconds = 60 },
    };
    var manager = clock.ClockManager.init(config);

    manager.handleEvent(.user_start_timer);
    manager.handleEvent(.{ .tick = 50000 });
    manager.handleEvent(.{ .tick = 20000 });

    try std.testing.expect(manager.state.STOPWATCH_MODE.is_finished);
    try std.testing.expectEqual(manager.state.STOPWATCH_MODE.esplased_ms, 60000);
}

test "countdown 循环模式 reset 后继续" {
    const config: interface.ClockTaskConfig = .{
        .countdown = .{
            .duration_seconds = 10,
            .loop = true,
            .loop_count = 3,
            .loop_interval_seconds = 5,
        },
    };
    var manager = clock.ClockManager.init(config);

    manager.handleEvent(.user_start_timer);
    manager.handleEvent(.{ .tick = 10000 });

    try std.testing.expect(manager.state.COUNTDOWN_MODE.in_rest);
    try std.testing.expectEqual(manager.state.COUNTDOWN_MODE.loop_remaining, 2);

    manager.handleEvent(.user_reset_timer);

    try std.testing.expectEqual(manager.state.COUNTDOWN_MODE.loop_remaining, 3);
    try std.testing.expect(!manager.state.COUNTDOWN_MODE.in_rest);
}

test "countdown finish 后 finish_timer 事件" {
    const config: interface.ClockTaskConfig = .{
        .countdown = .{
            .duration_seconds = 5,
            .loop = false,
            .loop_count = 0,
            .loop_interval_seconds = 0,
        },
    };
    var manager = clock.ClockManager.init(config);

    manager.handleEvent(.user_start_timer);
    manager.handleEvent(.{ .tick = 5000 });

    try std.testing.expect(manager.state.COUNTDOWN_MODE.is_finished);

    manager.handleEvent(.user_finish_timer);

    try std.testing.expect(manager.state.COUNTDOWN_MODE.is_finished);
    try std.testing.expect(manager.state.COUNTDOWN_MODE.is_paused);
}

test "stopwatch finish 后 finish_timer 事件" {
    const config: interface.ClockTaskConfig = .{
        .default_mode = .STOPWATCH_MODE,
        .stopwatch = .{ .max_seconds = 5 },
    };
    var manager = clock.ClockManager.init(config);

    manager.handleEvent(.user_start_timer);
    manager.handleEvent(.{ .tick = 5000 });

    try std.testing.expect(manager.state.STOPWATCH_MODE.is_finished);

    manager.handleEvent(.user_finish_timer);

    try std.testing.expect(manager.state.STOPWATCH_MODE.is_finished);
    try std.testing.expect(manager.state.STOPWATCH_MODE.is_paused);
}

test "连续 tick 不会累积误差" {
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

    var expected_remaining: i64 = 100 * 1000;
    for (0..10) |_| {
        manager.handleEvent(.{ .tick = 1000 });
        expected_remaining -= 1000;
        try std.testing.expectEqual(manager.state.COUNTDOWN_MODE.remaining_ms, expected_remaining);
    }
}

test "暂停状态下 change_config 不影响运行状态" {
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
    manager.handleEvent(.{ .tick = 10000 });
    manager.handleEvent(.user_pause_timer);

    const paused_remaining = manager.state.COUNTDOWN_MODE.remaining_ms;

    const new_config: interface.ClockTaskConfig = .{
        .default_mode = .COUNTDOWN_MODE,
        .countdown = .{
            .duration_seconds = 120,
            .loop = false,
            .loop_count = 0,
            .loop_interval_seconds = 0,
        },
    };
    manager.handleEvent(.{ .user_change_config = new_config });

    try std.testing.expectEqual(manager.state.COUNTDOWN_MODE.remaining_ms, 120 * 1000);
    try std.testing.expect(manager.state.COUNTDOWN_MODE.is_paused);
    _ = paused_remaining;
}
