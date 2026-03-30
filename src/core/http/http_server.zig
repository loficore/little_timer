const std = @import("std");
const json = std.json;
const httpz = @import("httpz");
const logger = @import("../logger.zig");
const clock = @import("../clock.zig");
const settings = @import("../../settings/mod.zig");
const habit_crud = @import("../../storage/habit_crud.zig");
const MainApplication = @import("../app.zig").MainApplication;
const build_options = @import("build_options");

const ClockState = clock.ClockState;
const ModeEnumT = clock.ModeEnumT;

/// HTTP 处理器上下文，包含应用状态和配置
pub const HttpHandler = struct {
    app: *MainApplication,
    allocator: std.mem.Allocator,
    sse_pending: bool,
};

/// 处理根路径请求，返回前端界面或提示信息
/// 参数：
/// - **h** : HTTP 处理器上下文，包含应用状态和配置
/// - **request** : HTTP 请求对象，包含客户端请求信息
/// - **response** : HTTP 响应对象，用于发送 HTML 内容
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

/// 处理获取计时器状态的请求，返回当前时间、模式、运行状态等信息
/// 参数：
/// - **h** : HTTP 处理器上下文，包含应用状态和配置
/// - **request** : HTTP 请求对象，包含客户端请求信息
fn handleGetState(h: *HttpHandler, request: *httpz.Request, response: *httpz.Response) !void {
    _ = request;

    const display_data = h.app.clock_manager.update();
    const mode_key = switch (display_data.getMode()) {
        .COUNTDOWN_MODE => "countdown",
        .STOPWATCH_MODE => "stopwatch",
    };

    const timezone: i8 = h.app.settings_manager.config.basic.timezone;

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

inline fn triggerSSEPush(h: *HttpHandler) void {
    h.sse_pending = true;
}

fn handleStart(h: *HttpHandler, request: *httpz.Request, response: *httpz.Response) !void {
    const body = request.body() orelse "";

    var habit_id: ?i64 = null;
    if (body.len > 0) {
        const parsed = std.json.parseFromSlice(std.json.Value, h.allocator, body, .{}) catch null;
        if (parsed) |p| {
            defer p.deinit();
            if (p.value.object.get("habit_id")) |hid| {
                if (hid == .integer) {
                    habit_id = hid.integer;
                }
            }
        }
    }

    h.app.current_habit_id = habit_id;
    h.app.clock_manager.handleEvent(.user_start_timer);
    triggerSSEPush(h);
    try response.json(.{ .status = "started", .habit_id = habit_id }, .{});
}

fn handleStartRest(h: *HttpHandler, request: *httpz.Request, response: *httpz.Response) !void {
    _ = request;

    // 获取休息时长（默认5分钟）
    const rest_seconds: i64 = 5 * 60;

    // 切换到倒计时模式并设置休息时间
    h.app.clock_manager.handleEvent(.{ .user_change_config = .{
        .default_mode = .COUNTDOWN_MODE,
        .countdown = .{
            .duration_seconds = @intCast(rest_seconds),
            .loop = false,
            .loop_count = 0,
            .loop_interval_seconds = 0,
        },
        .stopwatch = .{ .max_seconds = 24 * 60 * 60 },
    } });

    // 开始倒计时
    h.app.clock_manager.handleEvent(.user_start_timer);
    triggerSSEPush(h);
    try response.json(.{ .status = "rest_started", .rest_seconds = rest_seconds }, .{});
}

fn handlePause(h: *HttpHandler, request: *httpz.Request, response: *httpz.Response) !void {
    _ = request;
    h.app.clock_manager.handleEvent(.user_pause_timer);
    triggerSSEPush(h);
    try response.json(.{ .status = "paused" }, .{});
}

fn handleReset(h: *HttpHandler, request: *httpz.Request, response: *httpz.Response) !void {
    _ = request;
    h.app.current_habit_id = null;
    h.app.clock_manager.handleEvent(.user_reset_timer);
    triggerSSEPush(h);
    try response.json(.{ .status = "reset" }, .{});
}

fn handleModeChange(h: *HttpHandler, request: *httpz.Request, response: *httpz.Response) !void {
    const body = request.body() orelse "";
    const mode_str = std.mem.trim(u8, body, " \n\r\t");

    const new_mode = if (std.mem.eql(u8, mode_str, "countdown"))
        ModeEnumT.COUNTDOWN_MODE
    else if (std.mem.eql(u8, mode_str, "stopwatch"))
        ModeEnumT.STOPWATCH_MODE
    else {
        try response.json(std.json.Value{ .object = std.StringArrayHashMap(std.json.Value).init(h.allocator) }, .{});
        return;
    };

    h.app.clock_manager.handleEvent(.{ .user_change_mode = new_mode });
    triggerSSEPush(h);
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

fn buildStateJson(allocator: std.mem.Allocator, display_data: *const ClockState, timezone: i8, habit_id: ?i64) ![]u8 {
    const mode_key = switch (display_data.getMode()) {
        .COUNTDOWN_MODE => "countdown",
        .STOPWATCH_MODE => "stopwatch",
    };

    const habit_id_json = if (habit_id) |hid| try std.fmt.allocPrint(allocator, ",\"habit_id\":{}", .{hid}) else "";
    const elapsed_seconds = display_data.getElapsedSeconds();

    return try std.fmt.allocPrint(allocator, "{{\"time\":{},\"elapsed\":{},\"mode\":\"{s}\",\"is_running\":{},\"is_finished\":{},\"in_rest\":{},\"loop_remaining\":{},\"loop_total\":{},\"rest_remaining\":{},\"timezone\":{}{s}}}", .{
        display_data.getTimeInfo(),
        elapsed_seconds,
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

/// 处理 Server-Sent Events 连接，持续推送计时器状态更新
/// 参数：
/// - **h** : HTTP 处理器上下文，包含应用状态和配置
/// - **request** : HTTP 请求对象，包含客户端请求信息
/// - **response** : HTTP 响应对象，用于发送 SSE 数据流
fn handleSSE(h: *HttpHandler, request: *httpz.Request, response: *httpz.Response) !void {
    _ = request;

    logger.global_logger.info("SSE 客户端连接建立", .{});

    const stream = try response.startEventStreamSync();

    var last_tick_ts = std.time.timestamp();
    var last_state_json: ?[]const u8 = null;
    var last_heartbeat_ts = std.time.timestamp();
    var pending_push = false;

    while (true) {
        var sleep_ns: u64 = 1_000_000_000;
        if (pending_push) {
            sleep_ns = 0;
            pending_push = false;
        }

        const now = std.time.timestamp();
        const delta_s = now - last_tick_ts;
        last_tick_ts = now;

        if (delta_s > 0) {
            h.app.clock_manager.handleEvent(.{ .tick = delta_s * 1000 });
        }

        const display_data = h.app.clock_manager.update();
        const timezone: i8 = h.app.settings_manager.config.basic.timezone;
        const habit_id = h.app.current_habit_id;

        const buf = try buildStateJson(h.allocator, display_data, timezone, habit_id);
        defer h.allocator.free(buf);

        const has_change = if (last_state_json) |last| !std.mem.eql(u8, last, buf) else true;

        if (has_change) {
            if (last_state_json) |last| h.allocator.free(last);
            last_state_json = try h.allocator.dupe(u8, buf);
            last_heartbeat_ts = now;

            try stream.writeAll("data: ");
            try stream.writeAll(buf);
            try stream.writeAll("\n\n");
        } else {
            if (now - last_heartbeat_ts >= 10) {
                last_heartbeat_ts = now;
                try stream.writeAll(": heartbeat\n\n");
            }
        }

        std.Thread.sleep(sleep_ns);

        if (h.sse_pending) {
            h.sse_pending = false;
            pending_push = true;
        }
    }
}

// === 习惯集 API ===

fn handleGetHabitSets(h: *HttpHandler, _: *httpz.Request, response: *httpz.Response) !void {
    const habit_sets = h.app.settings_manager.sqlite_db.?.*.habit_manager.getAllHabitSets() catch |err| {
        logger.global_logger.err("获取习惯集失败: {any}", .{err});
        try response.json(.{ .err = "Failed to get habit sets" }, .{});
        return;
    };
    try response.json(habit_sets, .{});
}

fn handleCreateHabitSet(h: *HttpHandler, request: *httpz.Request, response: *httpz.Response) !void {
    const body = request.body() orelse "";

    var parsed = std.json.parseFromSlice(std.json.Value, h.allocator, body, .{}) catch |err| {
        logger.global_logger.err("解析习惯集创建请求失败: {any}", .{err});
        try response.json(.{ .err = "Invalid JSON" }, .{});
        return;
    };
    defer parsed.deinit();

    const root = parsed.value.object;
    const name_val = root.get("name") orelse {
        try response.json(.{ .err = "Missing name" }, .{});
        return;
    };

    if (name_val != .string or name_val.string.len == 0) {
        try response.json(.{ .err = "Invalid name" }, .{});
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

    const id = h.app.settings_manager.sqlite_db.?.*.habit_manager.createHabitSet(
        name_val.string,
        description_str,
        color_str,
    ) catch |err| {
        logger.global_logger.err("创建习惯集失败: {any}", .{err});
        try response.json(.{ .err = "Failed to create habit set" }, .{});
        return;
    };

    try response.json(.{ .id = id, .name = name_val.string, .description = description_str, .color = color_str }, .{});
}

fn handleUpdateHabitSet(h: *HttpHandler, request: *httpz.Request, response: *httpz.Response) !void {
    const id_str = request.params.get("id") orelse {
        try response.json(.{ .err = "Missing id" }, .{});
        return;
    };
    const id = std.fmt.parseInt(i64, id_str, 10) catch {
        try response.json(.{ .err = "Invalid id" }, .{});
        return;
    };

    const body = request.body() orelse "";
    var parsed = std.json.parseFromSlice(std.json.Value, h.allocator, body, .{}) catch |err| {
        logger.global_logger.err("解析习惯集更新请求失败: {any}", .{err});
        try response.json(.{ .err = "Invalid JSON" }, .{});
        return;
    };
    defer parsed.deinit();

    const root = parsed.value.object;

    var name: []const u8 = undefined;
    var description_str: []const u8 = undefined;
    var color_str: []const u8 = undefined;

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

    if (name.len == 0) {
        try response.json(.{ .err = "Missing name" }, .{});
        return;
    }

    if (description_str.len == 0) description_str = "";
    if (color_str.len == 0) color_str = "#6366f1";

    h.app.settings_manager.sqlite_db.?.*.habit_manager.updateHabitSet(id, name, description_str, color_str) catch |err| {
        logger.global_logger.err("更新习惯集失败: {any}", .{err});
        try response.json(.{ .err = "Failed to update habit set" }, .{});
        return;
    };

    try response.json(.{ .id = id, .name = name, .description = description_str, .color = color_str }, .{});
}

fn handleDeleteHabitSet(h: *HttpHandler, request: *httpz.Request, response: *httpz.Response) !void {
    const id_str = request.params.get("id") orelse {
        try response.json(.{ .err = "Missing id" }, .{});
        return;
    };
    const id = std.fmt.parseInt(i64, id_str, 10) catch {
        try response.json(.{ .err = "Invalid id" }, .{});
        return;
    };

    h.app.settings_manager.sqlite_db.?.*.habit_manager.deleteHabitSet(id) catch |err| {
        logger.global_logger.err("删除习惯集失败: {any}", .{err});
        try response.json(.{ .err = "Failed to delete habit set" }, .{});
        return;
    };

    try response.json(.{ .success = true }, .{});
}

// === 习惯 API ===

fn handleGetHabits(h: *HttpHandler, _: *httpz.Request, response: *httpz.Response) !void {
    const habits = h.app.settings_manager.sqlite_db.?.*.habit_manager.getAllHabits() catch {
        try response.json(.{ .err = "Failed to get habits" }, .{});
        return;
    };
    try response.json(habits, .{});
}

fn handleCreateHabit(h: *HttpHandler, request: *httpz.Request, response: *httpz.Response) !void {
    const body = request.body() orelse "";

    var parsed = std.json.parseFromSlice(std.json.Value, h.allocator, body, .{}) catch |err| {
        logger.global_logger.err("解析习惯创建请求失败: {any}", .{err});
        try response.json(.{ .err = "Invalid JSON" }, .{});
        return;
    };
    defer parsed.deinit();

    const root = parsed.value.object;
    const set_id_val = root.get("set_id") orelse {
        try response.json(.{ .err = "Missing set_id" }, .{});
        return;
    };
    const name_val = root.get("name") orelse {
        try response.json(.{ .err = "Missing name" }, .{});
        return;
    };

    if (set_id_val != .integer or name_val != .string or name_val.string.len == 0) {
        try response.json(.{ .err = "Invalid parameters" }, .{});
        return;
    }

    const set_id = set_id_val.integer;
    const name = name_val.string;
    const goal_seconds: i64 = if (root.get("goal_seconds")) |v| if (v == .integer) v.integer else 1500 else 1500;
    var color_str: []const u8 = "#6366f1";
    if (root.get("color")) |val| {
        if (val == .string) color_str = val.string;
    }

    const id = h.app.settings_manager.sqlite_db.?.*.habit_manager.createHabit(
        set_id,
        name,
        goal_seconds,
        color_str,
    ) catch |err| {
        logger.global_logger.err("创建习惯失败: {any}", .{err});
        try response.json(.{ .err = "Failed to create habit" }, .{});
        return;
    };

    try response.json(.{ .id = id, .set_id = set_id, .name = name, .goal_seconds = goal_seconds, .color = color_str }, .{});
}

fn handleDeleteHabit(h: *HttpHandler, request: *httpz.Request, response: *httpz.Response) !void {
    const id_str = request.params.get("id") orelse {
        try response.json(.{ .err = "Missing id" }, .{});
        return;
    };
    const id = std.fmt.parseInt(i64, id_str, 10) catch {
        try response.json(.{ .err = "Invalid id" }, .{});
        return;
    };

    h.app.settings_manager.sqlite_db.?.*.habit_manager.deleteHabit(id) catch |err| {
        logger.global_logger.err("删除习惯失败: {any}", .{err});
        try response.json(.{ .err = "Failed to delete habit" }, .{});
        return;
    };

    try response.json(.{ .success = true }, .{});
}

fn handleUpdateHabit(h: *HttpHandler, request: *httpz.Request, response: *httpz.Response) !void {
    const id_str = request.params.get("id") orelse {
        try response.json(.{ .err = "Missing id" }, .{});
        return;
    };
    const id = std.fmt.parseInt(i64, id_str, 10) catch {
        try response.json(.{ .err = "Invalid id" }, .{});
        return;
    };

    const body = request.body() orelse "";
    var parsed = std.json.parseFromSlice(std.json.Value, h.allocator, body, .{}) catch |err| {
        logger.global_logger.err("解析习惯更新请求失败: {any}", .{err});
        try response.json(.{ .err = "Invalid JSON" }, .{});
        return;
    };
    defer parsed.deinit();

    const root = parsed.value.object;

    var name: []const u8 = undefined;
    var goal_seconds: i64 = 0;
    var color_str: []const u8 = undefined;
    var set_id: i64 = 0;

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
    if (root.get("set_id")) |val| {
        if (val == .integer) set_id = val.integer;
    }

    if (name.len == 0) {
        try response.json(.{ .err = "Missing name" }, .{});
        return;
    }

    if (goal_seconds == 0) goal_seconds = 1500;
    if (color_str.len == 0) color_str = "#6366f1";

    h.app.settings_manager.sqlite_db.?.*.habit_manager.updateHabit(id, name, goal_seconds, color_str) catch |err| {
        logger.global_logger.err("更新习惯失败: {any}", .{err});
        try response.json(.{ .err = "Failed to update habit" }, .{});
        return;
    };

    try response.json(.{ .id = id, .name = name, .goal_seconds = goal_seconds, .color = color_str }, .{});
}

// === 记录 API ===

fn handleCreateSession(h: *HttpHandler, request: *httpz.Request, response: *httpz.Response) !void {
    const body = request.body() orelse "";

    var parsed = std.json.parseFromSlice(std.json.Value, h.allocator, body, .{}) catch |err| {
        logger.global_logger.err("解析记录创建请求失败: {any}", .{err});
        try response.json(.{ .err = "Invalid JSON" }, .{});
        return;
    };
    defer parsed.deinit();

    const root = parsed.value.object;

    const habit_id_val = root.get("habit_id") orelse {
        try response.json(.{ .err = "Missing habit_id" }, .{});
        return;
    };
    const duration_val = root.get("duration_seconds") orelse {
        try response.json(.{ .err = "Missing duration_seconds" }, .{});
        return;
    };

    if (habit_id_val != .integer or duration_val != .integer) {
        try response.json(.{ .err = "Invalid parameters" }, .{});
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

    const id = h.app.settings_manager.sqlite_db.?.*.habit_manager.createSession(
        habit_id,
        duration_seconds,
        count,
        date_str,
    ) catch |err| {
        logger.global_logger.err("创建记录失败: {any}", .{err});
        try response.json(.{ .err = "Failed to create session" }, .{});
        return;
    };

    try response.json(.{ .id = id, .habit_id = habit_id, .duration_seconds = duration_seconds, .date = date_str }, .{});
}

fn handleGetSessions(h: *HttpHandler, request: *httpz.Request, response: *httpz.Response) !void {
    const date = request.params.get("date");
    const start_date = request.params.get("start_date");
    const end_date = request.params.get("end_date");

    var sessions: []habit_crud.SessionRow = &.{};

    if (start_date != null and end_date != null) {
        sessions = h.app.settings_manager.sqlite_db.?.*.habit_manager.getSessionsByDateRange(start_date.?, end_date.?) catch {
            try response.json(.{ .err = "Failed to get sessions" }, .{});
            return;
        };
    } else if (date != null) {
        sessions = h.app.settings_manager.sqlite_db.?.*.habit_manager.getSessionsByDate(date.?) catch {
            try response.json(.{ .err = "Failed to get sessions" }, .{});
            return;
        };
    } else {
        const timestamp: i64 = @intCast(std.time.timestamp());
        const es = std.time.epoch.EpochSeconds{ .secs = @as(u64, @intCast(timestamp)) };
        const yd = es.getEpochDay().calculateYearDay();
        const md = yd.calculateMonthDay();
        var buffer: [10]u8 = undefined;
        const today = std.fmt.bufPrint(&buffer, "{d:0>4}-{d:0>2}-{d:0>2}", .{ yd.year, md.month.numeric(), md.day_index + 1 }) catch "";
        sessions = h.app.settings_manager.sqlite_db.?.*.habit_manager.getSessionsByDate(today) catch {
            try response.json(.{ .err = "Failed to get sessions" }, .{});
            return;
        };
    }

    try response.json(sessions, .{});
}

fn handleGetHabitStreak(h: *HttpHandler, request: *httpz.Request, response: *httpz.Response) !void {
    const id_str = request.params.get("id") orelse {
        try response.json(.{ .err = "Missing id" }, .{});
        return;
    };
    const habit_id = std.fmt.parseInt(i64, id_str, 10) catch {
        try response.json(.{ .err = "Invalid id" }, .{});
        return;
    };

    const goal_seconds: i64 = if (request.params.get("goal_seconds")) |gs|
        std.fmt.parseInt(i64, gs, 10) catch 1500
    else
        1500;

    const streak = h.app.settings_manager.sqlite_db.?.*.habit_manager.getHabitStreak(habit_id, goal_seconds) catch {
        try response.json(.{ .err = "Failed to get streak" }, .{});
        return;
    };

    try response.json(.{ .habit_id = habit_id, .streak = streak }, .{});
}

pub const HttpServerManager = struct {
    server: httpz.Server(*HttpHandler),
    handler: HttpHandler,

    pub fn init(allocator: std.mem.Allocator, port: u16, app: *MainApplication) !HttpServerManager {
        logger.global_logger.info("初始化 HTTP 服务器，端口: {}", .{port});

        var handler = HttpHandler{ .app = app, .allocator = allocator, .sse_pending = false };
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

        // 习惯集 API
        router.get("/api/habit-sets", handleGetHabitSets, .{});
        router.post("/api/habit-sets", handleCreateHabitSet, .{});
        router.put("/api/habit-sets/:id", handleUpdateHabitSet, .{});
        router.delete("/api/habit-sets/:id", handleDeleteHabitSet, .{});

        // 习惯 API
        router.get("/api/habits", handleGetHabits, .{});
        router.post("/api/habits", handleCreateHabit, .{});
        router.put("/api/habits/:id", handleUpdateHabit, .{});
        router.delete("/api/habits/:id", handleDeleteHabit, .{});

        // 记录 API
        router.post("/api/sessions", handleCreateSession, .{});
        router.get("/api/sessions", handleGetSessions, .{});
        router.get("/api/habits/:id/streak", handleGetHabitStreak, .{});

        // 计时器 API
        router.post("/api/timer/rest", handleStartRest, .{});

        logger.global_logger.info("HTTP 服务器路由注册完成", .{});

        return HttpServerManager{
            .server = server,
            .handler = handler,
        };
    }

    pub fn start(self: *HttpServerManager) !void {
        logger.global_logger.info("HTTP 服务器开始监听...", .{});
        try self.server.listen();
    }

    pub fn stop(self: *HttpServerManager) void {
        logger.global_logger.info("HTTP 服务器停止中...", .{});
        self.server.stop();
    }

    pub fn deinit(self: *HttpServerManager) void {
        logger.global_logger.info("HTTP 服务器释放资源...", .{});
        self.server.deinit();
    }
};
