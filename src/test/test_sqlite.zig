const std = @import("std");
const testing = std.testing;
const interface = @import("../core/interface.zig");
const storage_sqlite = @import("../storage/storage_sqlite.zig");
const fixtures = @import("fixtures/test_fixtures.zig");

fn createTempDbPath(allocator: std.mem.Allocator, tmp: *std.testing.TmpDir) ![:0]const u8 {
    const path = try std.fmt.allocPrint(allocator, ".zig-cache/tmp/{s}/test_{d}.db", .{
        tmp.sub_path,
        std.time.timestamp(),
    });
    defer allocator.free(path);
    // Make a null-terminated copy
    const path_null = try allocator.allocSentinel(u8, path.len, 0);
    @memcpy(path_null[0..path.len], path);
    return path_null;
}

test "SQLite 管理器初始化（内存模式）" {
    const allocator = testing.allocator;

    const db_path = ":memory:";
    const backup_dir = "/tmp";

    var manager = try storage_sqlite.SqliteManager.init(allocator, db_path, backup_dir);
    defer manager.deinit();

    try testing.expect(!manager.is_initialized);
    try testing.expect(manager.db == null);
}

test "SQLite 打开和关闭（内存模式）" {
    const allocator = testing.allocator;

    const db_path = ":memory:";
    const backup_dir = "/tmp";

    var manager = try storage_sqlite.SqliteManager.init(allocator, db_path, backup_dir);
    defer manager.deinit();

    try manager.open();
    try testing.expect(manager.is_initialized);
    try testing.expect(manager.db != null);

    manager.close();
    try testing.expect(!manager.is_initialized);
    try testing.expect(manager.db == null);
}

test "预设 CRUD 完整流程（内存模式）" {
    const allocator = testing.allocator;

    const db_path = ":memory:";
    const backup_dir = "/tmp";

    var manager = try storage_sqlite.SqliteManager.init(allocator, db_path, backup_dir);
    defer manager.deinit();
    try manager.open();

    const preset = fixtures.TimerPresetFixture{
        .name = "测试倒计时",
        .mode = "countdown",
        .config = .{
            .duration_seconds = 300,
            .loop = false,
            .loop_count = 0,
            .loop_interval_seconds = 60,
        },
    };
    const config_json = try fixtures.generatePresetJson(allocator, preset);
    defer allocator.free(config_json);

    const timer_preset = try fixtures.fixtureToTimerPreset(allocator, preset);
    defer allocator.free(timer_preset.name);
    try manager.insertPreset(timer_preset, config_json);

    var presets = try manager.queryAllPresets(allocator);
    defer {
        for (presets.items) |row| {
            allocator.free(row.name);
            allocator.free(row.config_json);
        }
        presets.deinit(allocator);
    }

    try testing.expect(presets.items.len == 1);
    try testing.expectEqualStrings("测试倒计时", presets.items[0].name);

    const preset2 = fixtures.TimerPresetFixture{
        .name = "番茄工作法",
        .mode = "countdown",
        .config = .{
            .duration_seconds = 1500,
            .loop = true,
            .loop_count = 4,
            .loop_interval_seconds = 300,
        },
    };
    const config_json2 = try fixtures.generatePresetJson(allocator, preset2);
    defer allocator.free(config_json2);

    const timer_preset2 = try fixtures.fixtureToTimerPreset(allocator, preset2);
    defer allocator.free(timer_preset2.name);
    try manager.insertPreset(timer_preset2, config_json2);

    var presets_after_add = try manager.queryAllPresets(allocator);
    defer {
        for (presets_after_add.items) |row| {
            allocator.free(row.name);
            allocator.free(row.config_json);
        }
        presets_after_add.deinit(allocator);
    }
    try testing.expect(presets_after_add.items.len == 2);

    try manager.deletePresetByName("测试倒计时");

    var presets_after_delete = try manager.queryAllPresets(allocator);
    defer {
        for (presets_after_delete.items) |row| {
            allocator.free(row.name);
            allocator.free(row.config_json);
        }
        presets_after_delete.deinit(allocator);
    }
    try testing.expect(presets_after_delete.items.len == 1);
    try testing.expectEqualStrings("番茄工作法", presets_after_delete.items[0].name);
}

test "预设唯一性约束（内存模式）" {
    const allocator = testing.allocator;

    const db_path = ":memory:";
    const backup_dir = "/tmp";

    var manager = try storage_sqlite.SqliteManager.init(allocator, db_path, backup_dir);
    defer manager.deinit();
    try manager.open();

    const preset1 = fixtures.TimerPresetFixture{
        .name = "唯一名称",
        .mode = "countdown",
        .config = .{ .duration_seconds = 60 },
    };
    const config_json1 = try fixtures.generatePresetJson(allocator, preset1);
    defer allocator.free(config_json1);

    const timer_preset1 = try fixtures.fixtureToTimerPreset(allocator, preset1);
    defer allocator.free(timer_preset1.name);
    try manager.insertPreset(timer_preset1, config_json1);

    const preset2 = fixtures.TimerPresetFixture{
        .name = "唯一名称",
        .mode = "stopwatch",
        .config = .{ .duration_seconds = 120 },
    };
    const config_json2 = try fixtures.generatePresetJson(allocator, preset2);
    defer allocator.free(config_json2);

    const timer_preset2 = try fixtures.fixtureToTimerPreset(allocator, preset2);
    defer allocator.free(timer_preset2.name);
    try testing.expectError(storage_sqlite.SqliteError.InsertFailed, manager.insertPreset(timer_preset2, config_json2));
}

test "清空所有预设（内存模式）" {
    const allocator = testing.allocator;

    const db_path = ":memory:";
    const backup_dir = "/tmp";

    var manager = try storage_sqlite.SqliteManager.init(allocator, db_path, backup_dir);
    defer manager.deinit();
    try manager.open();

    for (0..5) |i| {
        const preset = fixtures.TimerPresetFixture{
            .name = try std.fmt.allocPrint(allocator, "预设{d}", .{i}),
            .mode = "countdown",
            .config = .{ .duration_seconds = 60 * (@as(u64, i) + 1) },
        };
        const config_json = try fixtures.generatePresetJson(allocator, preset);
        defer allocator.free(config_json);

        const timer_preset = try fixtures.fixtureToTimerPreset(allocator, preset);
        defer allocator.free(timer_preset.name);
        try manager.insertPreset(timer_preset, config_json);
        allocator.free(preset.name);
    }

    var presets_before = try manager.queryAllPresets(allocator);
    defer {
        for (presets_before.items) |row| {
            allocator.free(row.name);
            allocator.free(row.config_json);
        }
        presets_before.deinit(allocator);
    }
    try testing.expect(presets_before.items.len == 5);

    try manager.clearAllPresets();

    var presets_after = try manager.queryAllPresets(allocator);
    defer presets_after.deinit(allocator);
    try testing.expect(presets_after.items.len == 0);
}

test "设置保存和加载（内存模式）" {
    const allocator = testing.allocator;

    const db_path = ":memory:";
    const backup_dir = "/tmp";

    var manager = try storage_sqlite.SqliteManager.init(allocator, db_path, backup_dir);
    defer manager.deinit();
    try manager.open();

    const settings = interface.SettingsConfig{
        .basic = .{
            .timezone = 8,
            .language = try allocator.dupe(u8, "ZH"),
            .default_mode = .countdown,
            .theme_mode = try allocator.dupe(u8, "dark"),
        },
        .clock_defaults = .{
            .countdown = .{
                .duration_seconds = 300,
                .loop = true,
                .loop_count = 3,
                .loop_interval_seconds = 120,
            },
            .stopwatch = .{
                .max_seconds = 3600,
            },
            .world_clock = .{
                .timezone = 8,
            },
        },
        .logging = .{
            .level = try allocator.dupe(u8, "debug"),
            .enable_timestamp = true,
            .tick_interval_ms = 100,
        },
    };
    defer {
        allocator.free(settings.basic.language);
        allocator.free(settings.basic.theme_mode);
        allocator.free(settings.logging.level);
    }

    try manager.saveSettings(settings);

    const loaded = try manager.loadSettings(allocator);
    defer {
        allocator.free(loaded.basic.language);
        allocator.free(loaded.basic.theme_mode);
        allocator.free(loaded.logging.level);
    }

    try testing.expectEqual(@as(i8, 8), loaded.basic.timezone);
    try testing.expectEqualStrings("ZH", loaded.basic.language);
    try testing.expect(loaded.clock_defaults.countdown.loop);
    try testing.expectEqual(@as(u32, 3), loaded.clock_defaults.countdown.loop_count);
}

test "预设存在性检查（内存模式）" {
    const allocator = testing.allocator;

    const db_path = ":memory:";
    const backup_dir = "/tmp";

    var manager = try storage_sqlite.SqliteManager.init(allocator, db_path, backup_dir);
    defer manager.deinit();
    try manager.open();

    const exists1 = try manager.presetExists("不存在");
    try testing.expect(!exists1);

    const preset = fixtures.TimerPresetFixture{
        .name = "存在性测试",
        .mode = "countdown",
        .config = .{ .duration_seconds = 60 },
    };
    const config_json = try fixtures.generatePresetJson(allocator, preset);
    defer allocator.free(config_json);

    const timer_preset = try fixtures.fixtureToTimerPreset(allocator, preset);
    defer allocator.free(timer_preset.name);
    try manager.insertPreset(timer_preset, config_json);

    const exists2 = try manager.presetExists("存在性测试");
    try testing.expect(exists2);

    const exists3 = try manager.presetExists("不存在");
    try testing.expect(!exists3);
}

test "预设统计信息（内存模式）" {
    const allocator = testing.allocator;

    const db_path = ":memory:";
    const backup_dir = "/tmp";

    var manager = try storage_sqlite.SqliteManager.init(allocator, db_path, backup_dir);
    defer manager.deinit();
    try manager.open();

    const stats0 = try manager.getPresetStats();
    try testing.expectEqual(@as(u32, 0), stats0.total_presets);

    const preset1 = fixtures.TimerPresetFixture{
        .name = "倒计时1",
        .mode = "countdown",
        .config = .{ .duration_seconds = 60 },
    };
    const config_json1 = try fixtures.generatePresetJson(allocator, preset1);
    defer allocator.free(config_json1);
    const timer_preset1 = try fixtures.fixtureToTimerPreset(allocator, preset1);
    defer allocator.free(timer_preset1.name);
    try manager.insertPreset(timer_preset1, config_json1);

    const preset2 = fixtures.TimerPresetFixture{
        .name = "正计时1",
        .mode = "stopwatch",
        .config = .{ .duration_seconds = 0, .max_seconds = 3600 },
    };
    const config_json2 = try fixtures.generatePresetJson(allocator, preset2);
    defer allocator.free(config_json2);
    const timer_preset2 = try fixtures.fixtureToTimerPreset(allocator, preset2);
    defer allocator.free(timer_preset2.name);
    try manager.insertPreset(timer_preset2, config_json2);

    const preset3 = fixtures.TimerPresetFixture{
        .name = "世界时钟1",
        .mode = "world_clock",
        .config = .{ .timezone = 8 },
    };
    const config_json3 = try fixtures.generatePresetJson(allocator, preset3);
    defer allocator.free(config_json3);
    const timer_preset3 = try fixtures.fixtureToTimerPreset(allocator, preset3);
    defer allocator.free(timer_preset3.name);
    try manager.insertPreset(timer_preset3, config_json3);

    const stats = try manager.getPresetStats();
    try testing.expectEqual(@as(u32, 3), stats.total_presets);
    try testing.expectEqual(@as(u32, 1), stats.countdown_presets);
    try testing.expectEqual(@as(u32, 1), stats.stopwatch_presets);
    try testing.expectEqual(@as(u32, 1), stats.world_clock_presets);
}

test "数据库健康检查（内存模式）" {
    const allocator = testing.allocator;

    const db_path = ":memory:";
    const backup_dir = "/tmp";

    var manager = try storage_sqlite.SqliteManager.init(allocator, db_path, backup_dir);
    defer manager.deinit();
    try manager.open();

    try manager.performHealthCheck();

    const is_healthy = try manager.isHealthy();
    try testing.expect(is_healthy);

    const info = try manager.getHealthInfo();
    defer {
        allocator.free(info.status);
        allocator.free(info.last_check);
    }
    try testing.expectEqualStrings("healthy", info.status);
}

test "多模式预设混合操作（内存模式）" {
    const allocator = testing.allocator;

    const db_path = ":memory:";
    const backup_dir = "/tmp";

    var manager = try storage_sqlite.SqliteManager.init(allocator, db_path, backup_dir);
    defer manager.deinit();
    try manager.open();

    const countdown = fixtures.TimerPresetFixture{
        .name = "倒计时模式",
        .mode = "countdown",
        .config = .{ .duration_seconds = 300, .loop = true, .loop_count = 2 },
    };
    const countdown_json = try fixtures.generatePresetJson(allocator, countdown);
    defer allocator.free(countdown_json);
    const countdown_preset = try fixtures.fixtureToTimerPreset(allocator, countdown);
    defer allocator.free(countdown_preset.name);
    try manager.insertPreset(countdown_preset, countdown_json);

    const stopwatch = fixtures.TimerPresetFixture{
        .name = "正计时模式",
        .mode = "stopwatch",
        .config = .{ .max_seconds = 7200 },
    };
    const stopwatch_json = try fixtures.generatePresetJson(allocator, stopwatch);
    defer allocator.free(stopwatch_json);
    const stopwatch_preset = try fixtures.fixtureToTimerPreset(allocator, stopwatch);
    defer allocator.free(stopwatch_preset.name);
    try manager.insertPreset(stopwatch_preset, stopwatch_json);

    const world_clock = fixtures.TimerPresetFixture{
        .name = "世界时钟模式",
        .mode = "world_clock",
        .config = .{ .timezone = 9 },
    };
    const world_clock_json = try fixtures.generatePresetJson(allocator, world_clock);
    defer allocator.free(world_clock_json);
    const world_clock_preset = try fixtures.fixtureToTimerPreset(allocator, world_clock);
    defer allocator.free(world_clock_preset.name);
    try manager.insertPreset(world_clock_preset, world_clock_json);

    var all_presets = try manager.queryAllPresets(allocator);
    defer {
        for (all_presets.items) |row| {
            allocator.free(row.name);
            allocator.free(row.config_json);
        }
        all_presets.deinit(allocator);
    }
    try testing.expectEqual(@as(usize, 3), all_presets.items.len);

    const stats = try manager.getPresetStats();
    try testing.expectEqual(@as(u32, 1), stats.countdown_presets);
    try testing.expectEqual(@as(u32, 1), stats.stopwatch_presets);
    try testing.expectEqual(@as(u32, 1), stats.world_clock_presets);
}

test "预设 CRUD（临时文件模式）" {
    const allocator = testing.allocator;

    var tmp = std.testing.tmpDir(.{});
    defer tmp.cleanup();

    const db_path = try createTempDbPath(allocator, &tmp);
    defer allocator.free(db_path);

    const backup_dir = ".zig-cache/tmp";

    var manager = try storage_sqlite.SqliteManager.init(allocator, db_path, backup_dir);
    defer manager.deinit();
    try manager.open();

    const preset = fixtures.TimerPresetFixture{
        .name = "文件模式测试",
        .mode = "countdown",
        .config = .{ .duration_seconds = 600 },
    };
    const config_json = try fixtures.generatePresetJson(allocator, preset);
    defer allocator.free(config_json);

    const timer_preset = try fixtures.fixtureToTimerPreset(allocator, preset);
    defer allocator.free(timer_preset.name);
    try manager.insertPreset(timer_preset, config_json);

    var presets = try manager.queryAllPresets(allocator);
    defer {
        for (presets.items) |row| {
            allocator.free(row.name);
            allocator.free(row.config_json);
        }
        presets.deinit(allocator);
    }
    try testing.expectEqual(@as(usize, 1), presets.items.len);
}

test "数据库健康检查（临时文件模式）" {
    const allocator = testing.allocator;

    var tmp = std.testing.tmpDir(.{});
    defer tmp.cleanup();

    const db_path = try createTempDbPath(allocator, &tmp);
    defer allocator.free(db_path);

    const backup_dir = ".zig-cache/tmp";

    var manager = try storage_sqlite.SqliteManager.init(allocator, db_path, backup_dir);
    defer manager.deinit();
    try manager.open();

    try manager.performHealthCheck();

    const is_healthy = try manager.isHealthy();
    try testing.expect(is_healthy);
}

test "重新打开已存在的数据库" {
    const allocator = testing.allocator;

    var tmp = std.testing.tmpDir(.{});
    defer tmp.cleanup();

    const db_path = try createTempDbPath(allocator, &tmp);
    defer allocator.free(db_path);

    const backup_dir = ".zig-cache/tmp";

    {
        var manager = try storage_sqlite.SqliteManager.init(allocator, db_path, backup_dir);
        defer manager.deinit();
        try manager.open();

        const preset = fixtures.TimerPresetFixture{
            .name = "持久化测试",
            .mode = "countdown",
            .config = .{ .duration_seconds = 120 },
        };
        const config_json = try fixtures.generatePresetJson(allocator, preset);
        defer allocator.free(config_json);

        const timer_preset = try fixtures.fixtureToTimerPreset(allocator, preset);
        defer allocator.free(timer_preset.name);
        try manager.insertPreset(timer_preset, config_json);
    }

    {
        var manager = try storage_sqlite.SqliteManager.init(allocator, db_path, backup_dir);
        defer manager.deinit();
        try manager.open();

        var presets = try manager.queryAllPresets(allocator);
        defer {
            for (presets.items) |row| {
                allocator.free(row.name);
                allocator.free(row.config_json);
            }
            presets.deinit(allocator);
        }

        try testing.expectEqual(@as(usize, 1), presets.items.len);
        try testing.expectEqualStrings("持久化测试", presets.items[0].name);
    }
}

test "设置跨会话持久化" {
    const allocator = testing.allocator;

    var tmp = std.testing.tmpDir(.{});
    defer tmp.cleanup();

    const db_path = try createTempDbPath(allocator, &tmp);
    defer allocator.free(db_path);

    const backup_dir = ".zig-cache/tmp";

    {
        var manager = try storage_sqlite.SqliteManager.init(allocator, db_path, backup_dir);
        defer manager.deinit();
        try manager.open();

        const settings = interface.SettingsConfig{
            .basic = .{
                .timezone = 9,
                .language = try allocator.dupe(u8, "EN"),
                .default_mode = .stopwatch,
                .theme_mode = try allocator.dupe(u8, "light"),
            },
            .clock_defaults = .{
                .countdown = .{
                    .duration_seconds = 180,
                    .loop = false,
                    .loop_count = 0,
                    .loop_interval_seconds = 0,
                },
                .stopwatch = .{
                    .max_seconds = 1800,
                },
                .world_clock = .{
                    .timezone = 9,
                },
            },
            .logging = .{
                .level = try allocator.dupe(u8, "info"),
                .enable_timestamp = false,
                .tick_interval_ms = 200,
            },
        };
        defer {
            allocator.free(settings.basic.language);
            allocator.free(settings.basic.theme_mode);
            allocator.free(settings.logging.level);
        }

        try manager.saveSettings(settings);
    }

    {
        var manager = try storage_sqlite.SqliteManager.init(allocator, db_path, backup_dir);
        defer manager.deinit();
        try manager.open();

        const loaded = try manager.loadSettings(allocator);
        defer {
            allocator.free(loaded.basic.language);
            allocator.free(loaded.basic.theme_mode);
            allocator.free(loaded.logging.level);
        }

        try testing.expectEqual(@as(i8, 9), loaded.basic.timezone);
        try testing.expectEqualStrings("EN", loaded.basic.language);
        try testing.expectEqual(.stopwatch, loaded.basic.default_mode);
        try testing.expectEqual(@as(u64, 1800), loaded.clock_defaults.stopwatch.max_seconds);
    }
}

test "PRAGMA integrity_check 在内存模式下可用" {
    const allocator = testing.allocator;

    const db_path = ":memory:";
    const backup_dir = "/tmp";

    var manager = try storage_sqlite.SqliteManager.init(allocator, db_path, backup_dir);
    defer manager.deinit();
    try manager.open();

    const health_info = try manager.performDeepHealthCheck();
    defer {
        allocator.free(health_info.status);
        allocator.free(health_info.last_check);
    }

    try testing.expectEqualStrings("healthy", health_info.status);
    try testing.expect(health_info.record_count >= 0);
}
