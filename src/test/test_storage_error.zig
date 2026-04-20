//! 文件/数据库错误处理测试
const std = @import("std");
const zqlite = @import("zqlite");
const habit_crud = @import("../storage/habit_crud.zig");
const storage_sqlite = @import("../storage/storage_sqlite.zig");
const storage_backup = @import("../storage/storage_backup.zig");
const storage_health = @import("../storage/storage_health.zig");
const storage_migration = @import("../storage/storage_migration.zig");

test "HabitCrudManager 无数据库时查询返回错误" {
    const allocator = std.testing.allocator;
    const manager = habit_crud.HabitCrudManager.init(allocator, null);

    const result = manager.getHabitById(1);
    try std.testing.expectError(habit_crud.HabitError.QueryFailed, result);
}

test "HabitCrudManager 无数据库时获取所有习惯返回错误" {
    const allocator = std.testing.allocator;
    const manager = habit_crud.HabitCrudManager.init(allocator, null);

    const result = manager.getHabitsBySet(1);
    try std.testing.expectError(habit_crud.HabitError.QueryFailed, result);
}

test "HabitCrudManager 无数据库时创建习惯返回错误" {
    const allocator = std.testing.allocator;
    const manager = habit_crud.HabitCrudManager.init(allocator, null);

    const result = manager.createHabit(1, "测试习惯", 1500, "#6366f1", "");
    try std.testing.expectError(habit_crud.HabitError.InsertFailed, result);
}

test "HabitCrudManager 无数据库时删除习惯返回错误" {
    const allocator = std.testing.allocator;
    const manager = habit_crud.HabitCrudManager.init(allocator, null);

    const result = manager.deleteHabit(1);
    try std.testing.expectError(habit_crud.HabitError.DeleteFailed, result);
}

test "HabitCrudManager 无数据库时更新习惯返回错误" {
    const allocator = std.testing.allocator;
    const manager = habit_crud.HabitCrudManager.init(allocator, null);

    const result = manager.updateHabit(1, "新名称", null, null, null, null);
    try std.testing.expectError(habit_crud.HabitError.UpdateFailed, result);
}

test "BackupManager 无数据库时备份返回错误" {
    const allocator = std.testing.allocator;
    const db_path: [:0]const u8 = ":memory:";
    const backup_dir: []const u8 = "/tmp/nonexistent_backup_dir_12345";
    const manager = storage_backup.BackupManager.init(allocator, db_path, backup_dir);

    const result = manager.createBackup();
    try std.testing.expectError(storage_backup.BackupError.CreateBackupFailed, result);
}

test "BackupManager 空数据库时备份返回错误" {
    const allocator = std.testing.allocator;
    const db_path: [:0]const u8 = ":memory:";
    const backup_dir: []const u8 = "/tmp/test_backup_empty";

    std.fs.cwd().makeDir(backup_dir) catch {};

    var manager = storage_backup.BackupManager.init(allocator, db_path, backup_dir) catch unreachable;
    defer {
        manager.deinit();
        std.fs.cwd().deleteTree(backup_dir) catch {};
    }

    const result = manager.createBackup();
    try std.testing.expectError(storage_backup.BackupError.CreateBackupFailed, result);
}

test "BackupError 错误类型定义" {
    try std.testing.expectEqual(@as(u16, 0x01), @intFromEnum(storage_backup.BackupError.InitFailed));
    try std.testing.expectEqual(@as(u16, 0x02), @intFromEnum(storage_backup.BackupError.CreateBackupFailed));
    try std.testing.expectEqual(@as(u16, 0x04), @intFromEnum(storage_backup.BackupError.RestoreFailed));
    try std.testing.expectEqual(@as(u16, 0x08), @intFromEnum(storage_backup.BackupError.BackupNotFound));
    try std.testing.expectEqual(@as(u16, 0x10), @intFromEnum(storage_backup.BackupError.InvalidBackup));
}

test "HealthCheckManager 未初始化时健康检查返回错误" {
    const allocator = std.testing.allocator;
    const manager = storage_health.HealthCheckManager.init(allocator);

    const result = manager.performCheck();
    try std.testing.expectError(storage_health.HealthCheckError.NotInitialized, result);
}

test "HealthCheckManager 未初始化时 isHealthy 返回 false" {
    const allocator = std.testing.allocator;
    const manager = storage_health.HealthCheckManager.init(allocator);

    try std.testing.expect(!manager.isHealthy());
}

test "HealthCheckManager 未初始化时 getInfo 返回 null" {
    const allocator = std.testing.allocator;
    const manager = storage_health.HealthCheckManager.init(allocator);

    const info = manager.getInfo();
    try std.testing.expect(info == null);
}

test "HealthCheckError 错误类型定义" {
    try std.testing.expectEqual(@as(u16, 0x01), @intFromEnum(storage_health.HealthCheckError.InitFailed));
    try std.testing.expectEqual(@as(u16, 0x02), @intFromEnum(storage_health.HealthCheckError.CheckFailed));
    try std.testing.expectEqual(@as(u16, 0x04), @intFromEnum(storage_health.HealthCheckError.NotInitialized));
    try std.testing.expectEqual(@as(u16, 0x08), @intFromEnum(storage_health.HealthCheckError.DatabaseCorrupted));
}

test "MigrationManager 初始版本为 5" {
    try std.testing.expectEqual(@as(u32, 5), storage_migration.CURRENT_SCHEMA_VERSION);
}

test "MigrationError 错误类型定义" {
    try std.testing.expectEqual(@as(u16, 0x01), @intFromEnum(storage_migration.MigrationError.InitFailed));
    try std.testing.expectEqual(@as(u16, 0x02), @intFromEnum(storage_migration.MigrationError.MigrationFailed));
    try std.testing.expectEqual(@as(u16, 0x04), @intFromEnum(storage_migration.MigrationError.DatabaseCorrupted));
}

test "SqliteManager 未初始化时操作返回错误" {
    const allocator = std.testing.allocator;
    const db_path: [:0]const u8 = ":memory:";
    const backup_dir: []const u8 = "/tmp/test_sqlite_error";

    var manager = storage_sqlite.SqliteManager.init(allocator, db_path, backup_dir) catch unreachable;
    defer manager.deinit();

    try std.testing.expect(!manager.is_initialized);

    const result = manager.habit_manager.getAllHabitSets();
    try std.testing.expectError(habit_crud.HabitError.QueryFailed, result);
}

test "SqliteError 错误码定义" {
    try std.testing.expectEqual(@as(u16, 0x01), @intFromEnum(storage_sqlite.SqliteError.DatabaseOpenFailed));
    try std.testing.expectEqual(@as(u16, 0x02), @intFromEnum(storage_sqlite.SqliteError.TableCreationFailed));
    try std.testing.expectEqual(@as(u16, 0x04), @intFromEnum(storage_sqlite.SqliteError.InsertFailed));
    try std.testing.expectEqual(@as(u16, 0x08), @intFromEnum(storage_sqlite.SqliteError.UpdateFailed));
    try std.testing.expectEqual(@as(u16, 0x10), @intFromEnum(storage_sqlite.SqliteError.DeleteFailed));
    try std.testing.expectEqual(@as(u16, 0x20), @intFromEnum(storage_sqlite.SqliteError.QueryFailed));
    try std.testing.expectEqual(@as(u16, 0x40), @intFromEnum(storage_sqlite.SqliteError.TransactionFailed));
}

test "HabitCrudManager 无数据库时创建 TimerSession 返回错误" {
    const allocator = std.testing.allocator;
    const manager = habit_crud.HabitCrudManager.init(allocator, null);

    const result = manager.createTimerSession(null, "stopwatch", 1500, 0, 0);
    try std.testing.expectError(habit_crud.HabitError.InsertFailed, result);
}

test "HabitCrudManager 无数据库时获取活跃 TimerSession 返回 null" {
    const allocator = std.testing.allocator;
    const manager = habit_crud.HabitCrudManager.init(allocator, null);

    const result = manager.getActiveTimerSession();
    try std.testing.expect(result == null);
}

test "HabitCrudManager 无数据库时获取习惯连续天数返回错误" {
    const allocator = std.testing.allocator;
    const manager = habit_crud.HabitCrudManager.init(allocator, null);

    const result = manager.getHabitStreak(1);
    try std.testing.expectError(habit_crud.HabitError.QueryFailed, result);
}

test "HabitCrudManager 无数据库时获取习惯今日专注秒数返回错误" {
    const allocator = std.testing.allocator;
    const manager = habit_crud.HabitCrudManager.init(allocator, null);

    const result = manager.getHabitTodaySeconds(1);
    try std.testing.expectError(habit_crud.HabitError.QueryFailed, result);
}
