//! Session 边界测试
const std = @import("std");
const zqlite = @import("zqlite");
const habit_crud = @import("../storage/habit_crud.zig");
const storage_sqlite = @import("../storage/storage_sqlite.zig");
const storage_migration = @import("../storage/storage_migration.zig");

test "TimerSessionRow 初始化各字段" {
    const row: habit_crud.TimerSessionRow = .{
        .id = 1,
        .habit_id = 1,
        .mode = "stopwatch",
        .started_at = 1704067200,
        .updated_at = 1704067200,
        .is_running = true,
        .is_finished = false,
        .is_paused = false,
        .elapsed_seconds = 100,
        .paused_total_seconds = 0,
        .pause_started_at = null,
        .last_synced_at = null,
        .remaining_seconds = null,
        .work_duration = 1500,
        .rest_duration = 300,
        .loop_count = 0,
        .current_round = 1,
        .in_rest = false,
    };

    try std.testing.expectEqual(row.id, 1);
    try std.testing.expectEqual(row.is_running, true);
    try std.testing.expectEqual(row.elapsed_seconds, 100);
    try std.testing.expectEqual(row.current_round, 1);
}

test "TimerSessionRow with null optional fields" {
    const row: habit_crud.TimerSessionRow = .{
        .id = 1,
        .habit_id = null,
        .mode = "countdown",
        .started_at = 1704067200,
        .updated_at = 1704067200,
        .is_running = false,
        .is_finished = true,
        .is_paused = false,
        .elapsed_seconds = 1500,
        .paused_total_seconds = 0,
        .pause_started_at = null,
        .last_synced_at = null,
        .remaining_seconds = null,
        .work_duration = 1500,
        .rest_duration = 0,
        .loop_count = 0,
        .current_round = 1,
        .in_rest = false,
    };

    try std.testing.expect(row.habit_id == null);
    try std.testing.expectEqualStrings(row.mode, "countdown");
    try std.testing.expectEqual(row.elapsed_seconds, 1500);
}

test "TimerSessionRow pause_started_at 有值" {
    const row: habit_crud.TimerSessionRow = .{
        .id = 1,
        .habit_id = 1,
        .mode = "stopwatch",
        .started_at = 1704067200,
        .updated_at = 1704067300,
        .is_running = false,
        .is_finished = false,
        .is_paused = true,
        .elapsed_seconds = 100,
        .paused_total_seconds = 10,
        .pause_started_at = 1704067300,
        .last_synced_at = null,
        .remaining_seconds = null,
        .work_duration = 1500,
        .rest_duration = 0,
        .loop_count = 0,
        .current_round = 1,
        .in_rest = false,
    };

    try std.testing.expect(row.pause_started_at != null);
    try std.testing.expectEqual(row.pause_started_at.?, 1704067300);
}

test "TimerSessionRow remaining_seconds 有值（countdown 模式）" {
    const row: habit_crud.TimerSessionRow = .{
        .id = 1,
        .habit_id = 1,
        .mode = "countdown",
        .started_at = 1704067200,
        .updated_at = 1704067250,
        .is_running = false,
        .is_finished = false,
        .is_paused = true,
        .elapsed_seconds = 50,
        .paused_total_seconds = 0,
        .pause_started_at = 1704067250,
        .last_synced_at = null,
        .remaining_seconds = 1450,
        .work_duration = 1500,
        .rest_duration = 0,
        .loop_count = 0,
        .current_round = 1,
        .in_rest = false,
    };

    try std.testing.expect(row.remaining_seconds != null);
    try std.testing.expectEqual(row.remaining_seconds.?, 1450);
}

test "SessionRow 初始化" {
    const row: habit_crud.SessionRow = .{
        .id = 1,
        .habit_id = 1,
        .duration_seconds = 1500,
        .count = 1,
        .started_at = "2026-01-01 10:00:00",
        .date = "2026-01-01",
    };

    try std.testing.expectEqual(row.id, 1);
    try std.testing.expectEqual(row.habit_id, 1);
    try std.testing.expectEqual(row.duration_seconds, 1500);
    try std.testing.expectEqual(row.count, 1);
}

test "SessionRow 多 count" {
    const row: habit_crud.SessionRow = .{
        .id = 1,
        .habit_id = 1,
        .duration_seconds = 3000,
        .count = 2,
        .started_at = "2026-01-01 10:00:00",
        .date = "2026-01-01",
    };

    try std.testing.expectEqual(row.count, 2);
    try std.testing.expectEqual(row.duration_seconds, 3000);
}

test "HabitSetRow 完整初始化" {
    const row: habit_crud.HabitSetRow = .{
        .id = 1,
        .name = "学习习惯集",
        .description = "每日学习计划",
        .color = "#6366f1",
        .wallpaper = "default.jpg",
    };

    try std.testing.expectEqual(row.id, 1);
    try std.testing.expectEqualStrings(row.name, "学习习惯集");
    try std.testing.expectEqualStrings(row.description, "每日学习计划");
    try std.testing.expectEqualStrings(row.color, "#6366f1");
    try std.testing.expectEqualStrings(row.wallpaper, "default.jpg");
}

test "HabitRow 初始化" {
    const row: habit_crud.HabitRow = .{
        .id = 1,
        .set_id = 1,
        .name = "背单词",
        .goal_seconds = 1800,
        .color = "#10b981",
        .wallpaper = "",
    };

    try std.testing.expectEqual(row.id, 1);
    try std.testing.expectEqual(row.set_id, 1);
    try std.testing.expectEqualStrings(row.name, "背单词");
    try std.testing.expectEqual(row.goal_seconds, 1800);
}

test "HabitRow goal_seconds 为零" {
    const row: habit_crud.HabitRow = .{
        .id = 1,
        .set_id = 1,
        .name = "自由计时",
        .goal_seconds = 0,
        .color = "#6366f1",
        .wallpaper = "",
    };

    try std.testing.expectEqual(row.goal_seconds, 0);
}

test "HabitRow wallpaper 为空" {
    const row: habit_crud.HabitRow = .{
        .id = 1,
        .set_id = 1,
        .name = "测试习惯",
        .goal_seconds = 1500,
        .color = "#6366f1",
        .wallpaper = "",
    };

    try std.testing.expectEqualStrings(row.wallpaper, "");
}
