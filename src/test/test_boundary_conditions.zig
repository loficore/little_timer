//! 边界条件综合测试 - 验证所有关键问题修复
const std = @import("std");
const clock = @import("../clock.zig");
const settings_module = @import("../settings.zig");
const interface = @import("../interface.zig");

// ============ Clock 模块边界条件测试 ============

test "问题1: 倒计时整数溢出检查" {
    // duration_seconds 接近最大值时不应该 panic
    // 这个值会导致 * 1000 溢出（如果不检查）
    // 但 ClockManager.init 会检查并降级到安全值
    const large_duration: u64 = std.math.maxInt(u64) / 2;

    const config: interface.ClockTaskConfig = .{
        .countdown = .{
            .duration_seconds = large_duration,
            .loop = false,
            .loop_count = 0,
            .loop_interval_seconds = 0,
        },
        .stopwatch = .{ .max_seconds = 24 * 60 * 60 },
        .world_clock = .{ .timezone = 8 },
    };

    const manager = clock.ClockManager.init(config);

    // 验证已降级到安全值
    try std.testing.expectEqual(manager.state.COUNTDOWN_MODE.duration_ms, 25 * 60 * 1000);
}

test "问题1: 正计时整数溢出检查" {
    // 使用一个大的但安全的值来验证不会溢出
    const large_max_seconds: u64 = 86400 * 365; // 365天

    const config: interface.ClockTaskConfig = .{
        .default_mode = .STOPWATCH_MODE,
        .countdown = .{ .duration_seconds = 25 * 60, .loop = false, .loop_count = 0, .loop_interval_seconds = 0 },
        .stopwatch = .{
            .max_seconds = large_max_seconds,
        },
        .world_clock = .{ .timezone = 8 },
    };

    const manager = clock.ClockManager.init(config);

    // 验证值被正确保存（不被修改）
    try std.testing.expectEqual(manager.state.STOPWATCH_MODE.max_ms, large_max_seconds * 1000);
}

test "问题1: 安全的 duration 值不被修改" {
    const safe_duration: u64 = 3600; // 1小时

    const config: interface.ClockTaskConfig = .{
        .countdown = .{
            .duration_seconds = safe_duration,
            .loop = false,
            .loop_count = 0,
            .loop_interval_seconds = 0,
        },
        .stopwatch = .{ .max_seconds = 24 * 60 * 60 },
        .world_clock = .{ .timezone = 8 },
    };

    const manager = clock.ClockManager.init(config);

    try std.testing.expectEqual(manager.state.COUNTDOWN_MODE.duration_ms, 3600 * 1000);
}

test "问题2: 倒计时拒绝负数 tick" {
    const config: interface.ClockTaskConfig = .{
        .countdown = .{
            .duration_seconds = 60,
            .loop = false,
            .loop_count = 0,
            .loop_interval_seconds = 0,
        },
    };
    var manager = clock.ClockManager.init(config);

    manager.handleEvent(.user_start_timer);

    const remaining_before = manager.state.COUNTDOWN_MODE.remaining_ms;

    // 发送负数 tick（应该被拒绝）
    manager.handleEvent(.{ .tick = -5000 });

    // 验证时间没有增加
    try std.testing.expectEqual(manager.state.COUNTDOWN_MODE.remaining_ms, remaining_before);
}

test "问题2: 正计时拒绝负数 tick" {
    const config: interface.ClockTaskConfig = .{
        .default_mode = .STOPWATCH_MODE,
        .stopwatch = .{
            .max_seconds = 3600,
        },
    };
    var manager = clock.ClockManager.init(config);

    manager.handleEvent(.user_start_timer);
    manager.handleEvent(.{ .tick = 1000 });

    const elapsed_before = manager.state.STOPWATCH_MODE.esplased_ms;

    // 发送负数 tick（应该被拒绝）
    manager.handleEvent(.{ .tick = -500 });

    // 验证时间没有倒退
    try std.testing.expectEqual(manager.state.STOPWATCH_MODE.esplased_ms, elapsed_before);
}

test "问题2: 倒计时接受零 tick" {
    const config: interface.ClockTaskConfig = .{
        .countdown = .{
            .duration_seconds = 60,
            .loop = false,
            .loop_count = 0,
            .loop_interval_seconds = 0,
        },
    };
    var manager = clock.ClockManager.init(config);

    manager.handleEvent(.user_start_timer);

    const remaining_before = manager.state.COUNTDOWN_MODE.remaining_ms;

    // 零 tick 不应该改变任何状态
    manager.handleEvent(.{ .tick = 0 });

    try std.testing.expectEqual(manager.state.COUNTDOWN_MODE.remaining_ms, remaining_before);
}

test "问题3: 倒计时在暂停时不会消耗" {
    const config: interface.ClockTaskConfig = .{
        .countdown = .{
            .duration_seconds = 10,
            .loop = false,
            .loop_count = 0,
            .loop_interval_seconds = 0,
        },
        .stopwatch = .{ .max_seconds = 3600 },
        .default_mode = .COUNTDOWN_MODE,
        .world_clock = .{ .timezone = 8 },
    };

    var manager = clock.ClockManager.init(config);

    // 不启动，保持暂停状态
    const initial_time = manager.state.COUNTDOWN_MODE.remaining_ms;

    // 发送 tick，但由于暂停，时间不应该变化
    manager.handleEvent(.{ .tick = 5000 });

    try std.testing.expectEqual(manager.state.COUNTDOWN_MODE.remaining_ms, initial_time);
}

// ============ Settings 模块边界条件测试 ============

test "问题4: duration_seconds JSON 负数被拒绝" {
    const allocator = std.testing.allocator;
    var manager = try settings_module.SettingsManager.init(allocator, "");
    defer manager.deinit();

    const json_input = "{\"basic\":{\"timezone\":8,\"language\":\"ZH\",\"default_mode\":\"countdown\",\"theme_mode\":\"dark\"},\"clock_defaults\":{\"countdown\":{\"duration_seconds\":-100,\"loop\":false,\"loop_count\":0,\"loop_interval_seconds\":0},\"stopwatch\":{\"max_seconds\":86400}},\"logging\":{\"level\":\"INFO\",\"enable_timestamp\":true,\"tick_interval_ms\":1000}}";

    try manager.jsonToSettings(json_input);

    // 验证没有被修改（保持默认值 1500）
    try std.testing.expectEqual(manager.config.clock_defaults.countdown.duration_seconds, 1500);
}

test "问题4: loop_count 超大值被拒绝" {
    const allocator = std.testing.allocator;
    var manager = try settings_module.SettingsManager.init(allocator, "");
    defer manager.deinit();

    const json_input = "{\"basic\":{\"timezone\":8,\"language\":\"ZH\",\"default_mode\":\"countdown\",\"theme_mode\":\"dark\"},\"clock_defaults\":{\"countdown\":{\"duration_seconds\":1500,\"loop\":false,\"loop_count\":5000000000,\"loop_interval_seconds\":0},\"stopwatch\":{\"max_seconds\":86400}},\"logging\":{\"level\":\"INFO\",\"enable_timestamp\":true,\"tick_interval_ms\":1000}}";

    try manager.jsonToSettings(json_input);

    // 验证没有被修改（保持默认值 0）
    try std.testing.expectEqual(manager.config.clock_defaults.countdown.loop_count, 0);
}

test "问题4: max_seconds 超大值被拒绝" {
    const allocator = std.testing.allocator;
    var manager = try settings_module.SettingsManager.init(allocator, "");
    defer manager.deinit();

    const json_input = "{\"basic\":{\"timezone\":8,\"language\":\"ZH\",\"default_mode\":\"stopwatch\",\"theme_mode\":\"dark\"},\"clock_defaults\":{\"countdown\":{\"duration_seconds\":1500,\"loop\":false,\"loop_count\":0,\"loop_interval_seconds\":0},\"stopwatch\":{\"max_seconds\":99999999999}},\"logging\":{\"level\":\"INFO\",\"enable_timestamp\":true,\"tick_interval_ms\":1000}}";

    try manager.jsonToSettings(json_input);

    // 验证没有被修改
    try std.testing.expectEqual(manager.config.clock_defaults.stopwatch.max_seconds, 86400);
}

test "问题5: 预设列表满返回正确错误类型" {
    const allocator = std.testing.allocator;
    var manager = try settings_module.SettingsManager.init(allocator, "");
    defer manager.deinit();

    // 添加 10 个预设（达到上限）
    for (0..10) |i| {
        var buf: [50]u8 = undefined;
        const name = try std.fmt.bufPrint(&buf, "预设{d}", .{i});
        const preset: interface.TimerPreset = .{
            .name = try allocator.dupe(u8, name),
            .mode = .COUNTDOWN_MODE,
            .config = .{ .countdown = .{ .duration_seconds = 1500, .loop = false, .loop_count = 0, .loop_interval_seconds = 0 } },
        };
        try manager.addPreset(preset);
    }

    // 尝试添加第 11 个（应该返回 PresetListFull）
    const name = try allocator.dupe(u8, "预设10");
    const preset: interface.TimerPreset = .{
        .name = name,
        .mode = .COUNTDOWN_MODE,
        .config = .{ .countdown = .{ .duration_seconds = 1500, .loop = false, .loop_count = 0, .loop_interval_seconds = 0 } },
    };

    const result = manager.addPreset(preset);

    try std.testing.expectError(settings_module.SettingsError.PresetListFull, result);

    // 清理
    allocator.free(name);
}

test "问题6: 语言代码过长被拒绝" {
    const allocator = std.testing.allocator;
    var manager = try settings_module.SettingsManager.init(allocator, "");
    defer manager.deinit();

    const json_input = "{\"basic\":{\"timezone\":8,\"language\":\"VERY_LONG_LANGUAGE_CODE_EXCEEDS_LIMIT\",\"default_mode\":\"countdown\",\"theme_mode\":\"dark\"},\"clock_defaults\":{\"countdown\":{\"duration_seconds\":1500,\"loop\":false,\"loop_count\":0,\"loop_interval_seconds\":0},\"stopwatch\":{\"max_seconds\":86400}},\"logging\":{\"level\":\"INFO\",\"enable_timestamp\":true,\"tick_interval_ms\":1000}}";

    try manager.jsonToSettings(json_input);

    // 验证没有被修改
    try std.testing.expect(std.mem.eql(u8, manager.config.basic.language, "ZH"));
}

test "问题6: 语言代码空字符串被拒绝" {
    const allocator = std.testing.allocator;
    var manager = try settings_module.SettingsManager.init(allocator, "");
    defer manager.deinit();

    const json_input = "{\"basic\":{\"timezone\":8,\"language\":\"\",\"default_mode\":\"countdown\",\"theme_mode\":\"dark\"},\"clock_defaults\":{\"countdown\":{\"duration_seconds\":1500,\"loop\":false,\"loop_count\":0,\"loop_interval_seconds\":0},\"stopwatch\":{\"max_seconds\":86400}},\"logging\":{\"level\":\"INFO\",\"enable_timestamp\":true,\"tick_interval_ms\":1000}}";

    try manager.jsonToSettings(json_input);

    // 验证没有被修改
    try std.testing.expect(std.mem.eql(u8, manager.config.basic.language, "ZH"));
}

test "问题7: 时区超出范围被记录为警告并保持旧值" {
    const allocator = std.testing.allocator;
    var manager = try settings_module.SettingsManager.init(allocator, "");
    defer manager.deinit();

    manager.config.basic.timezone = 8; // 设置初始时区

    const json_input = "{\"basic\":{\"timezone\":20,\"language\":\"ZH\",\"default_mode\":\"countdown\",\"theme_mode\":\"dark\"},\"clock_defaults\":{\"countdown\":{\"duration_seconds\":1500,\"loop\":false,\"loop_count\":0,\"loop_interval_seconds\":0},\"stopwatch\":{\"max_seconds\":86400}},\"logging\":{\"level\":\"INFO\",\"enable_timestamp\":true,\"tick_interval_ms\":1000}}";

    try manager.jsonToSettings(json_input);

    // 验证时区没有被修改（静默忽略但记录日志）
    try std.testing.expectEqual(manager.config.basic.timezone, 8);
}

test "问题6: updateBasic 方法验证语言代码长度" {
    const allocator = std.testing.allocator;
    var manager = try settings_module.SettingsManager.init(allocator, "");
    defer manager.deinit();

    // 尝试设置超长语言代码
    const result = manager.updateBasic(.{
        .timezone = 8,
        .language = "THIS_IS_A_VERY_LONG_LANGUAGE_CODE_EXCEEDS_10_CHARS",
        .default_mode = .countdown,
    });

    try std.testing.expectError(settings_module.SettingsError.InvalidLanguage, result);
}

test "问题6: updateBasic 方法验证语言代码不为空" {
    const allocator = std.testing.allocator;
    var manager = try settings_module.SettingsManager.init(allocator, "");
    defer manager.deinit();

    // 尝试设置空语言代码
    const result = manager.updateBasic(.{
        .timezone = 8,
        .language = "",
        .default_mode = .countdown,
    });

    try std.testing.expectError(settings_module.SettingsError.InvalidLanguage, result);
}
