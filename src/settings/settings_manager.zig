//! 设置管理模块 - 纯SQLite驱动版本
//! 移除TOML和JSON文件支持，所有数据存储在SQLite数据库中
const std = @import("std");
const interface = @import("../core/interface.zig");
const logger = @import("../core/logger.zig");
const validator = @import("settings_validator.zig");
const presets_mod = @import("settings_presets.zig");
const settings_sqlite = @import("../storage/storage_sqlite.zig");
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
const default_db_path = "little_timer.db"; // 统一使用一个数据库文件

pub const SettingsManager = struct {
    config: SettingsConfig,
    allocator: std.mem.Allocator,
    db_path: ?[]u8 = null, // SQLite 数据库文件路径
    sqlite_db: ?*settings_sqlite.SqliteManager = null, // SQLite 数据库管理器
    is_dirty: bool = false,
    /// 预设管理器（动态数组，最多999个预设）
    presets: PresetsManager,
    /// 动态分配的字符串字段（用于内存管理）
    owned_language: ?[]u8 = null,
    owned_theme_mode: ?[]u8 = null,
    owned_log_level: ?[]u8 = null,

    /// 设置模块初始化
    ///
    /// 参数:
    /// - **allocator**: 内存分配器
    /// - **db_path**: 数据库文件路径（可选，默认使用"little_timer.db"）
    ///
    /// 返回:
    /// - !SettingsManager: 如果初始化失败则返回错误
    pub fn init(allocator: std.mem.Allocator, db_path: []const u8) !SettingsManager {
        // 计算 SQLite 路径
        const db_path_str = if (db_path.len > 0) db_path else default_db_path;
        const db_path_len = db_path_str.len;
        const db_path_z_slice = try allocator.alloc(u8, db_path_len + 1);
        @memcpy(db_path_z_slice[0..db_path_len], db_path_str);
        db_path_z_slice[db_path_len] = 0;
        const db_path_full: [:0]const u8 = @ptrCast(db_path_z_slice[0..db_path_len :0]);

        var presets_mgr = PresetsManager.init(allocator);
        // 在堆上分配 SQLite 管理器，避免悬垂指针
        var sqlite_db_ptr = try allocator.create(settings_sqlite.SqliteManager);
        sqlite_db_ptr.* = try settings_sqlite.SqliteManager.init(allocator, db_path_full, "");
        try sqlite_db_ptr.open();
        // 预设从 SQLite 加载
        presets_mgr.loadFromSqlite(sqlite_db_ptr) catch |err| {
            logger.global_logger.warn("⚠️ 加载 SQLite 预设失败: {any}，将使用空预设列表", .{err});
        };
        presets_mgr.db = sqlite_db_ptr;

        const settings_manager = SettingsManager{
            .allocator = allocator,
            .db_path = db_path_z_slice,
            .sqlite_db = sqlite_db_ptr,
            .config = SettingsConfig{},
            .presets = presets_mgr,
        };
        return settings_manager;
    }

    /// 加载设置
    ///
    /// 参数:
    /// - **self**: SettingsManager实例指针
    ///
    /// 返回:
    /// - !void: 如果加载失败则返回错误
    pub fn load(self: *SettingsManager) !void {
        // 从 SQLite 加载设置
        try self.loadSettingsFromSqlite();

        self.is_dirty = false;
        logger.global_logger.info("✓ 设置已从 SQLite 加载", .{});
    }

    /// 保存设置到数据库
    ///
    /// 参数:
    /// - **self**: SettingsManager实例指针
    ///
    /// 返回:
    /// - !void: 如果保存失败则返回错误
    pub fn save(self: *SettingsManager) !void {
        // 保存设置到 SQLite
        try self.saveSettingsToSqlite();

        self.is_dirty = false;
        logger.global_logger.info("✓ 设置已保存到 SQLite", .{});
    }

    /// 从 SQLite 加载设置
    fn loadSettingsFromSqlite(self: *SettingsManager) !void {
        self.config = self.sqlite_db.?.*.loadSettings(self.allocator) catch |err| {
            logger.global_logger.err("从 SQLite 加载设置失败: {any}", .{err});
            try self.initializeDefaultSettings();
            return;
        };
        logger.global_logger.info("✓ 设置已从 SQLite 加载", .{});
    }

    /// 保存设置到 SQLite
    fn saveSettingsToSqlite(self: *SettingsManager) !void {
        self.sqlite_db.?.*.saveSettings(self.config) catch |err| {
            logger.global_logger.err("保存设置到 SQLite 失败: {any}", .{err});
            return;
        };
        logger.global_logger.info("✓ 设置已保存到 SQLite", .{});
    }

    /// 初始化默认设置
    fn initializeDefaultSettings(self: *SettingsManager) !void {
        // 释放旧的动态字符串
        if (self.owned_language) |old| self.allocator.free(old);
        if (self.owned_theme_mode) |old| self.allocator.free(old);
        if (self.owned_log_level) |old| self.allocator.free(old);

        // 创建默认设置
        self.owned_language = try self.allocator.dupe(u8, "ZH");
        self.owned_theme_mode = try self.allocator.dupe(u8, "dark");
        self.owned_log_level = try self.allocator.dupe(u8, "INFO");

        self.config = SettingsConfig{
            .basic = .{
                .timezone = 8,
                .language = self.owned_language.?,
                .default_mode = .countdown,
                .theme_mode = self.owned_theme_mode.?,
            },
            .clock_defaults = .{
                .countdown = .{
                    .duration_seconds = 1500,
                    .loop = false,
                    .loop_count = 0,
                    .loop_interval_seconds = 0,
                },
                .stopwatch = .{
                    .max_seconds = 86400,
                },
                .world_clock = .{
                    .timezone = 8,
                },
            },
            .logging = .{
                .level = self.owned_log_level.?,
                .enable_timestamp = true,
                .tick_interval_ms = 1000,
            },
        };

        // 保存到数据库
        try self.saveSettingsToSqlite();
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
        const storage_mod = @import("../storage/mod.zig");
        return storage_mod.toJsonAlloc(self.allocator, &self.config, self.presets);
    }

    pub fn handleSettingsEvent(self: *SettingsManager, e: interface.SettingsEvent) !void {
        logger.global_logger.debug("处理设置事件", .{});
        switch (e) {
            .change_settings => |new_settings_json| {
                logger.global_logger.info("收到设置变更，当前预设数量: {}", .{self.presets.count()});
                logger.global_logger.debug("JSON长度: {} 字节", .{new_settings_json.len});

                // 简化的JSON处理：只处理基本设置和预设
                try self.parseSettingsFromJson(new_settings_json);

                // 释放传入的 JSON 字符串内存（由调用者分配）
                self.allocator.free(new_settings_json);

                logger.global_logger.info("处理完成，当前预设数量: {}", .{self.presets.count()});

                // 保存到数据库
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

    /// 简化的JSON设置解析
    fn parseSettingsFromJson(self: *SettingsManager, json_str: []const u8) !void {
        logger.global_logger.debug("parseSettingsFromJson 收到 JSON: {s}", .{json_str});

        var parsed = try std.json.parseFromSlice(std.json.Value, self.allocator, json_str, .{});
        defer parsed.deinit();

        const root = parsed.value;

        // 更新基本设置
        if (root.object.get("basic")) |basic_val| {
            if (basic_val.object.get("timezone")) |tz_val| {
                if (tz_val.integer >= -12 and tz_val.integer <= 14) {
                    self.config.basic.timezone = @intCast(tz_val.integer);
                }
            }

            if (basic_val.object.get("language")) |lang_val| {
                if (lang_val.string.len > 0 and lang_val.string.len <= 10) {
                    if (self.owned_language) |old| self.allocator.free(old);
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
                if (self.owned_theme_mode) |old| self.allocator.free(old);
                self.owned_theme_mode = try self.allocator.dupe(u8, theme_val.string);
                self.config.basic.theme_mode = self.owned_theme_mode.?;
            }
        }

        // 更新时钟默认值
        if (root.object.get("clock_defaults")) |defaults_val| {
            if (defaults_val.object.get("countdown")) |countdown_val| {
                if (countdown_val.object.get("duration_seconds")) |dur_val| {
                    if (dur_val.integer >= 1 and dur_val.integer <= 86400) {
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
                    if (count_val.integer >= 0 and count_val.integer <= 1000) {
                        self.config.clock_defaults.countdown.loop_count = @intCast(count_val.integer);
                    }
                }
                if (countdown_val.object.get("loop_interval_seconds")) |interval_val| {
                    if (interval_val.integer >= 0 and interval_val.integer <= 3600) {
                        self.config.clock_defaults.countdown.loop_interval_seconds = @intCast(interval_val.integer);
                    }
                }
            }

            if (defaults_val.object.get("stopwatch")) |stopwatch_val| {
                if (stopwatch_val.object.get("max_seconds")) |max_val| {
                    if (max_val.integer > 0 and max_val.integer <= 86400 * 365) {
                        self.config.clock_defaults.stopwatch.max_seconds = @intCast(max_val.integer);
                    }
                }
            }

            if (defaults_val.object.get("world_clock")) |world_clock_val| {
                if (world_clock_val.object.get("timezone")) |tz_val| {
                    if (tz_val.integer >= -12 and tz_val.integer <= 14) {
                        self.config.clock_defaults.world_clock.timezone = @intCast(tz_val.integer);
                    }
                }
            }
        }

        // 更新日志设置
        if (root.object.get("logging")) |logging_val| {
            if (logging_val.object.get("level")) |level_val| {
                if (self.owned_log_level) |old| self.allocator.free(old);
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

        // 只有明确包含预设时才更新预设
        if (root.object.get("presets")) |presets_val| {
            if (presets_val == .array) {
                logger.global_logger.debug("收到预设数据: {} 个", .{presets_val.array.items.len});

                // 只有明确包含预设时才处理，避免意外清空
                if (presets_val.array.items.len > 0) {
                    // 清空现有预设
                    self.presets.clear();

                    // 解析新预设
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

                        logger.global_logger.debug("解析预设: name={s}, mode={s}", .{ name_val.string, mode_val.string });

                        // 支持大小写模式字符串：前端可能发送 "countdown" 或 "COUNTDOWN_MODE"
                        const is_countdown = std.mem.eql(u8, mode_val.string, "countdown") or
                            std.mem.eql(u8, mode_val.string, "COUNTDOWN_MODE");
                        const is_stopwatch = std.mem.eql(u8, mode_val.string, "stopwatch") or
                            std.mem.eql(u8, mode_val.string, "STOPWATCH_MODE");
                        const is_world_clock = std.mem.eql(u8, mode_val.string, "world_clock") or
                            std.mem.eql(u8, mode_val.string, "WORLD_CLOCK_MODE");

                        if (is_countdown) {
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
                        } else if (is_stopwatch) {
                            const max = if (cfg_val.object.get("max_seconds")) |v|
                                validator.safeU64FromJson(v.integer, 1, 86400 * 365) orelse 24 * 3600
                            else
                                24 * 3600;
                            config_union = .{ .countdown = .{ .duration_seconds = 25 * 60, .loop = false, .loop_count = 0, .loop_interval_seconds = 0 }, .stopwatch = .{ .max_seconds = max }, .world_clock = .{ .timezone = 8 } };
                            mode_enum = .STOPWATCH_MODE;
                        } else if (is_world_clock) {
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

                        self.presets.add(preset) catch |err| {
                            logger.global_logger.warn("⚠️ 添加预设失败: {any}", .{err});
                            continue;
                        };
                    }

                    logger.global_logger.info("✓ 预设更新完成，共 {} 个", .{self.presets.count()});
                } else {
                    logger.global_logger.debug("JSON中预设数组为空，保留现有预设", .{});
                }
            }
        }

        self.is_dirty = true;
        logger.global_logger.info("✓ 设置已更新", .{});
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

        // 保存到数据库
        try self.save();

        self.is_dirty = true;
        logger.global_logger.info("✅ 配置已重置为默认值", .{});
    }

    /// 备份损坏的配置文件（纯 SQLite 版本，保留日志）
    ///
    /// 参数:
    /// - **self**: SettingsManager实例指针
    ///
    /// 返回:
    /// - !void: 如果备份失败则返回错误
    pub fn backupCorruptedFile(_: *SettingsManager) !void {
        logger.global_logger.info("📦 SQLite 持久化无需文件备份，数据存储在 little_timer.db", .{});
    }

    /// 清理设置管理器资源
    ///
    /// 参数:
    /// - **self**: SettingsManager实例指针
    pub fn deinit(self: *SettingsManager) void {
        // 清理预设管理器（会关闭 SQLite 数据库）
        self.presets.deinit();

        // 释放路径字符串
        self.allocator.free(self.presets_file_path);
        if (self.db_path) |path| self.allocator.free(path);

        // 释放动态分配的字符串字段
        if (self.owned_language) |s| self.allocator.free(s);
        if (self.owned_theme_mode) |s| self.allocator.free(s);
        if (self.owned_log_level) |s| self.allocator.free(s);

        // 释放堆上分配的 sqlite_db
        if (self.sqlite_db != null) {
            self.allocator.destroy(self.sqlite_db.?);
        }
    }

    /// 将预设保存到单独的 JSON 文件（与 settings.toml 同目录）
    pub fn savePresetsToFile(self: *SettingsManager) !void {
        try self.presets.saveToFile(self.presets_file_path);
    }

    /// 从预设文件加载预设（与 settings.toml 同目录）
    pub fn loadPresetsFromFile(self: *SettingsManager) !void {
        try self.presets.loadFromFile(self.presets_file_path, self.config.basic.timezone);
    }

    pub fn getPresets(self: *const SettingsManager) []const interface.TimerPreset {
        return self.presets.getAll();
    }
};
