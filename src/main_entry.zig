const std = @import("std");
const builtin = @import("builtin");

const app = @import("core/app.zig");
const logger = @import("core/logger.zig");

// 测试文件导入（仅在测试构建时编译）
comptime {
    if (builtin.is_test) {
        _ = @import("test/test_logger.zig");
        _ = @import("test/test_clock.zig");
        _ = @import("test/test_settings.zig");
        _ = @import("test/test_settings_validator.zig");
        _ = @import("test/test_settings_presets.zig");
        _ = @import("test/test_boundary_conditions.zig");
        _ = @import("test/test_error_recovery.zig");
    }
}

pub fn main() !void {
    logger.global_logger.info("Little Timer HTTP Server 启动中...", .{});

    var gpa = std.heap.GeneralPurposeAllocator(.{}){};
    defer _ = gpa.deinit();
    const allocator = gpa.allocator();

    const main_app = try allocator.create(app.MainApplication);
    defer {
        allocator.destroy(main_app);
    }

    main_app.* = undefined;

    logger.global_logger.info("初始化应用程序...", .{});
    try main_app.init(allocator);

    logger.global_logger.info("启动 HTTP 服务器...", .{});
    try main_app.run();
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
