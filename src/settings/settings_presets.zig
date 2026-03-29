//! 预设管理模块 - 计时器预设的增删查改与持久化
//! 架构：内存缓冲 + JSON 文件后端
const std = @import("std");
const interface = @import("../core/interface.zig");
const logger = @import("../core/logger.zig");
const validator = @import("settings_validator.zig");
const settings_json = @import("../storage/storage_json.zig");

/// 预设管理错误类型
pub const PresetsError = error{
    PresetNameConflict, // 预设名称已存在
    PresetNotFound, // 预设未找到
    PresetNameEmpty, // 预设名称为空
    PresetListFull, // 预设列表已满
    InvalidPresetIndex, // 无效的预设索引
    JsonPersistenceFailed, // JSON 持久化失败
};

/// 预设管理器 - 负责预设的内存管理和持久化
pub const PresetsManager = struct {
    presets: std.ArrayListUnmanaged(interface.TimerPreset),
    max_count: usize = 999,
    allocator: std.mem.Allocator,

    pub fn init(allocator: std.mem.Allocator) PresetsManager {
        return .{
            .presets = .{},
            .allocator = allocator,
        };
    }

    pub fn add(self: *PresetsManager, preset: interface.TimerPreset) !void {
        validator.validatePresetName(preset.name) catch {
            logger.global_logger.err("错误: 预设名称无效: '{s}'", .{preset.name});
            return error.PresetNameEmpty;
        };

        if (self.presets.items.len >= self.max_count) {
            logger.global_logger.err("错误: 预设列表已满（最多 {} 个）", .{self.max_count});
            return error.PresetListFull;
        }

        for (self.presets.items) |existing| {
            if (std.mem.eql(u8, existing.name, preset.name)) {
                logger.global_logger.err("错误: 预设名称 '{s}' 已存在", .{preset.name});
                return error.PresetNameConflict;
            }
        }

        const name_copy = try self.allocator.dupe(u8, preset.name);
        errdefer self.allocator.free(name_copy);

        try self.presets.append(self.allocator, .{
            .name = name_copy,
            .mode = preset.mode,
            .config = preset.config,
        });

        logger.global_logger.info("✓ 预设 '{s}' 已添加 (共 {} 个预设)", .{ preset.name, self.presets.items.len });
    }

    pub fn remove(self: *PresetsManager, index: usize) PresetsError!void {
        if (index >= self.presets.items.len) {
            return error.InvalidPresetIndex;
        }

        const preset = self.presets.items[index];
        self.allocator.free(preset.name);
        _ = self.presets.orderedRemove(index);

        logger.global_logger.info("✓ 预设已移除 (剩余 {} 个预设)", .{self.presets.items.len});
    }

    pub fn removeByName(self: *PresetsManager, name: []const u8) PresetsError!void {
        for (self.presets.items, 0..) |preset, i| {
            if (std.mem.eql(u8, preset.name, name)) {
                return self.remove(i);
            }
        }
        return error.PresetNotFound;
    }

    pub fn get(self: *const PresetsManager, index: usize) ?*const interface.TimerPreset {
        if (index >= self.presets.items.len) {
            return null;
        }
        return &self.presets.items[index];
    }

    pub fn getAll(self: *const PresetsManager) []const interface.TimerPreset {
        return self.presets.items;
    }

    pub fn getByName(self: *const PresetsManager, name: []const u8) ?*const interface.TimerPreset {
        for (self.presets.items) |*preset| {
            if (std.mem.eql(u8, preset.name, name)) {
                return preset;
            }
        }
        return null;
    }

    pub fn clear(self: *PresetsManager) void {
        for (self.presets.items) |preset| {
            self.allocator.free(preset.name);
        }
        self.presets.clearRetainingCapacity();
        logger.global_logger.info("✓ 所有预设已清空", .{});
    }

    pub fn count(self: *const PresetsManager) usize {
        return self.presets.items.len;
    }

    pub fn saveToFile(self: *const PresetsManager, path: []const u8) !void {
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

        try settings_json.deserializePresetsOnly(self.allocator, buf, self, default_timezone);
        logger.global_logger.info("✓ 已从 JSON 文件 {s} 加载 {} 个预设", .{ path, self.presets.items.len });
    }

    pub fn deinit(self: *PresetsManager) void {
        for (self.presets.items) |preset| {
            self.allocator.free(preset.name);
        }
        self.presets.deinit(self.allocator);
    }
};
