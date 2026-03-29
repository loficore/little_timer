//! 设置管理模块单元测试
const std = @import("std");
const settings_module = @import("../settings/settings_manager.zig");
const interface = @import("../core/interface.zig");

test "默认设置配置初始化" {
    const default_config = interface.SettingsConfig{};
    try std.testing.expectEqual(default_config.basic.timezone, 8);
    try std.testing.expect(std.mem.eql(u8, default_config.basic.language, "ZH"));
    try std.testing.expectEqual(default_config.basic.default_mode, .countdown);
}

test "倒计时默认配置" {
    const default_config = interface.SettingsConfig{};
    try std.testing.expectEqual(default_config.clock_defaults.countdown.duration_seconds, 25 * 60);
    try std.testing.expect(!default_config.clock_defaults.countdown.loop);
    try std.testing.expectEqual(default_config.clock_defaults.countdown.loop_count, 0);
}

test "日志默认配置" {
    const default_config = interface.SettingsConfig{};
    try std.testing.expect(std.mem.startsWith(u8, default_config.logging.level, "INFO"));
    try std.testing.expect(default_config.logging.enable_timestamp);
}

test "时区范围校验 - 有效范围 -12 到 14" {
    const allocator = std.testing.allocator;
    var manager = try settings_module.SettingsManager.init(allocator, "");
    defer manager.deinit();

    try manager.updateBasic(.{ .timezone = -12, .language = "ZH", .default_mode = .countdown });
    try std.testing.expectEqual(manager.config.basic.timezone, -12);

    try manager.updateBasic(.{ .timezone = 14, .language = "ZH", .default_mode = .countdown });
    try std.testing.expectEqual(manager.config.basic.timezone, 14);
}

test "时区校验 - 超出范围低于 -12" {
    const allocator = std.testing.allocator;
    var manager = try settings_module.SettingsManager.init(allocator, "");
    defer manager.deinit();

    const result = manager.updateBasic(.{ .timezone = -13, .language = "ZH", .default_mode = .countdown });
    try std.testing.expectError(settings_module.ValidationError.InvalidTimezone, result);
}

test "时区校验 - 超出范围高于 14" {
    const allocator = std.testing.allocator;
    var manager = try settings_module.SettingsManager.init(allocator, "");
    defer manager.deinit();

    const result = manager.updateBasic(.{ .timezone = 15, .language = "ZH", .default_mode = .countdown });
    try std.testing.expectError(settings_module.ValidationError.InvalidTimezone, result);
}

test "时区边界值 0" {
    const allocator = std.testing.allocator;
    var manager = try settings_module.SettingsManager.init(allocator, "");
    defer manager.deinit();

    try manager.updateBasic(.{ .timezone = 0, .language = "ZH", .default_mode = .countdown });
    try std.testing.expectEqual(manager.config.basic.timezone, 0);
}

test "更新基本设置 - 时区和语言" {
    const allocator = std.testing.allocator;
    var manager = try settings_module.SettingsManager.init(allocator, "");
    defer manager.deinit();

    try manager.updateBasic(.{ .timezone = -5, .language = "EN", .default_mode = .stopwatch });

    try std.testing.expectEqual(manager.config.basic.timezone, -5);
    try std.testing.expect(std.mem.eql(u8, manager.config.basic.language, "EN"));
    try std.testing.expectEqual(manager.config.basic.default_mode, .stopwatch);
}

test "更新倒计时默认配置" {
    const allocator = std.testing.allocator;
    var manager = try settings_module.SettingsManager.init(allocator, "");
    defer manager.deinit();

    manager.config.clock_defaults.countdown.duration_seconds = 1800;
    manager.config.clock_defaults.countdown.loop = true;

    try std.testing.expectEqual(manager.config.clock_defaults.countdown.duration_seconds, 1800);
    try std.testing.expect(manager.config.clock_defaults.countdown.loop);
}

test "toJsonAlloc 基本功能" {
    const allocator = std.testing.allocator;
    var manager = try settings_module.SettingsManager.init(allocator, "");
    defer manager.deinit();

    const json_str = try manager.toJsonAlloc();
    defer allocator.free(json_str);

    try std.testing.expect(std.mem.containsAtLeast(u8, json_str, 1, "timezone"));
    try std.testing.expect(std.mem.containsAtLeast(u8, json_str, 1, "language"));
    try std.testing.expect(std.mem.containsAtLeast(u8, json_str, 1, "countdown"));
}

test "toJsonAlloc 时区值正确" {
    const allocator = std.testing.allocator;
    var manager = try settings_module.SettingsManager.init(allocator, "");
    defer manager.deinit();

    manager.config.basic.timezone = -5;

    const json_str = try manager.toJsonAlloc();
    defer allocator.free(json_str);

    try std.testing.expect(std.mem.containsAtLeast(u8, json_str, 1, "timezone"));
    try std.testing.expect(std.mem.containsAtLeast(u8, json_str, 1, "-5"));
}

test "toJsonAlloc 默认模式值" {
    const allocator = std.testing.allocator;
    var manager = try settings_module.SettingsManager.init(allocator, "");
    defer manager.deinit();

    manager.config.basic.default_mode = .stopwatch;

    const json_str = try manager.toJsonAlloc();
    defer allocator.free(json_str);

    try std.testing.expect(std.mem.containsAtLeast(u8, json_str, 1, "stopwatch"));
}

test "添加单个预设" {
    const allocator = std.testing.allocator;
    var manager = try settings_module.SettingsManager.init(allocator, "");
    defer manager.deinit();
    defer manager.presets.clear();

    const preset: interface.TimerPreset = .{
        .name = "测试_添加单个预设_番茄钟",
        .mode = .COUNTDOWN_MODE,
        .config = .{ .countdown = .{ .duration_seconds = 1500, .loop = false, .loop_count = 0, .loop_interval_seconds = 0 } },
    };

    try manager.addPreset(preset);

    try std.testing.expectEqual(@as(usize, 1), manager.presets.count());
    const p0 = manager.presets.get(0).?;
    try std.testing.expect(std.mem.eql(u8, p0.name, "测试_添加单个预设_番茄钟"));
}

test "添加多个预设" {
    const allocator = std.testing.allocator;
    var manager = try settings_module.SettingsManager.init(allocator, "");
    defer manager.deinit();
    defer manager.presets.clear();

    const preset1: interface.TimerPreset = .{
        .name = "测试_添加多个预设_番茄钟",
        .mode = .COUNTDOWN_MODE,
        .config = .{ .countdown = .{ .duration_seconds = 1500, .loop = false, .loop_count = 0, .loop_interval_seconds = 0 } },
    };

    const preset2: interface.TimerPreset = .{
        .name = "测试_添加多个预设_短休息",
        .mode = .COUNTDOWN_MODE,
        .config = .{ .countdown = .{ .duration_seconds = 300, .loop = false, .loop_count = 0, .loop_interval_seconds = 0 } },
    };

    try manager.addPreset(preset1);
    try manager.addPreset(preset2);

    try std.testing.expectEqual(@as(usize, 2), manager.presets.count());
    const p1 = manager.presets.get(0).?;
    try std.testing.expect(std.mem.eql(u8, p1.name, "测试_添加多个预设_番茄钟"));
    const p2 = manager.presets.get(1).?;
    try std.testing.expect(std.mem.eql(u8, p2.name, "测试_添加多个预设_短休息"));
}

test "预设名称冲突检测" {
    const allocator = std.testing.allocator;
    var manager = try settings_module.SettingsManager.init(allocator, "");
    defer manager.deinit();
    defer manager.presets.clear();

    const preset: interface.TimerPreset = .{
        .name = "测试_预设名称冲突检测_唯一名称",
        .mode = .COUNTDOWN_MODE,
        .config = .{ .countdown = .{ .duration_seconds = 1500, .loop = false, .loop_count = 0, .loop_interval_seconds = 0 } },
    };

    try manager.addPreset(preset);

    const duplicate_preset: interface.TimerPreset = .{
        .name = "测试_预设名称冲突检测_唯一名称",
        .mode = .COUNTDOWN_MODE,
        .config = .{ .countdown = .{ .duration_seconds = 600, .loop = false, .loop_count = 0, .loop_interval_seconds = 0 } },
    };

    const result = manager.addPreset(duplicate_preset);
    try std.testing.expectError(settings_module.PresetsError.PresetNameConflict, result);
}

test "预设名称为空检测" {
    const allocator = std.testing.allocator;
    var manager = try settings_module.SettingsManager.init(allocator, "");
    defer manager.deinit();

    const empty_preset: interface.TimerPreset = .{
        .name = "",
        .mode = .COUNTDOWN_MODE,
        .config = .{ .countdown = .{ .duration_seconds = 1500, .loop = false, .loop_count = 0, .loop_interval_seconds = 0 } },
    };

    const result = manager.addPreset(empty_preset);
    try std.testing.expectError(settings_module.PresetsError.PresetNameEmpty, result);
}

test "buildClockConfig 倒计时模式" {
    const allocator = std.testing.allocator;
    var manager = try settings_module.SettingsManager.init(allocator, "");
    defer manager.deinit();

    manager.config.basic.default_mode = .countdown;
    manager.config.clock_defaults.countdown.duration_seconds = 900;

    const config = manager.buildClockConfig();

    try std.testing.expectEqual(config.countdown.duration_seconds, 900);
}

test "buildClockConfig 正计时模式" {
    const allocator = std.testing.allocator;
    var manager = try settings_module.SettingsManager.init(allocator, "");
    defer manager.deinit();

    manager.config.basic.default_mode = .stopwatch;
    manager.config.clock_defaults.stopwatch.max_seconds = 3600;

    const config = manager.buildClockConfig();

    try std.testing.expectEqual(config.stopwatch.max_seconds, 3600);
}

test "更新基本设置后设置脏标记" {
    const allocator = std.testing.allocator;
    var manager = try settings_module.SettingsManager.init(allocator, "");
    defer manager.deinit();

    manager.is_dirty = false;

    try manager.updateBasic(.{ .timezone = 10, .language = "EN", .default_mode = .stopwatch });

    try std.testing.expect(manager.is_dirty);
}

test "极端时区值 -12 和 14" {
    const allocator = std.testing.allocator;
    var manager = try settings_module.SettingsManager.init(allocator, "");
    defer manager.deinit();

    try manager.updateBasic(.{ .timezone = -12, .language = "ZH", .default_mode = .countdown });
    try std.testing.expectEqual(manager.config.basic.timezone, -12);

    try manager.updateBasic(.{ .timezone = 14, .language = "ZH", .default_mode = .countdown });
    try std.testing.expectEqual(manager.config.basic.timezone, 14);
}

test "零秒倒计时配置" {
    const allocator = std.testing.allocator;
    var manager = try settings_module.SettingsManager.init(allocator, "");
    defer manager.deinit();

    manager.config.clock_defaults.countdown.duration_seconds = 0;
    const config = manager.buildClockConfig();

    try std.testing.expectEqual(config.countdown.duration_seconds, 0);
}

test "resetToDefaults() 重置配置" {
    const allocator = std.testing.allocator;
    var manager = try settings_module.SettingsManager.init(allocator, "");
    defer manager.deinit();
    defer manager.presets.clear();

    manager.config.basic.timezone = -10;
    manager.config.clock_defaults.countdown.duration_seconds = 100;

    const preset: interface.TimerPreset = .{
        .name = "测试_resetToDefaults_预设",
        .mode = .COUNTDOWN_MODE,
        .config = .{ .countdown = .{ .duration_seconds = 500, .loop = false, .loop_count = 0, .loop_interval_seconds = 0 } },
    };
    try manager.addPreset(preset);

    try manager.resetToDefaults();

    try std.testing.expectEqual(manager.config.basic.timezone, 8);
    try std.testing.expect(std.mem.eql(u8, manager.config.basic.language, "ZH"));
    try std.testing.expectEqual(@as(usize, 0), manager.presets.count());
    try std.testing.expect(manager.is_dirty);
}

test "handleSettingsEvent() 通过 get_settings 获取配置" {
    const allocator = std.testing.allocator;
    var manager = try settings_module.SettingsManager.init(allocator, "");
    defer manager.deinit();

    manager.config.basic.timezone = 5;

    var buffer: [1]u8 = .{0};
    const event: interface.SettingsEvent = .{ .get_settings = buffer[0..0 :0] };

    try manager.handleSettingsEvent(event);
}

test "toJsonAlloc() 动态分配 JSON" {
    const allocator = std.testing.allocator;
    var manager = try settings_module.SettingsManager.init(allocator, "");
    defer manager.deinit();

    manager.config.basic.timezone = 7;
    manager.config.clock_defaults.countdown.duration_seconds = 2000;

    const json_str = try manager.toJsonAlloc();
    defer allocator.free(json_str);

    try std.testing.expect(std.mem.containsAtLeast(u8, json_str, 1, "timezone"));
    try std.testing.expect(std.mem.containsAtLeast(u8, json_str, 1, ":7"));
    try std.testing.expect(std.mem.containsAtLeast(u8, json_str, 1, "2000"));
}
