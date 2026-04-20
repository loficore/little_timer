//! 习惯 CRUD 操作模块单元测试
const std = @import("std");
const zqlite = @import("zqlite");
const habit_crud = @import("../storage/habit_crud.zig");
const storage_migration = @import("../storage/storage_migration.zig");

var test_db: ?zqlite.Conn = null;
var test_allocator: std.mem.Allocator = undefined;

fn createTestDb() !void {
    const flags = zqlite.OpenFlags.Create | zqlite.OpenFlags.ReadWrite;
    test_db = try zqlite.open(":memory:", flags);

    var migration_manager = storage_migration.MigrationManager.init(test_allocator, test_db);
    try migration_manager.checkAndMigrate();
}

fn closeTestDb() void {
    if (test_db) |db| {
        db.close();
        test_db = null;
    }
}

test "HabitCrudManager 初始化（空数据库）" {
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

test "HabitError 错误类型" {
    try std.testing.expectEqual(@as(u8, 0x01), @intFromEnum(habit_crud.HabitError.InsertFailed));
    try std.testing.expectEqual(@as(u8, 0x02), @intFromEnum(habit_crud.HabitError.UpdateFailed));
    try std.testing.expectEqual(@as(u8, 0x04), @intFromEnum(habit_crud.HabitError.DeleteFailed));
    try std.testing.expectEqual(@as(u8, 0x08), @intFromEnum(habit_crud.HabitError.QueryFailed));
    try std.testing.expectEqual(@as(u8, 0x10), @intFromEnum(habit_crud.HabitError.NotFound));
}

test "习惯集 CRUD - 创建习惯集" {
    test_allocator = std.testing.allocator;
    defer closeTestDb();
    try createTestDb();

    var manager = habit_crud.HabitCrudManager.init(test_allocator, test_db);

    const set_id = try manager.createHabitSet("学习习惯集", "每日学习", "#6366f1");
    try std.testing.expect(set_id > 0);
}

test "习惯集 CRUD - 获取所有习惯集" {
    test_allocator = std.testing.allocator;
    defer closeTestDb();
    try createTestDb();

    var manager = habit_crud.HabitCrudManager.init(test_allocator, test_db);

    _ = try manager.createHabitSet("学习习惯集", "每日学习", "#6366f1");
    _ = try manager.createHabitSet("运动习惯集", "每日运动", "#22c55e");

    const sets = try manager.getAllHabitSets();
    defer manager.freeHabitSets(sets);

    try std.testing.expectEqual(sets.len, 2);
    try std.testing.expectEqualStrings(sets[0].name, "运动习惯集");
    try std.testing.expectEqualStrings(sets[1].name, "学习习惯集");
}

test "习惯集 CRUD - 更新习惯集" {
    test_allocator = std.testing.allocator;
    defer closeTestDb();
    try createTestDb();

    var manager = habit_crud.HabitCrudManager.init(test_allocator, test_db);

    const set_id = try manager.createHabitSet("原名称", "原描述", "#6366f1");
    try manager.updateHabitSet(set_id, "新名称", "新描述", "#22c55e", "");

    const sets = try manager.getAllHabitSets();
    defer manager.freeHabitSets(sets);

    try std.testing.expectEqualStrings(sets[0].name, "新名称");
    try std.testing.expectEqualStrings(sets[0].color, "#22c55e");
}

test "习惯集 CRUD - 删除习惯集" {
    test_allocator = std.testing.allocator;
    defer closeTestDb();
    try createTestDb();

    var manager = habit_crud.HabitCrudManager.init(test_allocator, test_db);

    const set_id = try manager.createHabitSet("待删除", "删除测试", "#6366f1");
    try manager.deleteHabitSet(set_id);

    const sets = try manager.getAllHabitSets();
    defer manager.freeHabitSets(sets);

    try std.testing.expectEqual(sets.len, 0);
}

test "习惯 CRUD - 创建习惯" {
    test_allocator = std.testing.allocator;
    defer closeTestDb();
    try createTestDb();

    var manager = habit_crud.HabitCrudManager.init(test_allocator, test_db);

    const set_id = try manager.createHabitSet("习惯集", "测试", "#6366f1");
    const habit_id = try manager.createHabit(set_id, "背单词", 1500, "#6366f1");

    try std.testing.expect(habit_id > 0);
}

test "习惯 CRUD - 获取所有习惯" {
    test_allocator = std.testing.allocator;
    defer closeTestDb();
    try createTestDb();

    var manager = habit_crud.HabitCrudManager.init(test_allocator, test_db);

    const set_id = try manager.createHabitSet("习惯集", "测试", "#6366f1");
    _ = try manager.createHabit(set_id, "背单词", 1500, "#6366f1");
    _ = try manager.createHabit(set_id, "听写", 900, "#22c55e");

    const habits = try manager.getAllHabits();
    defer manager.freeHabits(habits);

    try std.testing.expectEqual(habits.len, 2);
}

test "习惯 CRUD - 按习惯集获取习惯" {
    test_allocator = std.testing.allocator;
    defer closeTestDb();
    try createTestDb();

    var manager = habit_crud.HabitCrudManager.init(test_allocator, test_db);

    const set1_id = try manager.createHabitSet("习惯集1", "测试", "#6366f1");
    const set2_id = try manager.createHabitSet("习惯集2", "测试", "#22c55e");
    _ = try manager.createHabit(set1_id, "习惯A", 1500, "#6366f1");
    _ = try manager.createHabit(set2_id, "习惯B", 1500, "#22c55e");

    const habits = try manager.getHabitsBySet(set1_id);
    defer manager.freeHabits(habits);

    try std.testing.expectEqual(habits.len, 1);
    try std.testing.expectEqualStrings(habits[0].name, "习惯A");
}

test "习惯 CRUD - 更新习惯" {
    test_allocator = std.testing.allocator;
    defer closeTestDb();
    try createTestDb();

    var manager = habit_crud.HabitCrudManager.init(test_allocator, test_db);

    const set_id = try manager.createHabitSet("习惯集", "测试", "#6366f1");
    const habit_id = try manager.createHabit(set_id, "原名称", 1500, "#6366f1");
    try manager.updateHabit(habit_id, "新名称", 1800, "#22c55e", "");

    const habit = try manager.getHabitById(habit_id);
    defer {
        if (habit) |h| {
            test_allocator.free(h.name);
            test_allocator.free(h.color);
            test_allocator.free(h.wallpaper);
        }
    }

    try std.testing.expect(habit != null);
    try std.testing.expectEqualStrings(habit.?.name, "新名称");
    try std.testing.expectEqual(habit.?.goal_seconds, 1800);
}

test "习惯 CRUD - 删除习惯" {
    test_allocator = std.testing.allocator;
    defer closeTestDb();
    try createTestDb();

    var manager = habit_crud.HabitCrudManager.init(test_allocator, test_db);

    const set_id = try manager.createHabitSet("习惯集", "测试", "#6366f1");
    const habit_id = try manager.createHabit(set_id, "待删除", 1500, "#6366f1");
    try manager.deleteHabit(habit_id);

    const habits = try manager.getAllHabits();
    defer manager.freeHabits(habits);

    try std.testing.expectEqual(habits.len, 0);
}

test "习惯 CRUD - 获取习惯详情" {
    test_allocator = std.testing.allocator;
    defer closeTestDb();
    try createTestDb();

    var manager = habit_crud.HabitCrudManager.init(test_allocator, test_db);

    const set_id = try manager.createHabitSet("习惯集", "测试", "#6366f1");
    const habit_id = try manager.createHabit(set_id, "背单词", 1500, "#6366f1");

    const habit = try manager.getHabitById(habit_id);
    defer {
        if (habit) |h| {
            test_allocator.free(h.name);
            test_allocator.free(h.color);
            test_allocator.free(h.wallpaper);
        }
    }

    try std.testing.expect(habit != null);
    try std.testing.expectEqual(habit.?.id, habit_id);
    try std.testing.expectEqualStrings(habit.?.name, "背单词");
    try std.testing.expectEqual(habit.?.goal_seconds, 1500);
}

test "记录 CRUD - 创建记录" {
    test_allocator = std.testing.allocator;
    defer closeTestDb();
    try createTestDb();

    var manager = habit_crud.HabitCrudManager.init(test_allocator, test_db);

    const set_id = try manager.createHabitSet("习惯集", "测试", "#6366f1");
    const habit_id = try manager.createHabit(set_id, "背单词", 1500, "#6366f1");

    const session_id = try manager.createSession(habit_id, 1500, 1, "2026-04-06");

    try std.testing.expect(session_id > 0);
}

test "记录 CRUD - 按日期获取记录" {
    test_allocator = std.testing.allocator;
    defer closeTestDb();
    try createTestDb();

    var manager = habit_crud.HabitCrudManager.init(test_allocator, test_db);

    const set_id = try manager.createHabitSet("习惯集", "测试", "#6366f1");
    const habit_id = try manager.createHabit(set_id, "背单词", 1500, "#6366f1");

    _ = try manager.createSession(habit_id, 1500, 1, "2026-04-06");
    _ = try manager.createSession(habit_id, 600, 1, "2026-04-06");
    _ = try manager.createSession(habit_id, 900, 1, "2026-04-05");

    const sessions = try manager.getSessionsByDate("2026-04-06");
    defer manager.freeSessions(sessions);

    try std.testing.expectEqual(sessions.len, 2);
}

test "记录 CRUD - 按日期范围获取记录" {
    test_allocator = std.testing.allocator;
    defer closeTestDb();
    try createTestDb();

    var manager = habit_crud.HabitCrudManager.init(test_allocator, test_db);

    const set_id = try manager.createHabitSet("习惯集", "测试", "#6366f1");
    const habit_id = try manager.createHabit(set_id, "背单词", 1500, "#6366f1");

    _ = try manager.createSession(habit_id, 1500, 1, "2026-04-01");
    _ = try manager.createSession(habit_id, 600, 1, "2026-04-05");
    _ = try manager.createSession(habit_id, 900, 1, "2026-04-10");

    const sessions = try manager.getSessionsByDateRange("2026-04-01", "2026-04-07");
    defer manager.freeSessions(sessions);

    try std.testing.expectEqual(sessions.len, 2);
}

test "记录 CRUD - 获取习惯今日时长" {
    test_allocator = std.testing.allocator;
    defer closeTestDb();
    try createTestDb();

    var manager = habit_crud.HabitCrudManager.init(test_allocator, test_db);

    const set_id = try manager.createHabitSet("习惯集", "测试", "#6366f1");
    const habit_id = try manager.createHabit(set_id, "背单词", 1500, "#6366f1");

    _ = try manager.createSession(habit_id, 1500, 1, "2026-04-06");
    _ = try manager.createSession(habit_id, 600, 1, "2026-04-06");

    const seconds = try manager.getHabitTodaySeconds(habit_id, "2026-04-06");

    try std.testing.expectEqual(seconds, 2100);
}

test "记录 CRUD - 获取习惯连胜" {
    test_allocator = std.testing.allocator;
    defer closeTestDb();
    try createTestDb();

    var manager = habit_crud.HabitCrudManager.init(test_allocator, test_db);

    const set_id = try manager.createHabitSet("习惯集", "测试", "#6366f1");
    const habit_id = try manager.createHabit(set_id, "背单词", 1500, "#6366f1");

    _ = try manager.createSession(habit_id, 1500, 1, "2026-04-06");
    _ = try manager.createSession(habit_id, 1500, 1, "2026-04-05");
    _ = try manager.createSession(habit_id, 1500, 1, "2026-04-04");
    _ = try manager.createSession(habit_id, 500, 1, "2026-04-03");

    const streak = try manager.getHabitStreak(habit_id, 1500);

    try std.testing.expectEqual(streak, 3);
}

test "TimerSession CRUD - 创建计时会话" {
    test_allocator = std.testing.allocator;
    defer closeTestDb();
    try createTestDb();

    var manager = habit_crud.HabitCrudManager.init(test_allocator, test_db);

    const session_id = try manager.createTimerSession(null, "stopwatch", 1500, 300, 0);

    try std.testing.expect(session_id > 0);
}

test "TimerSession CRUD - 创建带习惯的计时会话" {
    test_allocator = std.testing.allocator;
    defer closeTestDb();
    try createTestDb();

    var manager = habit_crud.HabitCrudManager.init(test_allocator, test_db);

    const set_id = try manager.createHabitSet("习惯集", "测试", "#6366f1");
    const habit_id = try manager.createHabit(set_id, "背单词", 1500, "#6366f1");

    const session_id = try manager.createTimerSession(habit_id, "stopwatch", 1500, 300, 0);

    try std.testing.expect(session_id > 0);
}

test "TimerSession CRUD - 获取活跃计时会话" {
    test_allocator = std.testing.allocator;
    defer closeTestDb();
    try createTestDb();

    var manager = habit_crud.HabitCrudManager.init(test_allocator, test_db);

    _ = try manager.createTimerSession(null, "stopwatch", 1500, 300, 0);

    const session = try manager.getActiveTimerSession();

    try std.testing.expect(session != null);
    try std.testing.expectEqual(session.?.is_running, true);
}

test "TimerSession CRUD - 更新计时会话" {
    test_allocator = std.testing.allocator;
    defer closeTestDb();
    try createTestDb();

    var manager = habit_crud.HabitCrudManager.init(test_allocator, test_db);

    const session_id = try manager.createTimerSession(null, "stopwatch", 1500, 300, 0);
    try manager.updateTimerSession(
        session_id,
        100,
        1400,
        0,
        null,
        null,
        true,
        false,
        false,
        1,
        false,
    );

    const session = try manager.getTimerSessionById(session_id);

    try std.testing.expect(session != null);
    try std.testing.expectEqual(session.?.elapsed_seconds, 100);
    try std.testing.expectEqual(session.?.remaining_seconds, 1400);
}

test "TimerSession CRUD - 暂停计时会话" {
    test_allocator = std.testing.allocator;
    defer closeTestDb();
    try createTestDb();

    var manager = habit_crud.HabitCrudManager.init(test_allocator, test_db);

    const session_id = try manager.createTimerSession(null, "stopwatch", 1500, 300, 0);
    const now = std.time.timestamp();
    try manager.updateTimerSession(
        session_id,
        100,
        1400,
        0,
        now,
        now,
        false,
        true,
        false,
        1,
        false,
    );

    const session = try manager.getTimerSessionById(session_id);

    try std.testing.expect(session != null);
    try std.testing.expectEqual(session.?.is_paused, true);
    try std.testing.expectEqual(session.?.pause_started_at, now);
}

test "TimerSession CRUD - 完成计时会话" {
    test_allocator = std.testing.allocator;
    defer closeTestDb();
    try createTestDb();

    var manager = habit_crud.HabitCrudManager.init(test_allocator, test_db);

    const session_id = try manager.createTimerSession(null, "stopwatch", 1500, 300, 0);
    try manager.finishTimerSession(session_id);

    const session = try manager.getTimerSessionById(session_id);

    try std.testing.expect(session != null);
    try std.testing.expectEqual(session.?.is_finished, true);
    try std.testing.expectEqual(session.?.is_running, false);
}

test "TimerSession CRUD - 删除计时会话" {
    test_allocator = std.testing.allocator;
    defer closeTestDb();
    try createTestDb();

    var manager = habit_crud.HabitCrudManager.init(test_allocator, test_db);

    const session_id = try manager.createTimerSession(null, "stopwatch", 1500, 300, 0);
    try manager.deleteTimerSession(session_id);

    const session = try manager.getTimerSessionById(session_id);

    try std.testing.expect(session == null);
}

test "内存释放 - freeHabitSets" {
    test_allocator = std.testing.allocator;
    defer closeTestDb();
    try createTestDb();

    var manager = habit_crud.HabitCrudManager.init(test_allocator, test_db);

    const set_id = try manager.createHabitSet("测试", "测试", "#6366f1");
    _ = set_id;

    const sets = try manager.getAllHabitSets();
    manager.freeHabitSets(sets);
}

test "内存释放 - freeHabits" {
    test_allocator = std.testing.allocator;
    defer closeTestDb();
    try createTestDb();

    var manager = habit_crud.HabitCrudManager.init(test_allocator, test_db);

    const set_id = try manager.createHabitSet("习惯集", "测试", "#6366f1");
    _ = try manager.createHabit(set_id, "背单词", 1500, "#6366f1");

    const habits = try manager.getAllHabits();
    manager.freeHabits(habits);
}

test "内存释放 - freeSessions" {
    test_allocator = std.testing.allocator;
    defer closeTestDb();
    try createTestDb();

    var manager = habit_crud.HabitCrudManager.init(test_allocator, test_db);

    const set_id = try manager.createHabitSet("习惯集", "测试", "#6366f1");
    const habit_id = try manager.createHabit(set_id, "背单词", 1500, "#6366f1");
    _ = try manager.createSession(habit_id, 1500, 1, "2026-04-06");

    const sessions = try manager.getSessionsByDate("2026-04-06");
    manager.freeSessions(sessions);
}
