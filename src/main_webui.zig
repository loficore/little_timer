const std = @import("std");
const app = @import("app.zig");
const clock = @import("clock.zig");

/// 应用程序入口函数 (WebUI 版本)
///
/// 创建内存分配器、初始化应用程序并运行 WebUI 主循环
///
/// 返回:
/// - !void: 如果运行失败则返回错误
pub fn main() !void {
    std.debug.print("Little Timer WebUI 启动中...\n", .{});

    // 1. 创建内存分配器
    var gpa = std.heap.GeneralPurposeAllocator(.{}){};
    defer _ = gpa.deinit();
    const allocator = gpa.allocator();

    // 2. 在堆上分配应用程序实例
    const main_app = try allocator.create(app.MainApplication);
    defer allocator.destroy(main_app);

    const clock_config = clock.ClockTaskConfig{
        .countdown = .{
            .duration_seconds = 25 * 60, // 25分钟倒计时
            .loop = false,
        },
    };

    // 3. 初始化应用程序（在指针上原地初始化）
    std.debug.print("初始化应用程序...\n", .{});
    main_app.init(allocator, clock_config) catch |err| {
        std.debug.print("初始化失败: {any}\n", .{err});
        return err;
    };

    // 4. 设置全局指针（这样回调函数才能访问 app 实例）
    std.debug.print("设置全局指针...\n", .{});
    main_app.setGlobalApp();

    // 5. 运行应用程序
    std.debug.print("启动 WebUI 主循环...\n", .{});
    // 根据编译选项，这将启动GTK或WebUI主循环
    main_app.run() catch |err| {
        std.debug.print("运行失败: {any}\n", .{err});
        return err;
    };
}
