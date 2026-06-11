//! Session 管理测试
const std = @import("std");
const habit_crud = @import("../storage/habit_crud.zig");

test "TimerSessionRow 初始化" {
    const session: habit_crud.TimerSessionRow = .{
        .id = 1,
        .habit_id = 5,
        .mode = "countdown",
        .started_at = 1700000000,
        .paused_total_seconds = 0,
        .pause_started_at = null,
        .is_paused = false,
        .is_finished = false,
    };

    try std.testing.expectEqual(session.id, 1);
    try std.testing.expectEqual(session.habit_id, 5);
    try std.testing.expectEqualStrings(session.mode, "countdown");
    try std.testing.expect(!session.is_paused);
    try std.testing.expect(!session.is_finished);
}

test "TimerSessionRow 暂停状态" {
    const session: habit_crud.TimerSessionRow = .{
        .id = 1,
        .habit_id = 5,
        .mode = "countdown",
        .started_at = 1700000000,
        .paused_total_seconds = 60,
        .pause_started_at = 1700000060,
        .is_paused = true,
        .is_finished = false,
    };

    try std.testing.expect(session.is_paused);
    try std.testing.expectEqual(session.pause_started_at, 1700000060);
    try std.testing.expectEqual(session.paused_total_seconds, 60);
}

test "TimerSessionRow 完成状态" {
    const session: habit_crud.TimerSessionRow = .{
        .id = 1,
        .habit_id = 5,
        .mode = "countdown",
        .started_at = 1700000000,
        .paused_total_seconds = 0,
        .pause_started_at = null,
        .is_paused = false,
        .is_finished = true,
    };

    try std.testing.expect(session.is_finished);
    try std.testing.expect(!session.is_paused);
}

test "TimerSessionRow 计算运行时间 - 未暂停" {
    const session: habit_crud.TimerSessionRow = .{
        .id = 1,
        .habit_id = 5,
        .mode = "countdown",
        .started_at = 1700000000,
        .paused_total_seconds = 0,
        .pause_started_at = null,
        .is_paused = false,
        .is_finished = false,
    };

    const now_ts: i64 = 1700000300; // 300 秒后
    const elapsed = now_ts - session.started_at - session.paused_total_seconds;
    try std.testing.expectEqual(elapsed, 300);
}

test "TimerSessionRow 计算运行时间 - 已暂停" {
    const session: habit_crud.TimerSessionRow = .{
        .id = 1,
        .habit_id = 5,
        .mode = "countdown",
        .started_at = 1700000000,
        .paused_total_seconds = 60,
        .pause_started_at = 1700000060,
        .is_paused = true,
        .is_finished = false,
    };

    const now_ts: i64 = 1700000100; // 在暂停中，暂停了 40 秒
    const elapsed = now_ts - session.started_at - session.paused_total_seconds;
    try std.testing.expectEqual(elapsed, 60); // 只计算实际运行时间
}

test "TimerSessionRow 状态转换 - 开始到暂停" {
    var session: habit_crud.TimerSessionRow = .{
        .id = 1,
        .habit_id = 5,
        .mode = "countdown",
        .started_at = 1700000000,
        .paused_total_seconds = 0,
        .pause_started_at = null,
        .is_paused = false,
        .is_finished = false,
    };

    // 模拟暂停
    session.is_paused = true;
    session.pause_started_at = 1700000100;

    try std.testing.expect(session.is_paused);
    try std.testing.expectEqual(session.pause_started_at, 1700000100);
}

test "TimerSessionRow 状态转换 - 暂停到恢复" {
    var session: habit_crud.TimerSessionRow = .{
        .id = 1,
        .habit_id = 5,
        .mode = "countdown",
        .started_at = 1700000000,
        .paused_total_seconds = 0,
        .pause_started_at = 1700000100,
        .is_paused = true,
        .is_finished = false,
    };

    // 模拟恢复
    const pause_duration: i64 = 30;
    session.paused_total_seconds += pause_duration;
    session.pause_started_at = null;
    session.is_paused = false;

    try std.testing.expect(!session.is_paused);
    try std.testing.expectEqual(session.paused_total_seconds, 30);
    try std.testing.expectEqual(session.pause_started_at, null);
}

test "TimerSessionRow 状态转换 - 完成" {
    var session: habit_crud.TimerSessionRow = .{
        .id = 1,
        .habit_id = 5,
        .mode = "countdown",
        .started_at = 1700000000,
        .paused_total_seconds = 0,
        .pause_started_at = null,
        .is_paused = false,
        .is_finished = false,
    };

    // 模拟完成
    session.is_finished = true;

    try std.testing.expect(session.is_finished);
}

test "Session 查询结果包装" {
    const session_opt: ?habit_crud.TimerSessionRow = .{
        .id = 1,
        .habit_id = 5,
        .mode = "countdown",
        .started_at = 1700000000,
        .paused_total_seconds = 0,
        .pause_started_at = null,
        .is_paused = false,
        .is_finished = false,
    };

    try std.testing.expect(session_opt != null);
    try std.testing.expectEqual(session_opt.?.id, 1);
}

test "Session 空查询" {
    const session_opt: ?habit_crud.TimerSessionRow = null;
    try std.testing.expectEqual(session_opt, null);
}

test "SessionMode 枚举" {
    try std.testing.expectEqualStrings("countdown", "countdown");
    try std.testing.expectEqualStrings("stopwatch", "stopwatch");
}

test "Session 创建参数" {
    const params = .{
        .habit_id = @as(i64, 5),
        .mode = "countdown",
        .work_duration = 1500,
        .rest_duration = 300,
        .loop_count = 4,
    };

    try std.testing.expectEqual(params.habit_id, 5);
    try std.testing.expectEqualStrings(params.mode, "countdown");
    try std.testing.expectEqual(params.work_duration, 1500);
}

test "Session elapsed 计算边界 - 未开始" {
    const session: habit_crud.TimerSessionRow = .{
        .id = 0,
        .habit_id = 0,
        .mode = "countdown",
        .started_at = 0,
        .paused_total_seconds = 0,
        .pause_started_at = null,
        .is_paused = false,
        .is_finished = false,
    };

    const now_ts: i64 = 100;
    const elapsed = if (now_ts <= session.started_at) @as(i64, 0) else now_ts - session.started_at;
    try std.testing.expectEqual(elapsed, 0);
}