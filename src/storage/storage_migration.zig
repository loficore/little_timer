//! 数据库迁移管理模块
//! 职责：数据库版本管理、模式迁移、表创建基础逻辑
const std = @import("std");
const zqlite = @import("zqlite");
const interface = @import("../core/interface.zig");
const logger = @import("../core/logger.zig");

/// 数据库模式版本常量
pub const CURRENT_SCHEMA_VERSION = 1;

/// SQLite 错误类型
pub const MigrationError = error{
    InvalidSchemaVersion, // 无效的数据库模式版本
    MigrationFailed, // 数据库迁移失败
    TableCreationFailed, // 表创建失败
};

/// 数据库迁移管理器
pub const MigrationManager = struct {
    db: ?zqlite.Conn,
    allocator: std.mem.Allocator,

    /// 创建迁移管理器实例
    pub fn init(allocator: std.mem.Allocator, db: ?zqlite.Conn) MigrationManager {
        return .{
            .db = db,
            .allocator = allocator,
        };
    }

    /// 检查并执行数据库模式迁移
    pub fn checkAndMigrate(self: *MigrationManager) !void {
        if (self.db == null) {
            return MigrationError.TableCreationFailed;
        }

        // 创建版本管理表
        try self.createSchemaVersionTable();

        // 获取当前版本
        const current_version = try self.getSchemaVersion();

        if (current_version == 0) {
            // 新数据库，创建所有表
            logger.global_logger.info("🔄 创建新数据库模式 (版本 {})", .{CURRENT_SCHEMA_VERSION});
            try self.createTables();
            try self.setSchemaVersion(CURRENT_SCHEMA_VERSION);
        } else if (current_version < CURRENT_SCHEMA_VERSION) {
            // 需要迁移
            logger.global_logger.info("🔄 数据库版本: {} -> {}", .{ current_version, CURRENT_SCHEMA_VERSION });
            try self.migrateSchema(current_version, CURRENT_SCHEMA_VERSION);
        } else if (current_version > CURRENT_SCHEMA_VERSION) {
            // 数据库版本太新
            logger.global_logger.err("❌ 数据库版本 ({}) 过于新，请升级应用程序到最新版本 ({})", .{ current_version, CURRENT_SCHEMA_VERSION });
            return MigrationError.InvalidSchemaVersion;
        }

        logger.global_logger.info("✓ 数据库模式检查完成，版本: {}", .{try self.getSchemaVersion()});
    }

    /// 创建版本管理表
    fn createSchemaVersionTable(self: *MigrationManager) !void {
        const create_version_sql =
            \\CREATE TABLE IF NOT EXISTS schema_version (
            \\    version INTEGER PRIMARY KEY,
            \\    applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
            \\    description TEXT
            \\);
        ;

        self.db.?.exec(create_version_sql, .{}) catch |err| {
            logger.global_logger.err("❌ 创建版本表失败: {any}", .{err});
            return MigrationError.TableCreationFailed;
        };
    }

    /// 获取当前数据库模式版本
    fn getSchemaVersion(self: *MigrationManager) !i32 {
        var rows = try self.db.?.rows("SELECT MAX(version) FROM schema_version;", .{});
        defer rows.deinit();

        if (rows.next()) |row| {
            const version = row.get(?i64, 0);
            return if (version) |v| @intCast(v) else 0;
        }
        return 0;
    }

    /// 设置数据库模式版本
    fn setSchemaVersion(self: *MigrationManager, version: i32) !void {
        self.db.?.exec(
            "INSERT INTO schema_version (version, description) VALUES (?, ?);",
            .{ version, "Little Timer Database Schema" },
        ) catch |err| {
            logger.global_logger.err("❌ 设置版本失败: {any}", .{err});
            return MigrationError.MigrationFailed;
        };
    }

    /// 执行数据库模式迁移
    fn migrateSchema(self: *MigrationManager, from_version: i32, to_version: i32) !void {
        // 这里可以添加具体的迁移逻辑
        // 目前只有版本1，所以直接从0到1
        if (from_version == 0 and to_version == 1) {
            try self.createTables();
            try self.setSchemaVersion(to_version);
            logger.global_logger.info("✓ 数据库迁移完成", .{});
        } else {
            logger.global_logger.err("❌ 不支持的迁移路径: {} -> {}", .{ from_version, to_version });
            return MigrationError.MigrationFailed;
        }
    }

    /// 创建所有数据表（基础版本）
    fn createTables(self: *MigrationManager) !void {
        // 数据库健康检查表
        const create_health_check_sql =
            \\CREATE TABLE IF NOT EXISTS health_check (
            \\    id INTEGER PRIMARY KEY CHECK (id = 1),
            \\    last_check TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
            \\    status TEXT NOT NULL DEFAULT 'healthy',
            \\    checksum TEXT,
            \\    record_count INTEGER DEFAULT 0
            \\);
        ;

        // 优化的预设表：添加更多约束
        const create_presets_sql =
            \\CREATE TABLE IF NOT EXISTS presets (
            \\    id INTEGER PRIMARY KEY AUTOINCREMENT,
            \\    name TEXT NOT NULL CHECK(length(name) > 0 AND length(name) <= 100),
            \\    mode TEXT NOT NULL CHECK(mode IN ('countdown', 'stopwatch', 'world_clock')),
            \\    config_json TEXT NOT NULL CHECK(length(config_json) <= 8192),
            \\    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
            \\    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
            \\    UNIQUE(name)
            \\);
        ;

        // 优化的设置表：添加更多约束
        const create_settings_sql = "CREATE TABLE IF NOT EXISTS settings (" ++
            " id INTEGER PRIMARY KEY CHECK (id = 1)," ++
            " timezone INTEGER NOT NULL CHECK(timezone >= -12 AND timezone <= 14)," ++
            " language TEXT NOT NULL CHECK(length(language) >= 1 AND length(language) <= 10)," ++
            " default_mode TEXT NOT NULL CHECK(default_mode IN ('countdown', 'stopwatch', 'world_clock'))," ++
            " theme_mode TEXT NOT NULL CHECK(length(theme_mode) <= 20)," ++
            " duration_seconds INTEGER NOT NULL CHECK(duration_seconds >= 1 AND duration_seconds <= 86400)," ++
            " countdown_loop BOOLEAN NOT NULL DEFAULT 0," ++
            " countdown_loop_count INTEGER NOT NULL DEFAULT 0 CHECK(countdown_loop_count >= 0 AND countdown_loop_count <= 1000)," ++
            " countdown_loop_interval INTEGER NOT NULL DEFAULT 0 CHECK(countdown_loop_interval >= 0 AND countdown_loop_interval <= 3600)," ++
            " stopwatch_max_seconds INTEGER NOT NULL DEFAULT 86400 CHECK(stopwatch_max_seconds > 0 AND stopwatch_max_seconds <= 31536000)," ++
            " log_level TEXT NOT NULL CHECK(length(log_level) <= 10)," ++
            " log_enable_timestamp BOOLEAN NOT NULL DEFAULT 1," ++
            " log_tick_interval INTEGER NOT NULL DEFAULT 1000 CHECK(log_tick_interval >= 100 AND log_tick_interval <= 10000)," ++
            " updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP" ++
            ");";

        // 创建所有表
        self.db.?.exec(create_health_check_sql, .{}) catch |err| {
            logger.global_logger.err("❌ 创建健康检查表失败: {any}", .{err});
            return MigrationError.TableCreationFailed;
        };

        self.db.?.exec(create_presets_sql, .{}) catch |err| {
            logger.global_logger.err("❌ 创建预设表失败: {any}", .{err});
            return MigrationError.TableCreationFailed;
        };

        self.db.?.exec(create_settings_sql, .{}) catch |err| {
            logger.global_logger.err("❌ 创建设置表失败: {any}", .{err});
            return MigrationError.TableCreationFailed;
        };

        // 创建索引
        try self.createOptimizedIndexes();

        // 初始化默认设置
        try self.initializeDefaultSettings();

        logger.global_logger.info("✓ 所有数据表和索引已创建完成", .{});
    }

    /// 创建优化的索引
    fn createOptimizedIndexes(self: *MigrationManager) !void {
        const indexes = [_]struct { name: []const u8, sql: []const u8 }{
            .{ .name = "idx_presets_name", .sql = "CREATE INDEX IF NOT EXISTS idx_presets_name ON presets(name);" },
            .{ .name = "idx_presets_mode", .sql = "CREATE INDEX IF NOT EXISTS idx_presets_mode ON presets(mode);" },
            .{ .name = "idx_presets_updated", .sql = "CREATE INDEX IF NOT EXISTS idx_presets_updated ON presets(updated_at);" },
            .{ .name = "idx_settings_timezone", .sql = "CREATE INDEX IF NOT EXISTS idx_settings_timezone ON settings(timezone);" },
            .{ .name = "idx_settings_language", .sql = "CREATE INDEX IF NOT EXISTS idx_settings_language ON settings(language);" },
            .{ .name = "idx_health_check_status", .sql = "CREATE INDEX IF NOT EXISTS idx_health_check_status ON health_check(status);" },
        };

        for (indexes) |index| {
            self.db.?.exec(index.sql, .{}) catch |err| {
                logger.global_logger.err("⚠️ 创建索引 {s} 失败: {any}", .{ index.name, err });
                // 索引创建失败不是致命错误，继续执行
            };
        }

        logger.global_logger.info("✓ 索引创建完成", .{});
    }

    /// 初始化默认设置（如果设置表为空）
    fn initializeDefaultSettings(self: *MigrationManager) !void {
        // 检查是否已有设置数据
        var rows = try self.db.?.rows("SELECT COUNT(*) as count FROM settings WHERE id = 1;", .{});
        defer rows.deinit();

        if (rows.next()) |row| {
            const count = row.get(i64, 0);
            if (count > 0) {
                logger.global_logger.debug("设置表已有数据，跳过初始化", .{});
                return;
            }
        }

        // 插入默认设置
        self.db.?.exec(
            "INSERT INTO settings (id, timezone, language, default_mode, theme_mode, duration_seconds, countdown_loop, countdown_loop_count, countdown_loop_interval, stopwatch_max_seconds, log_level, log_enable_timestamp, log_tick_interval) VALUES (1, 8, 'ZH', 'countdown', 'dark', 1500, 0, 0, 0, 86400, 'INFO', 1, 1000);",
            .{},
        ) catch |err| {
            logger.global_logger.err("❌ 初始化默认设置失败: {any}", .{err});
            return err;
        };

        logger.global_logger.info("✓ 默认设置已初始化", .{});
    }
};
