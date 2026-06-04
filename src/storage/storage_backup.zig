//! 数据库备份恢复模块
//! 职责：数据库备份创建、备份恢复、备份文件管理
const std = @import("std");
const zqlite = @import("zqlite");
const logger = @import("../core/logger.zig");
const interface = @import("../core/interface.zig");
const backup = @import("backup/BackupAdapter.zig");

pub const BackupError = error{
    BackupFailed,
    RestoreFailed,
    InvalidBackupPath,
    DatabaseOpenFailed,
};

const BackupEntry = struct { name: []const u8, timestamp: i64 };

const BackupListItem = struct {
    name: []const u8,
    timestamp: i64,
    size_bytes: u64,
};

fn makeBackupList() std.ArrayListUnmanaged(BackupListItem) {
    return .{};
}

pub const BackupManager = struct {
    db: ?zqlite.Conn,
    allocator: std.mem.Allocator,
    db_path: [:0]const u8,
    backup_dir: []const u8,
    max_backups: u32 = 10,
    adapter: *backup.BackupAdapter = undefined,
    target_type: interface.BackupTargetType = .local,
    has_adapter: bool = false,

    pub fn init(allocator: std.mem.Allocator, db: ?zqlite.Conn, db_path: [:0]const u8, backup_dir: []const u8) BackupManager {
        return .{
            .db = db,
            .allocator = allocator,
            .db_path = db_path,
            .backup_dir = backup_dir,
        };
    }

    pub fn initWithConfig(allocator: std.mem.Allocator, db: ?zqlite.Conn, db_path: [:0]const u8, backup_dir: []const u8, config: *const interface.BackupConfig) !BackupManager {
        var self = BackupManager.init(allocator, db, db_path, backup_dir);
        self.target_type = config.target_type;
        self.adapter = try self.createAdapter(config);
        self.has_adapter = true;
        return self;
    }

    pub fn createAdapter(self: *BackupManager, config: *const interface.BackupConfig) !backup.BackupAdapter {
        switch (config.target_type) {
            .local => {
                const local_path = if (config.local_path.len > 0) config.local_path else self.backup_dir;
                return backup.createLocalAdapter(self.allocator, .{ .path = local_path });
            },
            .webdav => {
                const webdav_config = backup.WebDAVConfig{
                    .url = config.webdav_url,
                    .username = config.webdav_username,
                    .password = config.webdav_password,
                    .base_path = "/",
                };
                return backup.createWebDAVAdapter(self.allocator, webdav_config);
            },
            .s3 => {
                const s3_config = backup.S3Config{
                    .endpoint = config.s3_endpoint,
                    .bucket = config.s3_bucket,
                    .region = config.s3_region,
                    .access_key = config.s3_access_key,
                    .secret_key = config.s3_secret_key,
                    .path_prefix = if (config.s3_path_prefix.len > 0) config.s3_path_prefix else "little_timer/",
                };
                return backup.createS3Adapter(self.allocator, s3_config);
            },
        }
    }

    pub fn createBackup(self: *BackupManager) ![]const u8 {
        if (self.db == null) {
            return BackupError.DatabaseOpenFailed;
        }

        const timestamp = std.time.timestamp();
        var backup_buf: [512]u8 = undefined;
        const backup_filename = try std.fmt.bufPrint(&backup_buf, "presets_backup_{}.db", .{timestamp});

        if (self.has_adapter) {
            logger.global_logger.info("[Backup] Starting WebDAV backup: db_path={s}, backup_name={s}", .{ self.db_path, backup_filename });
            try self.closeDbForBackup();
            errdefer self.reopenDb() catch {};

            try self.adapter.push(self.db_path, backup_filename);

            try self.reopenDb();
            try self.cleanupOldBackupsViaAdapter();

            logger.global_logger.info("✓ 数据库备份已创建: {s}", .{backup_filename});
            return try self.allocator.dupe(u8, backup_filename);
        } else {
            const backup_path = try std.fs.path.join(self.allocator, &[_][]const u8{ self.backup_dir, backup_filename });
            errdefer self.allocator.free(backup_path);

            try self.performFileBackup(backup_path);
            try self.cleanupOldBackups();

            logger.global_logger.info("✓ 数据库备份已创建: {s}", .{backup_path});
            return backup_path;
        }
    }

    pub fn closeDbForBackup(self: *BackupManager) !void {
        if (self.db) |conn| {
            conn.close();
            self.db = null;
            logger.global_logger.debug("✓ 数据库连接已关闭（备份操作）", .{});
        }
    }

    pub fn reopenDb(self: *BackupManager) !void {
        const flags = zqlite.OpenFlags.ReadWrite;
        self.db = zqlite.open(self.db_path, flags) catch |err| {
            logger.global_logger.err("❌ 重新打开数据库失败: {any}", .{err});
            return BackupError.BackupFailed;
        };
    }

    fn cleanupOldBackupsViaAdapter(self: *BackupManager) !void {
        if (self.has_adapter) {
            const backups = self.adapter.list() catch |err| {
                logger.global_logger.warn("⚠️ 获取备份列表失败: {any}", .{err});
                return;
            };
            defer self.adapter.freeList(backups);

            if (backups.len > self.max_backups) {
                const to_delete = backups.len - self.max_backups;
                for (backups[0..to_delete]) |b| {
                    self.adapter.delete(b.name) catch |delete_err| {
                        logger.global_logger.warn("⚠️ 删除旧备份失败: {any}", .{delete_err});
                    };
                    logger.global_logger.info("✓ 已删除旧备份: {s}", .{b.name});
                }
            }
        }
    }

    fn performFileBackup(self: *BackupManager, backup_path: []const u8) !void {
        std.fs.cwd().access(self.backup_dir, .{}) catch {
            try std.fs.cwd().makeDir(self.backup_dir);
        };

        const old_db = self.db;
        self.db = null;

        errdefer {
            const flags = zqlite.OpenFlags.ReadWrite;
            if (zqlite.open(self.db_path, flags)) |new_conn| {
                self.db = new_conn;
            } else |err| {
                logger.global_logger.err("❌ 无法重新打开数据库: {any}", .{err});
                self.db = old_db;
            }
        }

        try std.fs.cwd().copyFile(self.db_path, std.fs.cwd(), backup_path, .{});

        const flags = zqlite.OpenFlags.ReadWrite;
        self.db = zqlite.open(self.db_path, flags) catch |err| {
            logger.global_logger.err("❌ 重新打开数据库失败: {any}", .{err});
            return BackupError.BackupFailed;
        };
    }

    pub fn restoreFromBackup(self: *BackupManager, backup_name: []const u8) !void {
        if (self.has_adapter) {
            var temp_path_buf: [512]u8 = undefined;
            const temp_path = try std.fmt.bufPrint(&temp_path_buf, "/tmp/{s}", .{backup_name});

            try self.closeDbForBackup();
            errdefer self.reopenDb() catch {};

            try self.adapter.pull(backup_name, temp_path);

            try std.fs.cwd().copyFile(temp_path, std.fs.cwd(), self.db_path, .{});
            std.fs.cwd().deleteFile(temp_path) catch {};

            try self.reopenDb();

            logger.global_logger.info("✓ 数据库已从备份恢复: {s}", .{backup_name});
        } else {
            const backup_path = try std.fs.path.join(self.allocator, &[_][]const u8{ self.backup_dir, backup_name });
            defer self.allocator.free(backup_path);

            std.fs.cwd().access(backup_path, .{}) catch {
                logger.global_logger.err("❌ 备份文件不存在: {s}", .{backup_path});
                return BackupError.InvalidBackupPath;
            };

            try self.closeDbForBackup();
            errdefer self.reopenDb() catch {};

            try std.fs.cwd().copyFile(backup_path, std.fs.cwd(), self.db_path, .{});

            try self.reopenDb();

            logger.global_logger.info("✓ 数据库已从备份恢复: {s}", .{backup_path});
        }
    }

    fn cleanupOldBackups(self: *BackupManager) !void {
        var backup_dir = std.fs.cwd().openDir(self.backup_dir, .{ .iterate = true }) catch |err| {
            if (err == error.FileNotFound) {
                return;
            }
            return err;
        };
        defer backup_dir.close();

        var backup_count: usize = 0;
        var backup_capacity: usize = 16;
        var backup_storage = try self.allocator.alloc(BackupEntry, backup_capacity);
        defer {
            for (backup_storage[0..backup_count]) |b| {
                self.allocator.free(b.name);
            }
            self.allocator.free(backup_storage);
        }

        var iter = backup_dir.iterate();
        while (try iter.next()) |entry| {
            if (entry.kind == .file and std.mem.startsWith(u8, entry.name, "presets_backup_") and std.mem.endsWith(u8, entry.name, ".db")) {
                const name_prefix = "presets_backup_";
                const name_suffix = ".db";
                const timestamp_start = name_prefix.len;
                const timestamp_end = entry.name.len - name_suffix.len;

                if (timestamp_end > timestamp_start) {
                    const timestamp_str = entry.name[timestamp_start..timestamp_end];
                    const timestamp = std.fmt.parseInt(i64, timestamp_str, 10) catch continue;

                    if (backup_count >= backup_capacity) {
                        backup_capacity *= 2;
                        const new_storage = try self.allocator.realloc(backup_storage, backup_capacity);
                        backup_storage = new_storage;
                    }

                    backup_storage[backup_count] = .{
                        .name = try self.allocator.dupe(u8, entry.name),
                        .timestamp = timestamp,
                    };
                    backup_count += 1;
                }
            }
        }

        std.mem.sort(BackupEntry, backup_storage[0..backup_count], {}, struct {
            fn less(_: void, a: BackupEntry, b: BackupEntry) bool {
                return a.timestamp < b.timestamp;
            }
        }.less);

        if (backup_count > self.max_backups) {
            const to_delete = backup_count - self.max_backups;
            for (0..to_delete) |i| {
                const backup_path = try std.fs.path.join(self.allocator, &[_][]const u8{ self.backup_dir, backup_storage[i].name });
                std.fs.cwd().deleteFile(backup_path) catch {
                    logger.global_logger.warn("⚠️ 删除旧备份失败: {s}", .{backup_path});
                };
                self.allocator.free(backup_path);
                self.allocator.free(backup_storage[i].name);
                logger.global_logger.info("✓ 已删除旧备份: {s}", .{backup_storage[i].name});
            }
        }
    }

    pub fn getBackupInfo(self: *BackupManager) !struct {
        total_backups: u32,
        total_size_bytes: u64,
        oldest_backup: ?[]const u8,
        newest_backup: ?[]const u8,
    } {
        var backup_dir = std.fs.cwd().openDir(self.backup_dir, .{ .iterate = true }) catch |err| {
            if (err == error.FileNotFound) {
                return .{ .total_backups = 0, .total_size_bytes = 0, .oldest_backup = null, .newest_backup = null };
            }
            return err;
        };
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
                const file = try backup_dir.openFile(entry.name, .{});
                defer file.close();

                const stat = try file.stat();
                total_size += stat.size;

                backup_count += 1;

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

    pub fn deleteBackup(self: *BackupManager, backup_name: []const u8) !void {
        if (self.has_adapter) {
            try self.adapter.delete(backup_name);
            logger.global_logger.info("✓ 已删除备份: {s}", .{backup_name});
        } else {
            const backup_path = try std.fs.path.join(self.allocator, &[_][]const u8{ self.backup_dir, backup_name });
            defer self.allocator.free(backup_path);

            std.fs.cwd().deleteFile(backup_path) catch {
                logger.global_logger.err("❌ 删除备份文件失败: {s}", .{backup_path});
                return BackupError.BackupFailed;
            };

            logger.global_logger.info("✓ 已删除备份: {s}", .{backup_name});
        }
    }

    pub fn listBackups(self: *BackupManager) ![]BackupListItem {
        if (self.has_adapter) {
            const items = try self.adapter.list();
            const result = try self.allocator.alloc(BackupListItem, items.len);
            for (items, 0..) |item, i| {
                result[i] = .{ .name = item.name, .timestamp = item.timestamp, .size_bytes = item.size_bytes };
            }
            self.adapter.freeList(items);
            return result;
        } else {
            var backup_dir = std.fs.cwd().openDir(self.backup_dir, .{ .iterate = true }) catch |err| {
                if (err == error.FileNotFound) {
                    return &.{};
                }
                return err;
            };
            defer backup_dir.close();

            var list = makeBackupList();
            defer list.deinit(self.allocator);

            var iter = backup_dir.iterate();
            while (try iter.next()) |entry| {
                if (entry.kind == .file and std.mem.startsWith(u8, entry.name, "presets_backup_") and std.mem.endsWith(u8, entry.name, ".db")) {
                    const file = try backup_dir.openFile(entry.name, .{});
                    defer file.close();

                    const stat = try file.stat();

                    const name_prefix = "presets_backup_";
                    const name_suffix = ".db";
                    const timestamp_start = name_prefix.len;
                    const timestamp_end = entry.name.len - name_suffix.len;

                    if (timestamp_end > timestamp_start) {
                        const timestamp_str = entry.name[timestamp_start..timestamp_end];
                        const timestamp = std.fmt.parseInt(i64, timestamp_str, 10) catch continue;

                        try list.append(self.allocator, .{
                            .name = try self.allocator.dupe(u8, entry.name),
                            .timestamp = timestamp,
                            .size_bytes = stat.size,
                        });
                    }
                }
            }

            return try list.toOwnedSlice(self.allocator);
        }
    }

    pub fn freeBackupList(self: *BackupManager, items: []BackupListItem) void {
        for (items) |item| {
            self.allocator.free(item.name);
        }
        self.allocator.free(items);
    }
};