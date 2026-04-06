//! 数据库备份恢复模块单元测试
const std = @import("std");
const zqlite = @import("zqlite");
const storage_backup = @import("../storage/storage_backup.zig");
const storage_migration = @import("../storage/storage_migration.zig");

var test_db: ?zqlite.Conn = null;
var test_allocator: std.mem.Allocator = undefined;
var test_db_path: []const u8 = "";
var test_backup_dir: []const u8 = "";

fn createTestDbAndBackupDirs() !void {
    test_allocator = std.testing.allocator;

    const tmp_dir = try std.fs.cwd().makeOpenPath("test_tmp", .{});
    defer tmp_dir.close();

    test_db_path = try std.fs.path.join(test_allocator, &[_][]const u8{ "test_tmp", "test_presets.db" });
    test_backup_dir = try std.fs.path.join(test_allocator, &[_][]const u8{ "test_tmp", "backups" });

    const flags = zqlite.OpenFlags.Create | zqlite.OpenFlags.ReadWrite;
    test_db = try zqlite.open(test_db_path, flags);

    var migration_manager = storage_migration.MigrationManager.init(test_allocator, test_db);
    try migration_manager.checkAndMigrate();
}

fn cleanupTestFiles() void {
    if (test_db) |db| {
        db.close();
        test_db = null;
    }

    if (test_db_path.len > 0) {
        std.fs.cwd().deleteFile(test_db_path) catch {};
        test_allocator.free(test_db_path);
        test_db_path = "";
    }

    if (test_backup_dir.len > 0) {
        std.fs.cwd().deleteTree("test_tmp/backups") catch {};
        std.fs.cwd().deleteTree("test_tmp") catch {};
        test_allocator.free(test_backup_dir);
        test_backup_dir = "";
    }
}

test "BackupManager 初始化" {
    const allocator = std.testing.allocator;
    const manager = storage_backup.BackupManager.init(allocator, null, ":memory:", "/tmp/backups");

    try std.testing.expect(manager.db == null);
    try std.testing.expectEqual(manager.max_backups, 10);
}

test "BackupError 错误类型" {
    try std.testing.expectEqual(@as(u8, 0x01), @intFromEnum(storage_backup.BackupError.BackupFailed));
    try std.testing.expectEqual(@as(u8, 0x02), @intFromEnum(storage_backup.BackupError.RestoreFailed));
    try std.testing.expectEqual(@as(u8, 0x04), @intFromEnum(storage_backup.BackupError.InvalidBackupPath));
    try std.testing.expectEqual(@as(u8, 0x08), @intFromEnum(storage_backup.BackupError.DatabaseOpenFailed));
}

test "BackupManager 空数据库备份失败" {
    const allocator = std.testing.allocator;
    var manager = storage_backup.BackupManager.init(allocator, null, ":memory:", "/tmp/backups");

    const result = manager.createBackup();
    try std.testing.expectError(storage_backup.BackupError.DatabaseOpenFailed, result);
}

test "创建数据库备份" {
    test_allocator = std.testing.allocator;
    defer cleanupTestFiles();
    try createTestDbAndBackupDirs();

    var manager = storage_backup.BackupManager.init(test_allocator, test_db, test_db_path, test_backup_dir);

    const backup_path = try manager.createBackup();
    defer test_allocator.free(backup_path);

    const backup_file = try std.fs.cwd().openFile(backup_path, .{});
    defer backup_file.close();

    const stat = try backup_file.stat();
    try std.testing.expect(stat.size > 0);
}

test "从备份恢复数据库" {
    test_allocator = std.testing.allocator;
    defer cleanupTestFiles();
    try createTestDbAndBackupDirs();

    var manager = storage_backup.BackupManager.init(test_allocator, test_db, test_db_path, test_backup_dir);

    const backup_path = try manager.createBackup();
    defer test_allocator.free(backup_path);

    const reopenFn = struct {
        fn reopen(mgr: *storage_backup.BackupManager) anyerror!void {
            const flags = zqlite.OpenFlags.Create | zqlite.OpenFlags.ReadWrite;
            mgr.db = try zqlite.open(test_db_path, flags);
        }
    }.reopen;

    try manager.restoreFromBackup(backup_path, reopenFn);

    try std.testing.expect(test_db != null);
}

test "获取备份目录信息" {
    test_allocator = std.testing.allocator;
    defer cleanupTestFiles();
    try createTestDbAndBackupDirs();

    var manager = storage_backup.BackupManager.init(test_allocator, test_db, test_db_path, test_backup_dir);

    _ = try manager.createBackup();
    _ = try manager.createBackup();
    _ = try manager.createBackup();

    const info = try manager.getBackupInfo();
    defer manager.freeBackupInfo(info);

    try std.testing.expectEqual(info.total_backups, 3);
    try std.testing.expect(info.total_size_bytes > 0);
    try std.testing.expect(info.oldest_backup != null);
    try std.testing.expect(info.newest_backup != null);
}

test "清理旧备份 - 超出限制" {
    test_allocator = std.testing.allocator;
    defer cleanupTestFiles();
    try createTestDbAndBackupDirs();

    var manager = storage_backup.BackupManager.init(test_allocator, test_db, test_db_path, test_backup_dir);
    manager.max_backups = 3;

    _ = try manager.createBackup();
    _ = try manager.createBackup();
    _ = try manager.createBackup();
    _ = try manager.createBackup();
    _ = try manager.createBackup();

    const info = try manager.getBackupInfo();
    defer manager.freeBackupInfo(info);

    try std.testing.expectEqual(info.total_backups, 3);
}

test "清理旧备份 - 未超出限制" {
    test_allocator = std.testing.allocator;
    defer cleanupTestFiles();
    try createTestDbAndBackupDirs();

    var manager = storage_backup.BackupManager.init(test_allocator, test_db, test_db_path, test_backup_dir);
    manager.max_backups = 5;

    _ = try manager.createBackup();
    _ = try manager.createBackup();
    _ = try manager.createBackup();

    const info = try manager.getBackupInfo();
    defer manager.freeBackupInfo(info);

    try std.testing.expectEqual(info.total_backups, 3);
}

test "备份文件名称格式正确" {
    test_allocator = std.testing.allocator;
    defer cleanupTestFiles();
    try createTestDbAndBackupDirs();

    var manager = storage_backup.BackupManager.init(test_allocator, test_db, test_db_path, test_backup_dir);

    const backup_path = try manager.createBackup();
    defer test_allocator.free(backup_path);

    const name = std.fs.path.basename(backup_path);
    try std.testing.expect(std.mem.startsWith(u8, name, "presets_backup_"));
    try std.testing.expect(std.mem.endsWith(u8, name, ".db"));
}

test "备份后数据库仍然可用" {
    test_allocator = std.testing.allocator;
    defer cleanupTestFiles();
    try createTestDbAndBackupDirs();

    var manager = storage_backup.BackupManager.init(test_allocator, test_db, test_db_path, test_backup_dir);

    _ = try manager.createBackup();

    try std.testing.expect(test_db != null);

    var rows = test_db.?.rows("SELECT COUNT(*) FROM settings;", .{});
    defer rows.deinit();

    try std.testing.expect(rows.next() != null);
}

test "空备份目录信息" {
    test_allocator = std.testing.allocator;
    defer cleanupTestFiles();
    try createTestDbAndBackupDirs();

    const backup_dir = try std.fs.cwd().makeOpenPath(test_backup_dir, .{});
    defer backup_dir.close();

    var manager = storage_backup.BackupManager.init(test_allocator, test_db, test_db_path, test_backup_dir);

    const info = try manager.getBackupInfo();
    defer manager.freeBackupInfo(info);

    try std.testing.expectEqual(info.total_backups, 0);
    try std.testing.expectEqual(info.total_size_bytes, 0);
    try std.testing.expect(info.oldest_backup == null);
    try std.testing.expect(info.newest_backup == null);
}

test "恢复备份 - 文件不存在失败" {
    test_allocator = std.testing.allocator;
    defer cleanupTestFiles();
    try createTestDbAndBackupDirs();

    var manager = storage_backup.BackupManager.init(test_allocator, test_db, test_db_path, test_backup_dir);

    const reopenFn = struct {
        fn reopen(mgr: *storage_backup.BackupManager) anyerror!void {
            const flags = zqlite.OpenFlags.Create | zqlite.OpenFlags.ReadWrite;
            mgr.db = try zqlite.open(test_db_path, flags);
        }
    }.reopen;

    const result = manager.restoreFromBackup("/nonexistent/path.db", reopenFn);
    try std.testing.expectError(storage_backup.BackupError.InvalidBackupPath, result);
}
