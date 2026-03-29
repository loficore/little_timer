//! 数据库CRUD操作模块
//! 职责：设置的具体增删改查操作
const std = @import("std");
const zqlite = @import("zqlite");
const interface = @import("../core/interface.zig");
const logger = @import("../core/logger.zig");

/// CRUD 操作错误类型
pub const CrudError = error{
    InsertFailed, // 插入失败
    DeleteFailed, // 删除失败
    QueryFailed, // 查询失败
    SettingsNotFound, // 设置未找到
    SettingsSaveFailed, // 设置保存失败
    DatabaseOpenFailed, // 数据库打开失败
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
        else
            .stopwatch;

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
};
