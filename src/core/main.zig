const std = @import("std");
const little_timer = @import("little_timer");
const builtin = @import("builtin");
const thread = std.Thread;
const webview = @import("webview_c.zig");

const app = little_timer.app;
const logger = little_timer.logger;

pub fn main() !void {
    logger.global_logger.info("Little Timer WebUI 启动中...", .{});

    var gpa = std.heap.GeneralPurposeAllocator(.{}){};
    defer _ = gpa.deinit();
    const allocator = gpa.allocator();

    const main_app = try allocator.create(app.MainApplication);
    defer {
        main_app.deinit();
        allocator.destroy(main_app);
    }

    main_app.* = undefined;

    logger.global_logger.info("初始化应用程序...", .{});
    try main_app.init(allocator);

    logger.global_logger.info("启动 WebUI 主循环...", .{});
    var app_thread = thread.spawn(.{}, struct {
        fn t(app_ptr: *app.MainApplication) void {
            app_ptr.run() catch |err| {
                logger.global_logger.err("MainApplication.run 失败: {any}", .{err});
            };
        }
    }.t, .{main_app}) catch |err| {
        logger.global_logger.err("启动运行线程失败: {any}", .{err});
        return;
    };

    defer main_app.*.stop() catch |err| {
        logger.global_logger.err("停止 MainApplication 失败: {any}", .{err});
    };

    defer app_thread.join();

    // 主线程运行 webview；开发模式打开 Vite，发布模式打开后端内嵌页
    var win = webview.Window.openDefault(false) catch |err| {
        logger.global_logger.err("创建 WebView 窗口失败: {any}", .{err});
        return;
    };
    defer win.destroy() catch |err| {
        logger.global_logger.err("销毁 WebView 窗口失败: {any}", .{err});
    };

    win.setTitle("Little Timer") catch |err| {
        logger.global_logger.err("设置 WebView 标题失败: {any}", .{err});
        return;
    };
    win.setSize(1200, 780) catch |err| {
        logger.global_logger.err("设置 WebView 尺寸失败: {any}", .{err});
        return;
    };
    win.navigate("http://127.0.0.1:8080") catch |err| {
        logger.global_logger.err("导航到前端页面失败: {any}", .{err});
        return;
    };
    win.run() catch |err| {
        logger.global_logger.err("WebView 主循环失败: {any}", .{err});
    };
}

pub export fn Java_com_zig_little_1timer_MainActivity_startZigLogic(env: ?*anyopaque, thiz: ?*anyopaque) void {
    _ = env;
    _ = thiz;
    if (builtin.target.abi != .android) return;

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
