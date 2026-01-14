const std = @import("std");
const app = @import("app.zig");
const clock = @import("clock.zig");
const interface = @import("interface.zig");

/// 应用程序入口函数（WebUI版本）
///
/// 创建内存分配器、初始化应用程序并运行 WebUI 主循环
///
/// 返回:
/// - !void: 如果运行失败则返回错误
pub fn main() !void {
    std.debug.print("Little Timer WebUI 启动中...\n", .{});

    var gpa = std.heap.GeneralPurposeAllocator(.{}){};
    defer _ = gpa.deinit();
    const allocator = gpa.allocator();

    const main_app = try allocator.create(app.MainApplication);
    defer allocator.destroy(main_app);

    const clock_config = interface.ClockTaskConfig{
        .countdown = .{
            .duration_seconds = 25 * 60,
            .loop = false,
        },
    };

    std.debug.print("初始化应用程序...\n", .{});
    main_app.init(allocator, clock_config) catch |err| {
        std.debug.print("初始化失败: {any}\n", .{err});
        return err;
    };

    std.debug.print("设置全局指针...\n", .{});
    main_app.setGlobalApp();

    std.debug.print("启动 WebUI 主循环...\n", .{});
    main_app.run() catch |err| {
        std.debug.print("运行失败: {any}\n", .{err});
        return err;
    };
}

test "simple test" {
    const gpa = std.testing.allocator;
    var list: std.ArrayList(i32) = .empty;
    defer list.deinit(gpa); // Try commenting this out and see if zig detects the memory leak!
    try list.append(gpa, 42);
    try std.testing.expectEqual(@as(i32, 42), list.pop());
}

test "fuzz example" {
    const Context = struct {
        fn testOne(context: @This(), input: []const u8) anyerror!void {
            _ = context;
            // Try passing `--fuzz` to `zig build test` and see if it manages to fail this test case!
            try std.testing.expect(!std.mem.eql(u8, "canyoufindme", input));
        }
    };
    try std.testing.fuzz(Context{}, Context.testOne, .{});
}
