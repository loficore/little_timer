//! 预设管理模块 - 计时器预设的增删查改与持久化
//! 架构：内存缓冲 + SQLite 后端
//! - 启动时从 SQLite 全量加载到内存
//! - 修改时同步写入 SQLite（保证持久化）
//! - 前端请求时从内存返回（性能最优）
const std = @import("std");
const interface = @import("../core/interface.zig");
const logger = @import("../core/logger.zig");
const validator = @import("settings_validator.zig");
const settings_json = @import("../storage/storage_json.zig");
const settings_sqlite = @import("../storage/storage_sqlite.zig");

/// 预设管理错误类型
pub const PresetsError = error{
    PresetNameConflict, // 预设名称已存在
    PresetNotFound, // 预设未找到
    PresetNameEmpty, // 预设名称为空
    PresetListFull, // 预设列表已满
    InvalidPresetIndex, // 无效的预设索引
    SqlitePersistenceFailed, // SQLite 持久化失败
};

/// 预设管理器 - 负责预设的内存管理和持久化
pub const PresetsManager = struct {
    /// 预设列表（动态数组，无托管分配器）
    presets: std.ArrayListUnmanaged(interface.TimerPreset),
    /// 最大预设数量限制
    max_count: usize = 999,
    /// 内存分配器
    allocator: std.mem.Allocator,
    /// SQLite 数据库管理器（可选）
    db: ?*settings_sqlite.SqliteManager = null,

    /// 初始化预设管理器（不含数据库）
    ///
    /// 参数:
    /// - **allocator**: 内存分配器
    ///
    /// 返回:
    /// - PresetsManager: 初始化后的预设管理器实例
    pub fn init(allocator: std.mem.Allocator) PresetsManager {
        return .{
            .presets = .{},
            .allocator = allocator,
            .db = null,
        };
    }

    /// 添加预设（同步到 SQLite）
    ///
    /// 参数:
    /// - **self**: PresetsManager 实例指针
    /// - **preset**: 要添加的计时器预设
    ///
    /// 返回:
    /// - PresetsError!void: 如果预设名称无效或已存在则返回错误
    pub fn add(self: *PresetsManager, preset: interface.TimerPreset) !void {
        // 验证预设名称
        validator.validatePresetName(preset.name) catch {
            logger.global_logger.err("错误: 预设名称无效: '{s}'", .{preset.name});
            return error.PresetNameEmpty;
        };

        // 检查数量限制
        if (self.presets.items.len >= self.max_count) {
            logger.global_logger.err("错误: 预设列表已满（最多 {} 个）", .{self.max_count});
            return error.PresetListFull;
        }

        // 检查是否已存在同名预设
        for (self.presets.items) |existing| {
            if (std.mem.eql(u8, existing.name, preset.name)) {
                logger.global_logger.err("错误: 预设名称 '{s}' 已存在", .{preset.name});
                return error.PresetNameConflict;
            }
        }

        // 复制名称到堆上（确保所有权清晰）
        const name_copy = try self.allocator.dupe(u8, preset.name);
        errdefer self.allocator.free(name_copy);

        try self.presets.append(self.allocator, .{
            .name = name_copy,
            .mode = preset.mode,
            .config = preset.config,
        });

        // 同步到 SQLite 后端
        if (self.db != null) {
            // 先将预设配置序列化为 JSON
            const config_json = try settings_json.serializePresetConfigOnly(self.allocator, preset.mode, preset.config);
            defer self.allocator.free(config_json);

            // 写入 SQLite
            self.db.?.insertPreset(preset, config_json) catch |err| {
                logger.global_logger.err("⚠️ SQLite 写入失败: {any}，但内存缓冲已更新", .{err});
                // 不中断内存操作，只记录警告
            };

            logger.global_logger.debug("预设已保存到 SQLite: name={s}, mode={s}, config_json={s}", .{ preset.name, @tagName(preset.mode), config_json });
        }

        logger.global_logger.info("✓ 预设 '{s}' 已添加 (共 {} 个预设)", .{ preset.name, self.presets.items.len });
    }

    /// 移除预设（按索引，同步到 SQLite）
    ///
    /// 参数:
    /// - **self**: PresetsManager 实例指针
    /// - **index**: 预设索引
    ///
    /// 返回:
    /// - PresetsError!void: 如果索引无效则返回错误
    pub fn remove(self: *PresetsManager, index: usize) PresetsError!void {
        if (index >= self.presets.items.len) {
            return error.InvalidPresetIndex;
        }

        // 保存预设名称（用于 SQLite 删除）
        const preset = self.presets.items[index];
        const preset_name = preset.name;

        // 同步删除 SQLite
        if (self.db != null) {
            self.db.?.deletePresetByName(preset_name) catch |err| {
                logger.global_logger.err("⚠️ SQLite 删除失败: {any}，但内存缓冲已更新", .{err});
                // 不中断内存操作
            };
        }

        // 释放名称字符串内存
        self.allocator.free(preset.name);

        // 从数组中移除
        _ = self.presets.orderedRemove(index);

        logger.global_logger.info("✓ 预设已移除 (剩余 {} 个预设)", .{self.presets.items.len});
    }

    /// 移除预设（按名称）
    ///
    /// 参数:
    /// - **self**: PresetsManager 实例指针
    /// - **name**: 预设名称
    ///
    /// 返回:
    /// - PresetsError!void: 如果预设不存在则返回错误
    pub fn removeByName(self: *PresetsManager, name: []const u8) PresetsError!void {
        for (self.presets.items, 0..) |preset, i| {
            if (std.mem.eql(u8, preset.name, name)) {
                return self.remove(i);
            }
        }
        return error.PresetNotFound;
    }

    /// 获取预设（按索引）
    ///
    /// 参数:
    /// - **self**: PresetsManager 实例指针
    /// - **index**: 预设索引
    ///
    /// 返回:
    /// - ?*const interface.TimerPreset: 如果索引有效则返回预设指针，否则返回 null
    pub fn get(self: *const PresetsManager, index: usize) ?*const interface.TimerPreset {
        if (index >= self.presets.items.len) {
            return null;
        }
        return &self.presets.items[index];
    }

    /// 获取所有预设（只读视图）
    pub fn getAll(self: *const PresetsManager) []const interface.TimerPreset {
        return self.presets.items;
    }

    /// 获取预设（按名称）
    ///
    /// 参数:
    /// - **self**: PresetsManager 实例指针
    /// - **name**: 预设名称
    ///
    /// 返回:
    /// - ?*const interface.TimerPreset: 如果预设存在则返回指针，否则返回 null
    pub fn getByName(self: *const PresetsManager, name: []const u8) ?*const interface.TimerPreset {
        for (self.presets.items) |*preset| {
            if (std.mem.eql(u8, preset.name, name)) {
                return preset;
            }
        }
        return null;
    }

    /// 清空所有预设
    ///
    /// 参数:
    /// - **self**: PresetsManager 实例指针
    pub fn clear(self: *PresetsManager) void {
        // 先清空 SQLite 数据库
        if (self.db) |db| {
            db.clearAllPresets() catch |err| {
                logger.global_logger.err("⚠️ 清空 SQLite 预设失败: {any}", .{err});
            };
        }

        // 释放所有名称字符串
        for (self.presets.items) |preset| {
            self.allocator.free(preset.name);
        }
        self.presets.clearRetainingCapacity();
        logger.global_logger.info("✓ 所有预设已清空", .{});
    }

    /// 获取预设数量
    ///
    /// 参数:
    /// - **self**: PresetsManager 实例指针
    ///
    /// 返回:
    /// - usize: 当前预设数量
    pub fn count(self: *const PresetsManager) usize {
        return self.presets.items.len;
    }

    /// 保存预设到 JSON 文件
    ///
    /// 参数:
    /// - **self**: PresetsManager 实例指针
    /// - **path**: 文件路径
    ///
    /// 返回:
    /// - !void: 如果保存失败则返回错误
    pub fn saveToFile(self: *const PresetsManager, path: []const u8) !void {
        // 委托给 settings_json 模块进行序列化
        const json_str = try settings_json.serializePresetsOnly(self.allocator, self);
        defer self.allocator.free(json_str);

        const file = std.fs.cwd().createFile(path, .{ .truncate = true }) catch |err| {
            logger.global_logger.err("无法创建预设文件 {s}: {}", .{ path, err });
            return err;
        };
        defer file.close();

        try file.writeAll(json_str);
        logger.global_logger.info("✓ 预设已保存到 {s} ({} 个预设)", .{ path, self.presets.items.len });
    }

    /// 从 JSON 文件加载预设（仅用于人工迁移/导入）
    ///
    /// 参数:
    /// - **self**: PresetsManager 实例指针
    /// - **path**: 文件路径
    /// - **default_timezone**: 默认时区（用于 world_clock 预设）
    ///
    /// 返回:
    /// - !void: 如果加载失败则返回错误
    pub fn loadFromJsonFile(self: *PresetsManager, path: []const u8, default_timezone: i8) !void {
        const file = std.fs.cwd().openFile(path, .{}) catch |err| {
            logger.global_logger.debug("预设文件 {s} 不存在或无法打开: {}", .{ path, err });
            return err;
        };
        defer file.close();

        const size = try file.getEndPos();
        const buf = try self.allocator.alloc(u8, @intCast(size));
        defer self.allocator.free(buf);

        _ = try file.readAll(buf);

        // 委托给 settings_json 模块进行反序列化和合法性校验
        try settings_json.deserializePresetsOnly(self.allocator, buf, self, default_timezone);
        logger.global_logger.info("✓ 已从 JSON 文件 {s} 加载 {} 个预设", .{ path, self.presets.items.len });
    }

    /// 从 JSON 文件加载预设的别名方法（用于 SettingsManager）
    pub fn loadFromFile(self: *PresetsManager, path: []const u8, default_timezone: i8) !void {
        return self.loadFromJsonFile(path, default_timezone);
    }

    /// 从 SQLite 数据库加载所有预设到内存（启动时调用）
    ///
    /// 参数:
    /// - **self**: PresetsManager 实例指针
    /// - **db**: SQLite 数据库管理器
    ///
    /// 返回:
    /// - !void: 如果加载失败则返回错误
    pub fn loadFromSqlite(self: *PresetsManager, db: *settings_sqlite.SqliteManager) !void {
        // 确保数据库已打开
        if (!db.is_initialized) {
            try db.open();
        }

        // 查询所有预设
        var rows = try db.queryAllPresets(self.allocator);
        defer {
            for (rows.items) |row| {
                // 只释放 config_json，name 的所有权已转移给预设
                self.allocator.free(row.config_json);
            }
            rows.deinit(self.allocator);
        }

        // 将预设加载到内存（转移 name 所有权）
        for (rows.items) |row| {
            const config = try settings_json.parsePresetConfigJson(self.allocator, row.config_json);

            try self.presets.append(self.allocator, .{
                .name = row.name, // 转移所有权
                .mode = row.mode,
                .config = config,
            });
        }

        // 保存数据库引用
        self.db = db;

        logger.global_logger.info("✓ 已从 SQLite 加载 {} 个预设到内存", .{self.presets.items.len});
    }

    /// 自动迁移机制：如果 JSON 存在但 SQLite 为空，则迁移旧预设
    ///
    /// 参数:
    /// - **self**: PresetsManager 实例指针
    /// - **db**: SQLite 数据库管理器
    /// - **json_path**: 旧 JSON 文件路径
    /// - **default_timezone**: 默认时区
    ///
    /// 返回:
    /// - !bool: 如果成功迁移则返回 true，否则返回 false
    pub fn autoMigrateFromJson(self: *PresetsManager, db: *settings_sqlite.SqliteManager, json_path: []const u8, default_timezone: i8) !bool {
        // 检查 JSON 文件是否存在
        const json_exists = std.fs.cwd().accessZ(json_path, std.fs.File.OpenFlags{}) catch false;
        if (!json_exists) {
            logger.global_logger.debug("JSON 预设文件不存在，无需迁移", .{});
            return false;
        }

        // 检查 SQLite 是否已有预设
        if (self.presets.items.len > 0) {
            logger.global_logger.debug("SQLite 已有预设，跳过 JSON 迁移", .{});
            return false;
        }

        logger.global_logger.info("🔄 检测到旧的 JSON 预设文件，准备迁移...", .{});

        // 尝试从 JSON 加载
        self.loadFromJsonFile(json_path, default_timezone) catch |err| {
            logger.global_logger.err("❌ 从 JSON 迁移失败: {any}", .{err});
            return false;
        };

        // 将所有加载的预设写入 SQLite
        for (self.presets.items) |preset| {
            const config_json = try settings_json.serializePresetConfigOnly(self.allocator, preset.mode, preset.config);
            defer self.allocator.free(config_json);

            db.insertPreset(preset, config_json) catch |err| {
                logger.global_logger.warn("⚠️ 迁移预设 '{s}' 到 SQLite 失败: {any}", .{ preset.name, err });
            };
        }

        // 备份旧的 JSON 文件
        const backup_path = try std.fmt.allocPrint(self.allocator, "{s}.backup", .{json_path});
        defer self.allocator.free(backup_path);

        std.fs.cwd().copyFile(json_path, std.fs.cwd(), backup_path, .{}) catch |err| {
            logger.global_logger.warn("⚠️ 备份 JSON 文件失败: {any}", .{err});
        };

        logger.global_logger.info("✅ 从 JSON 成功迁移 {} 个预设到 SQLite（旧文件已备份）", .{self.presets.items.len});
        return true;
    }

    /// 清理预设管理器资源
    ///
    /// 参数:
    /// - **self**: PresetsManager 实例指针
    pub fn deinit(self: *PresetsManager) void {
        // 只释放预设数据，不关闭数据库（由 SettingsManager 负责关闭）
        // 释放所有名称字符串
        for (self.presets.items) |preset| {
            self.allocator.free(preset.name);
        }
        self.presets.deinit(self.allocator);
    }
};
