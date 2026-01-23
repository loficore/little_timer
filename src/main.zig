const std = @import("std");
const app = @import("app.zig");
const logger = @import("logger.zig");

/// 应用程序入口函数（WebUI版本）
///
/// 创建内存分配器、初始化应用程序并运行 WebUI 主循环
///
/// 返回:
/// - !void: 如果运行失败则返回错误
pub fn main() !void {
    logger.global_logger.info("Little Timer WebUI 启动中...", .{});

    // 1. 创建内存分配器
    var gpa = std.heap.GeneralPurposeAllocator(.{}){};
    defer _ = gpa.deinit();
    const allocator = gpa.allocator();

    // 2. 在堆上分配应用程序实例
    const main_app = try allocator.create(app.MainApplication);
    defer {
        main_app.deinit();
        allocator.destroy(main_app);
    }

    // 初始化结构体的所有字段为默认值（重要！防止未初始化内存）
    main_app.* = undefined;

    // 3. 初始化应用程序（会自动加载设置并构建时钟配置）
    logger.global_logger.info("初始化应用程序...", .{});
    try main_app.init(allocator);

    // 4. 运行应用程序主循环
    logger.global_logger.info("启动 WebUI 主循环...", .{});
    try main_app.run();
}
