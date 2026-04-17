const std = @import("std");
const builtin = @import("builtin");
const json = std.json;
const httpx = @import("httpx");
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
    server: httpx.Server,
    allocator: std.mem.Allocator,

    pub fn init(allocator: std.mem.Allocator, port: u16, app: *MainApplication) !HttpServerManager {
        global_app = app;
        global_allocator = allocator;

        logger.global_logger.info("初始化 HTTP 服务器，端口: {}", .{port});

        var server = httpx.Server.initWithConfig(allocator, .{
            .host = "127.0.0.1",
            .port = port,
        });
        errdefer server.deinit();

        try server.get("/", handleRoot);
        try server.get("/api/state", handleGetState);
        try server.post("/api/start", handleStart);
        try server.post("/api/pause", handlePause);
        try server.post("/api/reset", handleReset);
        try server.post("/api/mode", handleModeChange);
        try server.get("/api/settings", handleGetSettings);
        try server.post("/api/settings", handleUpdateSettings);
        try server.post("/api/log", handleFrontendLog);

        try server.get("/api/habit-sets", handleGetHabitSets);
        try server.post("/api/habit-sets", handleCreateHabitSet);
        try server.put("/api/habit-sets/:id", handleUpdateHabitSet);
        try server.delete("/api/habit-sets/:id", handleDeleteHabitSet);

        try server.get("/api/habits", handleGetHabits);
        try server.post("/api/habits", handleCreateHabit);
        try server.put("/api/habits/:id", handleUpdateHabit);
        try server.delete("/api/habits/:id", handleDeleteHabit);

        try server.post("/api/sessions", handleCreateSession);
        try server.get("/api/sessions", handleGetSessions);
        try server.get("/api/habits/:id/streak", handleGetHabitStreak);
        try server.get("/api/habits/:id/detail", handleGetHabitDetail);

        try server.post("/api/timer/rest", handleStartRest);
        try server.post("/api/timer/finish", handleFinish);
        try server.get("/api/timer/progress", handleGetProgress);

        logger.global_logger.info("HTTP 服务器路由注册完成", .{});

        return HttpServerManager{
            .server = server,
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
        global_app = null;
        global_allocator = null;
        self.server.deinit();
    }
};

fn getApp() *MainApplication {
    return global_app orelse @panic("global_app not set");
}

fn getAllocator() std.mem.Allocator {
    return global_allocator orelse @panic("global_allocator not set");
}

fn handleRoot(ctx: *httpx.Context) !httpx.Response {
    if (build_options.embed_ui) {
        return ctx.status(200).html(build_options.embedded_html);
    } else {
        return ctx.status(200).html("<html><body><h1>Little Timer</h1><p>请先构建前端: cd assets && bun run build</p></body></html>");
    }
}

fn handleFrontendLog(ctx: *httpx.Context) !httpx.Response {
    const body = ctx.request.body orelse "";
    if (body.len == 0) {
        return ctx.json(.{ .success = false, .err = "empty body" });
    }

    const allocator = getAllocator();
    const parsed = std.json.parseFromSlice(std.json.Value, allocator, body, .{}) catch null;
    defer if (parsed) |p| p.deinit();

    if (parsed == null) {
        return ctx.json(.{ .success = false, .err = "invalid json" });
    }

    const value = parsed.?.value;
    if (value != .object) {
        return ctx.json(.{ .success = false, .err = "not an object" });
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

    return ctx.json(.{ .success = true });
}

fn handleGetState(ctx: *httpx.Context) !httpx.Response {
    logger.global_logger.debug("[API] GET /api/state", .{});

    const app = getApp();
    const display_data = app.clock_manager.update();
    const mode_key = switch (display_data.getMode()) {
        .COUNTDOWN_MODE => "countdown",
        .STOPWATCH_MODE => "stopwatch",
    };

    const timezone: i8 = app.settings_manager.config.basic.timezone;

    return ctx.json(.{
        .time = display_data.getTimeInfo(),
        .mode = mode_key,
        .is_running = !display_data.isPaused(),
        .is_finished = display_data.isFinished(),
        .in_rest = display_data.inRest(),
        .loop_remaining = display_data.getLoopRemaining(),
        .loop_total = display_data.getLoopTotal(),
        .rest_remaining = display_data.getRestRemainingTime(),
        .timezone = timezone,
    });
}

fn handleStart(ctx: *httpx.Context) !httpx.Response {
    logger.global_logger.info("[API] POST /api/start", .{});
    const body = ctx.request.body orelse "";
    const allocator = getAllocator();
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
        return ctx.json(.{ .err = "Database not open" });
    };

    if (app.current_timer_session_id) |session_id| {
        const session = db.habit_manager.getTimerSessionById(session_id) catch null;
        if (session) |s| {
            defer allocator.free(s.mode);

            if (s.is_running and !s.is_finished and !s.is_paused) {
                app.current_habit_id = habit_id orelse s.habit_id;
                return ctx.json(.{ .status = "already_running", .habit_id = app.current_habit_id, .session_id = session_id });
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
                return ctx.json(.{ .status = "started", .habit_id = app.current_habit_id, .session_id = session_id });
            }
        }

        app.resetTimerSession();
    }

    const session_id = app.createTimerSession(habit_id, mode, work_duration, rest_duration, loop_count) catch {
        app.current_habit_id = habit_id;
        app.clock_manager.handleEvent(.user_start_timer);
        return ctx.json(.{ .status = "started", .habit_id = habit_id });
    };

    app.current_habit_id = habit_id;
    app.clock_manager.handleEvent(.user_start_timer);
    return ctx.json(.{ .status = "started", .habit_id = habit_id, .session_id = session_id });
}

fn handleStartRest(ctx: *httpx.Context) !httpx.Response {
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
    return ctx.json(.{ .status = "rest_started", .rest_seconds = rest_seconds });
}

fn handlePause(ctx: *httpx.Context) !httpx.Response {
    logger.global_logger.info("[API] POST /api/pause", .{});
    const app = getApp();
    app.clock_manager.handleEvent(.user_pause_timer);
    app.saveTimerProgress();
    return ctx.json(.{ .status = "paused" });
}

fn handleReset(ctx: *httpx.Context) !httpx.Response {
    logger.global_logger.info("[API] POST /api/reset", .{});
    const app = getApp();
    app.resetTimerSession();
    app.current_habit_id = null;
    app.clock_manager.handleEvent(.user_reset_timer);
    return ctx.json(.{ .status = "reset" });
}

fn handleFinish(ctx: *httpx.Context) !httpx.Response {
    logger.global_logger.info("[API] POST /api/finish", .{});
    const app = getApp();

    const habit_id = app.current_habit_id;
    const session_id = app.current_timer_session_id;

    const elapsed = app.finishTimerSession() catch {
        app.clock_manager.handleEvent(.user_finish_timer);
        const clock_state = app.clock_manager.update();
        const elapsed_seconds = clock_state.getElapsedSeconds();
        return ctx.json(.{ .status = "finished", .elapsed_seconds = elapsed_seconds });
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

    return ctx.json(.{ .status = "finished", .elapsed_seconds = elapsed, .session_id = session_id });
}

fn handleGetProgress(ctx: *httpx.Context) !httpx.Response {
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

    return ctx.json(.{
        .session_id = session_id,
        .habit_id = habit_id,
        .mode = mode_key,
        .is_running = !clock_state.isPaused(),
        .is_paused = clock_state.isPaused(),
        .is_finished = clock_state.isFinished(),
        .elapsed_seconds = clock_state.getElapsedSeconds(),
        .remaining_seconds = clock_state.getRemainingSeconds(),
        .in_rest = clock_state.inRest(),
    });
}

fn handleModeChange(ctx: *httpx.Context) !httpx.Response {
    logger.global_logger.info("[API] POST /api/mode", .{});
    const body = ctx.request.body orelse "";
    const mode_str = std.mem.trim(u8, body, " \n\r\t");
    const app = getApp();
    const allocator = getAllocator();

    const new_mode = if (std.mem.eql(u8, mode_str, "countdown"))
        ModeEnumT.COUNTDOWN_MODE
    else if (std.mem.eql(u8, mode_str, "stopwatch"))
        ModeEnumT.STOPWATCH_MODE
    else {
        return ctx.json(std.json.Value{ .object = std.StringArrayHashMap(std.json.Value).init(allocator) });
    };

    app.clock_manager.handleEvent(.{ .user_change_mode = new_mode });
    return ctx.json(.{ .status = "mode_changed", .new_mode = mode_str });
}

fn handleGetSettings(ctx: *httpx.Context) !httpx.Response {
    logger.global_logger.debug("[API] GET /api/settings", .{});
    const app = getApp();
    const config = app.settings_manager.getConfig();
    return ctx.json(config);
}

fn handleUpdateSettings(ctx: *httpx.Context) !httpx.Response {
    const body = ctx.request.body orelse "";
    const app = getApp();
    const allocator = getAllocator();

    const body_copy: [:0]u8 = try allocator.allocSentinel(u8, body.len, 0);
    @memcpy(body_copy[0..body.len], body);
    try app.settings_manager.handleSettingsEvent(.{ .change_settings = body_copy });
    return ctx.json(.{ .status = "settings_updated" });
}

fn handleGetHabitSets(ctx: *httpx.Context) !httpx.Response {
    const app = getApp();
    const habit_manager = &app.settings_manager.sqlite_db.?.*.habit_manager;
    const habit_sets = habit_manager.getAllHabitSets() catch |err| {
        logger.global_logger.err("获取习惯集失败: {any}", .{err});
        return ctx.json(.{ .err = "Failed to get habit sets" });
    };
    defer habit_manager.freeHabitSets(habit_sets);
    return ctx.json(habit_sets);
}

fn handleCreateHabitSet(ctx: *httpx.Context) !httpx.Response {
    const body = ctx.request.body orelse "";
    const allocator = getAllocator();
    const app = getApp();

    var parsed = std.json.parseFromSlice(std.json.Value, allocator, body, .{}) catch |err| {
        logger.global_logger.err("解析习惯集创建请求失败: {any}", .{err});
        return ctx.json(.{ .err = "Invalid JSON" });
    };
    defer parsed.deinit();

    const root = parsed.value.object;
    const name_val = root.get("name") orelse {
        return ctx.json(.{ .err = "Missing name" });
    };

    if (name_val != .string or name_val.string.len == 0) {
        return ctx.json(.{ .err = "Invalid name" });
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
        return ctx.json(.{ .err = "Failed to create habit set" });
    };

    return ctx.json(.{ .id = id, .name = name_val.string, .description = description_str, .color = color_str });
}

fn handleUpdateHabitSet(ctx: *httpx.Context) !httpx.Response {
    const id_str = ctx.param("id") orelse {
        return ctx.json(.{ .err = "Missing id" });
    };
    const id = std.fmt.parseInt(i64, id_str, 10) catch {
        return ctx.json(.{ .err = "Invalid id" });
    };

    const body = ctx.request.body orelse "";
    const allocator = getAllocator();
    const app = getApp();

    var parsed = std.json.parseFromSlice(std.json.Value, allocator, body, .{}) catch |err| {
        logger.global_logger.err("解析习惯集更新请求失败: {any}", .{err});
        return ctx.json(.{ .err = "Invalid JSON" });
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
        return ctx.json(.{ .err = "Missing name" });
    }

    if (description_str.len == 0) description_str = "";
    if (color_str.len == 0) color_str = "#6366f1";
    if (wallpaper.len == 0) wallpaper = "";

    app.settings_manager.sqlite_db.?.*.habit_manager.updateHabitSet(id, name, description_str, color_str, wallpaper) catch |err| {
        logger.global_logger.err("更新习惯集失败: {any}", .{err});
        return ctx.json(.{ .err = "Failed to update habit set" });
    };

    return ctx.json(.{ .id = id, .name = name, .description = description_str, .color = color_str, .wallpaper = wallpaper });
}

fn handleDeleteHabitSet(ctx: *httpx.Context) !httpx.Response {
    const id_str = ctx.param("id") orelse {
        return ctx.json(.{ .err = "Missing id" });
    };
    const id = std.fmt.parseInt(i64, id_str, 10) catch {
        return ctx.json(.{ .err = "Invalid id" });
    };

    const app = getApp();
    app.settings_manager.sqlite_db.?.*.habit_manager.deleteHabitSet(id) catch |err| {
        logger.global_logger.err("删除习惯集失败: {any}", .{err});
        return ctx.json(.{ .err = "Failed to delete habit set" });
    };

    return ctx.json(.{ .success = true });
}

fn handleGetHabits(ctx: *httpx.Context) !httpx.Response {
    const app = getApp();
    const habit_manager = &app.settings_manager.sqlite_db.?.*.habit_manager;
    const habits = habit_manager.getAllHabits() catch {
        return ctx.json(.{ .err = "Failed to get habits" });
    };
    defer habit_manager.freeHabits(habits);
    return ctx.json(habits);
}

fn handleCreateHabit(ctx: *httpx.Context) !httpx.Response {
    const body = ctx.request.body orelse "";
    const allocator = getAllocator();
    const app = getApp();

    var parsed = std.json.parseFromSlice(std.json.Value, allocator, body, .{}) catch |err| {
        logger.global_logger.err("解析习惯创建请求失败: {any}", .{err});
        return ctx.json(.{ .err = "Invalid JSON" });
    };
    defer parsed.deinit();

    const root = parsed.value.object;
    const set_id_val = root.get("set_id") orelse {
        return ctx.json(.{ .err = "Missing set_id" });
    };
    const name_val = root.get("name") orelse {
        return ctx.json(.{ .err = "Missing name" });
    };

    if (set_id_val != .integer or name_val != .string or name_val.string.len == 0) {
        return ctx.json(.{ .err = "Invalid parameters" });
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
        return ctx.json(.{ .err = "Failed to create habit" });
    };

    return ctx.json(.{ .id = id, .set_id = set_id, .name = name, .goal_seconds = goal_seconds, .color = color_str });
}

fn handleDeleteHabit(ctx: *httpx.Context) !httpx.Response {
    const id_str = ctx.param("id") orelse {
        return ctx.json(.{ .err = "Missing id" });
    };
    const id = std.fmt.parseInt(i64, id_str, 10) catch {
        return ctx.json(.{ .err = "Invalid id" });
    };

    const app = getApp();
    app.settings_manager.sqlite_db.?.*.habit_manager.deleteHabit(id) catch |err| {
        logger.global_logger.err("删除习惯失败: {any}", .{err});
        return ctx.json(.{ .err = "Failed to delete habit" });
    };

    return ctx.json(.{ .success = true });
}

fn handleUpdateHabit(ctx: *httpx.Context) !httpx.Response {
    const id_str = ctx.param("id") orelse {
        return ctx.json(.{ .err = "Missing id" });
    };
    const id = std.fmt.parseInt(i64, id_str, 10) catch {
        return ctx.json(.{ .err = "Invalid id" });
    };

    const body = ctx.request.body orelse "";
    const allocator = getAllocator();
    const app = getApp();

    var parsed = std.json.parseFromSlice(std.json.Value, allocator, body, .{}) catch |err| {
        logger.global_logger.err("解析习惯更新请求失败: {any}", .{err});
        return ctx.json(.{ .err = "Invalid JSON" });
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
        return ctx.json(.{ .err = "Missing name" });
    }

    if (goal_seconds == 0) goal_seconds = 1500;
    if (color_str.len == 0) color_str = "#6366f1";
    if (wallpaper.len == 0) wallpaper = "";

    app.settings_manager.sqlite_db.?.*.habit_manager.updateHabit(id, name, goal_seconds, color_str, wallpaper) catch |err| {
        logger.global_logger.err("更新习惯失败: {any}", .{err});
        return ctx.json(.{ .err = "Failed to update habit" });
    };

    return ctx.json(.{ .id = id, .name = name, .goal_seconds = goal_seconds, .color = color_str, .wallpaper = wallpaper });
}

fn handleCreateSession(ctx: *httpx.Context) !httpx.Response {
    const body = ctx.request.body orelse "";
    const allocator = getAllocator();
    const app = getApp();

    var parsed = std.json.parseFromSlice(std.json.Value, allocator, body, .{}) catch |err| {
        logger.global_logger.err("解析记录创建请求失败: {any}", .{err});
        return ctx.json(.{ .err = "Invalid JSON" });
    };
    defer parsed.deinit();

    const root = parsed.value.object;

    const habit_id_val = root.get("habit_id") orelse {
        return ctx.json(.{ .err = "Missing habit_id" });
    };
    const duration_val = root.get("duration_seconds") orelse {
        return ctx.json(.{ .err = "Missing duration_seconds" });
    };

    if (habit_id_val != .integer or duration_val != .integer) {
        return ctx.json(.{ .err = "Invalid parameters" });
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
        return ctx.json(.{ .err = "Failed to create session" });
    };

    return ctx.json(.{ .id = id, .habit_id = habit_id, .duration_seconds = duration_seconds, .date = date_str });
}

fn handleGetSessions(ctx: *httpx.Context) !httpx.Response {
    const date = ctx.query("date");
    const start_date = ctx.query("start_date");
    const end_date = ctx.query("end_date");

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
            return ctx.json(.{ .err = "Failed to get sessions" });
        };
    } else if (date != null) {
        sessions = app.settings_manager.sqlite_db.?.*.habit_manager.getSessionsByDate(date.?) catch {
            return ctx.json(.{ .err = "Failed to get sessions" });
        };
    } else {
        const timestamp: i64 = @intCast(std.time.timestamp());
        const es = std.time.epoch.EpochSeconds{ .secs = @as(u64, @intCast(timestamp)) };
        const yd = es.getEpochDay().calculateYearDay();
        const md = yd.calculateMonthDay();
        var buffer: [10]u8 = undefined;
        const today = std.fmt.bufPrint(&buffer, "{d:0>4}-{d:0>2}-{d:0>2}", .{ yd.year, md.month.numeric(), md.day_index + 1 }) catch "";
        sessions = app.settings_manager.sqlite_db.?.*.habit_manager.getSessionsByDate(today) catch {
            return ctx.json(.{ .err = "Failed to get sessions" });
        };
    }

    owns_sessions = true;

    std.debug.print("[handleGetSessions] returning {} sessions\n", .{sessions.len});

    return ctx.json(sessions);
}

fn handleGetHabitStreak(ctx: *httpx.Context) !httpx.Response {
    const id_str = ctx.param("id") orelse {
        return ctx.json(.{ .err = "Missing id" });
    };
    const habit_id = std.fmt.parseInt(i64, id_str, 10) catch {
        return ctx.json(.{ .err = "Invalid id" });
    };

    const goal_seconds: i64 = if (ctx.param("goal_seconds")) |gs|
        std.fmt.parseInt(i64, gs, 10) catch 1500
    else
        1500;

    const app = getApp();
    const streak = app.settings_manager.sqlite_db.?.*.habit_manager.getHabitStreak(habit_id, goal_seconds) catch {
        return ctx.json(.{ .err = "Failed to get streak" });
    };

    return ctx.json(.{ .habit_id = habit_id, .streak = streak });
}

fn handleGetHabitDetail(ctx: *httpx.Context) !httpx.Response {
    const id_str = ctx.param("id") orelse {
        return ctx.json(.{ .err = "Missing id" });
    };
    const habit_id = std.fmt.parseInt(i64, id_str, 10) catch {
        return ctx.json(.{ .err = "Invalid id" });
    };

    const date_param = ctx.param("date") orelse "2026-03-31";

    const app = getApp();
    const habit_manager = &app.settings_manager.sqlite_db.?.*.habit_manager;

    const habit = habit_manager.getHabitById(habit_id) catch {
        return ctx.json(.{ .err = "Failed to get habit" });
    };

    const h_row = habit orelse {
        return ctx.json(.{ .err = "Habit not found" });
    };
    defer habit_manager.freeHabit(h_row);

    const today_seconds = habit_manager.getHabitTodaySeconds(habit_id, date_param) catch 0;
    const streak = habit_manager.getHabitStreak(habit_id, h_row.goal_seconds) catch 0;

    const progress_percent: i64 = if (h_row.goal_seconds > 0) @divTrunc(today_seconds * 100, h_row.goal_seconds) else 0;

    return ctx.json(.{
        .id = h_row.id,
        .name = h_row.name,
        .goal_seconds = h_row.goal_seconds,
        .color = h_row.color,
        .today_seconds = today_seconds,
        .streak = streak,
        .progress_percent = progress_percent,
    });
}
