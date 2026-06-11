//! storage_crud 模块单元测试
const std = @import("std");
const zqlite = @import("zqlite");
const crud = @import("../storage/storage_crud.zig");
const interface = @import("../core/interface.zig");
const migration = @import("../storage/storage_migration.zig");

var test_db: ?zqlite.Conn = null;
var test_allocator: std.mem.Allocator = undefined;
var test_db_path: []const u8 = "";

fn createTestDb() !void {
    test_allocator = std.testing.allocator;

    const tmp_dir = try std.fs.cwd().makeOpenPath("test_tmp", .{});
    defer tmp_dir.close();

    test_db_path = try std.fs.path.join(test_allocator, &[_][]const u8{ "test_tmp", "test_crud.db" });

    const flags = zqlite.OpenFlags.Create | zqlite.OpenFlags.ReadWrite;
    test_db = try zqlite.open(test_db_path, flags);

    var migration_manager = migration.MigrationManager.init(test_allocator, test_db);
    try migration_manager.checkAndMigrate();
}

fn cleanupTestFiles() void {
    if (test_db) |db| {
        db.close();
        test_db = null;
    }

    if (test_db_path.len > 0) {
        std.fs.cwd().deleteFile(test_db_path) catch {};
        std.fs.cwd().deleteTree("test_tmp") catch {};
        test_allocator.free(test_db_path);
        test_db_path = "";
    }
}

test "CrudManager init with valid db" {
    const allocator = std.testing.allocator;
    try createTestDb();
    defer cleanupTestFiles();

    var crud_manager = crud.CrudManager.init(test_allocator, test_db);
    try std.testing.expect(crud_manager.db != null);
    try std.testing.expect(crud_manager.allocator == test_allocator);
}

test "CrudManager init with null db" {
    const allocator = std.testing.allocator;
    var crud_manager = crud.CrudManager.init(allocator, null);
    try std.testing.expect(crud_manager.db == null);
}

test "saveSettings and loadSettings roundtrip" {
    const allocator = std.testing.allocator;
    try createTestDb();
    defer cleanupTestFiles();

    var crud_manager = crud.CrudManager.init(test_allocator, test_db);

    const config = interface.SettingsConfig{
        .basic = .{
            .timezone = 8,
            .language = try allocator.dupe(u8, "ZH"),
            .default_mode = .countdown,
            .theme_mode = try allocator.dupe(u8, "dark"),
            .wallpaper = try allocator.dupe(u8, ""),
        },
        .clock_defaults = .{
            .countdown = .{
                .duration_seconds = 1500,
                .loop = true,
                .loop_count = 4,
                .loop_interval_seconds = 300,
            },
            .stopwatch = .{
                .max_seconds = 3600,
            },
        },
        .logging = .{
            .level = try allocator.dupe(u8, "INFO"),
            .enable_timestamp = true,
            .tick_interval_ms = 100,
        },
    };
    defer {
        allocator.free(config.basic.language);
        allocator.free(config.basic.theme_mode);
        allocator.free(config.basic.wallpaper);
        allocator.free(config.logging.level);
    }

    try crud_manager.saveSettings(config);

    const loaded = try crud_manager.loadSettings(allocator);
    defer {
        allocator.free(loaded.basic.language);
        allocator.free(loaded.basic.theme_mode);
        allocator.free(loaded.basic.wallpaper);
        allocator.free(loaded.logging.level);
    }

    try std.testing.expectEqual(config.basic.timezone, loaded.basic.timezone);
    try std.testing.expectEqualStrings(config.basic.language, loaded.basic.language);
    try std.testing.expectEqual(config.basic.default_mode, loaded.basic.default_mode);
    try std.testing.expectEqualStrings(config.basic.theme_mode, loaded.basic.theme_mode);
    try std.testing.expectEqual(config.clock_defaults.countdown.duration_seconds, loaded.clock_defaults.countdown.duration_seconds);
    try std.testing.expectEqual(config.clock_defaults.countdown.loop, loaded.clock_defaults.countdown.loop);
    try std.testing.expectEqual(config.clock_defaults.countdown.loop_count, loaded.clock_defaults.countdown.loop_count);
    try std.testing.expectEqual(config.clock_defaults.stopwatch.max_seconds, loaded.clock_defaults.stopwatch.max_seconds);
}

test "loadSettings returns defaults when empty" {
    const allocator = std.testing.allocator;
    try createTestDb();
    defer cleanupTestFiles();

    var crud_manager = crud.CrudManager.init(test_allocator, test_db);

    const loaded = try crud_manager.loadSettings(allocator);
    defer {
        allocator.free(loaded.basic.language);
        allocator.free(loaded.basic.theme_mode);
        allocator.free(loaded.basic.wallpaper);
        allocator.free(loaded.logging.level);
    }

    try std.testing.expectEqual(@as(i8, 8), loaded.basic.timezone);
    try std.testing.expectEqualStrings("ZH", loaded.basic.language);
    try std.testing.expectEqual(interface.DefaultMode.countdown, loaded.basic.default_mode);
}

test "saveSettings with null db returns error" {
    const allocator = std.testing.allocator;
    var crud_manager = crud.CrudManager.init(allocator, null);

    const config = interface.SettingsConfig{};
    try std.testing.expectError(crud.CrudError.DatabaseOpenFailed, crud_manager.saveSettings(config));
}

test "loadSettings with null db returns error" {
    const allocator = std.testing.allocator;
    var crud_manager = crud.CrudManager.init(allocator, null);

    try std.testing.expectError(crud.CrudError.DatabaseOpenFailed, crud_manager.loadSettings(allocator));
}

test "loadSettings with stopwatch mode" {
    const allocator = std.testing.allocator;
    try createTestDb();
    defer cleanupTestFiles();

    var crud_manager = crud.CrudManager.init(test_allocator, test_db);

    const config = interface.SettingsConfig{
        .basic = .{
            .timezone = 0,
            .language = try allocator.dupe(u8, "EN"),
            .default_mode = .stopwatch,
            .theme_mode = try allocator.dupe(u8, "dark"),
            .wallpaper = try allocator.dupe(u8, ""),
        },
        .clock_defaults = .{
            .countdown = .{ .duration_seconds = 600, .loop = false, .loop_count = 0, .loop_interval_seconds = 0 },
            .stopwatch = .{ .max_seconds = 7200 },
        },
        .logging = .{
            .level = try allocator.dupe(u8, "DEBUG"),
            .enable_timestamp = false,
            .tick_interval_ms = 50,
        },
    };
    defer {
        allocator.free(config.basic.language);
        allocator.free(config.basic.theme_mode);
        allocator.free(config.basic.wallpaper);
        allocator.free(config.logging.level);
    }

    try crud_manager.saveSettings(config);

    const loaded = try crud_manager.loadSettings(allocator);
    defer {
        allocator.free(loaded.basic.language);
        allocator.free(loaded.basic.theme_mode);
        allocator.free(loaded.basic.wallpaper);
        allocator.free(loaded.logging.level);
    }

    try std.testing.expectEqual(interface.DefaultMode.stopwatch, loaded.basic.default_mode);
    try std.testing.expectEqual(@as(u64, 7200), loaded.clock_defaults.stopwatch.max_seconds);
}

test "saveBackupConfig and loadBackupConfig local roundtrip" {
    const allocator = std.testing.allocator;
    try createTestDb();
    defer cleanupTestFiles();

    var crud_manager = crud.CrudManager.init(test_allocator, test_db);

    const config = interface.BackupConfig{
        .enabled = true,
        .auto_backup = true,
        .auto_backup_interval = 86400,
        .target_type = .local,
        .local_path = try allocator.dupe(u8, "/tmp/backups"),
    };
    defer allocator.free(config.local_path);

    try crud_manager.saveBackupConfig(config, null);

    const loaded = try crud_manager.loadBackupConfig(allocator, null);
    defer {
        allocator.free(loaded.local_path);
        allocator.free(loaded.webdav_url);
        allocator.free(loaded.webdav_username);
        allocator.free(loaded.webdav_password);
        allocator.free(loaded.s3_endpoint);
        allocator.free(loaded.s3_bucket);
        allocator.free(loaded.s3_region);
        allocator.free(loaded.s3_access_key);
        allocator.free(loaded.s3_secret_key);
        allocator.free(loaded.s3_path_prefix);
    }

    try std.testing.expectEqual(config.enabled, loaded.enabled);
    try std.testing.expectEqual(config.auto_backup, loaded.auto_backup);
    try std.testing.expectEqual(config.auto_backup_interval, loaded.auto_backup_interval);
    try std.testing.expectEqual(config.target_type, loaded.target_type);
    try std.testing.expectEqualStrings(config.local_path, loaded.local_path);
}

test "saveBackupConfig with null db returns error" {
    const allocator = std.testing.allocator;
    var crud_manager = crud.CrudManager.init(allocator, null);

    const config = interface.BackupConfig{};
    try std.testing.expectError(crud.CrudError.DatabaseOpenFailed, crud_manager.saveBackupConfig(config, null));
}

test "loadBackupConfig with null db returns error" {
    const allocator = std.testing.allocator;
    var crud_manager = crud.CrudManager.init(allocator, null);

    try std.testing.expectError(crud.CrudError.DatabaseOpenFailed, crud_manager.loadBackupConfig(allocator, null));
}

test "loadBackupConfig returns defaults when empty" {
    const allocator = std.testing.allocator;
    try createTestDb();
    defer cleanupTestFiles();

    var crud_manager = crud.CrudManager.init(test_allocator, test_db);

    const loaded = try crud_manager.loadBackupConfig(allocator, null);
    defer {
        allocator.free(loaded.local_path);
        allocator.free(loaded.webdav_url);
        allocator.free(loaded.webdav_username);
        allocator.free(loaded.webdav_password);
        allocator.free(loaded.s3_endpoint);
        allocator.free(loaded.s3_bucket);
        allocator.free(loaded.s3_region);
        allocator.free(loaded.s3_access_key);
        allocator.free(loaded.s3_secret_key);
        allocator.free(loaded.s3_path_prefix);
    }

    try std.testing.expectEqual(@as(bool, false), loaded.enabled);
    try std.testing.expectEqual(@as(bool, false), loaded.auto_backup);
    try std.testing.expectEqual(interface.BackupTargetType.local, loaded.target_type);
}

test "CrudError error set values" {
    try std.testing.expectEqual(@as(u16, 0x01), @intFromEnum(crud.CrudError.InsertFailed));
    try std.testing.expectEqual(@as(u16, 0x02), @intFromEnum(crud.CrudError.DeleteFailed));
    try std.testing.expectEqual(@as(u16, 0x04), @intFromEnum(crud.CrudError.QueryFailed));
    try std.testing.expectEqual(@as(u16, 0x08), @intFromEnum(crud.CrudError.SettingsNotFound));
    try std.testing.expectEqual(@as(u16, 0x10), @intFromEnum(crud.CrudError.SettingsSaveFailed));
    try std.testing.expectEqual(@as(u16, 0x20), @intFromEnum(crud.CrudError.DatabaseOpenFailed));
    try std.testing.expectEqual(@as(u16, 0x40), @intFromEnum(crud.CrudError.EncryptionFailed));
    try std.testing.expectEqual(@as(u16, 0x80), @intFromEnum(crud.CrudError.DecryptionFailed));
    try std.testing.expectEqual(@as(u16, 0x100), @intFromEnum(crud.CrudError.MasterKeyNotFound));
}

test "SettingsRow struct initialization" {
    const row: crud.SettingsRow = .{
        .id = 1,
        .timezone = 8,
        .language = "ZH",
        .default_mode = "countdown",
        .theme_mode = "dark",
        .duration_seconds = 1500,
        .countdown_loop = true,
        .countdown_loop_count = 4,
        .countdown_loop_interval = 300,
        .stopwatch_max_seconds = 3600,
        .log_level = "INFO",
        .log_enable_timestamp = true,
        .log_tick_interval = 100,
    };

    try std.testing.expectEqual(@as(i64, 1), row.id);
    try std.testing.expectEqual(@as(i8, 8), row.timezone);
    try std.testing.expectEqualStrings("countdown", row.default_mode);
    try std.testing.expectEqual(@as(u64, 1500), row.duration_seconds);
    try std.testing.expectEqual(@as(bool, true), row.countdown_loop);
}

test "saveSettings overwrites previous settings" {
    const allocator = std.testing.allocator;
    try createTestDb();
    defer cleanupTestFiles();

    var crud_manager = crud.CrudManager.init(test_allocator, test_db);

    const config1 = interface.SettingsConfig{
        .basic = .{
            .timezone = 8,
            .language = try allocator.dupe(u8, "ZH"),
            .default_mode = .countdown,
            .theme_mode = try allocator.dupe(u8, "dark"),
            .wallpaper = try allocator.dupe(u8, ""),
        },
    };
    defer {
        allocator.free(config1.basic.language);
        allocator.free(config1.basic.theme_mode);
        allocator.free(config1.basic.wallpaper);
    }

    try crud_manager.saveSettings(config1);

    const config2 = interface.SettingsConfig{
        .basic = .{
            .timezone = 0,
            .language = try allocator.dupe(u8, "EN"),
            .default_mode = .stopwatch,
            .theme_mode = try allocator.dupe(u8, "light"),
            .wallpaper = try allocator.dupe(u8, "bg.png"),
        },
    };
    defer {
        allocator.free(config2.basic.language);
        allocator.free(config2.basic.theme_mode);
        allocator.free(config2.basic.wallpaper);
    }

    try crud_manager.saveSettings(config2);

    const loaded = try crud_manager.loadSettings(allocator);
    defer {
        allocator.free(loaded.basic.language);
        allocator.free(loaded.basic.theme_mode);
        allocator.free(loaded.basic.wallpaper);
        allocator.free(loaded.logging.level);
    }

    try std.testing.expectEqual(@as(i8, 0), loaded.basic.timezone);
    try std.testing.expectEqualStrings("EN", loaded.basic.language);
    try std.testing.expectEqual(interface.DefaultMode.stopwatch, loaded.basic.default_mode);
    try std.testing.expectEqualStrings("light", loaded.basic.theme_mode);
    try std.testing.expectEqualStrings("bg.png", loaded.basic.wallpaper);
}