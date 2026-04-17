const std = @import("std");
const builtin = @import("builtin");
const thread = std.Thread;

const app = @import("core/app.zig");
const logger = @import("core/logger.zig");
const webview = @import("core/webview_c.zig");

const RunState = struct {
    mutex: thread.Mutex = .{},
    failed: bool = false,
    address_in_use: bool = false,

    fn markFailure(self: *RunState, err: anyerror) void {
        self.mutex.lock();
        defer self.mutex.unlock();
        self.failed = true;
        self.address_in_use = err == error.AddressInUse;
    }

    fn snapshot(self: *RunState) struct { failed: bool, address_in_use: bool } {
        self.mutex.lock();
        defer self.mutex.unlock();
        return .{
            .failed = self.failed,
            .address_in_use = self.address_in_use,
        };
    }
};

fn parseArgs() enum { http_only, webview } {
    const default_mode = if (builtin.os.tag == .windows) .webview else .http_only;
    var args = std.process.argsWithAllocator(std.heap.page_allocator) catch return default_mode;
    defer args.deinit();
    _ = args.next();
    while (args.next()) |arg| {
        if (std.mem.eql(u8, arg, "--http-only")) {
            return .http_only;
        }
        if (std.mem.eql(u8, arg, "--webview")) {
            return .webview;
        }
    }
    return default_mode;
}

// 测试文件导入（仅在测试构建时编译）
comptime {
    if (builtin.is_test) {
        _ = @import("test/test_logger.zig");
        _ = @import("test/test_clock.zig");
        _ = @import("test/test_settings.zig");
        _ = @import("test/test_settings_validator.zig");
        _ = @import("test/test_boundary_conditions.zig");
        _ = @import("test/test_error_recovery.zig");
    }
}

pub fn main() !void {
    if (builtin.target.abi == .android) {
        logger.global_logger.info("Little Timer HTTP Server 启动中...", .{});

        var gpa = std.heap.GeneralPurposeAllocator(.{}){};
        defer _ = gpa.deinit();
        const allocator = gpa.allocator();

        const main_app = try allocator.create(app.MainApplication);
        errdefer allocator.destroy(main_app);

        main_app.* = undefined;

        logger.global_logger.info("初始化应用程序...", .{});
        try main_app.init(allocator);
        defer {
            main_app.deinit();
            allocator.destroy(main_app);
        }

        logger.global_logger.info("启动 HTTP 服务器...", .{});
        try main_app.run();
        return;
    }

    const mode = parseArgs();

    if (mode == .http_only) {
        logger.global_logger.info("Little Timer HTTP Server 启动中...", .{});
    } else {
        logger.global_logger.info("Little Timer WebView 启动中...", .{});
    }

    var gpa = std.heap.GeneralPurposeAllocator(.{}){};
    defer _ = gpa.deinit();
    const allocator = gpa.allocator();

    const main_app = try allocator.create(app.MainApplication);
    errdefer allocator.destroy(main_app);

    main_app.* = undefined;

    logger.global_logger.info("初始化应用程序...", .{});
    try main_app.init(allocator);
    defer {
        main_app.deinit();
        allocator.destroy(main_app);
    }

    var run_state = RunState{};

    var app_thread = thread.spawn(.{}, struct {
        fn t(app_ptr: *app.MainApplication, state: *RunState) void {
            app_ptr.run() catch |err| {
                logger.global_logger.err("MainApplication.run 失败: {any}", .{err});
                state.markFailure(err);
            };
        }
    }.t, .{ main_app, &run_state }) catch |err| {
        logger.global_logger.err("启动运行线程失败: {any}", .{err});
        return;
    };

    // 注意：由于 stop() 方法现在是幂等的（通过 stopped 标志防止重复调用），
    // defer 可以保留用于 HTTP 模式（服务器阻塞运行不会返回）
    defer main_app.stop() catch |err| {
        logger.global_logger.err("停止 MainApplication 失败: {any}", .{err});
    };

    // 给 HTTP 服务器一个短暂启动窗口，及时发现端口占用。
    std.Thread.sleep(250 * std.time.ns_per_ms);
    const state = run_state.snapshot();
    if (state.failed) {
        if (state.address_in_use) {
            logger.global_logger.err("HTTP 端口 8080 已被占用，已终止本次启动。请先释放端口后重试。", .{});
        }
        return;
    }

    if (mode == .webview) {
        var win = webview.Window.openDefault(false) catch |err| {
            logger.global_logger.err("创建 WebView 窗口失败: {any}", .{err});
            return;
        };
        defer win.destroy() catch |err| {
            logger.global_logger.err("销毁 WebView 窗口失败: {any}", .{err});
        };

        try win.bind("log", struct {
            fn callback(req: [:0]const u8) void {
                if (std.mem.indexOfScalar(u8, req, ',')) |comma_pos| {
                    const category = req[0..comma_pos];
                    const rest = req[comma_pos + 1 ..];
                    if (std.mem.indexOfScalar(u8, rest, ',')) |second_comma| {
                        const level = rest[0..second_comma];
                        const message = rest[second_comma + 1 ..];
                        if (std.mem.eql(u8, level, "error")) {
                            logger.global_logger.err("[前端 {s}] {s}", .{ category, message });
                        } else {
                            logger.global_logger.info("[前端 {s}] {s}", .{ category, message });
                        }
                    }
                }
            }
        }.callback);

        try win.run();

        // WebView 窗口已关闭，显式停止应用并等待线程结束
        logger.global_logger.info("WebView 窗口已关闭，停止应用...", .{});
        main_app.stop() catch |err| {
            logger.global_logger.err("停止 MainApplication 失败: {any}", .{err});
        };
        logger.global_logger.info("等待 HTTP 服务器线程结束...", .{});
        app_thread.join();
        logger.global_logger.info("HTTP 服务器线程已结束", .{});
    } else {
        // HTTP 模式下等待
        while (true) std.Thread.sleep(1 * std.time.ns_per_s);
    }
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
                main_app.deinit();
                allocator.destroy(main_app);
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
