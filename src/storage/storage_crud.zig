//! 数据库CRUD操作模块
//! 职责：预设和设置的具体增删改查操作
const std = @import("std");
const zqlite = @import("zqlite");
const interface = @import("../core/interface.zig");
const logger = @import("../core/logger.zig");
const validator = @import("../settings/settings_validator.zig");

/// CRUD 操作错误类型
pub const CrudError = error{
    InsertFailed, // 插入失败
    DeleteFailed, // 删除失败
    QueryFailed, // 查询失败
    InvalidPresetData, // 无效的预设数据
    SettingsNotFound, // 设置未找到
    SettingsSaveFailed, // 设置保存失败
    DatabaseOpenFailed, // 数据库打开失败
};

/// SQLite 查询结果行
pub const PresetRow = struct {
    id: i64,
    name: []const u8,
    mode: interface.ModeEnumT,
    config_json: []const u8,
};

/// SQLite 设置行数据
pub const SettingsRow = struct {
    id: i64 = 1,
    timezone: i8,
    language: []const u8,
    default_mode: []const u8,
    theme_mode: []const u8,
    duration_seconds: u64,
    countdown_loop: bool,
    countdown_loop_count: u32,
    countdown_loop_interval: u64,
    stopwatch_max_seconds: u64,
    log_level: []const u8,
    log_enable_timestamp: bool,
    log_tick_interval: i64,
};

/// CRUD 操作管理器
pub const CrudManager = struct {
    db: ?zqlite.Conn,
    allocator: std.mem.Allocator,

    /// 创建 CRUD 管理器实例
    pub fn init(allocator: std.mem.Allocator, db: ?zqlite.Conn) CrudManager {
        return .{
            .db = db,
            .allocator = allocator,
        };
    }

    /// 插入预设
    ///
    /// 参数:
    /// - **preset**: 要插入的预设
    /// - **config_json**: 预设配置的 JSON 表示
    ///
    /// 返回:
    /// - !void: 如果插入失败则返回错误
    pub fn insertPreset(self: *CrudManager, preset: interface.TimerPreset, config_json: []const u8) !void {
        if (self.db == null) {
            return CrudError.DatabaseOpenFailed;
        }

        // 验证预设数据
        try validator.validatePresetName(preset.name);
        if (config_json.len == 0 or config_json.len > 4096) {
            logger.global_logger.err("❌ 预设 JSON 长度无效: {}", .{config_json.len});
            return CrudError.InvalidPresetData;
        }

        // 模式转换
        const mode_str = switch (preset.mode) {
            .COUNTDOWN_MODE => "countdown",
            .STOPWATCH_MODE => "stopwatch",
            .WORLD_CLOCK_MODE => "world_clock",
        };

        // 使用高级 API 插入数据
        self.db.?.exec(
            "INSERT INTO presets (name, mode, config_json) VALUES (?, ?, ?);",
            .{ preset.name, mode_str, config_json },
        ) catch |err| {
            logger.global_logger.err("❌ 插入预设失败: {any}", .{err});
            return CrudError.InsertFailed;
        };

        logger.global_logger.info("✓ 预设 '{s}' 已插入 SQLite", .{preset.name});
    }

    /// 删除预设（按名称）
    ///
    /// 参数:
    /// - **name**: 预设名称
    ///
    /// 返回:
    /// - !void: 如果删除失败则返回错误
    pub fn deletePresetByName(self: *CrudManager, name: []const u8) !void {
        if (self.db == null) {
            return CrudError.DatabaseOpenFailed;
        }

        self.db.?.exec(
            "DELETE FROM presets WHERE name = ?;",
            .{name},
        ) catch |err| {
            logger.global_logger.err("❌ 删除预设失败: {any}", .{err});
            return CrudError.DeleteFailed;
        };

        logger.global_logger.info("✓ 预设 '{s}' 已从 SQLite 删除", .{name});
    }

    /// 查询所有预设
    ///
    /// 参数:
    /// - **allocator**: 内存分配器
    ///
    /// 返回:
    /// - !std.ArrayList(PresetRow): 预设行数据数组
    pub fn queryAllPresets(self: *CrudManager, allocator: std.mem.Allocator) !std.ArrayList(PresetRow) {
        if (self.db == null) {
            return CrudError.DatabaseOpenFailed;
        }

        var results = std.ArrayList(PresetRow){};

        var rows = try self.db.?.rows("SELECT id, name, mode, config_json FROM presets ORDER BY created_at ASC;", .{});
        defer rows.deinit();

        while (rows.next()) |row| {
            // 读取当前行
            const id = row.get(i64, 0);
            const name = row.get([]const u8, 1);
            const mode_str = row.get([]const u8, 2);
            const config_json = row.get([]const u8, 3);

            // 转换 mode
            const mode: interface.ModeEnumT = if (std.mem.eql(u8, mode_str, "countdown"))
                .COUNTDOWN_MODE
            else if (std.mem.eql(u8, mode_str, "stopwatch"))
                .STOPWATCH_MODE
            else if (std.mem.eql(u8, mode_str, "world_clock"))
                .WORLD_CLOCK_MODE
            else
                continue; // 无效模式，跳过

            // 复制字符串到堆上
            const name_copy = try allocator.dupe(u8, name);
            errdefer allocator.free(name_copy);

            const config_copy = try allocator.dupe(u8, config_json);
            errdefer allocator.free(config_copy);

            try results.append(allocator, .{
                .id = id,
                .name = name_copy,
                .mode = mode,
                .config_json = config_copy,
            });
        }

        logger.global_logger.info("✓ 从 SQLite 查询得到 {} 个预设", .{results.items.len});
        return results;
    }

    /// 检查预设是否存在（按名称）
    ///
    /// 参数:
    /// - **name**: 预设名称
    ///
    /// 返回:
    /// - !bool: 预设是否存在
    pub fn presetExists(self: *CrudManager, name: []const u8) !bool {
        if (self.db == null) {
            return CrudError.DatabaseOpenFailed;
        }

        var rows = try self.db.?.rows("SELECT COUNT(*) as count FROM presets WHERE name = ?;", .{name});
        defer rows.deinit();

        if (rows.next()) |row| {
            const count = row.get(i64, 0);
            return count > 0;
        }
        return false;
    }

    /// 清空所有预设
    ///
    /// 返回:
    /// - !void: 如果清空失败则返回错误
    pub fn clearAllPresets(self: *CrudManager) !void {
        if (self.db == null) {
            return CrudError.DatabaseOpenFailed;
        }

        self.db.?.exec(
            "DELETE FROM presets;",
            .{},
        ) catch |err| {
            logger.global_logger.err("❌ 清空预设失败: {any}", .{err});
            return CrudError.DeleteFailed;
        };

        logger.global_logger.info("✓ 所有预设已从 SQLite 清空", .{});
    }

    /// 保存设置到 SQLite
    ///
    /// 参数:
    /// - **config**: 要保存的设置配置
    ///
    /// 返回:
    /// - !void: 如果保存失败则返回错误
    pub fn saveSettings(self: *CrudManager, config: interface.SettingsConfig) !void {
        if (self.db == null) {
            return CrudError.DatabaseOpenFailed;
        }

        // 转换 DefaultMode 到字符串
        const default_mode_str = switch (config.basic.default_mode) {
            .countdown => "countdown",
            .stopwatch => "stopwatch",
            .world_clock => "world_clock",
        };

        // 使用 UPSERT 操作（INSERT OR REPLACE）
        self.db.?.exec(
            "INSERT OR REPLACE INTO settings (id, timezone, language, default_mode, theme_mode, duration_seconds, countdown_loop, countdown_loop_count, countdown_loop_interval, stopwatch_max_seconds, log_level, log_enable_timestamp, log_tick_interval) VALUES (1, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);",
            .{
                config.basic.timezone,
                config.basic.language,
                default_mode_str,
                config.basic.theme_mode,
                config.clock_defaults.countdown.duration_seconds,
                @intFromBool(config.clock_defaults.countdown.loop),
                config.clock_defaults.countdown.loop_count,
                config.clock_defaults.countdown.loop_interval_seconds,
                config.clock_defaults.stopwatch.max_seconds,
                config.logging.level,
                @intFromBool(config.logging.enable_timestamp),
                config.logging.tick_interval_ms,
            },
        ) catch |err| {
            logger.global_logger.err("❌ 保存设置失败: {any}", .{err});
            return CrudError.SettingsSaveFailed;
        };

        logger.global_logger.info("✓ 设置已保存到 SQLite", .{});
    }

    /// 从 SQLite 加载设置
    ///
    /// 参数:
    /// - **allocator**: 内存分配器
    ///
    /// 返回:
    /// - !interface.SettingsConfig: 加载的设置配置
    pub fn loadSettings(self: *CrudManager, allocator: std.mem.Allocator) !interface.SettingsConfig {
        if (self.db == null) {
            return CrudError.DatabaseOpenFailed;
        }

        var rows = try self.db.?.rows(
            "SELECT timezone, language, default_mode, theme_mode, duration_seconds, countdown_loop, countdown_loop_count, countdown_loop_interval, stopwatch_max_seconds, log_level, log_enable_timestamp, log_tick_interval FROM settings WHERE id = 1;",
            .{},
        );
        defer rows.deinit();

        const row = rows.next() orelse {
            logger.global_logger.warn("⚠️ 未找到设置数据，返回默认配置", .{});
            return interface.SettingsConfig{};
        };

        // 读取设置数据 - 使用正确的类型
        const timezone_raw = row.get(i64, 0);
        const language = row.get([]const u8, 1);
        const default_mode_str = row.get([]const u8, 2);
        const theme_mode = row.get([]const u8, 3);
        const duration_seconds_raw = row.get(i64, 4);
        const countdown_loop_raw = row.get(i64, 5);
        const countdown_loop_count_raw = row.get(i64, 6);
        const countdown_loop_interval_raw = row.get(i64, 7);
        const stopwatch_max_seconds_raw = row.get(i64, 8);
        const log_level = row.get([]const u8, 9);
        const log_enable_timestamp_raw = row.get(i64, 10);
        const log_tick_interval = row.get(i64, 11);

        // 类型转换
        const timezone: i8 = @intCast(timezone_raw);
        const duration_seconds: u64 = @intCast(duration_seconds_raw);
        const countdown_loop_count: u32 = @intCast(countdown_loop_count_raw);
        const countdown_loop_interval: u64 = @intCast(countdown_loop_interval_raw);
        const stopwatch_max_seconds: u64 = @intCast(stopwatch_max_seconds_raw);

        // 复制字符串到堆上
        const language_copy = try allocator.dupe(u8, language);
        errdefer allocator.free(language_copy);
        const theme_mode_copy = try allocator.dupe(u8, theme_mode);
        errdefer allocator.free(theme_mode_copy);
        const log_level_copy = try allocator.dupe(u8, log_level);
        errdefer allocator.free(log_level_copy);

        // 转换默认模式
        const default_mode: interface.DefaultMode = if (std.mem.eql(u8, default_mode_str, "countdown"))
            .countdown
        else if (std.mem.eql(u8, default_mode_str, "stopwatch"))
            .stopwatch
        else
            .world_clock;

        const settings = interface.SettingsConfig{
            .basic = .{
                .timezone = timezone,
                .language = language_copy,
                .default_mode = default_mode,
                .theme_mode = theme_mode_copy,
            },
            .clock_defaults = .{
                .countdown = .{
                    .duration_seconds = duration_seconds,
                    .loop = countdown_loop_raw != 0,
                    .loop_count = countdown_loop_count,
                    .loop_interval_seconds = countdown_loop_interval,
                },
                .stopwatch = .{
                    .max_seconds = stopwatch_max_seconds,
                },
                .world_clock = .{
                    .timezone = timezone,
                },
            },
            .logging = .{
                .level = log_level_copy,
                .enable_timestamp = log_enable_timestamp_raw != 0,
                .tick_interval_ms = log_tick_interval,
            },
        };

        logger.global_logger.info("✓ 已从 SQLite 加载设置", .{});
        return settings;
    }

    /// 获取预设统计信息
    pub fn getPresetStats(self: *CrudManager) !struct {
        total_presets: u32,
        countdown_presets: u32,
        stopwatch_presets: u32,
        world_clock_presets: u32,
    } {
        if (self.db == null) {
            return CrudError.DatabaseOpenFailed;
        }

        var rows = try self.db.?.rows(
            "SELECT COUNT(*) as total, COUNT(CASE WHEN mode = 'countdown' THEN 1 END) as countdown, COUNT(CASE WHEN mode = 'stopwatch' THEN 1 END) as stopwatch, COUNT(CASE WHEN mode = 'world_clock' THEN 1 END) as world_clock FROM presets;",
            .{},
        );
        defer rows.deinit();

        if (rows.next()) |row| {
            return .{
                .total_presets = @intCast(row.get(i64, 0)),
                .countdown_presets = @intCast(row.get(i64, 1)),
                .stopwatch_presets = @intCast(row.get(i64, 2)),
                .world_clock_presets = @intCast(row.get(i64, 3)),
            };
        }

        return .{
            .total_presets = 0,
            .countdown_presets = 0,
            .stopwatch_presets = 0,
            .world_clock_presets = 0,
        };
    }
};
