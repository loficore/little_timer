//! SQLite 数据库管理器 - 主模块
//! 职责：协调各个子模块，提供统一的数据库管理接口
const std = @import("std");
const zqlite = @import("zqlite");
const interface = @import("../core/interface.zig");
const logger = @import("../core/logger.zig");

// 导入子模块
const migration = @import("storage_migration.zig");
const health = @import("storage_health.zig");
const backup = @import("storage_backup.zig");
const crud = @import("storage_crud.zig");
const habit_crud = @import("habit_crud.zig");

/// SQLite 错误类型
pub const SqliteError = error{
    DatabaseOpenFailed, // 数据库打开失败
    TableCreationFailed, // 表创建失败
    InsertFailed, // 插入失败
    DeleteFailed, // 删除失败
    QueryFailed, // 查询失败
    DatabaseCorrupted, // 数据库损坏
    SettingsNotFound, // 设置未找到
    SettingsSaveFailed, // 设置保存失败
    InvalidSchemaVersion, // 无效的数据库模式版本
    MigrationFailed, // 数据库迁移失败
    BackupFailed, // 备份失败
    RestoreFailed, // 恢复失败
    DatabaseNotHealthy, // 数据库不健康
    IntegrityCheckFailed, // 完整性检查失败
    HealthCheckFailed, // 健康检查失败
};

/// SQLite 数据库管理器（模块化版本）
pub const SqliteManager = struct {
    db: ?zqlite.Conn = null,
    allocator: std.mem.Allocator,
    db_path: [:0]const u8, // 数据库文件路径（以 null 结尾）
    is_initialized: bool = false,
    backup_dir: []const u8, // 备份目录路径
    max_backups: u32 = 10, // 最大备份文件数量

    // 子模块实例
    migration_manager: migration.MigrationManager,
    health_manager: health.HealthCheckManager,
    backup_manager: backup.BackupManager,
    crud_manager: crud.CrudManager,
    habit_manager: habit_crud.HabitCrudManager,

    /// 初始化 SQLite 管理器
    ///
    /// 参数:
    /// - **allocator**: 内存分配器
    /// - **db_path**: 数据库文件路径（以 null 结尾）
    /// - **backup_dir**: 备份目录路径
    ///
    /// 返回:
    /// - !SqliteManager: 初始化后的管理器实例
    pub fn init(allocator: std.mem.Allocator, db_path: [:0]const u8, backup_dir: []const u8) !SqliteManager {
        var manager: SqliteManager = .{
            .allocator = allocator,
            .db_path = db_path,
            .backup_dir = backup_dir,
            .migration_manager = undefined,
            .health_manager = undefined,
            .backup_manager = undefined,
            .crud_manager = undefined,
            .habit_manager = undefined,
        };

        // 初始化子模块（暂时设置为 null，open 时会设置）
        manager.migration_manager = migration.MigrationManager.init(allocator, null);
        manager.health_manager = health.HealthCheckManager.init(allocator, null);
        manager.backup_manager = backup.BackupManager.init(allocator, null, db_path, backup_dir);
        manager.crud_manager = crud.CrudManager.init(allocator, null);
        manager.habit_manager = habit_crud.HabitCrudManager.init(allocator, null);

        return manager;
    }

    /// 打开或创建数据库
    ///
    /// 参数:
    /// - **self**: SqliteManager 实例指针
    ///
    /// 返回:
    /// - !void: 如果打开失败则返回错误
    pub fn open(self: *SqliteManager) !void {
        if (self.is_initialized) {
            logger.global_logger.debug("数据库已打开，跳过重复初始化", .{});
            return;
        }

        // 使用 zqlite 高级 API 打开数据库
        const flags = zqlite.OpenFlags.Create | zqlite.OpenFlags.ReadWrite;
        self.db = zqlite.open(self.db_path, flags) catch |err| {
            logger.global_logger.err("❌ SQLite 打开失败: {any}", .{err});
            return SqliteError.DatabaseOpenFailed;
        };

        self.is_initialized = true;
        logger.global_logger.info("✓ SQLite 数据库已打开: {s}", .{self.db_path});

        // 更新子模块的数据库连接
        self.migration_manager.db = self.db;
        self.health_manager.db = self.db;
        self.backup_manager.db = self.db;
        self.crud_manager.db = self.db;
        self.habit_manager.db = self.db;

        // 检查并执行数据库迁移
        try self.migration_manager.checkAndMigrate();

        // 初始化健康检查
        try self.health_manager.initialize();

        // 执行数据库健康检查
        try self.health_manager.performCheck();
    }

    /// 关闭数据库
    ///
    /// 参数:
    /// - **self**: SqliteManager 实例指针
    pub fn close(self: *SqliteManager) void {
        if (self.db != null) {
            self.db.?.close();
            self.db = null;
            self.is_initialized = false;

            // 清理子模块的数据库连接
            self.migration_manager.db = null;
            self.health_manager.db = null;
            self.backup_manager.db = null;
            self.crud_manager.db = null;
            self.habit_manager.db = null;

            logger.global_logger.info("✓ SQLite 数据库已关闭", .{});
        }
    }

    /// 清理资源
    ///
    /// 参数:
    /// - **self**: SqliteManager 实例指针
    pub fn deinit(self: *SqliteManager) void {
        self.close();
    }

    // === CRUD 操作代理方法 ===

    /// 保存设置到 SQLite
    pub fn saveSettings(self: *SqliteManager, config: interface.SettingsConfig) !void {
        return self.crud_manager.saveSettings(config);
    }

    /// 从 SQLite 加载设置
    pub fn loadSettings(self: *SqliteManager, allocator: std.mem.Allocator) !interface.SettingsConfig {
        return self.crud_manager.loadSettings(allocator);
    }

    // === 健康检查代理方法 ===

    /// 执行数据库健康检查
    pub fn performHealthCheck(self: *SqliteManager) !void {
        try self.health_manager.performCheck();
    }

    /// 执行深度健康检查
    pub fn performDeepHealthCheck(self: *SqliteManager) !health.HealthCheckInfo {
        return self.health_manager.performDeepCheck();
    }

    /// 检查数据库是否健康
    pub fn isHealthy(self: *SqliteManager) !bool {
        return self.health_manager.isHealthy();
    }

    /// 获取健康检查信息
    pub fn getHealthInfo(self: *SqliteManager) !health.HealthCheckInfo {
        return self.health_manager.getInfo();
    }

    // === 备份恢复代理方法 ===

    /// 创建数据库备份
    pub fn createBackup(self: *SqliteManager) ![]const u8 {
        return self.backup_manager.createBackup();
    }

    /// 从备份恢复数据库
    pub fn restoreFromBackup(self: *SqliteManager, backup_path: []const u8) !void {
        try self.backup_manager.restoreFromBackup(backup_path, reopenDb);
    }

    /// 获取备份目录信息
    pub fn getBackupInfo(self: *SqliteManager) !backup.BackupInfo {
        return self.backup_manager.getBackupInfo();
    }

    /// 清理已分配的备份信息
    pub fn freeBackupInfo(self: *SqliteManager, info: backup.BackupInfo) void {
        self.backup_manager.freeBackupInfo(info);
    }

    // === 内部方法 ===

    /// 重新打开数据库的回调函数
    fn reopenDb(manager: *SqliteManager) !void {
        const flags = zqlite.OpenFlags.Create | zqlite.OpenFlags.ReadWrite;
        manager.db = zqlite.open(manager.db_path, flags) catch |err| {
            logger.global_logger.err("❌ 重新打开数据库失败: {any}", .{err});
            return SqliteError.DatabaseOpenFailed;
        };

        // 更新子模块的数据库连接
        manager.migration_manager.db = manager.db;
        manager.health_manager.db = manager.db;
        manager.backup_manager.db = manager.db;
        manager.crud_manager.db = manager.db;
        manager.habit_manager.db = manager.db;

        // 重新初始化
        try manager.migration_manager.checkAndMigrate();
        try manager.health_manager.performCheck();
    }
};

// 类型别名，用于向后兼容
pub const SettingsRow = crud.SettingsRow;
pub const HealthCheckInfo = health.HealthCheckInfo;

// 备份信息类型的别名
pub const BackupInfo = struct {
    total_backups: u32,
    total_size_bytes: u64,
    oldest_backup: ?[]const u8,
    newest_backup: ?[]const u8,
};
