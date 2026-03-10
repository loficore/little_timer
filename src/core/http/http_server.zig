const std = @import("std");
const httpz = @import("httpz");
const logger = @import("../logger.zig");
const clock = @import("../clock.zig");
const settings = @import("../../settings/mod.zig");
const MainApplication = @import("../app.zig").MainApplication;

const ClockState = clock.ClockState;

const HttpServerManager = struct {
    server: ?httpz.Server(HttpHandler) = null,
    handler: HttpHandler,
    sse_clients: std.ArrayList(std.net.Stream),
    sse_mutex: std.Thread.Mutex,
    allocator: std.mem.Allocator,

    pub const HttpHandler = struct {
        app: *MainApplication,

        fn handleGetState(self: *HttpHandler, request: *httpz.Request, response: *httpz.Response) !void {
            _ = request;

            const display_data = self.app.clock_manager.update();
            const mode_key = switch (display_data.getMode()) {
                .COUNTDOWN_MODE => "countdown",
                .STOPWATCH_MODE => "stopwatch",
                .WORLD_CLOCK_MODE => "world_clock",
            };

            const timezone: i8 = switch (display_data.*) {
                .WORLD_CLOCK_MODE => |world_clock| world_clock.timezone,
                else => self.app.settings_manager.config.basic.timezone,
            };

            try response.json(.{
                .time = display_data.getTimeInfo(),
                .mode = mode_key,
                .is_running = !display_data.isPaused(),
                .is_finished = display_data.isFinished(),
                .in_rest = display_data.inRest(),
                .loop_remaining = display_data.getLoopRemaining(),
                .loop_total = display_data.getLoopTotal(),
                .rest_remaining = display_data.getRestRemainingTime(),
                .timezone = timezone,
            }, .{});
        }

        /// 处理开始计时的请求
        /// 参数:
        /// - **self**: HttpHandler实例指针
        /// - **request**: HTTP请求对象
        /// - **response**: HTTP响应对象
        fn handleStart(self: *HttpHandler, request: *httpz.Request, response: *httpz.Response) !void {
            _ = request;
            self.app.clock_manager.handleEvent(.user_start_timer);
            try response.json(.{ .status = "started" }, .{});
        }

        /// 处理暂停计时的请求
        /// 参数:
        /// - **self**: HttpHandler实例指针
        /// - **request**: HTTP请求对象
        /// - **response**: HTTP响应对象
        fn handlePause(self: *HttpHandler, request: *httpz.Request, response: *httpz.Response) !void {
            _ = request;
            self.app.clock_manager.handleEvent(.user_pause_timer);
            try response.json(.{ .status = "paused" }, .{});
        }

        fn handleReset(self: *HttpHandler, request: *httpz.Request, response: *httpz.Response) !void {
            _ = request;
            self.app.clock_manager.handleEvent(.user_reset_timer);
            try response.json(.{ .status = "reset" }, .{});
        }

        fn handleModeChange(self: *HttpHandler, request: *httpz.Request, response: *httpz.Response) !void {
            const body = try request.body();
            const mode_str = try std.utf8.trim(std.mem.sliceToString(body));

            const new_mode = if (std.mem.eql(u8, mode_str, "countdown"))
                ClockState.Mode.COUNTDOWN_MODE
            else if (std.mem.eql(u8, mode_str, "stopwatch"))
                ClockState.Mode.STOPWATCH_MODE
            else if (std.mem.eql(u8, mode_str, "world_clock"))
                ClockState.Mode.WORLD_CLOCK_MODE
            else {
                try response.json(std.json.Value{ .object = std.StringArrayHashMap(std.json.Value).init(self.app.allocator) }, .{});
                return;
            };

            self.app.clock_manager.handleEvent(.user_change_mode(new_mode));
            try response.json(.{ .status = "mode_changed", .new_mode = mode_str }, .{});
        }

        fn handleGetSettings(self: *HttpHandler, request: *httpz.Request, response: *httpz.Response) !void {}

        fn handleUpdateSettings(self: *HttpHandler, request: *httpz.Request, response: *httpz.Response) !void {}

        fn handleSSE(self: *HttpHandler, request: *httpz.Request, response: *httpz.Response) !void {
            _ = request;
            const display_data = self.app.clock_manager.update();

            const sse_data = .{
                .time = display_data.getTimeInfo(),
                .mode = switch (display_data.getMode()) {
                    .COUNTDOWN_MODE => "countdown",
                    .STOPWATCH_MODE => "stopwatch",
                    .WORLD_CLOCK_MODE => "world_clock",
                },
                .is_running = !display_data.isPaused(),
                .is_finished = display_data.isFinished(),
                .in_rest = display_data.inRest(),
                .loop_remaining = display_data.getLoopRemaining(),
                .loop_total = display_data.getLoopTotal(),
                .rest_remaining = display_data.getRestRemainingTime(),
            };

            // 发送 SSE 数据，这个value参数是anytype,且json方法会自动序列化为JSON字符串，所以这里直接传结构体即可
            try response.json(response, sse_data, .{});
        }
    };

    pub fn init(allocator: ?std.mem.Allocator, port: u16, app: *MainApplication) !HttpServerManager {
        if (allocator == null) {
            allocator = std.heap.page_allocator;
        }

        const App = struct {};
        const server = try httpz.Server(App).init(allocator, .{ .address = .{ .ip = std.net.Address.Ip4{ .octets = [4]u8{ 127, 0, 0, 1 }, .port = port } } });

        const server_temp = HttpServerManager{
            .server = server,
            .handler = HttpHandler{
                .app = app,
            },
        };

        // 路由配置
        var router = try server_temp.server.?.router(.{});
        router.get("/api/state", server_temp.handler.handleGetState, .{});
        router.post("/api/start", server_temp.handler.handleStart, .{});
        router.post("/api/pause", server_temp.handler.handlePause, .{});
        router.post("/api/reset", server_temp.handler.handleReset, .{});
        router.post("/api/mode", server_temp.handler.handleModeChange, .{});
        router.get("/api/settings", server_temp.handler.handleGetSettings, .{});
        router.post("/api/settings", server_temp.handler.handleUpdateSettings, .{});
        router.get("/api/events", server_temp.handler.handleSSE, .{});

        return server_temp;
    }

    pub fn start(self: *HttpServerManager) !void {
        try self.server.listen();
    }

    pub fn stop(self: *HttpServerManager) !void {
        try self.server.close();
    }
};
