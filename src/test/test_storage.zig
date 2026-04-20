//! 存储层单元测试 - SQLite 和 CRUD 操作
const std = @import("std");
const zqlite = @import("zqlite");
const habit_crud = @import("../storage/habit_crud.zig");
const storage_sqlite = @import("../storage/storage_sqlite.zig");

test "HabitCrudManager 初始化" {
    const allocator = std.testing.allocator;
    const manager = habit_crud.HabitCrudManager.init(allocator, null);

    try std.testing.expect(manager.db == null);
    try std.testing.expect(manager.allocator == allocator);
}

test "HabitCrudManager 空数据库查询失败" {
    const allocator = std.testing.allocator;
    const manager = habit_crud.HabitCrudManager.init(allocator, null);

    const result = manager.getAllHabitSets();
    try std.testing.expectError(habit_crud.HabitError.QueryFailed, result);
}

test "HabitSetRow 结构体初始化" {
    const row: habit_crud.HabitSetRow = .{
        .id = 1,
        .name = "测试习惯集",
        .description = "测试描述",
        .color = "#6366f1",
        .wallpaper = "",
    };

    try std.testing.expectEqual(row.id, 1);
    try std.testing.expectEqualStrings(row.name, "测试习惯集");
    try std.testing.expectEqualStrings(row.color, "#6366f1");
}

test "HabitRow 结构体初始化" {
    const row: habit_crud.HabitRow = .{
        .id = 1,
        .set_id = 1,
        .name = "背单词",
        .goal_seconds = 1500,
        .color = "#6366f1",
        .wallpaper = "",
    };

    try std.testing.expectEqual(row.id, 1);
    try std.testing.expectEqual(row.set_id, 1);
    try std.testing.expectEqual(row.goal_seconds, 1500);
}

test "SessionRow 结构体初始化" {
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
}

test "TimerSessionRow 结构体初始化" {
    const row: habit_crud.TimerSessionRow = .{
        .id = 1,
        .habit_id = 1,
        .mode = "stopwatch",
        .started_at = 1704067200,
        .updated_at = 1704067200,
        .is_running = true,
        .is_finished = false,
        .is_paused = false,
        .elapsed_seconds = 0,
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
    try std.testing.expectEqual(row.work_duration, 1500);
    try std.testing.expectEqual(row.current_round, 1);
}

test "SqliteManager 初始化" {
    const allocator = std.testing.allocator;
    const db_path: [:0]const u8 = ":memory:";
    const backup_dir: []const u8 = "/tmp/test_backups";

    var manager = storage_sqlite.SqliteManager.init(allocator, db_path, backup_dir) catch unreachable;
    defer manager.deinit();

    try std.testing.expect(!manager.is_initialized);
    try std.testing.expect(manager.db == null);
}

test "SqliteError 错误类型" {
    try std.testing.expectEqual(@as(u16, 0x01), @intFromEnum(storage_sqlite.SqliteError.DatabaseOpenFailed));
    try std.testing.expectEqual(@as(u16, 0x02), @intFromEnum(storage_sqlite.SqliteError.TableCreationFailed));
    try std.testing.expectEqual(@as(u16, 0x04), @intFromEnum(storage_sqlite.SqliteError.InsertFailed));
}

test "HabitError 错误类型" {
    try std.testing.expectEqual(@as(u8, 0x01), @intFromEnum(habit_crud.HabitError.InsertFailed));
    try std.testing.expectEqual(@as(u8, 0x02), @intFromEnum(habit_crud.HabitError.UpdateFailed));
    try std.testing.expectEqual(@as(u8, 0x04), @intFromEnum(habit_crud.HabitError.DeleteFailed));
    try std.testing.expectEqual(@as(u8, 0x08), @intFromEnum(habit_crud.HabitError.QueryFailed));
    try std.testing.expectEqual(@as(u8, 0x10), @intFromEnum(habit_crud.HabitError.NotFound));
}
