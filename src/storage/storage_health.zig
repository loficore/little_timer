//! 数据库健康检查模块
//! 职责：数据库完整性检查、健康状态监控、性能指标收集
const std = @import("std");
const zqlite = @import("zqlite");
const logger = @import("../core/logger.zig");

/// 健康检查错误类型
pub const HealthCheckError = error{
    IntegrityCheckFailed, // 完整性检查失败
    HealthCheckFailed, // 健康检查失败
    DatabaseNotHealthy, // 数据库不健康
};

/// 数据库健康检查信息
pub const HealthCheckInfo = struct {
    status: []const u8,
    last_check: []const u8,
    record_count: i64,
};

/// 数据库健康检查管理器
pub const HealthCheckManager = struct {
    db: ?zqlite.Conn,
    allocator: std.mem.Allocator,

    /// 创建健康检查管理器实例
    pub fn init(allocator: std.mem.Allocator, db: ?zqlite.Conn) HealthCheckManager {
        return .{
            .db = db,
            .allocator = allocator,
        };
    }

    /// 初始化健康检查记录
    pub fn initialize(self: *HealthCheckManager) !void {
        if (self.db == null) {
            return HealthCheckError.HealthCheckFailed;
        }

        // 检查是否已有健康检查记录
        var rows = try self.db.?.rows("SELECT COUNT(*) as count FROM health_check WHERE id = 1;", .{});
        defer rows.deinit();

        if (rows.next()) |row| {
            const count = row.get(i64, 0);
            if (count > 0) {
                return; // 已存在，跳过初始化
            }
        }

        // 插入初始健康检查记录
        self.db.?.exec(
            "INSERT INTO health_check (id, status, record_count) VALUES (1, 'healthy', 0);",
            .{},
        ) catch |err| {
            logger.global_logger.err("❌ 初始化健康检查失败: {any}", .{err});
            return err;
        };

        logger.global_logger.info("✓ 健康检查记录已初始化", .{});
    }

    /// 执行数据库健康检查
    pub fn performCheck(self: *HealthCheckManager) !void {
        if (self.db == null) {
            return HealthCheckError.HealthCheckFailed;
        }

        // 检查数据库完整性
        var integrity_rows = try self.db.?.rows("PRAGMA integrity_check;", .{});
        defer integrity_rows.deinit();

        if (integrity_rows.next()) |row| {
            const result = row.get([]const u8, 0);
            if (!std.mem.eql(u8, result, "ok")) {
                logger.global_logger.err("❌ 数据库完整性检查失败: {s}", .{result});
                return HealthCheckError.IntegrityCheckFailed;
            }
        }

        // 更新健康检查记录
        try self.updateRecord();

        logger.global_logger.debug("✓ 数据库健康检查通过", .{});
    }

    /// 更新健康检查记录
    pub fn updateRecord(self: *HealthCheckManager) !void {
        if (self.db == null) {
            return HealthCheckError.HealthCheckFailed;
        }

        // 获取记录数量
        var preset_rows = try self.db.?.rows("SELECT COUNT(*) FROM presets;", .{});
        defer preset_rows.deinit();

        const preset_count = if (preset_rows.next()) |row| row.get(i64, 0) else 0;

        // 更新健康检查记录
        self.db.?.exec(
            "INSERT OR REPLACE INTO health_check (id, last_check, status, record_count) VALUES (1, CURRENT_TIMESTAMP, 'healthy', ?);",
            .{preset_count},
        ) catch |err| {
            logger.global_logger.err("❌ 更新健康检查失败: {any}", .{err});
            return HealthCheckError.HealthCheckFailed;
        };
    }

    /// 获取健康检查信息
    pub fn getInfo(self: *HealthCheckManager) !HealthCheckInfo {
        if (self.db == null) {
            return HealthCheckError.HealthCheckFailed;
        }

        var rows = try self.db.?.rows(
            "SELECT status, last_check, record_count FROM health_check WHERE id = 1;",
            .{},
        );
        defer rows.deinit();

        if (rows.next()) |row| {
            const status = row.get([]const u8, 0);
            const last_check = row.get([]const u8, 1);
            const record_count = row.get(i64, 2);

            // 复制字符串到堆上
            const status_copy = try self.allocator.dupe(u8, status);
            errdefer self.allocator.free(status_copy);

            const last_check_copy = try self.allocator.dupe(u8, last_check);
            errdefer self.allocator.free(last_check_copy);

            return .{
                .status = status_copy,
                .last_check = last_check_copy,
                .record_count = record_count,
            };
        }

        return .{
            .status = "unknown",
            .last_check = "never",
            .record_count = 0,
        };
    }

    /// 检查数据库是否健康
    pub fn isHealthy(self: *HealthCheckManager) !bool {
        const info = try self.getInfo();
        defer {
            self.allocator.free(info.status);
            self.allocator.free(info.last_check);
        }

        return std.mem.eql(u8, info.status, "healthy");
    }

    /// 执行深度健康检查（包含性能指标）
    pub fn performDeepCheck(self: *HealthCheckManager) !HealthCheckInfo {
        try self.performCheck(); // 先执行基本检查

        // 获取更详细的健康信息
        const query = "SELECT " ++
            "(SELECT COUNT(*) FROM presets) as preset_count, " ++
            "(SELECT COUNT(*) FROM settings) as settings_count, " ++
            "(SELECT COUNT(*) FROM health_check) as health_records, " ++
            "(SELECT last_check FROM health_check WHERE id = 1) as last_check";
        var perf_rows = try self.db.?.rows(query, .{});
        defer perf_rows.deinit();

        if (perf_rows.next()) |row| {
            const preset_count = row.get(i64, 0);
            const settings_count = row.get(i64, 1);
            const health_records = row.get(i64, 2);
            const last_check_raw = row.get([]const u8, 3);

            const status_copy = try self.allocator.dupe(u8, "healthy");
            errdefer self.allocator.free(status_copy);

            const last_check_copy = try self.allocator.dupe(u8, last_check_raw);
            errdefer self.allocator.free(last_check_copy);

            const total_records = preset_count + settings_count;
            const _health_records = health_records; // 用于调试和验证

            logger.global_logger.debug("📊 数据库性能指标: 预设={}, 设置={}, 总计={}, 健康记录数={}", .{ preset_count, settings_count, total_records, _health_records });

            return .{
                .status = status_copy,
                .last_check = last_check_copy,
                .record_count = total_records,
            };
        }

        return try self.getInfo();
    }
};
