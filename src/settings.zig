//! 设置管理模块
const std = @import("std");
const toml = @import("toml");
const interface = @import("interface.zig");
const logger = @import("logger.zig");
const validator = @import("settings_validator.zig");
const presets_mod = @import("settings_presets.zig");
pub const SettingsConfig = interface.SettingsConfig;
const fs = std.fs;

// 重新导出子模块类型，方便外部使用
pub const ValidationError = validator.ValidationError;
pub const PresetsError = presets_mod.PresetsError;
pub const PresetsManager = presets_mod.PresetsManager;
// 为基本设置创建类型别名以便使用
pub const BasicConfig = struct {
    timezone: i8,
    language: []const u8, // 改为字符串切片
    default_mode: interface.DefaultMode,
};
const default_settings_path = "settings.toml";

pub const SettingsManager = struct {
    config: SettingsConfig,
    allocator: std.mem.Allocator,
    file_path: []const u8,
    presets_file_path: []u8, // 预设文件完整路径（动态分配）
    is_dirty: bool = false,
    /// 预设管理器（动态数组，最多999个预设）
    presets: PresetsManager,
    /// 动态分配的字符串字段（用于 JSON 解析后的内存管理）
    owned_language: ?[]u8 = null,
    owned_theme_mode: ?[]u8 = null,
    owned_log_level: ?[]u8 = null,

    /// 设置模块初始化
    ///
    /// 参数:
    /// - **allocator**: 内存分配器
    /// - **file_path**: 设置文件路径
    ///
    /// 返回:
    /// - !SettingsManager: 如果初始化失败则返回错误
    pub fn init(allocator: std.mem.Allocator, file_path: []const u8) !SettingsManager {
        // 计算预设文件路径：与 settings.toml 同目录，名为 presets.json
        const dir_path = fs.path.dirname(file_path) orelse ".";
        const presets_path = try fs.path.join(allocator, &[_][]const u8{ dir_path, "presets.json" });

        const settings_manager = SettingsManager{
            .allocator = allocator,
            .file_path = file_path,
            .presets_file_path = presets_path,
            .config = SettingsConfig{},
            .presets = PresetsManager.init(allocator),
        };
        return settings_manager;
    }

    /// 加载设置
    ///
    /// 参数:
    /// - **self**: SettingsManager实例指针
    ///
    /// 返回:
    /// - !void: 如果加载失败则返回错误（文件不存在、解析失败等）
    pub fn load(self: *SettingsManager) !void {
        const file = fs.cwd().openFile(self.file_path, .{}) catch |err| {
            logger.global_logger.err("无法打开设置文件 {s}: {}", .{ self.file_path, err });
            return err;
        };
        defer file.close();

        const file_size = try file.getEndPos();
        const buffer = try self.allocator.alloc(u8, @intCast(file_size));
        defer self.allocator.free(buffer);

        const bytes_read = try file.readAll(buffer);
        if (bytes_read != file_size) {
            logger.global_logger.err("读取设置文件不完整", .{});
            return error.Incomplete;
        }

        var toml_parser = toml.Parser(SettingsConfig).init(self.allocator);
        defer toml_parser.deinit();

        var result = toml_parser.parseString(buffer) catch |err| {
            logger.global_logger.err("解析TOML失败: {}", .{err});
            return err;
        };
        defer result.deinit();

        self.config = result.value;

        // 重要！复制 TOML 中的字符串字段，防止 result.deinit() 后内存失效
        // 释放旧的动态字符串
        if (self.owned_language) |old| self.allocator.free(old);
        if (self.owned_theme_mode) |old| self.allocator.free(old);
        if (self.owned_log_level) |old| self.allocator.free(old);

        // 复制新的字符串
        self.owned_language = try self.allocator.dupe(u8, result.value.basic.language);
        self.owned_theme_mode = try self.allocator.dupe(u8, result.value.basic.theme_mode);
        self.owned_log_level = try self.allocator.dupe(u8, result.value.logging.level);

        // 更新 config 中的指针指向我们的副本
        self.config.basic.language = self.owned_language.?;
        self.config.basic.theme_mode = self.owned_theme_mode.?;
        self.config.logging.level = self.owned_log_level.?;

        self.is_dirty = false;
        logger.global_logger.info("✓ 设置文件加载成功", .{});

        // 尝试加载预设文件（独立 JSON 持久化）
        self.presets.loadFromFile(self.presets_file_path, self.config.basic.timezone) catch |err| {
            logger.global_logger.warn("加载预设失败（将忽略）：{}", .{err});
        };
    }

    /// 保存设置到文件
    ///
    /// 参数:
    /// - **self**: SettingsManager实例指针
    ///
    /// 返回:
    /// - !void: 如果保存失败则返回错误
    pub fn save(self: *SettingsManager) !void {
        if (self.file_path.len == 0) {
            logger.global_logger.err("错误: 未指定设置文件路径", .{});
            return error.NoFilePath;
        }

        const file = fs.cwd().createFile(self.file_path, .{ .truncate = true }) catch |err| {
            logger.global_logger.err("无法创建设置文件 {s}: {}", .{ self.file_path, err });
            return err;
        };
        defer file.close();

        // 直接序列化配置（预设已经单独存储在 JSON 文件）
        // 使用动态缓冲区收集 TOML 内容
        var toml_buffer = std.ArrayList(u8){};
        defer toml_buffer.deinit(self.allocator);

        // 预分配合理的初始容量
        try toml_buffer.ensureTotalCapacity(self.allocator, 2048);

        // 使用固定大小缓冲区进行序列化（TOML 库要求）
        var temp_buf: [4096]u8 = undefined;
        var io_writer = std.Io.Writer.fixed(&temp_buf);
        try toml.serialize(self.allocator, self.config, &io_writer);

        // 将序列化结果追加到 ArrayList
        try toml_buffer.appendSlice(self.allocator, io_writer.buffered());

        // 写入文件
        _ = try file.writeAll(toml_buffer.items);

        self.is_dirty = false;
        logger.global_logger.info("✓ 设置文件保存成功", .{});

        // 单独保存预设 JSON
        self.presets.saveToFile(self.presets_file_path) catch |err| {
            logger.global_logger.err("保存预设失败: {}", .{err});
        };
    }

    /// 获取当前配置
    ///
    /// 参数:
    /// - **self**: SettingsManager实例指针
    ///
    /// 返回:
    /// - *const SettingsConfig: 当前配置的只读指针
    pub fn getConfig(self: *const SettingsManager) *const SettingsConfig {
        return &self.config;
    }

    /// 更新基本设置
    ///
    /// 参数:
    /// - **self**: SettingsManager实例指针
    /// - **basic_config**: 新的基本配置
    ///
    /// 返回:
    /// - ValidationError!void: 如果参数无效则返回错误
    pub fn updateBasic(self: *SettingsManager, basic_config: BasicConfig) ValidationError!void {
        // 使用 validator 模块进行验证
        try validator.validateTimezone(basic_config.timezone);
        try validator.validateLanguage(basic_config.language);

        self.config.basic.timezone = basic_config.timezone;
        self.config.basic.language = basic_config.language;
        self.config.basic.default_mode = basic_config.default_mode;
        self.is_dirty = true;

        logger.global_logger.info("✓ 基本设置已更新: 时区={}, 语言={s}, 默认模式={}", .{ basic_config.timezone, basic_config.language, basic_config.default_mode });
    }

    /// 添加预设
    ///
    /// 参数:
    /// - **self**: SettingsManager实例指针
    /// - **preset**: 要添加的计时器预设
    ///
    /// 返回:
    /// - !void: 如果预设名称无效或已存在则返回错误
    pub fn addPreset(self: *SettingsManager, preset: interface.TimerPreset) !void {
        try self.presets.add(preset);
        self.is_dirty = true;
    }

    /// 根据默认模式构建时钟配置
    ///
    /// 参数:
    /// - **self**: SettingsManager实例指针
    ///
    /// 返回:
    /// - interface.ClockTaskConfig: 根据设置构建的时钟配置
    pub fn buildClockConfig(self: *const SettingsManager) interface.ClockTaskConfig {
        const config = &self.config;

        // 转换 DefaultMode 到 ModeEnumT
        const mode = switch (config.basic.default_mode) {
            .countdown => interface.ModeEnumT.COUNTDOWN_MODE,
            .stopwatch => interface.ModeEnumT.STOPWATCH_MODE,
            .world_clock => interface.ModeEnumT.WORLD_CLOCK_MODE,
        };

        return interface.ClockTaskConfig{
            .default_mode = mode,
            .countdown = config.clock_defaults.countdown,
            .stopwatch = config.clock_defaults.stopwatch,
            .world_clock = .{ .timezone = config.basic.timezone },
        };
    }

    /// 将设置转换为 JSON 字符串（动态分配）
    ///
    /// 参数:
    /// - **self**: SettingsManager实例
    ///
    /// 返回:
    /// - ![]u8: 动态分配的 JSON 字符串（调用者需要通过 allocator.free() 释放）
    pub fn toJsonAlloc(self: *const SettingsManager) ![]u8 {
        const mode_str = switch (self.config.basic.default_mode) {
            .countdown => "countdown",
            .stopwatch => "stopwatch",
            .world_clock => "world_clock",
        };

        // 使用 ArrayList 动态构建 JSON，根据内容大小自动扩容
        var json_list = std.ArrayList(u8){};
        defer json_list.deinit(self.allocator);
        const w = json_list.writer(self.allocator);

        // 开始 JSON 对象
        try w.writeAll("{\"basic\":{\"timezone\":");
        try w.print("{}", .{self.config.basic.timezone});
        try w.writeAll(",\"language\":\"");
        try w.writeAll(self.config.basic.language);
        try w.writeAll("\",\"default_mode\":\"");
        try w.writeAll(mode_str);
        try w.writeAll("\",\"theme_mode\":\"");
        try w.writeAll(self.config.basic.theme_mode);
        try w.writeAll("\"},\"clock_defaults\":{\"countdown\":{\"duration_seconds\":");
        try w.print("{}", .{self.config.clock_defaults.countdown.duration_seconds});
        try w.writeAll(",\"loop\":");
        try w.print("{}", .{@intFromBool(self.config.clock_defaults.countdown.loop)});
        try w.writeAll(",\"loop_count\":");
        try w.print("{}", .{self.config.clock_defaults.countdown.loop_count});
        try w.writeAll(",\"loop_interval_seconds\":");
        try w.print("{}", .{self.config.clock_defaults.countdown.loop_interval_seconds});
        try w.writeAll("},\"stopwatch\":{\"max_seconds\":");
        try w.print("{}", .{self.config.clock_defaults.stopwatch.max_seconds});
        try w.writeAll("}},\"logging\":{\"level\":\"");
        try w.writeAll(self.config.logging.level);
        try w.writeAll("\",\"enable_timestamp\":");
        try w.print("{}", .{@intFromBool(self.config.logging.enable_timestamp)});
        try w.writeAll(",\"tick_interval_ms\":");
        try w.print("{}", .{self.config.logging.tick_interval_ms});

        // 添加预设序列化（使用动态预设列表）
        try w.writeAll("},\"presets\":[");
        const preset_items = self.presets.presets.items;
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
        return try json_list.toOwnedSlice(self.allocator);
    }

    /// 从 JSON 字符串解析并更新设置
    ///
    /// 参数:
    /// - **self**: SettingsManager实例指针
    /// - **json**: JSON 字符串
    ///
    /// 返回:
    /// - !void: 如果解析失败则返回错误
    pub fn jsonToSettings(self: *SettingsManager, json_str: []const u8) !void {
        var parsed = try std.json.parseFromSlice(std.json.Value, self.allocator, json_str, .{});
        defer parsed.deinit();

        const root = parsed.value;

        // 解析 basic 部分
        if (root.object.get("basic")) |basic_val| {
            if (basic_val.object.get("timezone")) |tz_val| {
                if (tz_val.integer >= -12 and tz_val.integer <= 14) {
                    self.config.basic.timezone = @intCast(tz_val.integer);
                } else {
                    // 问题7：时区验证不完整 - 添加日志
                    logger.global_logger.warn("⚠️ JSON中时区超出范围 [-12, 14]，当前值: {d}，保持旧值: {}", .{ tz_val.integer, self.config.basic.timezone });
                }
            }

            if (basic_val.object.get("language")) |lang_val| {
                // 问题6：验证语言代码长度
                if (lang_val.string.len == 0 or lang_val.string.len > 10) {
                    logger.global_logger.warn("⚠️ JSON中语言代码长度无效: {}, 保持旧值: {s}", .{ lang_val.string.len, self.config.basic.language });
                } else {
                    // 释放旧的动态分配的字符串
                    if (self.owned_language) |old| {
                        self.allocator.free(old);
                    }
                    // 复制新字符串
                    self.owned_language = try self.allocator.dupe(u8, lang_val.string);
                    self.config.basic.language = self.owned_language.?;
                }
            }

            if (basic_val.object.get("default_mode")) |mode_val| {
                if (std.mem.eql(u8, mode_val.string, "countdown")) {
                    self.config.basic.default_mode = .countdown;
                } else if (std.mem.eql(u8, mode_val.string, "stopwatch")) {
                    self.config.basic.default_mode = .stopwatch;
                } else if (std.mem.eql(u8, mode_val.string, "world_clock")) {
                    self.config.basic.default_mode = .world_clock;
                }
            }

            if (basic_val.object.get("theme_mode")) |theme_val| {
                // 释放旧的动态分配的字符串
                if (self.owned_theme_mode) |old| {
                    self.allocator.free(old);
                }
                // 复制新字符串
                self.owned_theme_mode = try self.allocator.dupe(u8, theme_val.string);
                self.config.basic.theme_mode = self.owned_theme_mode.?;
            }
        }

        // 解析 clock_defaults 部分
        if (root.object.get("clock_defaults")) |defaults_val| {
            if (defaults_val.object.get("countdown")) |countdown_val| {
                if (countdown_val.object.get("duration_seconds")) |dur_val| {
                    // 问题4：@intCast 未检查溢出 - 验证范围
                    if (dur_val.integer < 1 or dur_val.integer > 86400) { // 最多24小时
                        logger.global_logger.warn("⚠️ JSON中倒计时时长超出范围 [1, 86400], 当前值: {d}，保持旧值: {}", .{ dur_val.integer, self.config.clock_defaults.countdown.duration_seconds });
                    } else {
                        self.config.clock_defaults.countdown.duration_seconds = @intCast(dur_val.integer);
                    }
                }
                if (countdown_val.object.get("loop")) |loop_val| {
                    self.config.clock_defaults.countdown.loop = switch (loop_val) {
                        .bool => |b| b,
                        .integer => |i| i != 0,
                        else => false,
                    };
                }
                if (countdown_val.object.get("loop_count")) |count_val| {
                    // 问题4：@intCast 未检查溢出 - 验证范围
                    if (count_val.integer < 0 or count_val.integer > 1000) { // 最多1000次循环
                        logger.global_logger.warn("⚠️ JSON中循环次数超出范围 [0, 1000], 当前值: {d}，保持旧值: {}", .{ count_val.integer, self.config.clock_defaults.countdown.loop_count });
                    } else {
                        self.config.clock_defaults.countdown.loop_count = @intCast(count_val.integer);
                    }
                }
                if (countdown_val.object.get("loop_interval_seconds")) |interval_val| {
                    // 问题4：@intCast 未检查溢出 - 验证范围
                    if (interval_val.integer < 0 or interval_val.integer > 3600) { // 最多1小时休息
                        logger.global_logger.warn("⚠️ JSON中循环间隔超出范围 [0, 3600], 当前值: {d}，保持旧值: {}", .{ interval_val.integer, self.config.clock_defaults.countdown.loop_interval_seconds });
                    } else {
                        self.config.clock_defaults.countdown.loop_interval_seconds = @intCast(interval_val.integer);
                    }
                }
            }

            if (defaults_val.object.get("stopwatch")) |stopwatch_val| {
                if (stopwatch_val.object.get("max_seconds")) |max_val| {
                    // 问题4：@intCast 未检查溢出 - 验证范围
                    if (max_val.integer <= 0 or max_val.integer > 86400 * 365) { // 最多365天
                        logger.global_logger.warn("⚠️ JSON中正计时上限超出范围 (0, 31536000], 当前值: {d}，保持旧值: {}", .{ max_val.integer, self.config.clock_defaults.stopwatch.max_seconds });
                    } else {
                        self.config.clock_defaults.stopwatch.max_seconds = @intCast(max_val.integer);
                    }
                }
            }
        }

        // 解析 logging 部分
        if (root.object.get("logging")) |logging_val| {
            if (logging_val.object.get("level")) |level_val| {
                // 释放旧的动态分配的字符串
                if (self.owned_log_level) |old| {
                    self.allocator.free(old);
                }
                // 复制新字符串
                self.owned_log_level = try self.allocator.dupe(u8, level_val.string);
                self.config.logging.level = self.owned_log_level.?;
            }

            if (logging_val.object.get("enable_timestamp")) |ts_val| {
                self.config.logging.enable_timestamp = switch (ts_val) {
                    .bool => |b| b,
                    .integer => |i| i != 0,
                    else => false,
                };
            }

            if (logging_val.object.get("tick_interval_ms")) |interval_val| {
                if (interval_val.integer > 0) {
                    self.config.logging.tick_interval_ms = @intCast(interval_val.integer);
                }
            }
        }

        // 解析 presets（可选）
        if (root.object.get("presets")) |presets_val| {
            if (presets_val == .array) {
                // 清空现有预设
                self.presets.clear();

                for (presets_val.array.items) |pval| {
                    if (self.presets.count() >= self.presets.max_count) break;
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
                            validator.safeI8FromJson(v.integer, -12, 14) orelse self.config.basic.timezone
                        else
                            self.config.basic.timezone;
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
                    self.presets.add(preset) catch |err| {
                        logger.global_logger.warn("⚠️ 添加预设失败: {any}", .{err});
                        continue;
                    };
                }
            }
        }

        self.is_dirty = true;
        logger.global_logger.info("✓ 设置已从 JSON 更新", .{});
    }

    pub fn handleSettingsEvent(self: *SettingsManager, e: interface.SettingsEvent) !void {
        logger.global_logger.debug("处理设置事件", .{});
        switch (e) {
            .change_settings => |new_settings_json| {
                // 先处理 JSON 解析
                try self.jsonToSettings(new_settings_json);

                // 释放传入的 JSON 字符串内存（由调用者分配）
                self.allocator.free(new_settings_json);

                // 最后保存配置
                try self.save();
            },
            .get_settings => |buffer| {
                // 如果 buffer 长度为 0，说明调用者已在外部直接调用 toJsonAlloc()
                // 这种情况下不需要做任何处理（避免整数下溢）
                if (buffer.len == 0) {
                    logger.global_logger.debug("get_settings: 空缓冲区，跳过", .{});
                    return;
                }

                // 动态分配 JSON，然后复制到缓冲区
                const json_str = try self.toJsonAlloc();
                defer self.allocator.free(json_str);

                if (json_str.len > buffer.len - 1) {
                    logger.global_logger.err("❌ 缓冲区太小: 需要 {} 字节，但只有 {}", .{ json_str.len, buffer.len });
                    return error.BufferTooSmall;
                }

                @memcpy(buffer[0..json_str.len], json_str);
                buffer[json_str.len] = 0; // 添加 null 终止符
                logger.global_logger.debug("✓ 设置 JSON 已写入缓冲区，长度: {}", .{json_str.len});
            },
        }
    }

    /// 重置为默认配置（用于配置文件损坏时恢复）
    ///
    /// 参数:
    /// - **self**: SettingsManager实例指针
    ///
    /// 返回:
    /// - !void: 如果内存分配失败则返回错误
    pub fn resetToDefaults(self: *SettingsManager) !void {
        // 释放旧的动态字符串
        if (self.owned_language) |old| self.allocator.free(old);
        if (self.owned_theme_mode) |old| self.allocator.free(old);
        if (self.owned_log_level) |old| self.allocator.free(old);

        // 清空预设
        self.presets.clear();

        // 重置为默认配置（使用 SettingsConfig 的默认初始化值）
        self.config = SettingsConfig{};

        // 复制新的默认字符串
        self.owned_language = try self.allocator.dupe(u8, self.config.basic.language);
        self.owned_theme_mode = try self.allocator.dupe(u8, self.config.basic.theme_mode);
        self.owned_log_level = try self.allocator.dupe(u8, self.config.logging.level);

        // 更新指针指向我们的副本
        self.config.basic.language = self.owned_language.?;
        self.config.basic.theme_mode = self.owned_theme_mode.?;
        self.config.logging.level = self.owned_log_level.?;

        self.is_dirty = true;
        logger.global_logger.info("✅ 配置已重置为默认值", .{});
    }

    /// 备份损坏的配置文件（带时间戳）
    ///
    /// 参数:
    /// - **self**: SettingsManager实例指针
    ///
    /// 返回:
    /// - !void: 如果备份失败则返回错误
    pub fn backupCorruptedFile(self: *SettingsManager) !void {
        // 检查文件是否存在
        fs.cwd().access(self.file_path, .{}) catch {
            logger.global_logger.debug("配置文件不存在，无需备份", .{});
            return; // 文件不存在，无需备份
        };

        // 生成带时间戳的备份文件名
        const timestamp = std.time.timestamp();
        var backup_buf: [512]u8 = undefined;
        const backup_path = try std.fmt.bufPrint(&backup_buf, "{s}.corrupted.{d}", .{
            self.file_path,
            timestamp,
        });

        // 复制文件
        try fs.cwd().copyFile(self.file_path, fs.cwd(), backup_path, .{});
        logger.global_logger.info("📦 已备份损坏的配置文件到: {s}", .{backup_path});
    }

    /// 清理设置管理器资源
    ///
    /// 参数:
    /// - **self**: SettingsManager实例指针
    pub fn deinit(self: *SettingsManager) void {
        // 清理预设管理器
        self.presets.deinit();
        // 释放预设文件路径
        self.allocator.free(self.presets_file_path);

        // 释放动态分配的字符串字段
        if (self.owned_language) |s| self.allocator.free(s);
        if (self.owned_theme_mode) |s| self.allocator.free(s);
        if (self.owned_log_level) |s| self.allocator.free(s);
    }

    /// 将预设保存到单独的 JSON 文件（与 settings.toml 同目录）
    pub fn savePresetsToFile(self: *SettingsManager) !void {
        try self.presets.saveToFile(self.presets_file_path);
    }

    /// 从预设文件加载预设（与 settings.toml 同目录）
    pub fn loadPresetsFromFile(self: *SettingsManager) !void {
        try self.presets.loadFromFile(self.presets_file_path, self.config.basic.timezone);
    }
};
