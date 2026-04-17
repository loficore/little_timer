const std = @import("std");
const json = std.json;
const http = std.http;
const logger = @import("../logger.zig");
const clock = @import("../clock.zig");
const settings = @import("../../settings/mod.zig");
const habit_crud = @import("../../storage/habit_crud.zig");
const MainApplication = @import("../app.zig").MainApplication;
const build_options = @import("build_options");

const ClockState = clock.ClockState;
const ModeEnumT = clock.ModeEnumT;

var global_app: ?*MainApplication = null;
var global_allocator: ?std.mem.Allocator = null;

pub const HttpServerManager = struct {
    allocator: std.mem.Allocator,
    listener: std.net.Server,
    thread: std.Thread,
    running: std.atomic.Value(bool),

    pub fn init(allocator: std.mem.Allocator, port: u16, app: *MainApplication) !HttpServerManager {
        global_app = app;
        global_allocator = allocator;

        logger.global_logger.info("初始化 HTTP 服务器，端口: {}", .{port});

        const address = try std.net.Address.resolveIp("127.0.0.1", port);
        const listener = try address.listen(.{ .reuse_address = true });

        const running = std.atomic.Value(bool).init(true);

        logger.global_logger.info("HTTP 服务器监听完成，准备接受连接", .{});

        return HttpServerManager{
            .allocator = allocator,
            .listener = listener,
            .thread = undefined,
            .running = running,
        };
    }

    pub fn start(self: *HttpServerManager) !void {
        logger.global_logger.info("HTTP 服务器开始监听...", .{});

        self.thread = try std.Thread.spawn(.{}, serverLoop, .{
            &self.listener,
            &self.running,
        });
    }

    pub fn stop(self: *HttpServerManager) void {
        logger.global_logger.info("HTTP 服务器停止中...", .{});
        self.running.store(false, .release);
        self.listener.deinit();
    }

    pub fn deinit(self: *HttpServerManager) void {
        logger.global_logger.info("HTTP 服务器释放资源...", .{});
        global_app = null;
        global_allocator = null;
        self.thread.join();
    }
};

fn serverLoop(listener: *std.net.Server, running: *std.atomic.Value(bool)) void {
    while (running.load(.acquire)) {
        const stream = listener.accept() catch {
            continue;
        };
        const thread = std.Thread.spawn(.{}, handleConnection, .{
            stream,
            running,
        }) catch {
            stream.stream.close();
            continue;
        };
        thread.detach();
    }
}

fn handleConnection(conn: std.net.Server.Connection, running: *std.atomic.Value(bool)) void {
    defer conn.stream.close();

    var read_buffer: [8192]u8 = undefined;
    var reader = conn.stream.reader(&read_buffer);
    var write_buffer: [8192]u8 = undefined;
    var writer = conn.stream.writer(&write_buffer);
    var server = http.Server.init(reader.interface(), &writer.interface);

    while (running.load(.acquire)) {
        var request = server.receiveHead() catch |err| {
                switch (err) {
                    error.HttpConnectionClosing => {
                        logger.global_logger.debug("客户端连接关闭: {}", .{err});
                    },
                    else => {
                        logger.global_logger.err("接收请求失败: {}", .{err});
                    },
                }
            break;
        };

        handleRequest(&request) catch |err| {
            logger.global_logger.err("处理请求失败: {}", .{err});
            _ = request.respond("", .{ .status = .internal_server_error }) catch {};
            break;
        };
    }
}

/// 读取 HTTP 请求体
fn readRequestBody(request: *http.Server.Request, allocator: std.mem.Allocator) ![]u8 {
    const content_length_u64 = request.head.content_length orelse 0;
    const content_length: usize = @intCast(content_length_u64);
    const body_bytes = try allocator.alloc(u8, content_length);
    errdefer allocator.free(body_bytes);

    var body_buffer: [8192]u8 = undefined;
    const body_reader = if (request.head.expect != null)
        try request.readerExpectContinue(&body_buffer)
    else
        request.readerExpectNone(&body_buffer);
    var total_read: usize = 0;

    while (total_read < body_bytes.len) {
        const chunk = try body_reader.readSliceShort(body_bytes[total_read..]);
        if (chunk == 0) break;
        total_read += chunk;
    }

    return body_bytes[0..total_read];
}

fn handleRequest(request: *http.Server.Request) !void {
    const target = request.head.target;
    const path = if (std.mem.indexOfScalar(u8, target, '?')) |query_idx|
        target[0..query_idx]
    else
        target;

    if (request.head.method == .GET and std.mem.eql(u8, path, "/")) {
        try handleRoot(request);
    } else if (request.head.method == .GET and std.mem.eql(u8, path, "/api/state")) {
        try handleGetState(request);
    } else if (request.head.method == .POST and std.mem.eql(u8, path, "/api/start")) {
        try handleStart(request);
    } else if (request.head.method == .POST and std.mem.eql(u8, path, "/api/pause")) {
        try handlePause(request);
    } else if (request.head.method == .POST and std.mem.eql(u8, path, "/api/reset")) {
        try handleReset(request);
    } else if (request.head.method == .POST and std.mem.eql(u8, path, "/api/mode")) {
        try handleModeChange(request);
    } else if (request.head.method == .GET and std.mem.eql(u8, path, "/api/settings")) {
        try handleGetSettings(request);
    } else if (request.head.method == .POST and std.mem.eql(u8, path, "/api/settings")) {
        try handleUpdateSettings(request);
    } else if (request.head.method == .POST and std.mem.eql(u8, path, "/api/log")) {
        try handleFrontendLog(request);
    } else if (request.head.method == .GET and std.mem.eql(u8, path, "/api/events")) {
        try handleSSE(request);
    } else if (request.head.method == .GET and std.mem.eql(u8, path, "/api/habit-sets")) {
        try handleGetHabitSets(request);
    } else if (request.head.method == .POST and std.mem.eql(u8, path, "/api/habit-sets")) {
        try handleCreateHabitSet(request);
    } else if (request.head.method == .PUT and std.mem.startsWith(u8, path, "/api/habit-sets/")) {
        try handleUpdateHabitSet(request);
    } else if (request.head.method == .DELETE and std.mem.startsWith(u8, path, "/api/habit-sets/")) {
        try handleDeleteHabitSet(request);
    } else if (request.head.method == .GET and std.mem.eql(u8, path, "/api/habits")) {
        try handleGetHabits(request);
    } else if (request.head.method == .POST and std.mem.eql(u8, path, "/api/habits")) {
        try handleCreateHabit(request);
    } else if (request.head.method == .PUT and std.mem.startsWith(u8, path, "/api/habits/")) {
        try handleUpdateHabit(request);
    } else if (request.head.method == .DELETE and std.mem.startsWith(u8, path, "/api/habits/")) {
        try handleDeleteHabit(request);
    } else if (request.head.method == .POST and std.mem.eql(u8, path, "/api/sessions")) {
        try handleCreateSession(request);
    } else if (request.head.method == .GET and std.mem.eql(u8, path, "/api/sessions")) {
        try handleGetSessions(request);
    } else if (request.head.method == .GET and std.mem.startsWith(u8, path, "/api/habits/") and std.mem.endsWith(u8, path, "/streak")) {
        try handleGetHabitStreak(request);
    } else if (request.head.method == .GET and std.mem.startsWith(u8, path, "/api/habits/") and std.mem.endsWith(u8, path, "/detail")) {
        try handleGetHabitDetail(request);
    } else if (request.head.method == .POST and std.mem.eql(u8, path, "/api/timer/rest")) {
        try handleStartRest(request);
    } else if (request.head.method == .POST and std.mem.eql(u8, path, "/api/timer/finish")) {
        try handleFinish(request);
    } else if (request.head.method == .GET and std.mem.eql(u8, path, "/api/timer/progress")) {
        try handleGetProgress(request);
    } else {
        try request.respond("", .{ .status = .not_found });
    }
}

fn getApp() *MainApplication {
    return global_app orelse @panic("global_app not set");
}

fn getAllocator() std.mem.Allocator {
    return global_allocator orelse @panic("global_allocator not set");
}

fn parsePathId(path: []const u8, prefix: []const u8) !i64 {
    if (!std.mem.startsWith(u8, path, prefix)) return error.InvalidPath;
    const id_str = path[prefix.len..];
    if (id_str.len == 0) return error.InvalidPath;
    return std.fmt.parseInt(i64, id_str, 10);
}

fn parsePathIdWithSuffix(path: []const u8, prefix: []const u8, suffix: []const u8) !i64 {
    if (!std.mem.startsWith(u8, path, prefix)) return error.InvalidPath;
    if (!std.mem.endsWith(u8, path, suffix)) return error.InvalidPath;
    if (path.len <= prefix.len + suffix.len) return error.InvalidPath;
    const id_str = path[prefix.len .. path.len - suffix.len];
    return std.fmt.parseInt(i64, id_str, 10);
}

fn handleRoot(request: *http.Server.Request) !void {
    if (build_options.embed_ui) {
        try request.respond(build_options.embedded_html, .{
            .extra_headers = &.{
                .{ .name = "Content-Type", .value = "text/html; charset=utf-8" },
            },
        });
    } else {
        const html = "<html><body><h1>Little Timer</h1><p>请先构建前端: cd assets && bun run build</p></body></html>";
        try request.respond(html, .{
            .extra_headers = &.{
                .{ .name = "Content-Type", .value = "text/html; charset=utf-8" },
            },
        });
    }
}

fn handleFrontendLog(request: *http.Server.Request) !void {
    const allocator = getAllocator();
    const body = try readRequestBody(request, allocator);
    defer allocator.free(body);
    if (body.len == 0) {
        try request.respond("{\"success\":false,\"err\":\"empty body\"}", .{
            .extra_headers = &.{
                .{ .name = "Content-Type", .value = "application/json" },
            },
        });
        return;
    }

    const parsed = std.json.parseFromSlice(std.json.Value, allocator, body, .{}) catch null;
    defer if (parsed) |p| p.deinit();

    if (parsed == null) {
        try request.respond("{\"success\":false,\"err\":\"invalid json\"}", .{
            .extra_headers = &.{
                .{ .name = "Content-Type", .value = "application/json" },
            },
        });
        return;
    }

    const value = parsed.?.value;
    if (value != .object) {
        try request.respond("{\"success\":false,\"err\":\"not an object\"}", .{
            .extra_headers = &.{
                .{ .name = "Content-Type", .value = "application/json" },
            },
        });
        return;
    }

    const obj = value.object;
    const category = obj.get("category") orelse .null;
    const level = obj.get("level") orelse .null;
    const message = obj.get("message") orelse .null;
    const runtime = obj.get("runtime") orelse .null;

    const cat_str = if (category == .string) category.string else "unknown";
    const level_str = if (level == .string) level.string else "info";
    const msg_str = if (message == .string) message.string else "";
    const runtime_str = if (runtime == .string) runtime.string else "unknown";

    const actual_level = logger.LogLevel.fromString(level_str) orelse .INFO;
    const level_int = @intFromEnum(actual_level);

    if (level_int >= @intFromEnum(logger.LogLevel.ERROR)) {
        logger.global_logger.err("[前端:{s}][{s}] {s}", .{ cat_str, runtime_str, msg_str });
    } else if (level_int >= @intFromEnum(logger.LogLevel.WARN)) {
        logger.global_logger.warn("[前端:{s}][{s}] {s}", .{ cat_str, runtime_str, msg_str });
    } else if (level_int >= @intFromEnum(logger.LogLevel.INFO)) {
        logger.global_logger.info("[前端:{s}][{s}] {s}", .{ cat_str, runtime_str, msg_str });
    } else {
        logger.global_logger.debug("[前端:{s}][{s}] {s}", .{ cat_str, runtime_str, msg_str });
    }

    try request.respond("{\"success\":true}", .{
        .extra_headers = &.{
            .{ .name = "Content-Type", .value = "application/json" },
        },
    });
}

fn handleGetState(_request: *http.Server.Request) !void {
    logger.global_logger.debug("[API] GET /api/state", .{});

    const app = getApp();
    const display_data = app.clock_manager.update();
    const mode_key = switch (display_data.getMode()) {
        .COUNTDOWN_MODE => "countdown",
        .STOPWATCH_MODE => "stopwatch",
    };

    const timezone: i8 = app.settings_manager.config.basic.timezone;

    const response = try buildStateJson(display_data, mode_key, timezone, app.current_habit_id);
    defer global_allocator.?.free(response);

    try _request.respond(response, .{
        .extra_headers = &.{
            .{ .name = "Content-Type", .value = "application/json" },
        },
    });
}

fn buildStateJson(display_data: *const ClockState, mode_key: []const u8, timezone: i8, habit_id: ?i64) ![]u8 {
    const allocator = global_allocator.?;
    const habit_id_json = if (habit_id) |hid| try std.fmt.allocPrint(allocator, ",\"habit_id\":{}", .{hid}) else "";

    return try std.fmt.allocPrint(allocator, "{{\"time\":{},\"elapsed\":{},\"mode\":\"{s}\",\"is_running\":{},\"is_finished\":{},\"in_rest\":{},\"loop_remaining\":{},\"loop_total\":{},\"rest_remaining\":{},\"timezone\":{}{s}}}", .{
        display_data.getTimeInfo(),
        display_data.getElapsedSeconds(),
        mode_key,
        !display_data.isPaused(),
        display_data.isFinished(),
        display_data.inRest(),
        display_data.getLoopRemaining(),
        display_data.getLoopTotal(),
        display_data.getRestRemainingTime(),
        timezone,
        habit_id_json,
    });
}

fn handleStart(request: *http.Server.Request) !void {
    logger.global_logger.info("[API] POST /api/start", .{});
    const allocator = getAllocator();
    const body = try readRequestBody(request, allocator);
    defer allocator.free(body);
    const app = getApp();

    var habit_id: ?i64 = null;
    var mode: []const u8 = "stopwatch";
    var work_duration: i64 = 25 * 60;
    var rest_duration: i64 = 0;
    var loop_count: i64 = 0;

    if (body.len > 0) {
        const parsed = std.json.parseFromSlice(std.json.Value, allocator, body, .{}) catch null;
        defer if (parsed) |p| p.deinit();
        if (parsed) |p| {
            if (p.value.object.get("habit_id")) |hid| {
                if (hid == .integer) {
                    habit_id = hid.integer;
                }
            }
            if (p.value.object.get("mode")) |m| {
                if (m == .string) {
                    if (std.mem.eql(u8, m.string, "countdown")) {
                        mode = "countdown";
                    } else {
                        mode = "stopwatch";
                    }
                }
            }
            if (p.value.object.get("work_duration")) |wd| {
                if (wd == .integer) {
                    work_duration = wd.integer;
                }
            }
            if (p.value.object.get("rest_duration")) |rd| {
                if (rd == .integer) {
                    rest_duration = rd.integer;
                }
            }
            if (p.value.object.get("loop_count")) |lc| {
                if (lc == .integer) {
                    loop_count = lc.integer;
                }
            }
        }
    }

    const db = app.settings_manager.sqlite_db orelse {
        try request.respond("{\"err\":\"Database not open\"}", .{
            .extra_headers = &.{.{ .name = "Content-Type", .value = "application/json" }},
        });
        return;
    };

    if (app.current_timer_session_id) |session_id| {
        const session = db.habit_manager.getTimerSessionById(session_id) catch null;
        if (session) |s| {
            defer allocator.free(s.mode);

            if (s.is_running and !s.is_finished and !s.is_paused) {
                app.current_habit_id = habit_id orelse s.habit_id;
                const resp = try std.fmt.allocPrint(allocator, "{{\"status\":\"already_running\",\"habit_id\":{?},\"session_id\":{}}}", .{ app.current_habit_id, session_id });
                defer allocator.free(resp);
                try request.respond(resp, .{
                    .extra_headers = &.{.{ .name = "Content-Type", .value = "application/json" }},
                });
                return;
            }

            const current_clock = app.clock_manager.update();
            if ((current_clock.isPaused() and !current_clock.isFinished()) or s.is_paused) {
                var paused_total_seconds = s.paused_total_seconds;
                const now_ts: i64 = @intCast(std.time.timestamp());
                if (s.pause_started_at) |ps| {
                    if (now_ts > ps) {
                        paused_total_seconds += now_ts - ps;
                    }
                }

                app.clock_manager.handleEvent(.user_start_timer);
                _ = db.habit_manager.updateTimerSession(
                    session_id,
                    s.elapsed_seconds,
                    s.remaining_seconds,
                    paused_total_seconds,
                    null,
                    now_ts,
                    true,
                    false,
                    false,
                    s.current_round,
                    s.in_rest,
                ) catch {};

                app.current_habit_id = habit_id orelse s.habit_id;
                const resp = try std.fmt.allocPrint(allocator, "{{\"status\":\"started\",\"habit_id\":{?},\"session_id\":{}}}", .{ app.current_habit_id, session_id });
                defer allocator.free(resp);
                try request.respond(resp, .{
                    .extra_headers = &.{.{ .name = "Content-Type", .value = "application/json" }},
                });
                return;
            }
        }

        app.resetTimerSession();
    }

    const session_id = app.createTimerSession(habit_id, mode, work_duration, rest_duration, loop_count) catch {
        app.current_habit_id = habit_id;
        app.clock_manager.handleEvent(.user_start_timer);
        const resp = try std.fmt.allocPrint(allocator, "{{\"status\":\"started\",\"habit_id\":{?}}}", .{habit_id});
        defer allocator.free(resp);
        try request.respond(resp, .{
            .extra_headers = &.{.{ .name = "Content-Type", .value = "application/json" }},
        });
        return;
    };

    app.current_habit_id = habit_id;
    app.clock_manager.handleEvent(.user_start_timer);
    const resp = try std.fmt.allocPrint(allocator, "{{\"status\":\"started\",\"habit_id\":{?},\"session_id\":{}}}", .{ habit_id, session_id });
    defer allocator.free(resp);
    try request.respond(resp, .{
        .extra_headers = &.{.{ .name = "Content-Type", .value = "application/json" }},
    });
}

fn handleStartRest(request: *http.Server.Request) !void {
    const app = getApp();

    const rest_seconds: i64 = 5 * 60;

    app.clock_manager.handleEvent(.{ .user_change_config = .{
        .default_mode = .COUNTDOWN_MODE,
        .countdown = .{
            .duration_seconds = @intCast(rest_seconds),
            .loop = false,
            .loop_count = 0,
            .loop_interval_seconds = 0,
        },
        .stopwatch = .{ .max_seconds = 24 * 60 * 60 },
    } });

    app.clock_manager.handleEvent(.user_start_timer);

    const resp = try std.fmt.allocPrint(getAllocator(), "{{\"status\":\"rest_started\",\"rest_seconds\":{}}}", .{rest_seconds});
    defer getAllocator().free(resp);
    try request.respond(resp, .{
        .extra_headers = &.{.{ .name = "Content-Type", .value = "application/json" }},
    });
}

fn handlePause(request: *http.Server.Request) !void {
    logger.global_logger.info("[API] POST /api/pause", .{});
    const app = getApp();
    app.clock_manager.handleEvent(.user_pause_timer);
    app.saveTimerProgress();

    try request.respond("{\"status\":\"paused\"}", .{
        .extra_headers = &.{.{ .name = "Content-Type", .value = "application/json" }},
    });
}

fn handleReset(request: *http.Server.Request) !void {
    logger.global_logger.info("[API] POST /api/reset", .{});
    const app = getApp();
    app.resetTimerSession();
    app.current_habit_id = null;
    app.clock_manager.handleEvent(.user_reset_timer);

    try request.respond("{\"status\":\"reset\"}", .{
        .extra_headers = &.{.{ .name = "Content-Type", .value = "application/json" }},
    });
}

fn handleFinish(request: *http.Server.Request) !void {
    logger.global_logger.info("[API] POST /api/finish", .{});
    const app = getApp();

    const habit_id = app.current_habit_id;
    const session_id = app.current_timer_session_id;

    const elapsed = app.finishTimerSession() catch {
        app.clock_manager.handleEvent(.user_finish_timer);
        const clock_state = app.clock_manager.update();
        const elapsed_seconds = clock_state.getElapsedSeconds();
        const resp = try std.fmt.allocPrint(getAllocator(), "{{\"status\":\"finished\",\"elapsed_seconds\":{}}}", .{elapsed_seconds});
        defer getAllocator().free(resp);
        try request.respond(resp, .{
            .extra_headers = &.{.{ .name = "Content-Type", .value = "application/json" }},
        });
        return;
    };

    if (habit_id != null and elapsed > 0) {
        const timestamp: i64 = @intCast(std.time.timestamp());
        const es = std.time.epoch.EpochSeconds{ .secs = @as(u64, @intCast(timestamp)) };
        const yd = es.getEpochDay().calculateYearDay();
        const md = yd.calculateMonthDay();
        var buffer: [10]u8 = undefined;
        const today = std.fmt.bufPrint(&buffer, "{d:0>4}-{d:0>2}-{d:0>2}", .{ yd.year, md.month.numeric(), md.day_index + 1 }) catch "";

        _ = app.settings_manager.sqlite_db.?.habit_manager.createSession(
            habit_id.?,
            elapsed,
            1,
            today,
        ) catch |err| {
            logger.global_logger.err("创建日统计记录失败: {any}", .{err});
        };
    }

    app.resetTimerSession();

    const resp = try std.fmt.allocPrint(getAllocator(), "{{\"status\":\"finished\",\"elapsed_seconds\":{},\"session_id\":{?}}}", .{ elapsed, session_id });
    defer getAllocator().free(resp);
    try request.respond(resp, .{
        .extra_headers = &.{.{ .name = "Content-Type", .value = "application/json" }},
    });
}

fn handleGetProgress(request: *http.Server.Request) !void {
    const app = getApp();

    if (app.current_timer_session_id == null) {
        app.loadTimerProgress();
    }

    const session_id = app.current_timer_session_id;
    const habit_id = app.current_habit_id;
    const clock_state = app.clock_manager.update();
    const mode_key = switch (clock_state.getMode()) {
        .COUNTDOWN_MODE => "countdown",
        .STOPWATCH_MODE => "stopwatch",
    };

    const resp = try std.fmt.allocPrint(getAllocator(), "{{\"session_id\":{?},\"habit_id\":{?},\"mode\":\"{s}\",\"is_running\":{},\"is_paused\":{},\"is_finished\":{},\"elapsed_seconds\":{},\"remaining_seconds\":{},\"in_rest\":{}}}", .{
        session_id,
        habit_id,
        mode_key,
        !clock_state.isPaused(),
        clock_state.isPaused(),
        clock_state.isFinished(),
        clock_state.getElapsedSeconds(),
        clock_state.getRemainingSeconds(),
        clock_state.inRest(),
    });
    defer getAllocator().free(resp);
    try request.respond(resp, .{
        .extra_headers = &.{.{ .name = "Content-Type", .value = "application/json" }},
    });
}

fn handleModeChange(request: *http.Server.Request) !void {
    logger.global_logger.info("[API] POST /api/mode", .{});
    const allocator = getAllocator();
    const body = try readRequestBody(request, allocator);
    defer allocator.free(body);
    const mode_str = std.mem.trim(u8, body, " \n\r\t");
    const app = getApp();

    const new_mode = if (std.mem.eql(u8, mode_str, "countdown"))
        ModeEnumT.COUNTDOWN_MODE
    else if (std.mem.eql(u8, mode_str, "stopwatch"))
        ModeEnumT.STOPWATCH_MODE
    else {
        try request.respond("{}", .{
            .extra_headers = &.{.{ .name = "Content-Type", .value = "application/json" }},
        });
        return;
    };

    app.clock_manager.handleEvent(.{ .user_change_mode = new_mode });
    const resp = try std.fmt.allocPrint(getAllocator(), "{{\"status\":\"mode_changed\",\"new_mode\":\"{s}\"}}", .{mode_str});
    defer getAllocator().free(resp);
    try request.respond(resp, .{
        .extra_headers = &.{.{ .name = "Content-Type", .value = "application/json" }},
    });
}

fn handleGetSettings(request: *http.Server.Request) !void {
    logger.global_logger.debug("[API] GET /api/settings", .{});
    const app = getApp();
    const config = app.settings_manager.getConfig();

    const serialized = try std.fmt.allocPrint(getAllocator(), "{f}", .{std.json.fmt(config, .{})});
    defer getAllocator().free(serialized);
    try request.respond(serialized, .{
        .extra_headers = &.{.{ .name = "Content-Type", .value = "application/json" }},
    });
}

fn handleUpdateSettings(request: *http.Server.Request) !void {
    const app = getApp();
    const allocator = getAllocator();
    const body = try readRequestBody(request, allocator);
    defer allocator.free(body);

    const body_copy: [:0]u8 = try allocator.allocSentinel(u8, body.len, 0);
    @memcpy(body_copy[0..body.len], body);
    try app.settings_manager.handleSettingsEvent(.{ .change_settings = body_copy });

    try request.respond("{\"status\":\"settings_updated\"}", .{
        .extra_headers = &.{.{ .name = "Content-Type", .value = "application/json" }},
    });
}

fn handleSSE(request: *http.Server.Request) !void {
    logger.global_logger.info("SSE 客户端连接建立", .{});

    const app = getApp();
    var buffer: [8192]u8 = undefined;
    var last_tick_ts = std.time.timestamp();
    var last_heartbeat_ts = std.time.timestamp();

    var body_writer = try request.respondStreaming(&buffer, .{
        .respond_options = .{
            .status = .ok,
            .extra_headers = &.{
                .{ .name = "Content-Type", .value = "text/event-stream" },
                .{ .name = "Cache-Control", .value = "no-cache" },
                .{ .name = "Connection", .value = "keep-alive" },
            },
        },
    });
    defer body_writer.end() catch {};

    while (true) {
        std.Thread.sleep(1_000_000_000);

        const now = std.time.timestamp();
        const delta_s = now - last_tick_ts;
        last_tick_ts = now;

        if (delta_s > 0) {
            app.clock_manager.handleEvent(.{ .tick = delta_s * 1000 });
        }

        const display_data = app.clock_manager.update();
        const timezone: i8 = app.settings_manager.config.basic.timezone;
        const habit_id = app.current_habit_id;

        const state_json = try buildStateJson(display_data, switch (display_data.getMode()) {
            .COUNTDOWN_MODE => "countdown",
            .STOPWATCH_MODE => "stopwatch",
        }, timezone, habit_id);
        defer getAllocator().free(state_json);

        try body_writer.writer.print("data: {s}\n\n", .{state_json});
        try body_writer.flush();

        if (now - last_heartbeat_ts >= 10) {
            last_heartbeat_ts = now;
            try body_writer.writer.print(": heartbeat\n\n", .{});
            try body_writer.flush();
        }
    }
}

fn handleGetHabitSets(request: *http.Server.Request) !void {
    const app = getApp();
    const habit_manager = &app.settings_manager.sqlite_db.?.*.habit_manager;
    const habit_sets = habit_manager.getAllHabitSets() catch |err| {
        logger.global_logger.err("获取习惯集失败: {any}", .{err});
        try request.respond("{\"err\":\"Failed to get habit sets\"}", .{
            .extra_headers = &.{.{ .name = "Content-Type", .value = "application/json" }},
        });
        return;
    };
    defer habit_manager.freeHabitSets(habit_sets);

    const serialized = try std.fmt.allocPrint(getAllocator(), "{f}", .{std.json.fmt(habit_sets, .{})});
    defer getAllocator().free(serialized);
    try request.respond(serialized, .{
        .extra_headers = &.{.{ .name = "Content-Type", .value = "application/json" }},
    });
}

fn handleCreateHabitSet(request: *http.Server.Request) !void {
    const allocator = getAllocator();
    const body = try readRequestBody(request, allocator);
    defer allocator.free(body);
    const app = getApp();

    var parsed = std.json.parseFromSlice(std.json.Value, allocator, body, .{}) catch |err| {
        logger.global_logger.err("解析习惯集创建请求失败: {any}", .{err});
        try request.respond("{\"err\":\"Invalid JSON\"}", .{
            .extra_headers = &.{.{ .name = "Content-Type", .value = "application/json" }},
        });
        return;
    };
    defer parsed.deinit();

    const root = parsed.value.object;
    const name_val = root.get("name") orelse {
        try request.respond("{\"err\":\"Missing name\"}", .{
            .extra_headers = &.{.{ .name = "Content-Type", .value = "application/json" }},
        });
        return;
    };

    if (name_val != .string or name_val.string.len == 0) {
        try request.respond("{\"err\":\"Invalid name\"}", .{
            .extra_headers = &.{.{ .name = "Content-Type", .value = "application/json" }},
        });
        return;
    }

    var description_str: []const u8 = "";
    var color_str: []const u8 = "#6366f1";

    if (root.get("description")) |val| {
        if (val == .string) description_str = val.string;
    }
    if (root.get("color")) |val| {
        if (val == .string) color_str = val.string;
    }

    const id = app.settings_manager.sqlite_db.?.*.habit_manager.createHabitSet(
        name_val.string,
        description_str,
        color_str,
    ) catch |err| {
        logger.global_logger.err("创建习惯集失败: {any}", .{err});
        try request.respond("{\"err\":\"Failed to create habit set\"}", .{
            .extra_headers = &.{.{ .name = "Content-Type", .value = "application/json" }},
        });
        return;
    };

    const resp = try std.fmt.allocPrint(allocator, "{{\"id\":{},\"name\":\"{s}\",\"description\":\"{s}\",\"color\":\"{s}\"}}", .{
        id, name_val.string, description_str, color_str,
    });
    defer allocator.free(resp);
    try request.respond(resp, .{
        .extra_headers = &.{.{ .name = "Content-Type", .value = "application/json" }},
    });
}

fn handleUpdateHabitSet(request: *http.Server.Request) !void {
    const target = request.head.target;
    const path = if (std.mem.indexOfScalar(u8, target, '?')) |q_idx| target[0..q_idx] else target;
    const id = parsePathId(path, "/api/habit-sets/") catch {
        try request.respond("{\"err\":\"Invalid id\"}", .{
            .extra_headers = &.{.{ .name = "Content-Type", .value = "application/json" }},
        });
        return;
    };

    const allocator = getAllocator();
    const body = try readRequestBody(request, allocator);
    defer allocator.free(body);
    const app = getApp();

    var parsed = std.json.parseFromSlice(std.json.Value, allocator, body, .{}) catch |err| {
        logger.global_logger.err("解析习惯集更新请求失败: {any}", .{err});
        try request.respond("{\"err\":\"Invalid JSON\"}", .{
            .extra_headers = &.{.{ .name = "Content-Type", .value = "application/json" }},
        });
        return;
    };
    defer parsed.deinit();

    const root = parsed.value.object;

    var name: []const u8 = undefined;
    var description_str: []const u8 = undefined;
    var color_str: []const u8 = undefined;
    var wallpaper: []const u8 = undefined;

    if (root.get("name")) |val| {
        if (val == .string and val.string.len > 0) {
            name = val.string;
        }
    }
    if (root.get("description")) |val| {
        if (val == .string) description_str = val.string;
    }
    if (root.get("color")) |val| {
        if (val == .string) color_str = val.string;
    }
    if (root.get("wallpaper")) |val| {
        if (val == .string) wallpaper = val.string;
    }

    if (name.len == 0) {
        try request.respond("{\"err\":\"Missing name\"}", .{
            .extra_headers = &.{.{ .name = "Content-Type", .value = "application/json" }},
        });
        return;
    }

    if (description_str.len == 0) description_str = "";
    if (color_str.len == 0) color_str = "#6366f1";
    if (wallpaper.len == 0) wallpaper = "";

    app.settings_manager.sqlite_db.?.*.habit_manager.updateHabitSet(id, name, description_str, color_str, wallpaper) catch |err| {
        logger.global_logger.err("更新习惯集失败: {any}", .{err});
        try request.respond("{\"err\":\"Failed to update habit set\"}", .{
            .extra_headers = &.{.{ .name = "Content-Type", .value = "application/json" }},
        });
        return;
    };

    const resp = try std.fmt.allocPrint(allocator, "{{\"id\":{},\"name\":\"{s}\",\"description\":\"{s}\",\"color\":\"{s}\",\"wallpaper\":\"{s}\"}}", .{
        id, name, description_str, color_str, wallpaper,
    });
    defer allocator.free(resp);
    try request.respond(resp, .{
        .extra_headers = &.{.{ .name = "Content-Type", .value = "application/json" }},
    });
}

fn handleDeleteHabitSet(request: *http.Server.Request) !void {
    const target = request.head.target;
    const path = if (std.mem.indexOfScalar(u8, target, '?')) |q_idx| target[0..q_idx] else target;
    const id = parsePathId(path, "/api/habit-sets/") catch {
        try request.respond("{\"err\":\"Invalid id\"}", .{
            .extra_headers = &.{.{ .name = "Content-Type", .value = "application/json" }},
        });
        return;
    };

    const app = getApp();
    app.settings_manager.sqlite_db.?.*.habit_manager.deleteHabitSet(id) catch |err| {
        logger.global_logger.err("删除习惯集失败: {any}", .{err});
        try request.respond("{\"err\":\"Failed to delete habit set\"}", .{
            .extra_headers = &.{.{ .name = "Content-Type", .value = "application/json" }},
        });
        return;
    };

    try request.respond("{\"success\":true}", .{
        .extra_headers = &.{.{ .name = "Content-Type", .value = "application/json" }},
    });
}

fn handleGetHabits(request: *http.Server.Request) !void {
    const app = getApp();
    const habit_manager = &app.settings_manager.sqlite_db.?.*.habit_manager;
    const habits = habit_manager.getAllHabits() catch {
        try request.respond("{\"err\":\"Failed to get habits\"}", .{
            .extra_headers = &.{.{ .name = "Content-Type", .value = "application/json" }},
        });
        return;
    };
    defer habit_manager.freeHabits(habits);

    const serialized = try std.fmt.allocPrint(getAllocator(), "{f}", .{std.json.fmt(habits, .{})});
    defer getAllocator().free(serialized);
    try request.respond(serialized, .{
        .extra_headers = &.{.{ .name = "Content-Type", .value = "application/json" }},
    });
}

fn handleCreateHabit(request: *http.Server.Request) !void {
    const allocator = getAllocator();
    const body = try readRequestBody(request, allocator);
    defer allocator.free(body);
    const app = getApp();

    var parsed = std.json.parseFromSlice(std.json.Value, allocator, body, .{}) catch |err| {
        logger.global_logger.err("解析习惯创建请求失败: {any}", .{err});
        try request.respond("{\"err\":\"Invalid JSON\"}", .{
            .extra_headers = &.{.{ .name = "Content-Type", .value = "application/json" }},
        });
        return;
    };
    defer parsed.deinit();

    const root = parsed.value.object;
    const set_id_val = root.get("set_id") orelse {
        try request.respond("{\"err\":\"Missing set_id\"}", .{
            .extra_headers = &.{.{ .name = "Content-Type", .value = "application/json" }},
        });
        return;
    };
    const name_val = root.get("name") orelse {
        try request.respond("{\"err\":\"Missing name\"}", .{
            .extra_headers = &.{.{ .name = "Content-Type", .value = "application/json" }},
        });
        return;
    };

    if (set_id_val != .integer or name_val != .string or name_val.string.len == 0) {
        try request.respond("{\"err\":\"Invalid parameters\"}", .{
            .extra_headers = &.{.{ .name = "Content-Type", .value = "application/json" }},
        });
        return;
    }

    const set_id = set_id_val.integer;
    const name = name_val.string;
    const goal_seconds: i64 = if (root.get("goal_seconds")) |v| if (v == .integer) v.integer else 1500 else 1500;
    var color_str: []const u8 = "#6366f1";
    if (root.get("color")) |val| {
        if (val == .string) color_str = val.string;
    }

    const id = app.settings_manager.sqlite_db.?.*.habit_manager.createHabit(
        set_id,
        name,
        goal_seconds,
        color_str,
    ) catch |err| {
        logger.global_logger.err("创建习惯失败: {any}", .{err});
        try request.respond("{\"err\":\"Failed to create habit\"}", .{
            .extra_headers = &.{.{ .name = "Content-Type", .value = "application/json" }},
        });
        return;
    };

    const resp = try std.fmt.allocPrint(allocator, "{{\"id\":{},\"set_id\":{},\"name\":\"{s}\",\"goal_seconds\":{},\"color\":\"{s}\"}}", .{
        id, set_id, name, goal_seconds, color_str,
    });
    defer allocator.free(resp);
    try request.respond(resp, .{
        .extra_headers = &.{.{ .name = "Content-Type", .value = "application/json" }},
    });
}

fn handleDeleteHabit(request: *http.Server.Request) !void {
    const target = request.head.target;
    const path = if (std.mem.indexOfScalar(u8, target, '?')) |q_idx| target[0..q_idx] else target;
    const id = parsePathId(path, "/api/habits/") catch {
        try request.respond("{\"err\":\"Invalid id\"}", .{
            .extra_headers = &.{.{ .name = "Content-Type", .value = "application/json" }},
        });
        return;
    };

    const app = getApp();
    app.settings_manager.sqlite_db.?.*.habit_manager.deleteHabit(id) catch |err| {
        logger.global_logger.err("删除习惯失败: {any}", .{err});
        try request.respond("{\"err\":\"Failed to delete habit\"}", .{
            .extra_headers = &.{.{ .name = "Content-Type", .value = "application/json" }},
        });
        return;
    };

    try request.respond("{\"success\":true}", .{
        .extra_headers = &.{.{ .name = "Content-Type", .value = "application/json" }},
    });
}

fn handleUpdateHabit(request: *http.Server.Request) !void {
    const target = request.head.target;
    const path = if (std.mem.indexOfScalar(u8, target, '?')) |q_idx| target[0..q_idx] else target;
    const id = parsePathId(path, "/api/habits/") catch {
        try request.respond("{\"err\":\"Invalid id\"}", .{
            .extra_headers = &.{.{ .name = "Content-Type", .value = "application/json" }},
        });
        return;
    };

    const allocator = getAllocator();
    const body = try readRequestBody(request, allocator);
    defer allocator.free(body);
    const app = getApp();

    var parsed = std.json.parseFromSlice(std.json.Value, allocator, body, .{}) catch |err| {
        logger.global_logger.err("解析习惯更新请求失败: {any}", .{err});
        try request.respond("{\"err\":\"Invalid JSON\"}", .{
            .extra_headers = &.{.{ .name = "Content-Type", .value = "application/json" }},
        });
        return;
    };
    defer parsed.deinit();

    const root = parsed.value.object;

    var name: []const u8 = undefined;
    var goal_seconds: i64 = 0;
    var color_str: []const u8 = undefined;
    var wallpaper: []const u8 = undefined;

    if (root.get("name")) |val| {
        if (val == .string and val.string.len > 0) {
            name = val.string;
        }
    }
    if (root.get("goal_seconds")) |val| {
        if (val == .integer) goal_seconds = val.integer;
    }
    if (root.get("color")) |val| {
        if (val == .string) color_str = val.string;
    }
    if (root.get("wallpaper")) |val| {
        if (val == .string) wallpaper = val.string;
    }

    if (name.len == 0) {
        try request.respond("{\"err\":\"Missing name\"}", .{
            .extra_headers = &.{.{ .name = "Content-Type", .value = "application/json" }},
        });
        return;
    }

    if (goal_seconds == 0) goal_seconds = 1500;
    if (color_str.len == 0) color_str = "#6366f1";
    if (wallpaper.len == 0) wallpaper = "";

    app.settings_manager.sqlite_db.?.*.habit_manager.updateHabit(id, name, goal_seconds, color_str, wallpaper) catch |err| {
        logger.global_logger.err("更新习惯失败: {any}", .{err});
        try request.respond("{\"err\":\"Failed to update habit\"}", .{
            .extra_headers = &.{.{ .name = "Content-Type", .value = "application/json" }},
        });
        return;
    };

    const resp = try std.fmt.allocPrint(allocator, "{{\"id\":{},\"name\":\"{s}\",\"goal_seconds\":{},\"color\":\"{s}\",\"wallpaper\":\"{s}\"}}", .{
        id, name, goal_seconds, color_str, wallpaper,
    });
    defer allocator.free(resp);
    try request.respond(resp, .{
        .extra_headers = &.{.{ .name = "Content-Type", .value = "application/json" }},
    });
}

fn handleCreateSession(request: *http.Server.Request) !void {
    const allocator = getAllocator();
    const body = try readRequestBody(request, allocator);
    defer allocator.free(body);
    const app = getApp();

    var parsed = std.json.parseFromSlice(std.json.Value, allocator, body, .{}) catch |err| {
        logger.global_logger.err("解析记录创建请求失败: {any}", .{err});
        try request.respond("{\"err\":\"Invalid JSON\"}", .{
            .extra_headers = &.{.{ .name = "Content-Type", .value = "application/json" }},
        });
        return;
    };
    defer parsed.deinit();

    const root = parsed.value.object;

    const habit_id_val = root.get("habit_id") orelse {
        try request.respond("{\"err\":\"Missing habit_id\"}", .{
            .extra_headers = &.{.{ .name = "Content-Type", .value = "application/json" }},
        });
        return;
    };
    const duration_val = root.get("duration_seconds") orelse {
        try request.respond("{\"err\":\"Missing duration_seconds\"}", .{
            .extra_headers = &.{.{ .name = "Content-Type", .value = "application/json" }},
        });
        return;
    };

    if (habit_id_val != .integer or duration_val != .integer) {
        try request.respond("{\"err\":\"Invalid parameters\"}", .{
            .extra_headers = &.{.{ .name = "Content-Type", .value = "application/json" }},
        });
        return;
    }

    const habit_id = habit_id_val.integer;
    const duration_seconds = duration_val.integer;
    const count: i64 = if (root.get("count")) |v| if (v == .integer) v.integer else 1 else 1;

    const timestamp: i64 = @intCast(std.time.timestamp());
    const es = std.time.epoch.EpochSeconds{ .secs = @as(u64, @intCast(timestamp)) };
    const yd = es.getEpochDay().calculateYearDay();
    const md = yd.calculateMonthDay();
    var buffer: [10]u8 = undefined;
    const date_str = std.fmt.bufPrint(&buffer, "{d:0>4}-{d:0>2}-{d:0>2}", .{ yd.year, md.month.numeric(), md.day_index + 1 }) catch "";

    const id = app.settings_manager.sqlite_db.?.*.habit_manager.createSession(
        habit_id,
        duration_seconds,
        count,
        date_str,
    ) catch |err| {
        logger.global_logger.err("创建记录失败: {any}", .{err});
        try request.respond("{\"err\":\"Failed to create session\"}", .{
            .extra_headers = &.{.{ .name = "Content-Type", .value = "application/json" }},
        });
        return;
    };

    const resp = try std.fmt.allocPrint(allocator, "{{\"id\":{},\"habit_id\":{},\"duration_seconds\":{},\"date\":\"{s}\"}}", .{
        id, habit_id, duration_seconds, date_str,
    });
    defer allocator.free(resp);
    try request.respond(resp, .{
        .extra_headers = &.{.{ .name = "Content-Type", .value = "application/json" }},
    });
}

fn handleGetSessions(request: *http.Server.Request) !void {
    const target = request.head.target;
    const query = if (std.mem.indexOfScalar(u8, target, '?')) |q_idx| target[q_idx + 1 ..] else null;
    var date: ?[]const u8 = null;
    var start_date: ?[]const u8 = null;
    var end_date: ?[]const u8 = null;

    if (query) |q| {
        var it = std.mem.splitScalar(u8, q, '&');
        while (it.next()) |pair| {
            if (std.mem.startsWith(u8, pair, "date=")) {
                date = pair[5..];
            } else if (std.mem.startsWith(u8, pair, "start_date=")) {
                start_date = pair[11..];
            } else if (std.mem.startsWith(u8, pair, "end_date=")) {
                end_date = pair[9..];
            }
        }
    }

    std.debug.print("[handleGetSessions] date={s}, start_date={s}, end_date={s}\n", .{ date orelse "null", start_date orelse "null", end_date orelse "null" });

    const app = getApp();

    var sessions: []habit_crud.SessionRow = &.{};
    var owns_sessions = false;
    defer if (owns_sessions) {
        app.settings_manager.sqlite_db.?.*.habit_manager.freeSessions(sessions);
    };

    if (start_date != null and end_date != null) {
        std.debug.print("[handleGetSessions] calling getSessionsByDateRange\n", .{});
        sessions = app.settings_manager.sqlite_db.?.*.habit_manager.getSessionsByDateRange(start_date.?, end_date.?) catch {
            try request.respond("{\"err\":\"Failed to get sessions\"}", .{
                .extra_headers = &.{.{ .name = "Content-Type", .value = "application/json" }},
            });
            return;
        };
    } else if (date != null) {
        sessions = app.settings_manager.sqlite_db.?.*.habit_manager.getSessionsByDate(date.?) catch {
            try request.respond("{\"err\":\"Failed to get sessions\"}", .{
                .extra_headers = &.{.{ .name = "Content-Type", .value = "application/json" }},
            });
            return;
        };
    } else {
        const timestamp: i64 = @intCast(std.time.timestamp());
        const es = std.time.epoch.EpochSeconds{ .secs = @as(u64, @intCast(timestamp)) };
        const yd = es.getEpochDay().calculateYearDay();
        const md = yd.calculateMonthDay();
        var buffer: [10]u8 = undefined;
        const today = std.fmt.bufPrint(&buffer, "{d:0>4}-{d:0>2}-{d:0>2}", .{ yd.year, md.month.numeric(), md.day_index + 1 }) catch "";
        sessions = app.settings_manager.sqlite_db.?.*.habit_manager.getSessionsByDate(today) catch {
            try request.respond("{\"err\":\"Failed to get sessions\"}", .{
                .extra_headers = &.{.{ .name = "Content-Type", .value = "application/json" }},
            });
            return;
        };
    }

    owns_sessions = true;

    std.debug.print("[handleGetSessions] returning {} sessions\n", .{sessions.len});

    const serialized = try std.fmt.allocPrint(getAllocator(), "{f}", .{std.json.fmt(sessions, .{})});
    defer getAllocator().free(serialized);
    try request.respond(serialized, .{
        .extra_headers = &.{.{ .name = "Content-Type", .value = "application/json" }},
    });
}

fn handleGetHabitStreak(request: *http.Server.Request) !void {
    const target = request.head.target;
    const path = if (std.mem.indexOfScalar(u8, target, '?')) |q_idx| target[0..q_idx] else target;
    const habit_id = parsePathIdWithSuffix(path, "/api/habits/", "/streak") catch {
        try request.respond("{\"err\":\"Invalid id\"}", .{
            .extra_headers = &.{.{ .name = "Content-Type", .value = "application/json" }},
        });
        return;
    };

    const app = getApp();
    const streak = app.settings_manager.sqlite_db.?.*.habit_manager.getHabitStreak(habit_id, 1500) catch {
        try request.respond("{\"err\":\"Failed to get streak\"}", .{
            .extra_headers = &.{.{ .name = "Content-Type", .value = "application/json" }},
        });
        return;
    };

    const resp = try std.fmt.allocPrint(getAllocator(), "{{\"habit_id\":{},\"streak\":{}}}", .{ habit_id, streak });
    defer getAllocator().free(resp);
    try request.respond(resp, .{
        .extra_headers = &.{.{ .name = "Content-Type", .value = "application/json" }},
    });
}

fn handleGetHabitDetail(request: *http.Server.Request) !void {
    const target = request.head.target;
    const path = if (std.mem.indexOfScalar(u8, target, '?')) |q_idx| target[0..q_idx] else target;
    const habit_id = parsePathIdWithSuffix(path, "/api/habits/", "/detail") catch {
        try request.respond("{\"err\":\"Invalid id\"}", .{
            .extra_headers = &.{.{ .name = "Content-Type", .value = "application/json" }},
        });
        return;
    };

    var date_param: []const u8 = "";
    if (std.mem.indexOfScalar(u8, target, '?')) |q_idx| {
        const query = target[q_idx + 1 ..];
        var it = std.mem.splitScalar(u8, query, '&');
        while (it.next()) |pair| {
            if (std.mem.startsWith(u8, pair, "date=")) {
                date_param = pair[5..];
                break;
            }
        }
    }

    var date_buffer: [10]u8 = undefined;
    if (date_param.len == 0) {
        const timestamp: i64 = @intCast(std.time.timestamp());
        const es = std.time.epoch.EpochSeconds{ .secs = @as(u64, @intCast(timestamp)) };
        const yd = es.getEpochDay().calculateYearDay();
        const md = yd.calculateMonthDay();
        date_param = std.fmt.bufPrint(&date_buffer, "{d:0>4}-{d:0>2}-{d:0>2}", .{ yd.year, md.month.numeric(), md.day_index + 1 }) catch "";
    }

    const app = getApp();
    const habit_manager = &app.settings_manager.sqlite_db.?.*.habit_manager;

    const habit = habit_manager.getHabitById(habit_id) catch {
        try request.respond("{\"err\":\"Failed to get habit\"}", .{
            .extra_headers = &.{.{ .name = "Content-Type", .value = "application/json" }},
        });
        return;
    };

    const h_row = habit orelse {
        try request.respond("{\"err\":\"Habit not found\"}", .{
            .extra_headers = &.{.{ .name = "Content-Type", .value = "application/json" }},
        });
        return;
    };
    defer habit_manager.freeHabit(h_row);

    const today_seconds = habit_manager.getHabitTodaySeconds(habit_id, date_param) catch 0;
    const streak = habit_manager.getHabitStreak(habit_id, h_row.goal_seconds) catch 0;

    const progress_percent: i64 = if (h_row.goal_seconds > 0) @divTrunc(today_seconds * 100, h_row.goal_seconds) else 0;

    const resp = try std.fmt.allocPrint(getAllocator(), "{{\"id\":{},\"name\":\"{s}\",\"goal_seconds\":{},\"color\":\"{s}\",\"today_seconds\":{},\"streak\":{},\"progress_percent\":{}}}", .{
        h_row.id, h_row.name, h_row.goal_seconds, h_row.color, today_seconds, streak, progress_percent,
    });
    defer getAllocator().free(resp);
    try request.respond(resp, .{
        .extra_headers = &.{.{ .name = "Content-Type", .value = "application/json" }},
    });
}
