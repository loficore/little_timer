//! 预设管理模块 - 计时器预设的增删查改与持久化
const std = @import("std");
const interface = @import("interface.zig");
const logger = @import("logger.zig");
const validator = @import("settings_validator.zig");
const settings_json = @import("settings_json.zig");

/// 预设管理错误类型
pub const PresetsError = error{
    PresetNameConflict, // 预设名称已存在
    PresetNotFound, // 预设未找到
    PresetNameEmpty, // 预设名称为空
    PresetListFull, // 预设列表已满
    InvalidPresetIndex, // 无效的预设索引
};

/// 预设管理器 - 负责预设的内存管理和持久化
pub const PresetsManager = struct {
    /// 预设列表（动态数组，无托管分配器）
    presets: std.ArrayListUnmanaged(interface.TimerPreset),
    /// 最大预设数量限制
    max_count: usize = 999,
    /// 内存分配器
    allocator: std.mem.Allocator,

    /// 初始化预设管理器
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
        };
    }

    /// 添加预设
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

        logger.global_logger.info("✓ 预设 '{s}' 已添加 (共 {} 个预设)", .{ preset.name, self.presets.items.len });
    }

    /// 移除预设（按索引）
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

        // 释放名称字符串内存
        const preset = self.presets.items[index];
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

    /// 从 JSON 文件加载预设
    ///
    /// 参数:
    /// - **self**: PresetsManager 实例指针
    /// - **path**: 文件路径
    /// - **default_timezone**: 默认时区（用于 world_clock 预设）
    ///
    /// 返回:
    /// - !void: 如果加载失败则返回错误
    pub fn loadFromFile(self: *PresetsManager, path: []const u8, default_timezone: i8) !void {
        const file = std.fs.cwd().openFile(path, .{}) catch |err| {
            logger.global_logger.debug("预设文件 {s} 不存在或无法打开: {}", .{ path, err });
            return err;
        };
        defer file.close();

        const size = try file.getEndPos();
        const buf = try self.allocator.alloc(u8, @intCast(size));
        defer self.allocator.free(buf);

        _ = try file.readAll(buf);

        // 委托给 settings_json 模块进行反序列化
        try settings_json.deserializePresetsOnly(self.allocator, buf, self, default_timezone);
        logger.global_logger.info("✓ 已从 {s} 加载 {} 个预设", .{ path, self.presets.items.len });
    }

    /// 清理预设管理器资源
    ///
    /// 参数:
    /// - **self**: PresetsManager 实例指针
    pub fn deinit(self: *PresetsManager) void {
        // 释放所有名称字符串
        for (self.presets.items) |preset| {
            self.allocator.free(preset.name);
        }
        self.presets.deinit(self.allocator);
    }
};
