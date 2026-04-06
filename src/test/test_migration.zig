//! 数据库迁移模块单元测试
const std = @import("std");
const storage_migration = @import("../storage/storage_migration.zig");

test "MigrationManager 初始化" {
    const allocator = std.testing.allocator;
    const manager = storage_migration.MigrationManager.init(allocator, null);

    try std.testing.expect(manager.db == null);
    try std.testing.expect(manager.allocator == allocator);
}

test "CURRENT_SCHEMA_VERSION 常量" {
    try std.testing.expectEqual(storage_migration.CURRENT_SCHEMA_VERSION, 5);
}

test "MigrationError 错误类型" {
    try std.testing.expectEqual(@as(u8, 0x01), @intFromEnum(storage_migration.MigrationError.InvalidSchemaVersion));
    try std.testing.expectEqual(@as(u8, 0x02), @intFromEnum(storage_migration.MigrationError.MigrationFailed));
    try std.testing.expectEqual(@as(u8, 0x04), @intFromEnum(storage_migration.MigrationError.TableCreationFailed));
}

test "MigrationError 空数据库检查失败" {
    const allocator = std.testing.allocator;
    var manager = storage_migration.MigrationManager.init(allocator, null);

    const result = manager.checkAndMigrate();
    try std.testing.expectError(storage_migration.MigrationError.TableCreationFailed, result);
}
