//! settings_presets 模块单元测试（已废弃模块）
const std = @import("std");
const presets = @import("../settings/settings_presets.zig");
const interface = @import("../core/interface.zig");

test "PresetsManager init returns empty manager" {
    const allocator = std.testing.allocator;
    var manager = presets.PresetsManager.init(allocator);

    try std.testing.expectEqual(@as(usize, 0), manager.presets.items.len);
    try std.testing.expectEqual(@as(usize, 999), manager.max_count);
}

test "count always returns 0 for deprecated module" {
    const allocator = std.testing.allocator;
    var manager = presets.PresetsManager.init(allocator);

    try std.testing.expectEqual(@as(usize, 0), manager.count());
}

test "get always returns null for deprecated module" {
    const allocator = std.testing.allocator;
    var manager = presets.PresetsManager.init(allocator);

    try std.testing.expect(null == manager.get(0));
    try std.testing.expect(null == manager.get(100));
}

test "getAll always returns empty slice" {
    const allocator = std.testing.allocator;
    var manager = presets.PresetsManager.init(allocator);

    const all = manager.getAll();
    try std.testing.expectEqual(@as(usize, 0), all.len);
}

test "getByName always returns null for deprecated module" {
    const allocator = std.testing.allocator;
    var manager = presets.PresetsManager.init(allocator);

    try std.testing.expect(null == manager.getByName("any name"));
}

test "add is no-op for deprecated module" {
    const allocator = std.testing.allocator;
    var manager = presets.PresetsManager.init(allocator);

    const preset = interface.TimerPreset{
        .name = try allocator.dupe(u8, "Test"),
        .mode = .COUNTDOWN_MODE,
        .config = .{
            .countdown = .{
                .duration_seconds = 300,
                .loop = false,
                .loop_count = 0,
                .loop_interval_seconds = 0,
            },
            .stopwatch = .{ .max_seconds = 3600 },
            .world_clock = .{ .timezone = 8 },
        },
    };
    defer allocator.free(preset.name);

    try std.testing.expect(manager.add(preset) == void);
    try std.testing.expectEqual(@as(usize, 0), manager.count());
}

test "remove is no-op for deprecated module" {
    const allocator = std.testing.allocator;
    var manager = presets.PresetsManager.init(allocator);

    manager.remove(0);
    try std.testing.expectEqual(@as(usize, 0), manager.count());
}

test "clear is no-op for deprecated module" {
    const allocator = std.testing.allocator;
    var manager = presets.PresetsManager.init(allocator);

    manager.clear();
    try std.testing.expectEqual(@as(usize, 0), manager.count());
}

test "deinit is no-op for deprecated module" {
    const allocator = std.testing.allocator;
    var manager = presets.PresetsManager.init(allocator);

    manager.deinit();
    try std.testing.expectEqual(@as(usize, 0), manager.count());
}

test "PresetsError error set values" {
    try std.testing.expectEqual(@as(u8, 0x01), @intFromEnum(presets.PresetsError.InvalidName));
    try std.testing.expectEqual(@as(u8, 0x02), @intFromEnum(presets.PresetsError.AlreadyExists));
    try std.testing.expectEqual(@as(u8, 0x04), @intFromEnum(presets.PresetsError.NotFound));
    try std.testing.expectEqual(@as(u8, 0x08), @intFromEnum(presets.PresetsError.LimitReached));
}