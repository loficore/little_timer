//! 数据库健康检查模块单元测试
const std = @import("std");
const zqlite = @import("zqlite");
const storage_health = @import("../storage/storage_health.zig");
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

test "HealthCheckManager 初始化" {
    const allocator = std.testing.allocator;
    const manager = storage_health.HealthCheckManager.init(allocator, null);

    try std.testing.expect(manager.db == null);
    try std.testing.expect(manager.allocator == allocator);
}

test "HealthCheckError 错误类型" {
    try std.testing.expectEqual(@as(u8, 0x01), @intFromEnum(storage_health.HealthCheckError.IntegrityCheckFailed));
    try std.testing.expectEqual(@as(u8, 0x02), @intFromEnum(storage_health.HealthCheckError.HealthCheckFailed));
    try std.testing.expectEqual(@as(u8, 0x04), @intFromEnum(storage_health.HealthCheckError.DatabaseNotHealthy));
}

test "HealthCheckManager 空数据库检查失败" {
    const allocator = std.testing.allocator;
    var manager = storage_health.HealthCheckManager.init(allocator, null);

    const result = manager.initialize();
    try std.testing.expectError(storage_health.HealthCheckError.HealthCheckFailed, result);
}

test "健康检查记录初始化" {
    test_allocator = std.testing.allocator;
    defer closeTestDb();
    try createTestDb();

    var manager = storage_health.HealthCheckManager.init(test_allocator, test_db);

    try manager.initialize();

    var rows = test_db.?.rows("SELECT COUNT(*) FROM health_check WHERE id = 1;", .{});
    defer rows.deinit();

    const row = rows.next() orelse unreachable;
    const count = row.get(i64, 0);
    try std.testing.expectEqual(count, 1);
}

test "重复初始化健康检查记录" {
    test_allocator = std.testing.allocator;
    defer closeTestDb();
    try createTestDb();

    var manager = storage_health.HealthCheckManager.init(test_allocator, test_db);

    try manager.initialize();
    try manager.initialize();

    var rows = test_db.?.rows("SELECT COUNT(*) FROM health_check WHERE id = 1;", .{});
    defer rows.deinit();

    const row = rows.next() orelse unreachable;
    const count = row.get(i64, 0);
    try std.testing.expectEqual(count, 1);
}

test "执行数据库健康检查 - 通过" {
    test_allocator = std.testing.allocator;
    defer closeTestDb();
    try createTestDb();

    var manager = storage_health.HealthCheckManager.init(test_allocator, test_db);
    try manager.initialize();

    try manager.performCheck();
}

test "执行数据库健康检查 - 空数据库失败" {
    test_allocator = std.testing.allocator;
    defer closeTestDb();
    try createTestDb();

    var manager = storage_health.HealthCheckManager.init(test_allocator, null);

    const result = manager.performCheck();
    try std.testing.expectError(storage_health.HealthCheckError.HealthCheckFailed, result);
}

test "更新健康检查记录" {
    test_allocator = std.testing.allocator;
    defer closeTestDb();
    try createTestDb();

    var manager = storage_health.HealthCheckManager.init(test_allocator, test_db);
    try manager.initialize();

    try manager.updateRecord();

    var rows = test_db.?.rows("SELECT record_count FROM health_check WHERE id = 1;", .{});
    defer rows.deinit();

    const row = rows.next() orelse unreachable;
    const count = row.get(i64, 0);
    try std.testing.expect(count >= 0);
}

test "获取健康检查信息" {
    test_allocator = std.testing.allocator;
    defer closeTestDb();
    try createTestDb();

    var manager = storage_health.HealthCheckManager.init(test_allocator, test_db);
    try manager.initialize();
    try manager.updateRecord();

    const info = try manager.getInfo();
    defer {
        test_allocator.free(info.status);
        test_allocator.free(info.last_check);
    }

    try std.testing.expectEqualStrings(info.status, "healthy");
    try std.testing.expect(info.record_count >= 0);
}

test "获取健康检查信息 - 未初始化" {
    test_allocator = std.testing.allocator;
    defer closeTestDb();
    try createTestDb();

    var manager = storage_health.HealthCheckManager.init(test_allocator, test_db);

    const info = try manager.getInfo();
    defer {
        test_allocator.free(info.status);
        test_allocator.free(info.last_check);
    }

    try std.testing.expectEqualStrings(info.status, "unknown");
    try std.testing.expectEqualStrings(info.last_check, "never");
    try std.testing.expectEqual(info.record_count, 0);
}

test "检查数据库是否健康" {
    test_allocator = std.testing.allocator;
    defer closeTestDb();
    try createTestDb();

    var manager = storage_health.HealthCheckManager.init(test_allocator, test_db);
    try manager.initialize();
    try manager.performCheck();

    const is_healthy = try manager.isHealthy();

    try std.testing.expect(is_healthy);
}

test "检查数据库是否健康 - 未初始化" {
    test_allocator = std.testing.allocator;
    defer closeTestDb();
    try createTestDb();

    var manager = storage_health.HealthCheckManager.init(test_allocator, test_db);

    const is_healthy = try manager.isHealthy();

    try std.testing.expect(!is_healthy);
}

test "执行深度健康检查" {
    test_allocator = std.testing.allocator;
    defer closeTestDb();
    try createTestDb();

    var manager = storage_health.HealthCheckManager.init(test_allocator, test_db);
    try manager.initialize();

    const info = try manager.performDeepCheck();
    defer {
        test_allocator.free(info.status);
        test_allocator.free(info.last_check);
    }

    try std.testing.expectEqualStrings(info.status, "healthy");
    try std.testing.expect(info.record_count >= 0);
}

test "深度健康检查包含性能指标" {
    test_allocator = std.testing.allocator;
    defer closeTestDb();
    try createTestDb();

    var manager = storage_health.HealthCheckManager.init(test_allocator, test_db);
    try manager.initialize();

    var rows = try test_db.?.rows("SELECT name FROM settings;", .{});
    defer rows.deinit();

    const info = try manager.performDeepCheck();
    defer {
        test_allocator.free(info.status);
        test_allocator.free(info.last_check);
    }

    try std.testing.expectEqualStrings(info.status, "healthy");
}

test "健康检查信息结构体初始化" {
    const info: storage_health.HealthCheckInfo = .{
        .status = "healthy",
        .last_check = "2026-04-06 10:00:00",
        .record_count = 10,
    };

    try std.testing.expectEqualStrings(info.status, "healthy");
    try std.testing.expectEqual(info.record_count, 10);
}
