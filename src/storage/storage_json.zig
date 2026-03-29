//! 设置 JSON 序列化/反序列化模块
//! 职责：处理 JSON 与 SettingsConfig 之间的转换，与后端/前端通信
const std = @import("std");
const interface = @import("../core/interface.zig");
const logger = @import("../core/logger.zig");
const validator = @import("../settings/settings_validator.zig");

pub const SettingsConfig = interface.SettingsConfig;

/// JSON 合法性错误类型
pub const JsonError = error{
    InvalidJson, // 无效的 JSON 格式
    InvalidDuration, // 无效的倒计时时长
    InvalidLoopCount, // 无效的循环次数
    InvalidLoopInterval, // 无效的循环间隔
    InvalidMaxSeconds, // 无效的秒表上限
    InvalidTimezone, // 无效的时区
    UnknownMode, // 未知的模式
};

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

/// 使用 std.json.stringify 直接序列化 SettingsConfig 及预设
///
/// 参数:
/// - **allocator**: 内存分配器
/// - **config**: 要序列化的设置配置
/// - **presets**: 预设数组（PresetsManager 或兼容类型，需含 presets 字段）
///
/// 返回:
/// - ![]u8: 动态分配的 JSON 字符串（调用者需要通过 allocator.free() 释放）
pub fn toJsonAlloc(
    allocator: std.mem.Allocator,
    config: *const SettingsConfig,
    presets: anytype,
) ![]u8 {
    var json_list = std.ArrayList(u8){};
    defer json_list.deinit(allocator);
    const w = json_list.writer(allocator);

    try w.writeAll("{");

    // 序列化 basic
    try w.writeAll("\"basic\":{");
    try w.print("\"timezone\":{},\"language\":\"{s}\",\"default_mode\":\"{s}\",\"theme_mode\":\"{s}\"", .{
        config.basic.timezone,
        config.basic.language,
        @tagName(config.basic.default_mode),
        config.basic.theme_mode,
    });
    try w.writeAll("},");

    // 序列化 clock_defaults（使用前端期望的格式）
    try w.writeAll("\"clock_defaults\":{");
    try w.writeAll("\"countdown\":{");
    try w.print("\"duration_seconds\":{},\"loop\":{},\"loop_count\":{},\"loop_interval_seconds\":{}", .{
        config.clock_defaults.countdown.duration_seconds,
        @intFromBool(config.clock_defaults.countdown.loop),
        config.clock_defaults.countdown.loop_count,
        config.clock_defaults.countdown.loop_interval_seconds,
    });
    try w.writeAll("},");
    try w.writeAll("\"stopwatch\":{");
    try w.print("\"max_seconds\":{}", .{config.clock_defaults.stopwatch.max_seconds});
    try w.writeAll("}},");

    // 序列化 logging
    try w.writeAll("\"logging\":{");
    try w.print("\"level\":\"{s}\",\"enable_timestamp\":{},\"tick_interval_ms\":{}", .{
        config.logging.level,
        @intFromBool(config.logging.enable_timestamp),
        config.logging.tick_interval_ms,
    });
    try w.writeAll("},");

    // 序列化 presets（使用前端期望的格式）
    try w.writeAll("\"presets\":[");
    const preset_items = presets.presets.items;
    for (preset_items, 0..) |preset, i| {
        if (i > 0) try w.writeByte(',');
        try w.writeAll("{\"name\":");
        try writeEscapedJsonString(w, preset.name);
        try w.writeAll(",\"mode\":");

        const mode_str = switch (preset.mode) {
            .COUNTDOWN_MODE => "countdown",
            .STOPWATCH_MODE => "stopwatch",
        };
        try w.print("\"{s}\",\"config\":", .{mode_str});

        // 根据模式写入对应的配置字段
        switch (preset.mode) {
            .COUNTDOWN_MODE => {
                try w.print("{{\"duration_seconds\":{},\"loop\":{},\"loop_count\":{},\"loop_interval_seconds\":{}}}", .{
                    preset.config.countdown.duration_seconds,
                    @intFromBool(preset.config.countdown.loop),
                    preset.config.countdown.loop_count,
                    preset.config.countdown.loop_interval_seconds,
                });
            },
            .STOPWATCH_MODE => {
                try w.print("{{\"max_seconds\":{}}}", .{preset.config.stopwatch.max_seconds});
            },
        }
        try w.writeAll("}"); // 关闭 preset 对象
    }
    try w.writeAll("]}"); // 关闭 presets 数组和整个对象

    return try json_list.toOwnedSlice(allocator);
}

/// 使用 std.json.parseFromSlice 直接反序列化 SettingsConfig 及预设
///
/// 参数:
/// - **allocator**: 内存分配器
/// - **config**: 要更新的设置配置指针
/// - **presets**: 要更新的预设管理器（需有 .add/.clear 方法）
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
    presets: anytype,
    owned_language: *?[]u8,
    owned_theme_mode: *?[]u8,
    owned_log_level: *?[]u8,
    json_str: []const u8,
) !void {
    // 定义临时 struct 以便反序列化
    const In = struct {
        basic: @TypeOf(config.basic),
        clock_defaults: @TypeOf(config.clock_defaults),
        logging: @TypeOf(config.logging),
        presets: []const interface.TimerPreset,
    };
    var parsed = try std.json.parseFromSlice(In, allocator, json_str, .{});
    defer parsed.deinit();
    config.basic = parsed.value.basic;
    config.clock_defaults = parsed.value.clock_defaults;
    config.logging = parsed.value.logging;
    // 处理动态分配的字符串字段（language/theme_mode/level）
    if (owned_language.*) |old| allocator.free(old);
    owned_language.* = try allocator.dupe(u8, config.basic.language);
    config.basic.language = owned_language.*.?;
    if (owned_theme_mode.*) |old| allocator.free(old);
    owned_theme_mode.* = try allocator.dupe(u8, config.basic.theme_mode);
    config.basic.theme_mode = owned_theme_mode.*.?;
    if (owned_log_level.*) |old| allocator.free(old);
    owned_log_level.* = try allocator.dupe(u8, config.logging.level);
    config.logging.level = owned_log_level.*.?;
    // 预设同步
    presets.clear();
    for (parsed.value.presets) |preset| {
        presets.add(preset) catch |err| {
            logger.global_logger.warn("⚠️ 添加预设失败: {any}", .{err});
            continue;
        };
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
    _: i8,
) !void {
    var parsed = try std.json.parseFromSlice(std.json.Value, allocator, json_str, .{});
    defer parsed.deinit();

    const root = parsed.value;
    if (root.object.get("presets")) |presets_val| {
        if (presets_val == .array) {
            // 只有在JSON中包含有效预设数据时才清空现有预设
            if (presets_val.array.items.len > 0) {
                presets.clear();
            } else {
                // 如果JSON预设数组为空，直接返回，保留现有预设
                return;
            }

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
                    };
                    mode_enum = .STOPWATCH_MODE;
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
/// 仅序列化单个预设配置为 JSON（用于 SQLite 持久化）
///
/// 参数:
/// - **allocator**: 内存分配器
/// - **mode**: 预设模式
/// - **config**: 预设配置
///
/// 返回:
/// - ![]u8: 动态分配的 JSON 字符串
pub fn serializePresetConfigOnly(
    allocator: std.mem.Allocator,
    mode: interface.ModeEnumT,
    config: interface.ClockTaskConfig,
) ![]u8 {
    var json_list = std.ArrayList(u8){};
    defer json_list.deinit(allocator);
    const w = json_list.writer(allocator);

    switch (mode) {
        .COUNTDOWN_MODE => {
            try w.print(
                \\{{"mode":"countdown","duration_seconds":{},"loop":{},"loop_count":{},"loop_interval_seconds":{}}}
            , .{
                config.countdown.duration_seconds,
                @intFromBool(config.countdown.loop),
                config.countdown.loop_count,
                config.countdown.loop_interval_seconds,
            });
        },
        .STOPWATCH_MODE => {
            try w.print(
                \\{{"mode":"stopwatch","max_seconds":{}}}
            , .{config.stopwatch.max_seconds});
        },
    }

    return try json_list.toOwnedSlice(allocator);
}

/// 反序列化单个预设配置从 JSON（用于 SQLite 读取）
///
/// 参数:
/// - **allocator**: 内存分配器
/// - **json_str**: JSON 字符串
///
/// 返回:
/// - !interface.ClockTaskConfig: 解析后的配置
pub fn parsePresetConfigJson(allocator: std.mem.Allocator, json_str: []const u8) !interface.ClockTaskConfig {
    var parsed = try std.json.parseFromSlice(std.json.Value, allocator, json_str, .{});
    defer parsed.deinit();

    const root = parsed.value;
    if (root != .object) {
        logger.global_logger.err("❌ 预设配置 JSON 根节点必须是对象", .{});
        return error.InvalidJson;
    }

    const mode_val = root.object.get("mode") orelse {
        logger.global_logger.err("❌ 预设配置 JSON 缺少 mode 字段", .{});
        return error.InvalidJson;
    };

    if (mode_val != .string) {
        logger.global_logger.err("❌ 预设配置 JSON mode 字段必须是字符串", .{});
        return error.InvalidJson;
    }

    if (std.mem.eql(u8, mode_val.string, "countdown")) {
        const duration = (root.object.get("duration_seconds") orelse return error.InvalidJson).integer;
        const loop_raw = (root.object.get("loop") orelse return error.InvalidJson).integer;
        const loop_count = (root.object.get("loop_count") orelse return error.InvalidJson).integer;
        const loop_interval = (root.object.get("loop_interval_seconds") orelse return error.InvalidJson).integer;

        // 合法性校验
        if (duration < 1 or duration > 86400) {
            logger.global_logger.warn("⚠️ 预设 JSON 中倒计时时长超出范围: {d}", .{duration});
            return error.InvalidDuration;
        }
        if (loop_count < 0 or loop_count > 1000) {
            logger.global_logger.warn("⚠️ 预设 JSON 中循环次数超出范围: {d}", .{loop_count});
            return error.InvalidLoopCount;
        }
        if (loop_interval < 0 or loop_interval > 3600) {
            logger.global_logger.warn("⚠️ 预设 JSON 中循环间隔超出范围: {d}", .{loop_interval});
            return error.InvalidLoopInterval;
        }

        return .{
            .default_mode = .COUNTDOWN_MODE,
            .countdown = .{
                .duration_seconds = @intCast(duration),
                .loop = loop_raw != 0,
                .loop_count = @intCast(loop_count),
                .loop_interval_seconds = @intCast(loop_interval),
            },
            .stopwatch = .{ .max_seconds = 24 * 3600 },
        };
    } else if (std.mem.eql(u8, mode_val.string, "stopwatch")) {
        const max_sec = (root.object.get("max_seconds") orelse return error.InvalidJson).integer;

        if (max_sec <= 0 or max_sec > 86400 * 365) {
            logger.global_logger.warn("⚠️ 预设 JSON 中正计时上限超出范围: {d}", .{max_sec});
            return error.InvalidMaxSeconds;
        }

        return .{
            .default_mode = .STOPWATCH_MODE,
            .countdown = .{ .duration_seconds = 25 * 60, .loop = false, .loop_count = 0, .loop_interval_seconds = 0 },
            .stopwatch = .{ .max_seconds = @intCast(max_sec) },
        };
    } else {
        logger.global_logger.err("❌ 预设配置 JSON 中未知的模式: {s}", .{mode_val.string});
        return error.UnknownMode;
    }
}
