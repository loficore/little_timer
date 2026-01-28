//! 设置 JSON 序列化/反序列化模块
//! 职责：处理 JSON 与 SettingsConfig 之间的转换，与后端/前端通信
const std = @import("std");
const interface = @import("interface.zig");
const logger = @import("logger.zig");
const validator = @import("settings_validator.zig");

pub const SettingsConfig = interface.SettingsConfig;

/// 写入转义的 JSON 字符串（统一转义逻辑）
///
/// 参数:
/// - **writer**: 写入器
/// - **s**: 要转义的字符串
///
/// 返回:
/// - !void: 如果写入失败则返回错误
pub fn writeEscapedJsonString(writer: anytype, s: []const u8) !void {
    try writer.writeByte('"');
    for (s) |ch| {
        switch (ch) {
            '\"' => try writer.writeAll("\\\""),
            '\\' => try writer.writeAll("\\\\"),
            '\n' => try writer.writeAll("\\n"),
            '\r' => try writer.writeAll("\\r"),
            '\t' => try writer.writeAll("\\t"),
            else => try writer.writeByte(ch),
        }
    }
    try writer.writeByte('"');
}

/// 将设置转换为 JSON 字符串（动态分配）
///
/// 参数:
/// - **allocator**: 内存分配器
/// - **config**: 要序列化的设置配置
/// - **presets**: 预设数组
///
/// 返回:
/// - ![]u8: 动态分配的 JSON 字符串（调用者需要通过 allocator.free() 释放）
pub fn toJsonAlloc(
    allocator: std.mem.Allocator,
    config: *const SettingsConfig,
    presets: anytype, // PresetsManager 或兼容类型
) ![]u8 {
    const mode_str = switch (config.basic.default_mode) {
        .countdown => "countdown",
        .stopwatch => "stopwatch",
        .world_clock => "world_clock",
    };

    // 使用 ArrayList 动态构建 JSON，根据内容大小自动扩容
    var json_list = std.ArrayList(u8){};
    defer json_list.deinit(allocator);
    const w = json_list.writer(allocator);

    // 开始 JSON 对象
    try w.writeAll("{\"basic\":{\"timezone\":");
    try w.print("{}", .{config.basic.timezone});
    try w.writeAll(",\"language\":\"");
    try w.writeAll(config.basic.language);
    try w.writeAll("\",\"default_mode\":\"");
    try w.writeAll(mode_str);
    try w.writeAll("\",\"theme_mode\":\"");
    try w.writeAll(config.basic.theme_mode);
    try w.writeAll("\"},\"clock_defaults\":{\"countdown\":{\"duration_seconds\":");
    try w.print("{}", .{config.clock_defaults.countdown.duration_seconds});
    try w.writeAll(",\"loop\":");
    try w.print("{}", .{@intFromBool(config.clock_defaults.countdown.loop)});
    try w.writeAll(",\"loop_count\":");
    try w.print("{}", .{config.clock_defaults.countdown.loop_count});
    try w.writeAll(",\"loop_interval_seconds\":");
    try w.print("{}", .{config.clock_defaults.countdown.loop_interval_seconds});
    try w.writeAll("},\"stopwatch\":{\"max_seconds\":");
    try w.print("{}", .{config.clock_defaults.stopwatch.max_seconds});
    try w.writeAll("}},\"logging\":{\"level\":\"");
    try w.writeAll(config.logging.level);
    try w.writeAll("\",\"enable_timestamp\":");
    try w.print("{}", .{@intFromBool(config.logging.enable_timestamp)});
    try w.writeAll(",\"tick_interval_ms\":");
    try w.print("{}", .{config.logging.tick_interval_ms});

    // 添加预设序列化（使用动态预设列表）
    try w.writeAll("},\"presets\":[");
    const preset_items = presets.presets.items;
    for (preset_items, 0..) |preset, i| {
        if (i > 0) try w.writeByte(',');

        try w.writeAll("{\"name\":\"");
        // 转义字符串中的特殊字符
        for (preset.name) |ch| {
            switch (ch) {
                '\"' => try w.writeAll("\\\""),
                '\\' => try w.writeAll("\\\\"),
                else => try w.writeByte(ch),
            }
        }
        try w.writeAll("\",\"mode\":\"");

        switch (preset.mode) {
            .COUNTDOWN_MODE => {
                try w.writeAll("countdown\",\"config\":{\"duration_seconds\":");
                try w.print("{}", .{preset.config.countdown.duration_seconds});
                try w.writeAll(",\"loop\":");
                try w.print("{}", .{@intFromBool(preset.config.countdown.loop)});
                try w.writeAll(",\"loop_count\":");
                try w.print("{}", .{preset.config.countdown.loop_count});
                try w.writeAll(",\"loop_interval_seconds\":");
                try w.print("{}", .{preset.config.countdown.loop_interval_seconds});
                try w.writeAll("}}");
            },
            .STOPWATCH_MODE => {
                try w.writeAll("stopwatch\",\"config\":{\"max_seconds\":");
                try w.print("{}", .{preset.config.stopwatch.max_seconds});
                try w.writeAll("}}");
            },
            .WORLD_CLOCK_MODE => {
                try w.writeAll("world_clock\",\"config\":{\"timezone\":");
                try w.print("{}", .{preset.config.world_clock.timezone});
                try w.writeAll("}}");
            },
        }
    }
    try w.writeAll("]}");

    // 返回动态分配的 ArrayList 内容（所有权转移给调用者）
    // 调用者需要负责通过 allocator.free() 释放此内存
    return try json_list.toOwnedSlice(allocator);
}

/// 从 JSON 字符串解析并更新设置
///
/// 参数:
/// - **allocator**: 内存分配器
/// - **config**: 要更新的设置配置指针
/// - **presets**: 要更新的预设管理器
/// - **owned_language**: 指向现有语言字符串的指针，用于释放旧值
/// - **owned_theme_mode**: 指向现有主题字符串的指针，用于释放旧值
/// - **owned_log_level**: 指向现有日志等级字符串的指针，用于释放旧值
/// - **json_str**: JSON 字符串
///
/// 返回:
/// - !void: 如果解析失败则返回错误
pub fn jsonToSettings(
    allocator: std.mem.Allocator,
    config: *SettingsConfig,
    presets: anytype, // PresetsManager 或兼容类型
    owned_language: *?[]u8,
    owned_theme_mode: *?[]u8,
    owned_log_level: *?[]u8,
    json_str: []const u8,
) !void {
    var parsed = try std.json.parseFromSlice(std.json.Value, allocator, json_str, .{});
    defer parsed.deinit();

    const root = parsed.value;

    // 解析 basic 部分
    if (root.object.get("basic")) |basic_val| {
        if (basic_val.object.get("timezone")) |tz_val| {
            if (tz_val.integer >= -12 and tz_val.integer <= 14) {
                config.basic.timezone = @intCast(tz_val.integer);
            } else {
                logger.global_logger.warn("⚠️ JSON中时区超出范围 [-12, 14]，当前值: {d}，保持旧值: {}", .{ tz_val.integer, config.basic.timezone });
            }
        }

        if (basic_val.object.get("language")) |lang_val| {
            if (lang_val.string.len == 0 or lang_val.string.len > 10) {
                logger.global_logger.warn("⚠️ JSON中语言代码长度无效: {}, 保持旧值: {s}", .{ lang_val.string.len, config.basic.language });
            } else {
                // 释放旧的动态分配的字符串
                if (owned_language.*) |old| {
                    allocator.free(old);
                }
                // 复制新字符串
                owned_language.* = try allocator.dupe(u8, lang_val.string);
                config.basic.language = owned_language.*.?;
            }
        }

        if (basic_val.object.get("default_mode")) |mode_val| {
            if (std.mem.eql(u8, mode_val.string, "countdown")) {
                config.basic.default_mode = .countdown;
            } else if (std.mem.eql(u8, mode_val.string, "stopwatch")) {
                config.basic.default_mode = .stopwatch;
            } else if (std.mem.eql(u8, mode_val.string, "world_clock")) {
                config.basic.default_mode = .world_clock;
            }
        }

        if (basic_val.object.get("theme_mode")) |theme_val| {
            // 释放旧的动态分配的字符串
            if (owned_theme_mode.*) |old| {
                allocator.free(old);
            }
            // 复制新字符串
            owned_theme_mode.* = try allocator.dupe(u8, theme_val.string);
            config.basic.theme_mode = owned_theme_mode.*.?;
        }
    }

    // 解析 clock_defaults 部分
    if (root.object.get("clock_defaults")) |defaults_val| {
        if (defaults_val.object.get("countdown")) |countdown_val| {
            if (countdown_val.object.get("duration_seconds")) |dur_val| {
                if (dur_val.integer < 1 or dur_val.integer > 86400) { // 最多24小时
                    logger.global_logger.warn("⚠️ JSON中倒计时时长超出范围 [1, 86400], 当前值: {d}，保持旧值: {}", .{ dur_val.integer, config.clock_defaults.countdown.duration_seconds });
                } else {
                    config.clock_defaults.countdown.duration_seconds = @intCast(dur_val.integer);
                }
            }
            if (countdown_val.object.get("loop")) |loop_val| {
                config.clock_defaults.countdown.loop = switch (loop_val) {
                    .bool => |b| b,
                    .integer => |i| i != 0,
                    else => false,
                };
            }
            if (countdown_val.object.get("loop_count")) |count_val| {
                if (count_val.integer < 0 or count_val.integer > 1000) { // 最多1000次循环
                    logger.global_logger.warn("⚠️ JSON中循环次数超出范围 [0, 1000], 当前值: {d}，保持旧值: {}", .{ count_val.integer, config.clock_defaults.countdown.loop_count });
                } else {
                    config.clock_defaults.countdown.loop_count = @intCast(count_val.integer);
                }
            }
            if (countdown_val.object.get("loop_interval_seconds")) |interval_val| {
                if (interval_val.integer < 0 or interval_val.integer > 3600) { // 最多1小时休息
                    logger.global_logger.warn("⚠️ JSON中循环间隔超出范围 [0, 3600], 当前值: {d}，保持旧值: {}", .{ interval_val.integer, config.clock_defaults.countdown.loop_interval_seconds });
                } else {
                    config.clock_defaults.countdown.loop_interval_seconds = @intCast(interval_val.integer);
                }
            }
        }

        if (defaults_val.object.get("stopwatch")) |stopwatch_val| {
            if (stopwatch_val.object.get("max_seconds")) |max_val| {
                if (max_val.integer <= 0 or max_val.integer > 86400 * 365) { // 最多365天
                    logger.global_logger.warn("⚠️ JSON中正计时上限超出范围 (0, 31536000], 当前值: {d}，保持旧值: {}", .{ max_val.integer, config.clock_defaults.stopwatch.max_seconds });
                } else {
                    config.clock_defaults.stopwatch.max_seconds = @intCast(max_val.integer);
                }
            }
        }
    }

    // 解析 logging 部分
    if (root.object.get("logging")) |logging_val| {
        if (logging_val.object.get("level")) |level_val| {
            // 释放旧的动态分配的字符串
            if (owned_log_level.*) |old| {
                allocator.free(old);
            }
            // 复制新字符串
            owned_log_level.* = try allocator.dupe(u8, level_val.string);
            config.logging.level = owned_log_level.*.?;
        }

        if (logging_val.object.get("enable_timestamp")) |ts_val| {
            config.logging.enable_timestamp = switch (ts_val) {
                .bool => |b| b,
                .integer => |i| i != 0,
                else => false,
            };
        }

        if (logging_val.object.get("tick_interval_ms")) |interval_val| {
            if (interval_val.integer > 0) {
                config.logging.tick_interval_ms = @intCast(interval_val.integer);
            }
        }
    }

    // 解析 presets（可选）
    if (root.object.get("presets")) |presets_val| {
        if (presets_val == .array) {
            // 清空现有预设
            presets.clear();

            for (presets_val.array.items) |pval| {
                if (presets.count() >= presets.max_count) break;
                if (pval != .object) continue;

                const pobj = pval.object;
                const name_val_opt = pobj.get("name");
                const mode_val_opt = pobj.get("mode");
                const cfg_val_opt = pobj.get("config");
                if (name_val_opt == null or mode_val_opt == null or cfg_val_opt == null) continue;
                const name_val = name_val_opt.?;
                const mode_val = mode_val_opt.?;
                const cfg_val = cfg_val_opt.?;
                if (name_val != .string or mode_val != .string or cfg_val != .object) continue;

                var config_union: interface.ClockTaskConfig = undefined;
                var mode_enum: interface.ModeEnumT = undefined;
                if (std.mem.eql(u8, mode_val.string, "countdown")) {
                    const dur = if (cfg_val.object.get("duration_seconds")) |v|
                        validator.safeU64FromJson(v.integer, 1, 86400) orelse 25 * 60
                    else
                        25 * 60;
                    const loop = if (cfg_val.object.get("loop")) |v| switch (v) {
                        .bool => |b| b,
                        .integer => |i| i != 0,
                        else => false,
                    } else false;
                    const loop_count = if (cfg_val.object.get("loop_count")) |v|
                        validator.safeU32FromJson(v.integer, 1000) orelse 0
                    else
                        0;
                    const loop_interval = if (cfg_val.object.get("loop_interval_seconds")) |v|
                        validator.safeU64FromJson(v.integer, 0, 3600) orelse 0
                    else
                        0;
                    config_union = .{ .countdown = .{ .duration_seconds = dur, .loop = loop, .loop_count = loop_count, .loop_interval_seconds = loop_interval }, .stopwatch = .{ .max_seconds = 24 * 3600 }, .world_clock = .{ .timezone = 8 } };
                    mode_enum = .COUNTDOWN_MODE;
                } else if (std.mem.eql(u8, mode_val.string, "stopwatch")) {
                    const max = if (cfg_val.object.get("max_seconds")) |v|
                        validator.safeU64FromJson(v.integer, 1, 86400 * 365) orelse 24 * 3600
                    else
                        24 * 3600;
                    config_union = .{ .countdown = .{ .duration_seconds = 25 * 60, .loop = false, .loop_count = 0, .loop_interval_seconds = 0 }, .stopwatch = .{ .max_seconds = max }, .world_clock = .{ .timezone = 8 } };
                    mode_enum = .STOPWATCH_MODE;
                } else if (std.mem.eql(u8, mode_val.string, "world_clock")) {
                    const tz = if (cfg_val.object.get("timezone")) |v|
                        validator.safeI8FromJson(v.integer, -12, 14) orelse config.basic.timezone
                    else
                        config.basic.timezone;
                    config_union = .{ .countdown = .{ .duration_seconds = 25 * 60, .loop = false, .loop_count = 0, .loop_interval_seconds = 0 }, .stopwatch = .{ .max_seconds = 24 * 3600 }, .world_clock = .{ .timezone = tz } };
                    mode_enum = .WORLD_CLOCK_MODE;
                } else {
                    continue;
                }

                const preset: interface.TimerPreset = .{
                    .name = name_val.string,
                    .mode = mode_enum,
                    .config = config_union,
                };

                // 使用 PresetsManager 来处理拷贝与去重
                presets.add(preset) catch |err| {
                    logger.global_logger.warn("⚠️ 添加预设失败: {any}", .{err});
                    continue;
                };
            }
        }
    }

    logger.global_logger.info("✓ 设置已从 JSON 更新", .{});
}

/// 仅序列化预设为 JSON（用于 presets.json 持久化）
///
/// 参数:
/// - **allocator**: 内存分配器
/// - **presets**: 预设管理器或兼容类型
///
/// 返回:
/// - ![]u8: 动态分配的 JSON 字符串（调用者需要通过 allocator.free() 释放）
pub fn serializePresetsOnly(allocator: std.mem.Allocator, presets: anytype) ![]u8 {
    var json_list = std.ArrayList(u8){};
    defer json_list.deinit(allocator);
    const w = json_list.writer(allocator);

    try w.writeAll("{\"presets\":[");
    const preset_items = presets.presets.items;
    for (preset_items, 0..) |preset, i| {
        if (i > 0) try w.writeByte(',');

        try w.writeAll("{\"name\":");
        try writeEscapedJsonString(w, preset.name);
        try w.writeAll(",\"mode\":");

        switch (preset.mode) {
            .COUNTDOWN_MODE => {
                try w.writeAll("\"countdown\",\"config\":{\"duration_seconds\":");
                try w.print("{}", .{preset.config.countdown.duration_seconds});
                try w.writeAll(",\"loop\":");
                try w.print("{}", .{@intFromBool(preset.config.countdown.loop)});
                try w.writeAll(",\"loop_count\":");
                try w.print("{}", .{preset.config.countdown.loop_count});
                try w.writeAll(",\"loop_interval_seconds\":");
                try w.print("{}", .{preset.config.countdown.loop_interval_seconds});
                try w.writeAll("}}");
            },
            .STOPWATCH_MODE => {
                try w.writeAll("\"stopwatch\",\"config\":{\"max_seconds\":");
                try w.print("{}", .{preset.config.stopwatch.max_seconds});
                try w.writeAll("}}");
            },
            .WORLD_CLOCK_MODE => {
                try w.writeAll("\"world_clock\",\"config\":{\"timezone\":");
                try w.print("{}", .{preset.config.world_clock.timezone});
                try w.writeAll("}}");
            },
        }
    }
    try w.writeAll("]}");

    return try json_list.toOwnedSlice(allocator);
}

/// 仅反序列化预设从 JSON（用于 presets.json 加载）
///
/// 参数:
/// - **allocator**: 内存分配器
/// - **json_str**: JSON 字符串
/// - **presets**: 要填充的预设管理器
/// - **default_timezone**: 默认时区（用于 world_clock 预设）
///
/// 返回:
/// - !void: 如果解析失败则返回错误
pub fn deserializePresetsOnly(
    allocator: std.mem.Allocator,
    json_str: []const u8,
    presets: anytype,
    default_timezone: i8,
) !void {
    var parsed = try std.json.parseFromSlice(std.json.Value, allocator, json_str, .{});
    defer parsed.deinit();

    const root = parsed.value;
    if (root.object.get("presets")) |presets_val| {
        if (presets_val == .array) {
            // 清空现有预设
            presets.clear();

            for (presets_val.array.items) |pval| {
                if (presets.count() >= presets.max_count) break;
                if (pval != .object) continue;

                const pobj = pval.object;
                const name_val_opt = pobj.get("name");
                const mode_val_opt = pobj.get("mode");
                const cfg_val_opt = pobj.get("config");
                if (name_val_opt == null or mode_val_opt == null or cfg_val_opt == null) continue;
                const name_val = name_val_opt.?;
                const mode_val = mode_val_opt.?;
                const cfg_val = cfg_val_opt.?;
                if (name_val != .string or mode_val != .string or cfg_val != .object) continue;

                var config_union: interface.ClockTaskConfig = undefined;
                var mode_enum: interface.ModeEnumT = undefined;

                if (std.mem.eql(u8, mode_val.string, "countdown")) {
                    const dur = if (cfg_val.object.get("duration_seconds")) |v|
                        validator.safeU64FromJson(v.integer, 1, 86400) orelse 25 * 60
                    else
                        25 * 60;
                    const loop = if (cfg_val.object.get("loop")) |v| switch (v) {
                        .bool => |b| b,
                        .integer => |i| i != 0,
                        else => false,
                    } else false;
                    const loop_count = if (cfg_val.object.get("loop_count")) |v|
                        validator.safeU32FromJson(v.integer, 1000) orelse 0
                    else
                        0;
                    const loop_interval = if (cfg_val.object.get("loop_interval_seconds")) |v|
                        validator.safeU64FromJson(v.integer, 0, 3600) orelse 0
                    else
                        0;
                    config_union = .{
                        .countdown = .{
                            .duration_seconds = dur,
                            .loop = loop,
                            .loop_count = loop_count,
                            .loop_interval_seconds = loop_interval,
                        },
                        .stopwatch = .{ .max_seconds = 24 * 3600 },
                        .world_clock = .{ .timezone = default_timezone },
                    };
                    mode_enum = .COUNTDOWN_MODE;
                } else if (std.mem.eql(u8, mode_val.string, "stopwatch")) {
                    const max = if (cfg_val.object.get("max_seconds")) |v|
                        validator.safeU64FromJson(v.integer, 1, 86400 * 365) orelse 24 * 3600
                    else
                        24 * 3600;
                    config_union = .{
                        .countdown = .{ .duration_seconds = 25 * 60, .loop = false, .loop_count = 0, .loop_interval_seconds = 0 },
                        .stopwatch = .{ .max_seconds = max },
                        .world_clock = .{ .timezone = default_timezone },
                    };
                    mode_enum = .STOPWATCH_MODE;
                } else if (std.mem.eql(u8, mode_val.string, "world_clock")) {
                    const tz = if (cfg_val.object.get("timezone")) |v|
                        validator.safeI8FromJson(v.integer, -12, 14) orelse default_timezone
                    else
                        default_timezone;
                    config_union = .{
                        .countdown = .{ .duration_seconds = 25 * 60, .loop = false, .loop_count = 0, .loop_interval_seconds = 0 },
                        .stopwatch = .{ .max_seconds = 24 * 3600 },
                        .world_clock = .{ .timezone = tz },
                    };
                    mode_enum = .WORLD_CLOCK_MODE;
                } else {
                    continue;
                }

                const preset: interface.TimerPreset = .{
                    .name = name_val.string,
                    .mode = mode_enum,
                    .config = config_union,
                };

                // 使用 PresetsManager 来处理拷贝与去重
                presets.add(preset) catch |err| {
                    logger.global_logger.warn("⚠️ 添加预设失败: {any}", .{err});
                    continue;
                };
            }
        }
    }

    logger.global_logger.info("✓ 已反序列化 {} 个预设", .{presets.count()});
}
