//! 数据库备份恢复模块
//! 职责：数据库备份创建、备份恢复、备份文件管理
const std = @import("std");
const zqlite = @import("zqlite");
const logger = @import("../core/logger.zig");

/// 备份恢复错误类型
pub const BackupError = error{
    BackupFailed, // 备份失败
    RestoreFailed, // 恢复失败
    InvalidBackupPath, // 无效的备份路径
    DatabaseOpenFailed, // 数据库打开失败
};

/// 备份管理器
pub const BackupManager = struct {
    db: ?zqlite.Conn,
    allocator: std.mem.Allocator,
    db_path: [:0]const u8, // 数据库文件路径（以 null 结尾）
    backup_dir: []const u8, // 备份目录路径
    max_backups: u32 = 10, // 最大备份文件数量

    /// 创建备份管理器实例
    pub fn init(allocator: std.mem.Allocator, db: ?zqlite.Conn, db_path: [:0]const u8, backup_dir: []const u8) BackupManager {
        return .{
            .db = db,
            .allocator = allocator,
            .db_path = db_path,
            .backup_dir = backup_dir,
        };
    }

    /// 创建数据库备份
    ///
    /// 返回:
    /// - ![]const u8: 备份文件路径
    pub fn createBackup(self: *BackupManager) ![]const u8 {
        if (self.db == null) {
            return BackupError.DatabaseOpenFailed;
        }

        // 生成带时间戳的备份文件名
        const timestamp = std.time.timestamp();
        var backup_buf: [512]u8 = undefined;
        const backup_filename = try std.fmt.bufPrint(&backup_buf, "presets_backup_{}.db", .{timestamp});

        // 构建完整的备份路径
        const backup_path = try std.fs.path.join(self.allocator, &[_][]const u8{ self.backup_dir, backup_filename });
        errdefer self.allocator.free(backup_path);

        // 执行文件备份
        try self.performFileBackup(backup_path);

        // 清理旧备份
        try self.cleanupOldBackups();

        logger.global_logger.info("✓ 数据库备份已创建: {s}", .{backup_path});
        return backup_path;
    }

    /// 执行文件备份
    fn performFileBackup(self: *BackupManager, backup_path: []const u8) !void {
        // 确保备份目录存在
        std.fs.cwd().access(self.backup_dir, .{}) catch {
            try std.fs.cwd().makeDir(self.backup_dir);
        };

        // 关闭数据库连接以确保数据一致性
        self.db = null;

        // 恢复函数：出错时重新打开数据库
        errdefer {
            // 恢复数据库连接
            const flags = zqlite.OpenFlags.ReadWrite;
            self.db = zqlite.open(self.db_path, flags) catch {
                logger.global_logger.err("❌ 无法重新打开数据库", .{});
            };
        }

        try std.fs.cwd().copyFile(self.db_path, std.fs.cwd(), backup_path, .{});

        // 重新打开数据库
        const flags = zqlite.OpenFlags.ReadWrite;
        self.db = zqlite.open(self.db_path, flags) catch |err| {
            logger.global_logger.err("❌ 重新打开数据库失败: {any}", .{err});
            return BackupError.BackupFailed;
        };
    }

    /// 从备份恢复数据库
    ///
    /// 参数:
    /// - **backup_path**: 备份文件路径
    /// - **reopen_callback**: 重新打开数据库的回调函数
    ///
    /// 返回:
    /// - !void: 如果恢复失败则返回错误
    pub fn restoreFromBackup(self: *BackupManager, backup_path: []const u8, reopen_callback: fn (*BackupManager) anyerror!void) !void {
        // 检查备份文件是否存在
        std.fs.cwd().access(backup_path, .{}) catch {
            logger.global_logger.err("❌ 备份文件不存在: {s}", .{backup_path});
            return BackupError.InvalidBackupPath;
        };

        // 关闭当前数据库
        self.close();

        // 复制备份文件到原位置
        try std.fs.cwd().copyFile(backup_path, std.fs.cwd(), self.db_path, .{});

        // 重新打开数据库
        try reopen_callback(self);

        logger.global_logger.info("✓ 数据库已从备份恢复: {s}", .{backup_path});
    }

    /// 关闭数据库连接
    fn close(self: *BackupManager) void {
        if (self.db != null) {
            self.db.?.close();
            self.db = null;
            logger.global_logger.debug("✓ 数据库连接已关闭（备份操作）", .{});
        }
    }

    /// 清理旧备份文件
    fn cleanupOldBackups(self: *BackupManager) !void {
        // 获取备份目录中的所有备份文件
        const backup_dir = try std.fs.cwd().openDir(self.backup_dir, .{ .iterate = true });
        defer backup_dir.close();

        var backups = std.ArrayList(struct {
            name: []const u8,
            timestamp: i64,
        }).init(self.allocator);
        defer {
            for (backups.items) |backup| {
                self.allocator.free(backup.name);
            }
            backups.deinit();
        }

        // 收集所有备份文件
        var iter = backup_dir.iterate();
        while (try iter.next()) |entry| {
            if (entry.kind == .file and std.mem.startsWith(u8, entry.name, "presets_backup_") and std.mem.endsWith(u8, entry.name, ".db")) {
                // 提取时间戳
                const name_prefix = "presets_backup_";
                const name_suffix = ".db";
                const timestamp_start = name_prefix.len;
                const timestamp_end = entry.name.len - name_suffix.len;

                if (timestamp_end > timestamp_start) {
                    const timestamp_str = entry.name[timestamp_start..timestamp_end];
                    const timestamp = std.fmt.parseInt(i64, timestamp_str, 10) catch continue;

                    const name_copy = try self.allocator.dupe(u8, entry.name);
                    try backups.append(.{ .name = name_copy, .timestamp = timestamp });
                }
            }
        }

        // 按时间戳排序并删除旧备份
        std.mem.sort(struct { name: []const u8, timestamp: i64 }, backups.items, struct {
            fn less(a: struct { name: []const u8, timestamp: i64 }, b: struct { name: []const u8, timestamp: i64 }) bool {
                return a.timestamp < b.timestamp;
            }
        }.less);

        // 删除超出限制的旧备份
        if (backups.items.len > self.max_backups) {
            const to_delete = backups.items[0 .. backups.items.len - self.max_backups];
            for (to_delete) |backup| {
                const backup_path = try std.fs.path.join(self.allocator, &[_][]const u8{ self.backup_dir, backup.name });
                std.fs.cwd().deleteFile(backup_path) catch {
                    logger.global_logger.warn("⚠️ 删除旧备份失败: {s}", .{backup_path});
                };
                self.allocator.free(backup_path);
                logger.global_logger.info("✓ 已删除旧备份: {s}", .{backup.name});
            }
        }

        logger.global_logger.debug("✓ 备份清理完成，保留最新 {} 个备份", .{self.max_backups});
    }

    /// 获取备份目录信息
    pub fn getBackupInfo(self: *BackupManager) !struct {
        total_backups: u32,
        total_size_bytes: u64,
        oldest_backup: ?[]const u8,
        newest_backup: ?[]const u8,
    } {
        const backup_dir = try std.fs.cwd().openDir(self.backup_dir, .{ .iterate = true });
        defer backup_dir.close();

        var total_size: u64 = 0;
        var backup_count: u32 = 0;
        var oldest_timestamp: i64 = std.time.timestamp();
        var newest_timestamp: i64 = 0;
        var oldest_name: ?[]const u8 = null;
        var newest_name: ?[]const u8 = null;

        var iter = backup_dir.iterate();
        while (try iter.next()) |entry| {
            if (entry.kind == .file and std.mem.startsWith(u8, entry.name, "presets_backup_") and std.mem.endsWith(u8, entry.name, ".db")) {
                // 获取文件信息
                const file = try backup_dir.openFile(entry.name, .{});
                defer file.close();

                const stat = try file.stat();
                total_size += @intCast(stat.size);

                backup_count += 1;

                // 提取时间戳
                const name_prefix = "presets_backup_";
                const name_suffix = ".db";
                const timestamp_start = name_prefix.len;
                const timestamp_end = entry.name.len - name_suffix.len;

                if (timestamp_end > timestamp_start) {
                    const timestamp_str = entry.name[timestamp_start..timestamp_end];
                    const timestamp = std.fmt.parseInt(i64, timestamp_str, 10) catch continue;

                    if (timestamp < oldest_timestamp) {
                        oldest_timestamp = timestamp;
                        oldest_name = try self.allocator.dupe(u8, entry.name);
                    }
                    if (timestamp > newest_timestamp) {
                        newest_timestamp = timestamp;
                        newest_name = try self.allocator.dupe(u8, entry.name);
                    }
                }
            }
        }

        return .{
            .total_backups = backup_count,
            .total_size_bytes = total_size,
            .oldest_backup = oldest_name,
            .newest_backup = newest_name,
        };
    }

    /// 清理已分配的备份名称
    pub fn freeBackupInfo(self: *BackupManager, info: struct {
        total_backups: u32,
        total_size_bytes: u64,
        oldest_backup: ?[]const u8,
        newest_backup: ?[]const u8,
    }) void {
        if (info.oldest_backup) |name| {
            self.allocator.free(name);
        }
        if (info.newest_backup) |name| {
            self.allocator.free(name);
        }
    }
};
