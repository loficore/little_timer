//! 数据库CRUD操作模块
//! 职责：设置的具体增删改查操作
const std = @import("std");
const zqlite = @import("zqlite");
const interface = @import("../core/interface.zig");
const logger = @import("../core/logger.zig");
const crypto = @import("../core/utils/crypto.zig");
const secret_storage = @import("../core/utils/secret_storage.zig");

/// CRUD 操作错误类型
pub const CrudError = error{
    InsertFailed, // 插入失败
    DeleteFailed, // 删除失败
    QueryFailed, // 查询失败
    SettingsNotFound, // 设置未找到
    SettingsSaveFailed, // 设置保存失败
    DatabaseOpenFailed, // 数据库打开失败
    EncryptionFailed, // 加密失败
    DecryptionFailed, // 解密失败
    MasterKeyNotFound, // 主密钥未找到
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
            logger.global_logger.err("❌ saveSettings: db is null", .{});
            return CrudError.DatabaseOpenFailed;
        }

        // 转换 DefaultMode 到字符串
        const default_mode_str = switch (config.basic.default_mode) {
            .countdown => "countdown",
            .stopwatch => "stopwatch",
        };

        logger.global_logger.debug("保存设置到数据库...", .{});

        // 使用 UPSERT 操作（INSERT OR REPLACE）
        self.db.?.exec(
            "INSERT OR REPLACE INTO settings (id, timezone, language, default_mode, theme_mode, wallpaper, duration_seconds, countdown_loop, countdown_loop_count, countdown_loop_interval, stopwatch_max_seconds, log_level, log_enable_timestamp, log_tick_interval) VALUES (1, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);",
            .{
                config.basic.timezone,
                config.basic.language,
                default_mode_str,
                config.basic.theme_mode,
                config.basic.wallpaper,
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
            logger.global_logger.err("loadSettings: db is null", .{});
            return CrudError.DatabaseOpenFailed;
        }

        logger.global_logger.debug("从数据库加载设置...", .{});

        var rows = self.db.?.rows(
            "SELECT timezone, language, default_mode, theme_mode, COALESCE(wallpaper, ''), duration_seconds, countdown_loop, countdown_loop_count, countdown_loop_interval, stopwatch_max_seconds, log_level, log_enable_timestamp, log_tick_interval FROM settings WHERE id = 1;",
            .{},
        ) catch |err| {
            logger.global_logger.err("加载设置查询失败: {any}", .{err});
            return CrudError.QueryFailed;
        };
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
        const wallpaper = row.get([]const u8, 4);
        const duration_seconds_raw = row.get(i64, 5);
        const countdown_loop_raw = row.get(i64, 6);
        const countdown_loop_count_raw = row.get(i64, 7);
        const countdown_loop_interval_raw = row.get(i64, 8);
        const stopwatch_max_seconds_raw = row.get(i64, 9);
        const log_level = row.get([]const u8, 10);
        const log_enable_timestamp_raw = row.get(i64, 11);
        const log_tick_interval = row.get(i64, 12);

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
        const wallpaper_copy = try allocator.dupe(u8, wallpaper);
        errdefer allocator.free(wallpaper_copy);
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
                .wallpaper = wallpaper_copy,
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

    /// 保存备份配置到 SQLite，凭证使用 master_key 加密
    ///
    /// 参数:
    /// - **config**: 要保存的备份配置
    ///
    /// 返回:
    /// - !void: 如果保存失败则返回错误
    pub fn saveBackupConfig(self: *CrudManager, config: interface.BackupConfig) !void {
        if (self.db == null) {
            logger.global_logger.err("❌ saveBackupConfig: db is null", .{});
            return CrudError.DatabaseOpenFailed;
        }

        const target_type_str = switch (config.target_type) {
            .local => "local",
            .webdav => "webdav",
            .s3 => "s3",
        };

        logger.global_logger.debug("保存备份配置到数据库（加密凭证）...", .{});

        // 确保 master key 存在，如果不存在则自动生成
        var master_key: []u8 = undefined;
        const key_err = secret_storage.retrieveMasterKey();
        if (key_err) |key| {
            master_key = key;
        } else |err| {
            logger.global_logger.info("Master key 不存在 (err={any})，自动生成并存储到 keyring...", .{err});
            const new_key = try crypto.generateKeyOwned(self.allocator);
            try secret_storage.storeMasterKey(new_key);
            master_key = try secret_storage.retrieveMasterKey();
            self.allocator.free(new_key);
        }
        // master_key 来自 keyring 缓存，secret_storage 内部管理生命周期

        const encrypt_credential = struct {
            fn func(label: []const u8, plaintext: []const u8, key: []const u8, allocator: std.mem.Allocator) ![]u8 {
                if (plaintext.len == 0) {
                    return allocator.alloc(u8, 0);
                }
                if (key.len == 0) {
                    logger.global_logger.err("加密失败: key 为空 (field={s})", .{label});
                    return error.EncryptionFailed;
                }
                var key_arr: [crypto.AES256GCM_KEY_SIZE]u8 = undefined;
                @memcpy(key_arr[0..@min(key.len, crypto.AES256GCM_KEY_SIZE)], key[0..@min(key.len, crypto.AES256GCM_KEY_SIZE)]);
                const nonce = crypto.generateNonce();
                const ciphertext_len = plaintext.len + crypto.AES256GCM_TAG_SIZE;
                const result = try allocator.alloc(u8, ciphertext_len);
                crypto.encrypt(plaintext, key_arr, nonce, result) catch {
                    allocator.free(result);
                    logger.global_logger.err("加密失败: encrypt 错误 (field={s}, plaintext_len={d}, key_len={d})", .{label, plaintext.len, key.len});
                    return error.EncryptionFailed;
                };
                var combined = try allocator.alloc(u8, nonce.len + ciphertext_len);
                @memcpy(combined[0..nonce.len], &nonce);
                @memcpy(combined[nonce.len..], result);
                allocator.free(result);
                return combined;
            }
        }.func;

        const webdav_password_encrypted = if (config.target_type == .webdav)
            try encrypt_credential("webdav_password", config.webdav_password, master_key, self.allocator)
        else
            try self.allocator.alloc(u8, 0);
        defer self.allocator.free(webdav_password_encrypted);

        const s3_access_key_encrypted = if (config.target_type == .s3)
            try encrypt_credential("s3_access_key", config.s3_access_key, master_key, self.allocator)
        else
            try self.allocator.alloc(u8, 0);
        defer self.allocator.free(s3_access_key_encrypted);

        const s3_secret_key_encrypted = if (config.target_type == .s3)
            try encrypt_credential("s3_secret_key", config.s3_secret_key, master_key, self.allocator)
        else
            try self.allocator.alloc(u8, 0);
        defer self.allocator.free(s3_secret_key_encrypted);

        try self.db.?.exec(
            "INSERT OR REPLACE INTO backup_config (id, target_type, enabled, auto_backup, auto_backup_interval, local_path, webdav_url, webdav_username, webdav_password_encrypted, s3_endpoint, s3_bucket, s3_region, s3_access_key_encrypted, s3_secret_key_encrypted, s3_path_prefix) VALUES (1, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);",
            .{
                target_type_str,
                @intFromBool(config.enabled),
                @intFromBool(config.auto_backup),
                config.auto_backup_interval,
                config.local_path,
                config.webdav_url,
                config.webdav_username,
                webdav_password_encrypted,
                config.s3_endpoint,
                config.s3_bucket,
                config.s3_region,
                s3_access_key_encrypted,
                s3_secret_key_encrypted,
                config.s3_path_prefix,
            },
        );

        logger.global_logger.info("✓ 备份配置已保存到 SQLite（凭证已加密）", .{});
    }

    /// 从 SQLite 加载备份配置，凭证使用 master_key 解密
    ///
    /// 参数:
    /// - **allocator**: 内存分配器
    ///
    /// 返回:
    /// - !interface.BackupConfig: 加载的备份配置
    pub fn loadBackupConfig(self: *CrudManager, allocator: std.mem.Allocator) !interface.BackupConfig {
        if (self.db == null) {
            logger.global_logger.err("loadBackupConfig: db is null", .{});
            return CrudError.DatabaseOpenFailed;
        }

        logger.global_logger.debug("从数据库加载备份配置（解密凭证）...", .{});

        var rows = self.db.?.rows(
            "SELECT target_type, enabled, auto_backup, auto_backup_interval, COALESCE(local_path, ''), COALESCE(webdav_url, ''), COALESCE(webdav_username, ''), COALESCE(webdav_password_encrypted, ''), COALESCE(s3_endpoint, ''), COALESCE(s3_bucket, ''), COALESCE(s3_region, ''), COALESCE(s3_access_key_encrypted, ''), COALESCE(s3_secret_key_encrypted, ''), COALESCE(s3_path_prefix, 'little_timer/') FROM backup_config WHERE id = 1;",
            .{},
        ) catch |err| {
            logger.global_logger.err("加载备份配置查询失败: {any}", .{err});
            return CrudError.QueryFailed;
        };
        defer rows.deinit();

        const row = rows.next() orelse {
            logger.global_logger.warn("⚠️ 未找到备份配置数据，返回默认配置", .{});
            return interface.BackupConfig{};
        };

        const target_type_str = row.get([]const u8, 0);
        const enabled_raw = row.get(i64, 1);
        const auto_backup_raw = row.get(i64, 2);
        const auto_backup_interval_raw = row.get(i64, 3);
        const local_path = row.get([]const u8, 4);
        const webdav_url = row.get([]const u8, 5);
        const webdav_username = row.get([]const u8, 6);
        const webdav_password_encrypted = row.get([]const u8, 7);
        const s3_endpoint = row.get([]const u8, 8);
        const s3_bucket = row.get([]const u8, 9);
        const s3_region = row.get([]const u8, 10);
        const s3_access_key_encrypted = row.get([]const u8, 11);
        const s3_secret_key_encrypted = row.get([]const u8, 12);
        const s3_path_prefix = row.get([]const u8, 13);

        const target_type: interface.BackupTargetType = if (std.mem.eql(u8, target_type_str, "webdav"))
            .webdav
        else if (std.mem.eql(u8, target_type_str, "s3"))
            .s3
        else
            .local;

        const master_key = secret_storage.retrieveMasterKey() catch |key_err| {
            logger.global_logger.warn("retrieveMasterKey failed: {any}, credentials will not be decrypted", .{key_err});
            return interface.BackupConfig{
                .enabled = enabled_raw != 0,
                .auto_backup = auto_backup_raw != 0,
                .auto_backup_interval = @intCast(auto_backup_interval_raw),
                .target_type = target_type,
                .local_path = try allocator.dupe(u8, local_path),
                .webdav_url = try allocator.dupe(u8, webdav_url),
                .webdav_username = try allocator.dupe(u8, webdav_username),
                .webdav_password = try allocator.dupe(u8, ""),
                .s3_endpoint = try allocator.dupe(u8, s3_endpoint),
                .s3_bucket = try allocator.dupe(u8, s3_bucket),
                .s3_region = try allocator.dupe(u8, s3_region),
                .s3_access_key = try allocator.dupe(u8, ""),
                .s3_secret_key = try allocator.dupe(u8, ""),
                .s3_path_prefix = try allocator.dupe(u8, s3_path_prefix),
            };
        };
        defer allocator.free(master_key);

        const decrypt_credential = struct {
            fn func(encrypted: []const u8, key: []const u8, alloc: std.mem.Allocator) []u8 {
                if (encrypted.len == 0 or encrypted.len < crypto.AES256GCM_NONCE_SIZE + crypto.AES256GCM_TAG_SIZE) {
                    return &[_]u8{};
                }
                var key_arr: [crypto.AES256GCM_KEY_SIZE]u8 = undefined;
                @memcpy(key_arr[0..@min(key.len, crypto.AES256GCM_KEY_SIZE)], key[0..@min(key.len, crypto.AES256GCM_KEY_SIZE)]);
                const nonce: *const [crypto.AES256GCM_NONCE_SIZE]u8 = @ptrCast(@alignCast(encrypted[0..crypto.AES256GCM_NONCE_SIZE]));
                const ciphertext = encrypted[crypto.AES256GCM_NONCE_SIZE..];
                const plaintext = alloc.alloc(u8, ciphertext.len - crypto.AES256GCM_TAG_SIZE) catch return &[_]u8{};
                crypto.decrypt(ciphertext, key_arr, nonce.*, plaintext) catch {
                    alloc.free(plaintext);
                    return &[_]u8{};
                };
                return plaintext;
            }
        }.func;

        const webdav_password = decrypt_credential(webdav_password_encrypted, master_key, allocator);
        errdefer if (webdav_password.len > 0) allocator.free(webdav_password);

        const s3_access_key = decrypt_credential(s3_access_key_encrypted, master_key, allocator);
        errdefer if (s3_access_key.len > 0) allocator.free(s3_access_key);

        const s3_secret_key = decrypt_credential(s3_secret_key_encrypted, master_key, allocator);
        errdefer if (s3_secret_key.len > 0) allocator.free(s3_secret_key);

        const backup_config = interface.BackupConfig{
            .enabled = enabled_raw != 0,
            .auto_backup = auto_backup_raw != 0,
            .auto_backup_interval = @intCast(auto_backup_interval_raw),
            .target_type = target_type,
            .local_path = try allocator.dupe(u8, local_path),
            .webdav_url = try allocator.dupe(u8, webdav_url),
            .webdav_username = try allocator.dupe(u8, webdav_username),
            .webdav_password = webdav_password,
            .s3_endpoint = try allocator.dupe(u8, s3_endpoint),
            .s3_bucket = try allocator.dupe(u8, s3_bucket),
            .s3_region = try allocator.dupe(u8, s3_region),
            .s3_access_key = s3_access_key,
            .s3_secret_key = s3_secret_key,
            .s3_path_prefix = try allocator.dupe(u8, s3_path_prefix),
        };

        logger.global_logger.info("✓ 已从 SQLite 加载备份配置（凭证已解密）", .{});
        return backup_config;
    }
};
