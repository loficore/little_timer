const std = @import("std");
const presets_mod = @import("../settings/settings_presets.zig");
const interface = @import("../core/interface.zig");

fn countdownPreset(name: []const u8) interface.TimerPreset {
    return .{
        .name = name,
        .mode = .COUNTDOWN_MODE,
        .config = .{
            .countdown = .{ .duration_seconds = 1500, .loop = false, .loop_count = 0, .loop_interval_seconds = 0 },
            .stopwatch = .{ .max_seconds = 86400 },
            .world_clock = .{ .timezone = 8 },
        },
    };
}

// 基础增删测试
test "presets add and remove" {
    var gpa = std.heap.GeneralPurposeAllocator(.{}){};
    defer _ = gpa.deinit();
    const allocator = gpa.allocator();

    var mgr = presets_mod.PresetsManager.init(allocator);
    defer mgr.deinit();

    try mgr.add(countdownPreset("a"));
    try mgr.add(countdownPreset("b"));
    try std.testing.expectEqual(@as(usize, 2), mgr.count());

    try mgr.removeByName("a");
    try std.testing.expectEqual(@as(usize, 1), mgr.count());

    try std.testing.expectError(error.PresetNotFound, mgr.removeByName("missing"));
}

// 重复名称与上限测试
test "presets duplicate and limit" {
    var gpa = std.heap.GeneralPurposeAllocator(.{}){};
    defer _ = gpa.deinit();
    const allocator = gpa.allocator();

    var mgr = presets_mod.PresetsManager.init(allocator);
    defer mgr.deinit();

    try mgr.add(countdownPreset("dup"));
    try std.testing.expectError(error.PresetNameConflict, mgr.add(countdownPreset("dup")));

    mgr.max_count = 3;
    try mgr.add(countdownPreset("x"));
    try mgr.add(countdownPreset("y"));
    try std.testing.expectError(error.PresetListFull, mgr.add(countdownPreset("z")));
}

// 保存与加载测试（内存文件）
test "presets save and load" {
    var gpa = std.heap.GeneralPurposeAllocator(.{}){};
    defer _ = gpa.deinit();
    const allocator = gpa.allocator();

    var mgr = presets_mod.PresetsManager.init(allocator);
    defer mgr.deinit();

    try mgr.add(countdownPreset("p1"));
    try mgr.add(countdownPreset("p2"));

    // 使用临时目录
    var tmp = std.testing.tmpDir(.{});
    defer tmp.cleanup();

    const path = try std.fmt.allocPrint(allocator, ".zig-cache/tmp/{s}/presets_test.json", .{tmp.sub_path});
    defer allocator.free(path);

    try mgr.saveToFile(path);

    // 新实例加载
    var mgr2 = presets_mod.PresetsManager.init(allocator);
    defer mgr2.deinit();

    try mgr2.loadFromFile(path, 8); // 传入默认时区
    try std.testing.expectEqual(@as(usize, 2), mgr2.count());
    const p0 = mgr2.get(0).?;
    try std.testing.expect(std.mem.eql(u8, p0.name, "p1"));
}
