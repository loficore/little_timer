//! 设置管理模块单元测试
const std = @import("std");
const settings_module = @import("../settings.zig");
const interface = @import("../interface.zig");

// ============ 设置配置初始化测试 ============

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

// ============ 时区校验测试 ============

test "时区范围校验 - 有效范围 -12 到 14" {
    const allocator = std.testing.allocator;
    var manager = try settings_module.SettingsManager.init(allocator, "");
    defer manager.deinit();

    // 边界值
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
    try std.testing.expectError(settings_module.SettingsError.InvalidTimezone, result);
}

test "时区校验 - 超出范围高于 14" {
    const allocator = std.testing.allocator;
    var manager = try settings_module.SettingsManager.init(allocator, "");
    defer manager.deinit();

    const result = manager.updateBasic(.{ .timezone = 15, .language = "ZH", .default_mode = .countdown });
    try std.testing.expectError(settings_module.SettingsError.InvalidTimezone, result);
}

test "时区边界值 0" {
    const allocator = std.testing.allocator;
    var manager = try settings_module.SettingsManager.init(allocator, "");
    defer manager.deinit();

    try manager.updateBasic(.{ .timezone = 0, .language = "ZH", .default_mode = .countdown });
    try std.testing.expectEqual(manager.config.basic.timezone, 0);
}

// ============ 基本设置更新测试 ============

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

// ============ JSON 转换测试 ============

test "toJsonAlloc 基本功能" {
    const allocator = std.testing.allocator;
    var manager = try settings_module.SettingsManager.init(allocator, "");
    defer manager.deinit();

    const json_str = try manager.toJsonAlloc();
    defer allocator.free(json_str);

    // 验证 JSON 包含期望的字段
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

    // 验证时区值在 JSON 中
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

test "jsonToSettings 基本解析" {
    const allocator = std.testing.allocator;
    var manager = try settings_module.SettingsManager.init(allocator, "");
    defer manager.deinit();

    const json_input =
        \\{"basic":{"timezone":10,"language":"EN","default_mode":"stopwatch","theme_mode":"dark"},"clock_defaults":{"countdown":{"duration_seconds":1200,"loop":true,"loop_count":3,"loop_interval_seconds":30},"stopwatch":{"max_seconds":7200}},"logging":{"level":"DEBUG","enable_timestamp":true,"tick_interval_ms":1000}}
    ;

    try manager.jsonToSettings(json_input);

    try std.testing.expectEqual(manager.config.basic.timezone, 10);
    try std.testing.expect(std.mem.eql(u8, manager.config.basic.language, "EN"));
    try std.testing.expectEqual(manager.config.basic.default_mode, .stopwatch);
}

test "jsonToSettings 倒计时配置" {
    const allocator = std.testing.allocator;
    var manager = try settings_module.SettingsManager.init(allocator, "");
    defer manager.deinit();

    const json_input =
        \\{"basic":{"timezone":8,"language":"ZH","default_mode":"countdown","theme_mode":"dark"},"clock_defaults":{"countdown":{"duration_seconds":1500,"loop":true,"loop_count":5,"loop_interval_seconds":60},"stopwatch":{"max_seconds":86400}},"logging":{"level":"INFO","enable_timestamp":true,"tick_interval_ms":1000}}
    ;

    try manager.jsonToSettings(json_input);

    try std.testing.expectEqual(manager.config.clock_defaults.countdown.duration_seconds, 1500);
    try std.testing.expect(manager.config.clock_defaults.countdown.loop);
    try std.testing.expectEqual(manager.config.clock_defaults.countdown.loop_count, 5);
    try std.testing.expectEqual(manager.config.clock_defaults.countdown.loop_interval_seconds, 60);
}

test "jsonToSettings 日志配置" {
    const allocator = std.testing.allocator;
    var manager = try settings_module.SettingsManager.init(allocator, "");
    defer manager.deinit();

    const json_input =
        \\{"basic":{"timezone":8,"language":"ZH","default_mode":"countdown","theme_mode":"dark"},"clock_defaults":{"countdown":{"duration_seconds":1500,"loop":false,"loop_count":0,"loop_interval_seconds":0},"stopwatch":{"max_seconds":86400}},"logging":{"level":"WARN","enable_timestamp":false,"tick_interval_ms":1000}}
    ;

    try manager.jsonToSettings(json_input);

    try std.testing.expect(std.mem.eql(u8, manager.config.logging.level, "WARN"));
    try std.testing.expect(!manager.config.logging.enable_timestamp);
}

test "jsonToSettings 无效 JSON 处理" {
    const allocator = std.testing.allocator;
    var manager = try settings_module.SettingsManager.init(allocator, "");
    defer manager.deinit();

    const invalid_json = "{invalid json}";

    // 无效 JSON 会导致解析失败
    // 我们只是确认该调用会产生某种错误
    manager.jsonToSettings(invalid_json) catch {
        // 期望返回错误
        return;
    };

    // 如果没有返回错误，则测试失败
    try std.testing.expect(false);
}

test "jsonToSettings 时区边界值保留" {
    const allocator = std.testing.allocator;
    var manager = try settings_module.SettingsManager.init(allocator, "");
    defer manager.deinit();

    manager.config.basic.timezone = 8; // 初始值

    const json_input =
        \\{"basic":{"timezone":14,"language":"ZH","default_mode":"countdown","theme_mode":"dark"},"clock_defaults":{"countdown":{"duration_seconds":1500,"loop":false,"loop_count":0,"loop_interval_seconds":0},"stopwatch":{"max_seconds":86400}},"logging":{"level":"INFO","enable_timestamp":true,"tick_interval_ms":1000}}
    ;

    try manager.jsonToSettings(json_input);

    try std.testing.expectEqual(manager.config.basic.timezone, 14);
}

// ============ 预设管理测试 ============

test "添加单个预设" {
    const allocator = std.testing.allocator;
    var manager = try settings_module.SettingsManager.init(allocator, "");
    defer manager.deinit();

    const name = try allocator.dupe(u8, "番茄钟");
    const preset: interface.TimerPreset = .{
        .name = name,
        .mode = .COUNTDOWN_MODE,
        .config = .{ .countdown = .{ .duration_seconds = 1500, .loop = false, .loop_count = 0, .loop_interval_seconds = 0 } },
    };

    try manager.addPreset(preset);

    try std.testing.expectEqual(manager.preset_count, 1);
    try std.testing.expect(std.mem.eql(u8, manager.timer_presets[0].name, "番茄钟"));
}

test "添加多个预设" {
    const allocator = std.testing.allocator;
    var manager = try settings_module.SettingsManager.init(allocator, "");
    defer manager.deinit();

    const name1 = try allocator.dupe(u8, "番茄钟");
    const preset1: interface.TimerPreset = .{
        .name = name1,
        .mode = .COUNTDOWN_MODE,
        .config = .{ .countdown = .{ .duration_seconds = 1500, .loop = false, .loop_count = 0, .loop_interval_seconds = 0 } },
    };

    const name2 = try allocator.dupe(u8, "短休息");
    const preset2: interface.TimerPreset = .{
        .name = name2,
        .mode = .COUNTDOWN_MODE,
        .config = .{ .countdown = .{ .duration_seconds = 300, .loop = false, .loop_count = 0, .loop_interval_seconds = 0 } },
    };

    try manager.addPreset(preset1);
    try manager.addPreset(preset2);

    try std.testing.expectEqual(manager.preset_count, 2);
    try std.testing.expect(std.mem.eql(u8, manager.timer_presets[0].name, "番茄钟"));
    try std.testing.expect(std.mem.eql(u8, manager.timer_presets[1].name, "短休息"));
}

test "预设名称冲突检测" {
    const allocator = std.testing.allocator;
    var manager = try settings_module.SettingsManager.init(allocator, "");
    defer manager.deinit();

    const name1 = try allocator.dupe(u8, "番茄钟");
    const preset: interface.TimerPreset = .{
        .name = name1,
        .mode = .COUNTDOWN_MODE,
        .config = .{ .countdown = .{ .duration_seconds = 1500, .loop = false, .loop_count = 0, .loop_interval_seconds = 0 } },
    };

    try manager.addPreset(preset);

    const name2 = try allocator.dupe(u8, "番茄钟");
    const duplicate_preset: interface.TimerPreset = .{
        .name = name2,
        .mode = .COUNTDOWN_MODE,
        .config = .{ .countdown = .{ .duration_seconds = 600, .loop = false, .loop_count = 0, .loop_interval_seconds = 0 } },
    };

    const result = manager.addPreset(duplicate_preset);
    try std.testing.expectError(settings_module.SettingsError.PresetNameConflict, result);

    // 冲突时名称应该被释放
    allocator.free(name2);
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
    try std.testing.expectError(settings_module.SettingsError.PresetNameEmpty, result);
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

test "buildClockConfig 世界时钟模式" {
    const allocator = std.testing.allocator;
    var manager = try settings_module.SettingsManager.init(allocator, "");
    defer manager.deinit();

    manager.config.basic.default_mode = .world_clock;
    manager.config.basic.timezone = -8;

    const config = manager.buildClockConfig();

    try std.testing.expectEqual(config.world_clock.timezone, -8);
}

// ============ 脏标记测试 ============

test "更新基本设置后设置脏标记" {
    const allocator = std.testing.allocator;
    var manager = try settings_module.SettingsManager.init(allocator, "");
    defer manager.deinit();

    manager.is_dirty = false;

    try manager.updateBasic(.{ .timezone = 10, .language = "EN", .default_mode = .stopwatch });

    try std.testing.expect(manager.is_dirty);
}

test "JSON 解析后设置脏标记" {
    const allocator = std.testing.allocator;
    var manager = try settings_module.SettingsManager.init(allocator, "");
    defer manager.deinit();

    manager.is_dirty = false;

    const json_input =
        \\{"basic":{"timezone":5,"language":"FR","default_mode":"world_clock","theme_mode":"dark"},"clock_defaults":{"countdown":{"duration_seconds":1500,"loop":false,"loop_count":0,"loop_interval_seconds":0},"stopwatch":{"max_seconds":86400}},"logging":{"level":"INFO","enable_timestamp":true,"tick_interval_ms":1000}}
    ;

    try manager.jsonToSettings(json_input);

    try std.testing.expect(manager.is_dirty);
}

// ============ 边界条件测试 ============

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

// ============ 文件 I/O 和持久化测试 ============

test "save() 和 load() 基本功能" {
    var tmp_dir = try std.fs.cwd().makeOpenPath("test_tmp", .{});
    defer tmp_dir.close();

    const allocator = std.testing.allocator;
    const test_file_path = "test_tmp/test_settings.toml";

    // 清理旧文件
    std.fs.cwd().deleteFile(test_file_path) catch {};

    var manager = try settings_module.SettingsManager.init(allocator, test_file_path);
    defer manager.deinit();

    // 修改配置
    manager.config.basic.timezone = -5;
    manager.config.basic.language = "EN";
    manager.config.basic.default_mode = .stopwatch;

    // 保存到文件
    try manager.save();

    // 创建新的管理器并加载文件
    var manager2 = try settings_module.SettingsManager.init(allocator, test_file_path);
    defer manager2.deinit();

    try manager2.load();

    // 验证加载的配置与保存的配置相同
    try std.testing.expectEqual(manager2.config.basic.timezone, -5);
    try std.testing.expect(std.mem.eql(u8, manager2.config.basic.language, "EN"));
    try std.testing.expectEqual(manager2.config.basic.default_mode, .stopwatch);

    // 清理
    std.fs.cwd().deleteFile(test_file_path) catch {};
}

test "预设持久化 - savePresetsToFile() 和 loadPresetsFromFile()" {
    var tmp_dir = try std.fs.cwd().makeOpenPath("test_tmp", .{});
    defer tmp_dir.close();

    const allocator = std.testing.allocator;
    const test_file_path = "test_tmp/test_presets.json";

    // 清理旧文件
    std.fs.cwd().deleteFile(test_file_path) catch {};

    var manager = try settings_module.SettingsManager.init(allocator, "test_tmp/test_settings.toml");
    defer manager.deinit();

    // 添加预设
    const name1 = try allocator.dupe(u8, "Test Preset 1");
    const preset1: interface.TimerPreset = .{
        .name = name1,
        .mode = .COUNTDOWN_MODE,
        .config = .{ .countdown = .{ .duration_seconds = 900, .loop = false, .loop_count = 0, .loop_interval_seconds = 0 } },
    };

    const name2 = try allocator.dupe(u8, "Test Preset 2");
    const preset2: interface.TimerPreset = .{
        .name = name2,
        .mode = .STOPWATCH_MODE,
        .config = .{ .stopwatch = .{ .max_seconds = 3600 } },
    };

    try manager.addPreset(preset1);
    try manager.addPreset(preset2);

    // 保存预设
    try manager.savePresetsToFile();

    // 创建新管理器并加载预设
    var manager2 = try settings_module.SettingsManager.init(allocator, "test_tmp/test_settings.toml");
    defer manager2.deinit();

    try manager2.loadPresetsFromFile();

    // 验证预设已加载
    try std.testing.expectEqual(manager2.preset_count, 2);
    try std.testing.expect(std.mem.eql(u8, manager2.timer_presets[0].name, "Test Preset 1"));
    try std.testing.expect(std.mem.eql(u8, manager2.timer_presets[1].name, "Test Preset 2"));

    // 清理
    std.fs.cwd().deleteFile(test_file_path) catch {};
    std.fs.cwd().deleteFile("test_tmp/test_settings.toml") catch {};
}

test "损坏的文件恢复 - backupCorruptedFile()" {
    var tmp_dir = try std.fs.cwd().makeOpenPath("test_tmp", .{});
    defer tmp_dir.close();

    const allocator = std.testing.allocator;
    const test_file_path = "test_tmp/corrupted_settings.toml";

    // 创建一个损坏的配置文件
    var file = try std.fs.cwd().createFile(test_file_path, .{});
    try file.writeAll("invalid toml content {{{ corrupted");
    file.close();

    var manager = try settings_module.SettingsManager.init(allocator, test_file_path);
    defer manager.deinit();

    // 尝试备份（应该生成带时间戳的备份文件）
    try manager.backupCorruptedFile();

    // 验证原始文件仍然存在
    try std.fs.cwd().access(test_file_path, .{});

    // 清理（删除所有相关文件）
    var dir = try std.fs.cwd().openDir("test_tmp", .{ .iterate = true });
    defer dir.close();

    var iterator = dir.iterate();
    while (try iterator.next()) |entry| {
        if (entry.kind == .file and std.mem.startsWith(u8, entry.name, "corrupted_settings")) {
            var buf: [256]u8 = undefined;
            const file_path = std.fmt.bufPrint(&buf, "test_tmp/{s}", .{entry.name}) catch continue;
            std.fs.cwd().deleteFile(file_path) catch {};
        }
    }
}

test "resetToDefaults() 重置配置" {
    const allocator = std.testing.allocator;
    var manager = try settings_module.SettingsManager.init(allocator, "");
    defer manager.deinit();

    // 修改配置
    manager.config.basic.timezone = -10;
    manager.config.clock_defaults.countdown.duration_seconds = 100;

    // 添加预设
    const name = try allocator.dupe(u8, "Test Preset");
    const preset: interface.TimerPreset = .{
        .name = name,
        .mode = .COUNTDOWN_MODE,
        .config = .{ .countdown = .{ .duration_seconds = 500, .loop = false, .loop_count = 0, .loop_interval_seconds = 0 } },
    };
    try manager.addPreset(preset);

    // 重置为默认值
    try manager.resetToDefaults();

    // 验证配置已重置
    try std.testing.expectEqual(manager.config.basic.timezone, 8);
    try std.testing.expect(std.mem.eql(u8, manager.config.basic.language, "ZH"));
    try std.testing.expectEqual(manager.preset_count, 0); // 预设应该被清空
    try std.testing.expect(manager.is_dirty);
}

test "handleSettingsEvent() 通过 get_settings 获取配置" {
    const allocator = std.testing.allocator;
    var manager = try settings_module.SettingsManager.init(allocator, "");
    defer manager.deinit();

    manager.config.basic.timezone = 5;

    // 设置空缓冲区来调用 toJsonAlloc
    // 需要至少有 1 个元素的数组来存放哨兵
    var buffer: [1]u8 = .{0};
    const event: interface.SettingsEvent = .{ .get_settings = buffer[0..0 :0] };

    try manager.handleSettingsEvent(event);

    // 不应该崩溃或返回错误
}

test "handleSettingsEvent() 通过 change_settings 更新配置" {
    var tmp_dir = try std.fs.cwd().makeOpenPath("test_tmp", .{});
    defer tmp_dir.close();

    const allocator = std.testing.allocator;
    const test_file_path = "test_tmp/test_handle_event.toml";

    // 清理旧文件
    std.fs.cwd().deleteFile(test_file_path) catch {};

    var manager = try settings_module.SettingsManager.init(allocator, test_file_path);
    defer manager.deinit();

    const json_input =
        \\{"basic":{"timezone":9,"language":"JP","default_mode":"countdown","theme_mode":"dark"},"clock_defaults":{"countdown":{"duration_seconds":1200,"loop":false,"loop_count":0,"loop_interval_seconds":0},"stopwatch":{"max_seconds":86400}},"logging":{"level":"INFO","enable_timestamp":true,"tick_interval_ms":1000}}
    ;

    // 需要创建哨兵切片
    const json_owned = try allocator.dupeZ(u8, json_input);
    // 注意：handleSettingsEvent 内部会释放这个内存

    const event: interface.SettingsEvent = .{ .change_settings = json_owned };

    try manager.handleSettingsEvent(event);

    try std.testing.expectEqual(manager.config.basic.timezone, 9);
    try std.testing.expect(std.mem.eql(u8, manager.config.basic.language, "JP"));

    // 清理
    std.fs.cwd().deleteFile(test_file_path) catch {};
    std.fs.cwd().deleteFile("test_tmp/presets.json") catch {};
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
    // JSON 中数字不带引号
    try std.testing.expect(std.mem.containsAtLeast(u8, json_str, 1, ":7"));
    try std.testing.expect(std.mem.containsAtLeast(u8, json_str, 1, "2000"));
}
