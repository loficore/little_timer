//! 迁移路径测试
const std = @import("std");
const storage_migration = @import("../storage/storage_migration.zig");

test "MigrationError 错误类型" {
    try std.testing.expectEqual(@as(u16, 0x01), @intFromEnum(storage_migration.MigrationError.InitFailed));
    try std.testing.expectEqual(@as(u16, 0x02), @intFromEnum(storage_migration.MigrationError.MigrationFailed));
    try std.testing.expectEqual(@as(u16, 0x04), @intFromEnum(storage_migration.MigrationError.DatabaseCorrupted));
}

test "CURRENT_SCHEMA_VERSION 是当前版本" {
    try std.testing.expectEqual(@as(u32, 5), storage_migration.CURRENT_SCHEMA_VERSION);
}

test "MigrationManager 初始化" {
    const allocator = std.testing.allocator;
    const db_path: [:0]const u8 = ":memory:";

    const manager = storage_migration.MigrationManager.init(allocator, db_path);
    try std.testing.expectEqual(manager.allocator, allocator);
    try std.testing.expectEqualStrings(manager.db_path, ":memory:");
}
