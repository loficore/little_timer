const std = @import("std");
const app = @import("app.zig");
const logger = @import("logger.zig");
const builtin = @import("builtin");

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

/// Android 入口：供 Java 调用以启动后端逻辑（WebUI Server + Tick 线程）
/// 注意：JNI 命名规则对包名中的下划线需要使用 `_1` 转义。
/// 包名: com.zig.little_timer -> com_zig_little_1timer
pub export fn Java_com_zig_little_1timer_MainActivity_startZigLogic(env: ?*anyopaque, thiz: ?*anyopaque) void {
    _ = env;
    _ = thiz;
    // 仅在 Android 下运行此逻辑
    if (builtin.target.abi != .android) return;

    // 后台启动应用逻辑，避免阻塞 UI 线程
    const start_fn = struct {
        fn run() void {
            logger.global_logger.info("Android: 启动 Zig 后端逻辑", .{});

            var gpa = std.heap.GeneralPurposeAllocator(.{}){};
            const allocator = gpa.allocator();

            const main_app = allocator.create(app.MainApplication) catch {
                logger.global_logger.err("Android: 创建 MainApplication 失败", .{});
                return;
            };
            main_app.* = undefined;

            main_app.init(allocator) catch |err| {
                logger.global_logger.err("Android: MainApplication.init 失败: {any}", .{err});
                return;
            };

            // 在子线程中运行主循环（内部会启动 tick 线程并使用 startServer）
            std.Thread.spawn(.{}, struct {
                fn t(app_ptr: *app.MainApplication) void {
                    app_ptr.run() catch |err| {
                        logger.global_logger.err("Android: MainApplication.run 失败: {any}", .{err});
                    };
                }
            }.t, .{main_app}) catch |err| {
                logger.global_logger.err("Android: 启动运行线程失败: {any}", .{err});
            };
        }
    }.run;

    start_fn();
}
