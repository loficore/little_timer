//! 预设管理模块 - 已废弃
//! 此模块保留仅为兼容性目的，不再使用
const std = @import("std");
const logger = @import("../core/logger.zig");
const interface = @import("../core/interface.zig");

pub const PresetsError = error{
    InvalidName,
    AlreadyExists,
    NotFound,
    LimitReached,
};

pub const PresetsManager = struct {
    presets: std.ArrayListUnmanaged(interface.TimerPreset) = .{},
    max_count: usize = 999,

    pub fn init(allocator: std.mem.Allocator) PresetsManager {
        _ = allocator;
        return .{};
    }

    pub fn add(_: *PresetsManager, preset: interface.TimerPreset) !void {
        _ = preset;
        logger.global_logger.debug("预设功能已废弃", .{});
    }

    pub fn remove(_: *PresetsManager, _: usize) void {}

    pub fn get(_: *const PresetsManager, _: usize) ?*const interface.TimerPreset {
        return null;
    }

    pub fn getAll(_: *const PresetsManager) []const interface.TimerPreset {
        return &.{};
    }

    pub fn getByName(_: *const PresetsManager, _: []const u8) ?*const interface.TimerPreset {
        return null;
    }

    pub fn count(_: *const PresetsManager) usize {
        return 0;
    }

    pub fn clear(_: *PresetsManager) void {}

    pub fn deinit(_: *PresetsManager) void {}
};
