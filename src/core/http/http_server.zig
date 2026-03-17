const std = @import("std");
const httpz = @import("httpz");
const logger = @import("../logger.zig");
const clock = @import("../clock.zig");
const settings = @import("../../settings/mod.zig");
const MainApplication = @import("../app.zig").MainApplication;
const build_options = @import("build_options");

const ClockState = clock.ClockState;
const ModeEnumT = clock.ModeEnumT;

pub const HttpHandler = struct {
    app: *MainApplication,
    allocator: std.mem.Allocator,
};

fn handleRoot(h: *HttpHandler, request: *httpz.Request, response: *httpz.Response) !void {
    _ = h;
    _ = request;

    if (build_options.embed_ui) {
        response.header("content-type", "text/html; charset=utf-8");
        try response.chunk(build_options.embedded_html);
    } else {
        response.header("content-type", "text/html; charset=utf-8");
        try response.chunk("<html><body><h1>Little Timer</h1><p>请先构建前端: cd assets && bun run build</p></body></html>");
    }
}

fn handleGetState(h: *HttpHandler, request: *httpz.Request, response: *httpz.Response) !void {
    _ = request;

    const display_data = h.app.clock_manager.update();
    const mode_key = switch (display_data.getMode()) {
        .COUNTDOWN_MODE => "countdown",
        .STOPWATCH_MODE => "stopwatch",
        .WORLD_CLOCK_MODE => "world_clock",
    };

    const timezone: i8 = switch (display_data.*) {
        .WORLD_CLOCK_MODE => |world_clock| world_clock.timezone,
        else => h.app.settings_manager.config.basic.timezone,
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

fn handleStart(h: *HttpHandler, request: *httpz.Request, response: *httpz.Response) !void {
    _ = request;
    h.app.clock_manager.handleEvent(.user_start_timer);
    try response.json(.{ .status = "started" }, .{});
}

fn handlePause(h: *HttpHandler, request: *httpz.Request, response: *httpz.Response) !void {
    _ = request;
    h.app.clock_manager.handleEvent(.user_pause_timer);
    try response.json(.{ .status = "paused" }, .{});
}

fn handleReset(h: *HttpHandler, request: *httpz.Request, response: *httpz.Response) !void {
    _ = request;
    h.app.clock_manager.handleEvent(.user_reset_timer);
    try response.json(.{ .status = "reset" }, .{});
}

fn handleModeChange(h: *HttpHandler, request: *httpz.Request, response: *httpz.Response) !void {
    const body = request.body() orelse "";
    const mode_str = std.mem.trim(u8, body, " \n\r\t");

    const new_mode = if (std.mem.eql(u8, mode_str, "countdown"))
        ModeEnumT.COUNTDOWN_MODE
    else if (std.mem.eql(u8, mode_str, "stopwatch"))
        ModeEnumT.STOPWATCH_MODE
    else if (std.mem.eql(u8, mode_str, "world_clock"))
        ModeEnumT.WORLD_CLOCK_MODE
    else {
        try response.json(std.json.Value{ .object = std.StringArrayHashMap(std.json.Value).init(h.allocator) }, .{});
        return;
    };

    h.app.clock_manager.handleEvent(.{ .user_change_mode = new_mode });
    try response.json(.{ .status = "mode_changed", .new_mode = mode_str }, .{});
}

fn handleGetSettings(h: *HttpHandler, request: *httpz.Request, response: *httpz.Response) !void {
    _ = request;
    const config = h.app.settings_manager.getConfig();
    try response.json(config, .{});
}

fn handleUpdateSettings(h: *HttpHandler, request: *httpz.Request, response: *httpz.Response) !void {
    const body = request.body() orelse "";
    const body_copy: [:0]u8 = try h.allocator.allocSentinel(u8, body.len, 0);
    @memcpy(body_copy[0..body.len], body);
    try h.app.settings_manager.handleSettingsEvent(.{ .change_settings = body_copy });
    try response.json(.{ .status = "settings_updated" }, .{});
}

fn handleSSE(h: *HttpHandler, request: *httpz.Request, response: *httpz.Response) !void {
    _ = request;

    const stream = try response.startEventStreamSync();

    while (true) {
        const display_data = h.app.clock_manager.update();
        const mode_key = switch (display_data.getMode()) {
            .COUNTDOWN_MODE => "countdown",
            .STOPWATCH_MODE => "stopwatch",
            .WORLD_CLOCK_MODE => "world_clock",
        };

        const timezone: i8 = switch (display_data.*) {
            .WORLD_CLOCK_MODE => |world_clock| world_clock.timezone,
            else => h.app.settings_manager.config.basic.timezone,
        };

        const buf = try std.fmt.allocPrint(h.allocator, "{{\"time\":{},\"mode\":\"{s}\",\"is_running\":{},\"is_finished\":{},\"in_rest\":{},\"loop_remaining\":{},\"loop_total\":{},\"rest_remaining\":{},\"timezone\":{}}}", .{
            display_data.getTimeInfo(),
            mode_key,
            !display_data.isPaused(),
            display_data.isFinished(),
            display_data.inRest(),
            display_data.getLoopRemaining(),
            display_data.getLoopTotal(),
            display_data.getRestRemainingTime(),
            timezone,
        });
        defer h.allocator.free(buf);

        try stream.writeAll("data: ");
        try stream.writeAll(buf);
        try stream.writeAll("\n\n");

        std.Thread.sleep(1_000_000_000);
    }
}

fn handlePresets(h: *HttpHandler, request: *httpz.Request, response: *httpz.Response) !void {
    _ = request;
    const presets = h.app.settings_manager.getPresets();
    try response.json(presets, .{});
}

pub const HttpServerManager = struct {
    server: httpz.Server(*HttpHandler),
    handler: HttpHandler,

    pub fn init(allocator: std.mem.Allocator, port: u16, app: *MainApplication) !HttpServerManager {
        var handler = HttpHandler{ .app = app, .allocator = allocator };
        var server = try httpz.Server(*HttpHandler).init(allocator, .{
            .address = .localhost(port),
        }, &handler);

        var router = try server.router(.{});
        router.get("/", handleRoot, .{});
        router.get("/api/state", handleGetState, .{});
        router.post("/api/start", handleStart, .{});
        router.post("/api/pause", handlePause, .{});
        router.post("/api/reset", handleReset, .{});
        router.post("/api/mode", handleModeChange, .{});
        router.get("/api/settings", handleGetSettings, .{});
        router.post("/api/settings", handleUpdateSettings, .{});
        router.get("/api/events", handleSSE, .{});
        router.get("/api/presets", handlePresets, .{});

        return HttpServerManager{
            .server = server,
            .handler = handler,
        };
    }

    pub fn start(self: *HttpServerManager) !void {
        try self.server.listen();
    }

    pub fn stop(self: *HttpServerManager) void {
        self.server.stop();
    }

    pub fn deinit(self: *HttpServerManager) void {
        self.server.deinit();
    }
};
