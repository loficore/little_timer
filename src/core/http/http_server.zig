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

/// 处理前端日志接收请求
fn handleFrontendLog(h: *HttpHandler, request: *httpz.Request, response: *httpz.Response) !void {
    const body = request.body() orelse "";
    if (body.len == 0) {
        response.header("content-type", "application/json");
        try response.chunk("{\"success\":false,\"error\":\"empty body\"}");
        return;
    }

    const parsed = std.json.parseFromSlice(std.json.Value, h.allocator, body, .{}) catch null;
    defer if (parsed) |p| p.deinit();

    if (parsed == null) {
        response.header("content-type", "application/json");
        try response.chunk("{\"success\":false,\"error\":\"invalid json\"}");
        return;
    }

    const value = parsed.?.value;
    if (value != .object) {
        response.header("content-type", "application/json");
        try response.chunk("{\"success\":false,\"error\":\"not an object\"}");
        return;
    }

    const obj = value.object;
    const category = obj.get("category") orelse .null;
    const level = obj.get("level") orelse .null;
    const message = obj.get("message") orelse .null;

    const cat_str = if (category == .string) category.string else "unknown";
    const level_str = if (level == .string) level.string else "info";
    const msg_str = if (message == .string) message.string else "";

    const actual_level = logger.LogLevel.fromString(level_str) orelse .INFO;
    const level_int = @intFromEnum(actual_level);

    if (level_int >= @intFromEnum(logger.LogLevel.ERROR)) {
        logger.global_logger.err("[前端:{s}] {s}", .{ cat_str, msg_str });
    } else if (level_int >= @intFromEnum(logger.LogLevel.WARN)) {
        logger.global_logger.warn("[前端:{s}] {s}", .{ cat_str, msg_str });
    } else if (level_int >= @intFromEnum(logger.LogLevel.INFO)) {
        logger.global_logger.info("[前端:{s}] {s}", .{ cat_str, msg_str });
    } else {
        logger.global_logger.debug("[前端:{s}] {s}", .{ cat_str, msg_str });
    }

    response.header("content-type", "application/json");
    try response.chunk("{\"success\":true}");
}

/// 处理获取计时器状态的请求，返回当前时间、模式、运行状态等信息
/// 参数：
/// - **h** : HTTP 处理器上下文，包含应用状态和配置
/// - **request** : HTTP 请求对象，包含客户端请求信息
fn handleGetState(h: *HttpHandler, request: *httpz.Request, response: *httpz.Response) !void {
    _ = request;
    logger.global_logger.debug("[API] GET /api/state", .{});

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
    logger.global_logger.info("[API] POST /api/start", .{});
    const body = request.body() orelse "";

    var habit_id: ?i64 = null;
    var mode: []const u8 = "stopwatch";
    var work_duration: i64 = 25 * 60;
    var rest_duration: i64 = 0;
    var loop_count: i64 = 0;

    if (body.len > 0) {
        const parsed = std.json.parseFromSlice(std.json.Value, h.allocator, body, .{}) catch null;
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

    const db = h.app.settings_manager.sqlite_db orelse {
        try response.json(.{ .err = "Database not open" }, .{});
        return;
    };

    // 如果当前已有暂停中的会话，则视为恢复同一会话，而不是创建新会话
    if (h.app.current_timer_session_id) |session_id| {
        const session = db.habit_manager.getTimerSessionById(session_id) catch null;
        if (session) |s| {
            defer h.allocator.free(s.mode);

            // 幂等保护：已有运行中的会话时，重复 start 请求不再创建新会话。
            if (s.is_running and !s.is_finished and !s.is_paused) {
                h.app.current_habit_id = habit_id orelse s.habit_id;
                triggerSSEPush(h);
                try response.json(.{ .status = "already_running", .habit_id = h.app.current_habit_id, .session_id = session_id }, .{});
                return;
            }

            const current_clock = h.app.clock_manager.update();
            if ((current_clock.isPaused() and !current_clock.isFinished()) or s.is_paused) {
                var paused_total_seconds = s.paused_total_seconds;
                const now_ts: i64 = @intCast(std.time.timestamp());
                if (s.pause_started_at) |ps| {
                    if (now_ts > ps) {
                        paused_total_seconds += now_ts - ps;
                    }
                }

                h.app.clock_manager.handleEvent(.user_start_timer);
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

                h.app.current_habit_id = habit_id orelse s.habit_id;
                triggerSSEPush(h);
                try response.json(.{ .status = "started", .habit_id = h.app.current_habit_id, .session_id = session_id }, .{});
                return;
            }
        }

        h.app.resetTimerSession();
    }

    // 创建新的计时会话
    const session_id = h.app.createTimerSession(habit_id, mode, work_duration, rest_duration, loop_count) catch {
        // 如果创建失败，回退到旧行为
        h.app.current_habit_id = habit_id;
        h.app.clock_manager.handleEvent(.user_start_timer);
        triggerSSEPush(h);
        try response.json(.{ .status = "started", .habit_id = habit_id }, .{});
        return;
    };

    h.app.current_habit_id = habit_id;
    h.app.clock_manager.handleEvent(.user_start_timer);
    triggerSSEPush(h);
    try response.json(.{ .status = "started", .habit_id = habit_id, .session_id = session_id }, .{});
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
    logger.global_logger.info("[API] POST /api/pause", .{});
    _ = request;
    h.app.clock_manager.handleEvent(.user_pause_timer);
    h.app.saveTimerProgress();
    triggerSSEPush(h);
    try response.json(.{ .status = "paused" }, .{});
}

fn handleReset(h: *HttpHandler, request: *httpz.Request, response: *httpz.Response) !void {
    logger.global_logger.info("[API] POST /api/reset", .{});
    _ = request;
    h.app.resetTimerSession();
    h.app.current_habit_id = null;
    h.app.clock_manager.handleEvent(.user_reset_timer);
    triggerSSEPush(h);
    try response.json(.{ .status = "reset" }, .{});
}

fn handleFinish(h: *HttpHandler, request: *httpz.Request, response: *httpz.Response) !void {
    logger.global_logger.info("[API] POST /api/finish", .{});
    _ = request;

    const habit_id = h.app.current_habit_id;
    const session_id = h.app.current_timer_session_id;

    // 完成计时会话，获取用于统计的 elapsed
    const elapsed = h.app.finishTimerSession() catch {
        // 如果失败，回退到旧行为
        h.app.clock_manager.handleEvent(.user_finish_timer);
        const clock_state = h.app.clock_manager.update();
        const elapsed_seconds = clock_state.getElapsedSeconds();
        triggerSSEPush(h);
        try response.json(.{ .status = "finished", .elapsed_seconds = elapsed_seconds }, .{});
        return;
    };

    // 创建 session 记录（用于统计）
    if (habit_id != null and elapsed > 0) {
        // 获取今天的日期字符串
        const timestamp: i64 = @intCast(std.time.timestamp());
        const es = std.time.epoch.EpochSeconds{ .secs = @as(u64, @intCast(timestamp)) };
        const yd = es.getEpochDay().calculateYearDay();
        const md = yd.calculateMonthDay();
        var buffer: [10]u8 = undefined;
        const today = std.fmt.bufPrint(&buffer, "{d:0>4}-{d:0>2}-{d:0>2}", .{ yd.year, md.month.numeric(), md.day_index + 1 }) catch "";

        _ = h.app.settings_manager.sqlite_db.?.habit_manager.createSession(
            habit_id.?,
            elapsed,
            1,
            today,
        ) catch |err| {
            logger.global_logger.err("创建日统计记录失败: {any}", .{err});
        };
    }

    // 清理 session
    h.app.resetTimerSession();

    triggerSSEPush(h);
    try response.json(.{ .status = "finished", .elapsed_seconds = elapsed, .session_id = session_id }, .{});
}

fn handleGetProgress(h: *HttpHandler, request: *httpz.Request, response: *httpz.Response) !void {
    _ = request;

    // 仅在进程内尚未持有会话状态时才从数据库恢复，避免重复恢复覆盖当前运行态。
    if (h.app.current_timer_session_id == null) {
        h.app.loadTimerProgress();
    }

    const session_id = h.app.current_timer_session_id;
    const habit_id = h.app.current_habit_id;
    const clock_state = h.app.clock_manager.update();
    const mode_key = switch (clock_state.getMode()) {
        .COUNTDOWN_MODE => "countdown",
        .STOPWATCH_MODE => "stopwatch",
    };

    try response.json(.{
        .session_id = session_id,
        .habit_id = habit_id,
        .mode = mode_key,
        .is_running = !clock_state.isPaused(),
        .is_paused = clock_state.isPaused(),
        .is_finished = clock_state.isFinished(),
        .elapsed_seconds = clock_state.getElapsedSeconds(),
        .remaining_seconds = clock_state.getRemainingSeconds(),
        .in_rest = clock_state.inRest(),
    }, .{});
}

fn handleModeChange(h: *HttpHandler, request: *httpz.Request, response: *httpz.Response) !void {
    logger.global_logger.info("[API] POST /api/mode", .{});
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
    logger.global_logger.debug("[API] GET /api/settings", .{});
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
    const habit_manager = &h.app.settings_manager.sqlite_db.?.*.habit_manager;
    const habit_sets = habit_manager.getAllHabitSets() catch |err| {
        logger.global_logger.err("获取习惯集失败: {any}", .{err});
        try response.json(.{ .err = "Failed to get habit sets" }, .{});
        return;
    };
    defer habit_manager.freeHabitSets(habit_sets);
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
        try response.json(.{ .err = "Missing name" }, .{});
        return;
    }

    if (description_str.len == 0) description_str = "";
    if (color_str.len == 0) color_str = "#6366f1";
    if (wallpaper.len == 0) wallpaper = "";

    h.app.settings_manager.sqlite_db.?.*.habit_manager.updateHabitSet(id, name, description_str, color_str, wallpaper) catch |err| {
        logger.global_logger.err("更新习惯集失败: {any}", .{err});
        try response.json(.{ .err = "Failed to update habit set" }, .{});
        return;
    };

    try response.json(.{ .id = id, .name = name, .description = description_str, .color = color_str, .wallpaper = wallpaper }, .{});
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
    const habit_manager = &h.app.settings_manager.sqlite_db.?.*.habit_manager;
    const habits = habit_manager.getAllHabits() catch {
        try response.json(.{ .err = "Failed to get habits" }, .{});
        return;
    };
    defer habit_manager.freeHabits(habits);
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
        try response.json(.{ .err = "Missing name" }, .{});
        return;
    }

    if (goal_seconds == 0) goal_seconds = 1500;
    if (color_str.len == 0) color_str = "#6366f1";
    if (wallpaper.len == 0) wallpaper = "";

    h.app.settings_manager.sqlite_db.?.*.habit_manager.updateHabit(id, name, goal_seconds, color_str, wallpaper) catch |err| {
        logger.global_logger.err("更新习惯失败: {any}", .{err});
        try response.json(.{ .err = "Failed to update habit" }, .{});
        return;
    };

    try response.json(.{ .id = id, .name = name, .goal_seconds = goal_seconds, .color = color_str, .wallpaper = wallpaper }, .{});
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
    const query = try request.query();
    const date = query.get("date");
    const start_date = query.get("start_date");
    const end_date = query.get("end_date");

    std.debug.print("[handleGetSessions] date={s}, start_date={s}, end_date={s}\n", .{ date orelse "null", start_date orelse "null", end_date orelse "null" });

    var sessions: []habit_crud.SessionRow = &.{};

    if (start_date != null and end_date != null) {
        std.debug.print("[handleGetSessions] calling getSessionsByDateRange\n", .{});
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

    std.debug.print("[handleGetSessions] returning {} sessions\n", .{sessions.len});
    for (sessions) |s| {
        std.debug.print("  session: id={}, habit_id={}, duration={}, date={s}\n", .{ s.id, s.habit_id, s.duration_seconds, s.date });
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

fn handleGetHabitDetail(h: *HttpHandler, request: *httpz.Request, response: *httpz.Response) !void {
    const id_str = request.params.get("id") orelse {
        try response.json(.{ .err = "Missing id" }, .{});
        return;
    };
    const habit_id = std.fmt.parseInt(i64, id_str, 10) catch {
        try response.json(.{ .err = "Invalid id" }, .{});
        return;
    };

    const date_param = request.params.get("date") orelse "2026-03-31";

    const habit = h.app.settings_manager.sqlite_db.?.*.habit_manager.getHabitById(habit_id) catch {
        try response.json(.{ .err = "Failed to get habit" }, .{});
        return;
    };

    const h_row = habit orelse {
        try response.json(.{ .err = "Habit not found" }, .{});
        return;
    };

    const today_seconds = h.app.settings_manager.sqlite_db.?.*.habit_manager.getHabitTodaySeconds(habit_id, date_param) catch 0;
    const streak = h.app.settings_manager.sqlite_db.?.*.habit_manager.getHabitStreak(habit_id, h_row.goal_seconds) catch 0;

    const progress_percent: i64 = if (h_row.goal_seconds > 0) @divTrunc(today_seconds * 100, h_row.goal_seconds) else 0;

    try response.json(.{
        .id = h_row.id,
        .name = h_row.name,
        .goal_seconds = h_row.goal_seconds,
        .color = h_row.color,
        .today_seconds = today_seconds,
        .streak = streak,
        .progress_percent = progress_percent,
    }, .{});
}

pub const HttpServerManager = struct {
    server: httpz.Server(*HttpHandler),
    handler: *HttpHandler,
    allocator: std.mem.Allocator,

    pub fn init(allocator: std.mem.Allocator, port: u16, app: *MainApplication) !HttpServerManager {
        logger.global_logger.info("初始化 HTTP 服务器，端口: {}", .{port});

        const handler = try allocator.create(HttpHandler);
        errdefer allocator.destroy(handler);
        handler.* = HttpHandler{ .app = app, .allocator = allocator, .sse_pending = false };

        var server = try httpz.Server(*HttpHandler).init(allocator, .{
            .address = .localhost(port),
        }, handler);

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
        router.post("/api/log", handleFrontendLog, .{});

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
        router.get("/api/habits/:id/detail", handleGetHabitDetail, .{});

        // 计时器 API
        router.post("/api/timer/rest", handleStartRest, .{});
        router.post("/api/timer/finish", handleFinish, .{});
        router.get("/api/timer/progress", handleGetProgress, .{});

        logger.global_logger.info("HTTP 服务器路由注册完成", .{});

        return HttpServerManager{
            .server = server,
            .handler = handler,
            .allocator = allocator,
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
        self.allocator.destroy(self.handler);
    }
};
