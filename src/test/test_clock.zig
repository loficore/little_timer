//! 时钟模块单元测试
const std = @import("std");
const clock = @import("../clock.zig");
const interface = @import("../interface.zig");

// ============ 倒计时测试 ============

test "倒计时初始化" {
    const config: interface.ClockTaskConfig = .{
        .countdown = .{
            .duration_seconds = 60,
            .loop = false,
            .loop_count = 0,
            .loop_interval_seconds = 0,
        },
    };

    const manager = clock.ClockManager.init(config);

    // 验证初始状态
    try std.testing.expectEqual(manager.state.COUNTDOWN_MODE.remaining_ms, 60000);
    try std.testing.expect(manager.state.COUNTDOWN_MODE.is_paused);
    try std.testing.expect(!manager.state.COUNTDOWN_MODE.is_finished);
}

test "倒计时基础 tick" {
    const config: interface.ClockTaskConfig = .{
        .countdown = .{
            .duration_seconds = 60,
            .loop = false,
            .loop_count = 0,
            .loop_interval_seconds = 0,
        },
    };
    var manager = clock.ClockManager.init(config);

    // 开始计时
    manager.handleEvent(.user_start_timer);

    // 执行 1000ms 的 tick
    manager.handleEvent(.{ .tick = 1000 });

    // 验证时间已减少
    try std.testing.expectEqual(manager.state.COUNTDOWN_MODE.remaining_ms, 59000);
    try std.testing.expect(!manager.state.COUNTDOWN_MODE.is_finished);
}

test "倒计时暂停和恢复" {
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
    manager.handleEvent(.{ .tick = 5000 }); // 减少 5 秒

    // 暂停
    manager.handleEvent(.user_pause_timer);
    try std.testing.expect(manager.state.COUNTDOWN_MODE.is_paused);

    const remaining_before = manager.state.COUNTDOWN_MODE.remaining_ms;
    manager.handleEvent(.{ .tick = 3000 }); // 这个 tick 应该被忽略
    try std.testing.expectEqual(manager.state.COUNTDOWN_MODE.remaining_ms, remaining_before);

    // 恢复
    manager.handleEvent(.user_start_timer);
    try std.testing.expect(!manager.state.COUNTDOWN_MODE.is_paused);
    manager.handleEvent(.{ .tick = 2000 });
    try std.testing.expectEqual(manager.state.COUNTDOWN_MODE.remaining_ms, remaining_before - 2000);
}

test "倒计时完成" {
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
    manager.handleEvent(.{ .tick = 5000 }); // 倒计时完成

    try std.testing.expect(manager.state.COUNTDOWN_MODE.is_finished);
    try std.testing.expectEqual(manager.state.COUNTDOWN_MODE.remaining_ms, 0);
}

test "倒计时超时不能为负数" {
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
    manager.handleEvent(.{ .tick = 10000 }); // 超过总时长

    try std.testing.expectEqual(manager.state.COUNTDOWN_MODE.remaining_ms, 0);
    try std.testing.expect(manager.state.COUNTDOWN_MODE.is_finished);
}

test "倒计时重置" {
    const config: interface.ClockTaskConfig = .{
        .countdown = .{
            .duration_seconds = 30,
            .loop = false,
            .loop_count = 0,
            .loop_interval_seconds = 0,
        },
    };
    var manager = clock.ClockManager.init(config);

    manager.handleEvent(.user_start_timer);
    manager.handleEvent(.{ .tick = 10000 });

    // 重置
    manager.handleEvent(.user_reset_timer);

    try std.testing.expectEqual(manager.state.COUNTDOWN_MODE.remaining_ms, 30000);
    try std.testing.expect(manager.state.COUNTDOWN_MODE.is_paused);
    try std.testing.expect(!manager.state.COUNTDOWN_MODE.is_finished);
}

test "倒计时循环模式 - 有限次数 - 无休息时间" {
    const config: interface.ClockTaskConfig = .{
        .countdown = .{
            .duration_seconds = 5,
            .loop = true,
            .loop_count = 2,
            .loop_interval_seconds = 0,
        },
    };
    var manager = clock.ClockManager.init(config);

    try std.testing.expectEqual(manager.state.COUNTDOWN_MODE.loop_remaining, 2);

    manager.handleEvent(.user_start_timer);
    manager.handleEvent(.{ .tick = 5000 }); // 第一轮完成

    // 由于是循环模式且无休息，第一轮完成后会立即重置开始第二轮
    try std.testing.expectEqual(manager.state.COUNTDOWN_MODE.loop_remaining, 1);
    try std.testing.expect(!manager.state.COUNTDOWN_MODE.is_finished);
    try std.testing.expectEqual(manager.state.COUNTDOWN_MODE.remaining_ms, 5000);

    // 继续第二轮
    manager.handleEvent(.{ .tick = 5000 }); // 第二轮完成

    try std.testing.expect(manager.state.COUNTDOWN_MODE.loop_completed);
}

test "倒计时循环模式 - 有限次数 - 有休息时间" {
    const config: interface.ClockTaskConfig = .{
        .countdown = .{
            .duration_seconds = 5,
            .loop = true,
            .loop_count = 2,
            .loop_interval_seconds = 3,
        },
        .stopwatch = .{
            .max_seconds = 3600,
        },
        .default_mode = .COUNTDOWN_MODE,
        .world_clock = .{
            .timezone = 8,
        },
    };

    var manager = clock.ClockManager.init(config);

    try std.testing.expectEqual(manager.state.COUNTDOWN_MODE.loop_remaining, 2);

    manager.handleEvent(.user_start_timer);
    manager.handleEvent(.{ .tick = 5000 }); // 第一轮完成

    // 进入休息时间
    try std.testing.expectEqual(manager.state.COUNTDOWN_MODE.loop_remaining, 1);
    try std.testing.expect(!manager.state.COUNTDOWN_MODE.is_finished);
    try std.testing.expectEqual(manager.state.COUNTDOWN_MODE.remaining_ms, 0); // 倒计时已归零
    try std.testing.expect(manager.state.COUNTDOWN_MODE.in_rest);
    try std.testing.expect(manager.state.inRest()); // 公共接口确认
    try std.testing.expectEqual(manager.state.getRestRemainingTime(), 3); // 休息3秒

    // 休息中间状态检查
    manager.handleEvent(.{ .tick = 1500 }); // 休息进行到一半
    try std.testing.expect(manager.state.COUNTDOWN_MODE.in_rest);
    try std.testing.expectEqual(manager.state.getRestRemainingTime(), 1); // 剩余1.5秒（向下取整为1）

    // 休息时间结束，继续第二轮
    manager.handleEvent(.{ .tick = 1500 }); // 完成剩余休息时间

    try std.testing.expect(!manager.state.COUNTDOWN_MODE.in_rest);
    try std.testing.expectEqual(manager.state.COUNTDOWN_MODE.remaining_ms, 5000);
    try std.testing.expectEqual(manager.state.COUNTDOWN_MODE.loop_remaining, 1);
    try std.testing.expect(!manager.state.COUNTDOWN_MODE.is_finished);

    manager.handleEvent(.{ .tick = 5000 }); // 第二轮完成

    try std.testing.expect(manager.state.COUNTDOWN_MODE.loop_completed);
    try std.testing.expectEqual(manager.state.COUNTDOWN_MODE.remaining_ms, 0);
    try std.testing.expect(manager.state.COUNTDOWN_MODE.is_finished);
    try std.testing.expect(!manager.state.COUNTDOWN_MODE.in_rest);
}

test "倒计时循环模式 - 无限循环" {
    const config: interface.ClockTaskConfig = .{
        .countdown = .{
            .duration_seconds = 3,
            .loop = true,
            .loop_count = 0, // 0 表示无限循环
            .loop_interval_seconds = 2,
        },
    };
    var manager = clock.ClockManager.init(config);

    try std.testing.expectEqual(manager.state.COUNTDOWN_MODE.loop_remaining, 0);
    try std.testing.expectEqual(manager.state.getLoopTotal(), 0); // 无限循环

    manager.handleEvent(.user_start_timer);

    // 第一轮
    manager.handleEvent(.{ .tick = 3000 });
    try std.testing.expect(manager.state.COUNTDOWN_MODE.in_rest);
    try std.testing.expectEqual(manager.state.COUNTDOWN_MODE.loop_remaining, 0); // 无限循环不递减

    // 休息后继续
    manager.handleEvent(.{ .tick = 2000 });
    try std.testing.expect(!manager.state.COUNTDOWN_MODE.in_rest);
    try std.testing.expectEqual(manager.state.COUNTDOWN_MODE.remaining_ms, 3000);

    // 第二轮
    manager.handleEvent(.{ .tick = 3000 });
    try std.testing.expect(manager.state.COUNTDOWN_MODE.in_rest);
    try std.testing.expectEqual(manager.state.COUNTDOWN_MODE.loop_remaining, 0); // 依然不递减
    try std.testing.expect(!manager.state.COUNTDOWN_MODE.loop_completed); // 永不完成

    // 再次休息后继续
    manager.handleEvent(.{ .tick = 2000 });
    try std.testing.expect(!manager.state.COUNTDOWN_MODE.in_rest);
    try std.testing.expectEqual(manager.state.COUNTDOWN_MODE.remaining_ms, 3000);
}

test "倒计时循环查询接口" {
    const config: interface.ClockTaskConfig = .{
        .countdown = .{
            .duration_seconds = 5,
            .loop = true,
            .loop_count = 3,
            .loop_interval_seconds = 2,
        },
    };
    var manager = clock.ClockManager.init(config);

    // 测试初始状态查询
    try std.testing.expectEqual(manager.state.getLoopRemaining(), 3);
    try std.testing.expectEqual(manager.state.getLoopTotal(), 3);
    try std.testing.expect(!manager.state.inRest());
    try std.testing.expectEqual(manager.state.getRestRemainingTime(), 0);

    manager.handleEvent(.user_start_timer);
    manager.handleEvent(.{ .tick = 5000 }); // 第一轮完成

    // 测试休息状态查询
    try std.testing.expect(manager.state.inRest());
    try std.testing.expectEqual(manager.state.getRestRemainingTime(), 2);
    try std.testing.expectEqual(manager.state.getLoopRemaining(), 2);
    try std.testing.expectEqual(manager.state.getLoopTotal(), 3);
}

// ============ 正计时测试 ============

test "正计时初始化" {
    const config: interface.ClockTaskConfig = .{
        .default_mode = .STOPWATCH_MODE,
        .stopwatch = .{
            .max_seconds = 3600,
        },
    };
    const manager = clock.ClockManager.init(config);

    try std.testing.expectEqual(manager.state.STOPWATCH_MODE.esplased_ms, 0);
    try std.testing.expect(manager.state.STOPWATCH_MODE.is_paused);
}

test "正计时基础 tick" {
    const config: interface.ClockTaskConfig = .{
        .default_mode = .STOPWATCH_MODE,
        .stopwatch = .{
            .max_seconds = 3600,
        },
    };
    var manager = clock.ClockManager.init(config);

    manager.handleEvent(.user_start_timer);
    manager.handleEvent(.{ .tick = 1000 });

    try std.testing.expectEqual(manager.state.STOPWATCH_MODE.esplased_ms, 1000);
    try std.testing.expect(!manager.state.STOPWATCH_MODE.is_finished);
}

test "正计时到达上限" {
    const config: interface.ClockTaskConfig = .{
        .default_mode = .STOPWATCH_MODE,
        .stopwatch = .{
            .max_seconds = 5,
        },
    };
    var manager = clock.ClockManager.init(config);

    manager.handleEvent(.user_start_timer);
    manager.handleEvent(.{ .tick = 5000 });

    try std.testing.expectEqual(manager.state.STOPWATCH_MODE.esplased_ms, 5000);
    try std.testing.expect(manager.state.STOPWATCH_MODE.is_finished);
}

test "正计时超过上限不增加" {
    const config: interface.ClockTaskConfig = .{
        .default_mode = .STOPWATCH_MODE,
        .stopwatch = .{
            .max_seconds = 5,
        },
    };
    var manager = clock.ClockManager.init(config);

    manager.handleEvent(.user_start_timer);
    manager.handleEvent(.{ .tick = 6000 });

    try std.testing.expectEqual(manager.state.STOPWATCH_MODE.esplased_ms, 5000);
}

test "正计时重置" {
    const config: interface.ClockTaskConfig = .{
        .default_mode = .STOPWATCH_MODE,
        .stopwatch = .{
            .max_seconds = 3600,
        },
    };
    var manager = clock.ClockManager.init(config);

    manager.handleEvent(.user_start_timer);
    manager.handleEvent(.{ .tick = 5000 });
    manager.handleEvent(.user_reset_timer);

    try std.testing.expectEqual(manager.state.STOPWATCH_MODE.esplased_ms, 0);
    try std.testing.expect(manager.state.STOPWATCH_MODE.is_paused);
    try std.testing.expect(!manager.state.STOPWATCH_MODE.is_finished); // 确认 is_finished 被清理
}

test "正计时暂停后 tick 不生效" {
    const config: interface.ClockTaskConfig = .{
        .default_mode = .STOPWATCH_MODE,
        .stopwatch = .{
            .max_seconds = 3600,
        },
    };
    var manager = clock.ClockManager.init(config);

    manager.handleEvent(.user_start_timer);
    manager.handleEvent(.{ .tick = 2000 });

    // 暂停
    manager.handleEvent(.user_pause_timer);
    try std.testing.expect(manager.state.STOPWATCH_MODE.is_paused);

    const elapsed_before = manager.state.STOPWATCH_MODE.esplased_ms;

    // 暂停状态下的 tick 应该被忽略
    manager.handleEvent(.{ .tick = 3000 });
    try std.testing.expectEqual(manager.state.STOPWATCH_MODE.esplased_ms, elapsed_before);
}

test "正计时达到上限后继续 tick 不增长" {
    const config: interface.ClockTaskConfig = .{
        .default_mode = .STOPWATCH_MODE,
        .stopwatch = .{
            .max_seconds = 5,
        },
    };
    var manager = clock.ClockManager.init(config);

    manager.handleEvent(.user_start_timer);
    manager.handleEvent(.{ .tick = 5000 }); // 达到上限

    try std.testing.expect(manager.state.STOPWATCH_MODE.is_finished);

    // 继续 tick 不应该增长
    manager.handleEvent(.{ .tick = 2000 });
    try std.testing.expectEqual(manager.state.STOPWATCH_MODE.esplased_ms, 5000);
    try std.testing.expect(manager.state.STOPWATCH_MODE.is_finished);
}

// ============ 世界时钟测试 ============

test "世界时钟初始化" {
    const config: interface.ClockTaskConfig = .{
        .default_mode = .WORLD_CLOCK_MODE,
        .world_clock = .{
            .timezone = 8,
        },
    };
    const manager = clock.ClockManager.init(config);

    try std.testing.expectEqual(manager.state.WORLD_CLOCK_MODE.timezone, 8);
}

test "世界时钟不会暂停" {
    const config: interface.ClockTaskConfig = .{
        .default_mode = .WORLD_CLOCK_MODE,
        .world_clock = .{
            .timezone = 8,
        },
    };
    var manager = clock.ClockManager.init(config);

    // 世界时钟不支持暂停操作
    manager.handleEvent(.user_pause_timer);
    try std.testing.expect(!manager.state.isPaused());
}

test "世界时钟时间偏移验证" {
    const config: interface.ClockTaskConfig = .{
        .default_mode = .WORLD_CLOCK_MODE,
        .world_clock = .{
            .timezone = 8,
        },
    };
    var manager = clock.ClockManager.init(config);

    const time_info = manager.update().getTimeInfo();

    // 弱断言：检查时间戳是否在合理范围内
    // 2020-01-01 00:00:00 UTC = 1577836800 秒
    // 2030-01-01 00:00:00 UTC = 1893456000 秒
    try std.testing.expect(time_info > 1577836800);
    try std.testing.expect(time_info < 1893456000);
}

test "世界时钟不同时区" {
    // UTC+8 (东八区)
    const config1: interface.ClockTaskConfig = .{
        .default_mode = .WORLD_CLOCK_MODE,
        .world_clock = .{
            .timezone = 8,
        },
    };
    var manager1 = clock.ClockManager.init(config1);

    // UTC-5 (西五区，如纽约)
    const config2: interface.ClockTaskConfig = .{
        .default_mode = .WORLD_CLOCK_MODE,
        .world_clock = .{
            .timezone = -5,
        },
    };
    var manager2 = clock.ClockManager.init(config2);

    const time1 = manager1.update().getTimeInfo();
    const time2 = manager2.update().getTimeInfo();

    // 时区差应该是 13 小时 = 46800 秒
    const diff = time1 - time2;
    try std.testing.expectEqual(diff, 13 * 3600);
}

// ============ 模式切换测试 ============

test "模式切换 - 倒计时到正计时" {
    const config: interface.ClockTaskConfig = .{
        .countdown = .{
            .duration_seconds = 60,
            .loop = false,
            .loop_count = 0,
            .loop_interval_seconds = 0,
        },
    };
    var manager = clock.ClockManager.init(config);

    // 切换到正计时
    const new_config: interface.ClockTaskConfig = .{
        .default_mode = .STOPWATCH_MODE,
        .stopwatch = .{
            .max_seconds = 3600,
        },
    };
    manager.handleEvent(.{ .user_change_config = new_config });

    try std.testing.expectEqual(manager.state.STOPWATCH_MODE.esplased_ms, 0);
}

test "模式切换 - 正计时到世界时钟" {
    const config: interface.ClockTaskConfig = .{
        .default_mode = .STOPWATCH_MODE,
        .stopwatch = .{
            .max_seconds = 3600,
        },
    };
    var manager = clock.ClockManager.init(config);

    // 切换到世界时钟
    const new_config: interface.ClockTaskConfig = .{
        .default_mode = .WORLD_CLOCK_MODE,
        .world_clock = .{
            .timezone = -5,
        },
    };
    manager.handleEvent(.{ .user_change_config = new_config });

    try std.testing.expectEqual(manager.state.WORLD_CLOCK_MODE.timezone, -5);
}

test "user_change_mode 使用默认配置" {
    const config: interface.ClockTaskConfig = .{
        .countdown = .{
            .duration_seconds = 60,
            .loop = false,
            .loop_count = 0,
            .loop_interval_seconds = 0,
        },
    };
    var manager = clock.ClockManager.init(config);

    // 使用 user_change_mode 切换到正计时（应采用默认配置）
    manager.handleEvent(.{ .user_change_mode = .STOPWATCH_MODE });

    // 验证采用了默认配置：24小时上限
    try std.testing.expectEqual(manager.state.STOPWATCH_MODE.max_ms, 24 * 60 * 60 * 1000);
    try std.testing.expectEqual(manager.state.STOPWATCH_MODE.esplased_ms, 0);

    // 切换到世界时钟（默认东八区）
    manager.handleEvent(.{ .user_change_mode = .WORLD_CLOCK_MODE });
    try std.testing.expectEqual(manager.state.WORLD_CLOCK_MODE.timezone, 8);

    // 切换回倒计时（默认25分钟）
    manager.handleEvent(.{ .user_change_mode = .COUNTDOWN_MODE });
    try std.testing.expectEqual(manager.state.COUNTDOWN_MODE.duration_ms, 25 * 60 * 1000);
}

test "user_change_config 更新 initial_config" {
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

    // 修改配置为 120 秒
    const new_config: interface.ClockTaskConfig = .{
        .countdown = .{
            .duration_seconds = 120,
            .loop = false,
            .loop_count = 0,
            .loop_interval_seconds = 0,
        },
    };
    manager.handleEvent(.{ .user_change_config = new_config });

    // 验证当前状态已更新
    try std.testing.expectEqual(manager.state.COUNTDOWN_MODE.remaining_ms, 120000);

    // 重置后应该回到新配置（120秒），而非旧配置（60秒）
    manager.handleEvent(.user_start_timer);
    manager.handleEvent(.{ .tick = 10000 });
    manager.handleEvent(.user_reset_timer);

    try std.testing.expectEqual(manager.state.COUNTDOWN_MODE.remaining_ms, 120000);
}

// ============ 状态查询测试 ============

test "getTimeInfo 倒计时" {
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
    manager.handleEvent(.{ .tick = 23000 });

    const time_info = manager.update().getTimeInfo();
    try std.testing.expectEqual(time_info, 77); // 100 - 23 = 77 秒
}

test "getTimeInfo 正计时" {
    const config: interface.ClockTaskConfig = .{
        .default_mode = .STOPWATCH_MODE,
        .stopwatch = .{
            .max_seconds = 3600,
        },
    };
    var manager = clock.ClockManager.init(config);

    manager.handleEvent(.user_start_timer);
    manager.handleEvent(.{ .tick = 42000 });

    const time_info = manager.update().getTimeInfo();
    try std.testing.expectEqual(time_info, 42); // 42 秒
}

test "isPaused 和 isFinished 状态查询" {
    const config: interface.ClockTaskConfig = .{
        .countdown = .{
            .duration_seconds = 10,
            .loop = false,
            .loop_count = 0,
            .loop_interval_seconds = 0,
        },
    };
    var manager = clock.ClockManager.init(config);

    const state = manager.update();
    try std.testing.expect(state.isPaused());
    try std.testing.expect(!state.isFinished());

    manager.handleEvent(.user_start_timer);
    try std.testing.expect(!manager.update().isPaused());
}

// ============ 边界条件测试 ============

test "倒计时 0 秒" {
    const config: interface.ClockTaskConfig = .{
        .countdown = .{
            .duration_seconds = 0,
            .loop = false,
            .loop_count = 0,
            .loop_interval_seconds = 0,
        },
    };
    var manager = clock.ClockManager.init(config);

    manager.handleEvent(.user_start_timer);
    // 0 秒倒计时立即触发 tick 检查
    manager.handleEvent(.{ .tick = 1 });

    try std.testing.expectEqual(manager.state.COUNTDOWN_MODE.remaining_ms, 0);
    try std.testing.expect(manager.state.COUNTDOWN_MODE.is_finished);
}

test "正计时 1 毫秒粒度" {
    const config: interface.ClockTaskConfig = .{
        .default_mode = .STOPWATCH_MODE,
        .stopwatch = .{
            .max_seconds = 3600,
        },
    };
    var manager = clock.ClockManager.init(config);

    manager.handleEvent(.user_start_timer);
    manager.handleEvent(.{ .tick = 1 });

    try std.testing.expectEqual(manager.state.STOPWATCH_MODE.esplased_ms, 1);
}

test "连续多次 tick" {
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

    // 多个小的 tick
    for (0..10) |_| {
        manager.handleEvent(.{ .tick = 100 });
    }

    try std.testing.expectEqual(manager.state.COUNTDOWN_MODE.remaining_ms, 99000);
}
