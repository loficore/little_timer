const std = @import("std");
const json = std.json;
const http = std.http;
const logger = @import("../logger.zig");
const clock = @import("../clock.zig");
const settings = @import("../../settings/mod.zig");
const habit_crud = @import("../../storage/habit_crud.zig");
const MainApplication = @import("../app.zig").MainApplication;
const build_options = @import("build_options");
const backup_mod = @import("../../storage/backup/BackupAdapter.zig");
const crypto = @import("../utils/crypto.zig");

fn logWebdavErr(msg: []const u8) void {
    logger.global_logger.err("[WebDAV] {s}", .{msg});
}

fn createMasterPasswordError(allocator: std.mem.Allocator, error_code: []const u8, message: []const u8, action_mode: []const u8) ![]u8 {
    _ = error_code;
    _ = message;
    if (action_mode.len > 0) {
        return allocator.dupe(u8, "{\"success\":false,\"error\":\"error\",\"message\":\"msg\",\"action\":{\"type\":\"show_modal\",\"target\":\"master_password\",\"params\":{\"mode\":\"unlock\"}}}");
    }
    return allocator.dupe(u8, "{\"success\":false,\"error\":\"error\",\"message\":\"msg\",\"action\":{\"type\":\"show_modal\",\"target\":\"master_password\"}}");
}

const ClockState = clock.ClockState;
const ModeEnumT = clock.ModeEnumT;

pub const HttpError = error{
    RequestBodyTooLarge,
};

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
            self.allocator,
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

fn serverLoop(listener: *std.net.Server, running: *std.atomic.Value(bool), allocator: std.mem.Allocator) void {
    while (running.load(.acquire)) {
        const stream = listener.accept() catch {
            continue;
        };
        const thread = std.Thread.spawn(.{}, handleConnection, .{
            stream,
            running,
            allocator,
        }) catch {
            stream.stream.close();
            continue;
        };
        thread.detach();
    }
}

fn handleConnection(conn: std.net.Server.Connection, running: *std.atomic.Value(bool), allocator: std.mem.Allocator) void {
    defer conn.stream.close();

    const read_buffer = allocator.alloc(u8, 8192) catch return;
    defer allocator.free(read_buffer);
    const write_buffer = allocator.alloc(u8, 8192) catch return;
    defer allocator.free(write_buffer);

    var reader = conn.stream.reader(read_buffer);
    var writer = conn.stream.writer(write_buffer);
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

const MAX_REQUEST_BODY_SIZE = 10 * 1024 * 1024; // 10MB limit

/// 读取 HTTP 请求体
fn readRequestBody(request: *http.Server.Request, allocator: std.mem.Allocator) ![]u8 {
    const content_length_u64 = request.head.content_length orelse 0;
    const content_length: usize = @intCast(content_length_u64);

    if (content_length > MAX_REQUEST_BODY_SIZE) {
        logger.global_logger.err("请求体过大: {d} bytes", .{content_length});
        return error.RequestBodyTooLarge;
    }

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

    if (request.head.content_length == null and request.head.transfer_encoding == .none) {
        if (request.head.method.requestHasBody()) {
            request.head.content_length = 0;
        }
    }
    const path = if (std.mem.indexOfScalar(u8, target, '?')) |query_idx|
        target[0..query_idx]
    else
        target;

    // 公开端点（无需认证）
    const public_paths = [_][]const u8{ "/", "/api/log", "/api/events" };
    const is_public = for (public_paths) |p| {
        if (std.mem.eql(u8, path, p)) break true;
    } else false;

    // API 路由需要认证（除非是公开端点）
    if (!is_public and std.mem.startsWith(u8, path, "/api/")) {
        if (!validateAuth(request)) return;
    }

    // ============================================
    // 路由分发 - 按功能分组
    // ============================================

    // 静态资源
    if (request.head.method == .GET and std.mem.eql(u8, path, "/")) {
        try handleRoot(request);
    // 前端日志
    } else if (request.head.method == .POST and std.mem.eql(u8, path, "/api/log")) {
        try handleFrontendLog(request);
    // SSE 事件流
    } else if (request.head.method == .GET and std.mem.eql(u8, path, "/api/events")) {
        try handleSSE(request);

    // ============================================
    // Timer 相关路由
    // ============================================
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
    } else if (request.head.method == .POST and std.mem.eql(u8, path, "/api/timer/rest")) {
        try handleStartRest(request);
    } else if (request.head.method == .POST and std.mem.eql(u8, path, "/api/timer/finish")) {

        try handleFinish(request);
    } else if (request.head.method == .GET and std.mem.eql(u8, path, "/api/timer/progress")) {
        try handleGetProgress(request);

    // ============================================
    // Habit Sets 路由
    // ============================================
    } else if (request.head.method == .GET and std.mem.eql(u8, path, "/api/habit-sets")) {
        try handleGetHabitSets(request);
    } else if (request.head.method == .POST and std.mem.eql(u8, path, "/api/habit-sets")) {
        try handleCreateHabitSet(request);
    } else if (request.head.method == .PUT and std.mem.startsWith(u8, path, "/api/habit-sets/")) {
        try handleUpdateHabitSet(request);
    } else if (request.head.method == .DELETE and std.mem.startsWith(u8, path, "/api/habit-sets/")) {
        try handleDeleteHabitSet(request);

    // ============================================
    // Habits 路由
    // ============================================
    } else if (request.head.method == .GET and std.mem.eql(u8, path, "/api/habits")) {
        try handleGetHabits(request);
    } else if (request.head.method == .POST and std.mem.eql(u8, path, "/api/habits")) {
        try handleCreateHabit(request);
    } else if (request.head.method == .PUT and std.mem.startsWith(u8, path, "/api/habits/")) {
        try handleUpdateHabit(request);
    } else if (request.head.method == .DELETE and std.mem.startsWith(u8, path, "/api/habits/")) {
        try handleDeleteHabit(request);
    } else if (request.head.method == .GET and std.mem.startsWith(u8, path, "/api/habits/") and std.mem.endsWith(u8, path, "/detail")) {
        try handleGetHabitDetail(request);

    // ============================================
    // Sessions 路由
    // ============================================
    } else if (request.head.method == .GET and std.mem.eql(u8, path, "/api/sessions")) {
        try handleGetSessions(request);
    } else if (request.head.method == .POST and std.mem.eql(u8, path, "/api/sessions")) {
        try handleCreateSession(request);

    // ============================================
    // Settings 路由
    // ============================================
    } else if (request.head.method == .GET and std.mem.eql(u8, path, "/api/settings")) {
        try handleGetSettings(request);
    } else if (request.head.method == .POST and std.mem.eql(u8, path, "/api/settings")) {
        try handleUpdateSettings(request);

    // ============================================
    // Backup 路由
    // ============================================
    } else if (request.head.method == .GET and std.mem.eql(u8, path, "/api/backup/config")) {
        try handleGetBackupConfig(request);
    } else if (request.head.method == .POST and std.mem.eql(u8, path, "/api/backup/config")) {
        try handleUpdateBackupConfig(request);
    } else if (request.head.method == .POST and std.mem.eql(u8, path, "/api/backup/create")) {
        try handleBackupCreate(request);
    } else if (request.head.method == .POST and std.mem.eql(u8, path, "/api/backup/restore")) {
        try handleBackupRestore(request);
    } else if (request.head.method == .GET and std.mem.eql(u8, path, "/api/backup/list")) {
        try handleBackupList(request);
    } else if (request.head.method == .DELETE and std.mem.startsWith(u8, path, "/api/backup/")) {
        try handleBackupDelete(request);
    } else if (request.head.method == .POST and std.mem.eql(u8, path, "/api/backup/verify")) {
        try handleBackupVerify(request);
    } else if (request.head.method == .POST and std.mem.eql(u8, path, "/api/backup/unlock")) {
        try handleBackupUnlock(request);
    } else if (request.head.method == .POST and std.mem.eql(u8, path, "/api/backup/lock")) {
        try handleBackupLock(request);
    } else if (request.head.method == .GET and std.mem.eql(u8, path, "/api/backup/master-password")) {
        try handleGetMasterPasswordStatus(request);
    } else if (request.head.method == .POST and std.mem.eql(u8, path, "/api/backup/master-password")) {
        try handleSetMasterPassword(request);

    // ============================================
    // Auth 路由
    // ============================================
    } else if (request.head.method == .GET and std.mem.eql(u8, path, "/api/auth/status")) {
        try handleAuthStatus(request);
    } else if (request.head.method == .POST and std.mem.eql(u8, path, "/api/auth/enable")) {
        try handleAuthEnable(request);
    } else if (request.head.method == .POST and std.mem.eql(u8, path, "/api/auth/disable")) {
        try handleAuthDisable(request);

    // ============================================
    // Wallpapers 路由
    // ============================================
    } else if (request.head.method == .POST and std.mem.eql(u8, path, "/api/wallpapers")) {
        try handleUploadWallpaper(request);
    } else if (request.head.method == .GET and std.mem.eql(u8, path, "/api/wallpapers")) {
        try handleListWallpapers(request);
    } else if (request.head.method == .GET and std.mem.startsWith(u8, path, "/api/wallpapers/")) {
        try handleServeWallpaper(request);
    } else if (request.head.method == .DELETE and std.mem.startsWith(u8, path, "/api/wallpapers/")) {
        try handleDeleteWallpaper(request);

    // ============================================
    // 404 Fallback
    // ============================================
    } else {
        if (std.mem.indexOf(u8, path, "finish") != null) {

        }
        try request.respond("", .{ .status = .not_found });
    }
}

fn getApp() *MainApplication {
    return global_app orelse @panic("global_app not set");
}

fn getAllocator() std.mem.Allocator {
    return global_allocator orelse @panic("global_allocator not set");
}

/// 验证请求认证
/// 如果 auth_enabled 为 true，则检查 Authorization 头或 URL query 参数
/// 返回 true 表示认证通过，false 表示认证失败（已发送 401 响应）
///
/// 优先级:
/// 1. Authorization: Bearer <token> header (推荐)
/// 2. URL query 参数 auth_token=xxx (向后兼容)
fn validateAuth(request: *http.Server.Request) bool {
    const app = getApp();
    const auth_config = app.settings_manager.getConfig().auth;

    if (!auth_config.auth_enabled) {
        return true;
    }

    const auth_token = auth_config.auth_token;
    if (auth_token.len == 0) {
        return true;
    }

    // 优先检查 Authorization header
    var it = request.iterateHeaders();
    while (it.next()) |header| {
        if (std.mem.eql(u8, header.name, "Authorization")) {
            const bearer_prefix = "Bearer ";
            if (std.mem.startsWith(u8, header.value, bearer_prefix)) {
                const provided_token = header.value[bearer_prefix.len..];
                if (std.mem.eql(u8, provided_token, auth_token)) {
                    return true;
                }
            }
        }
    }

    // 向后兼容：检查 URL query 参数
    const target = request.head.target;
    const auth_param = "auth_token=";
    if (std.mem.indexOf(u8, target, auth_param)) |idx| {
        const token_start = idx + auth_param.len;
        const token_end = std.mem.indexOfScalar(u8, target[token_start..], '&') orelse target.len;
        const provided_token = target[token_start..token_start + token_end];
        if (std.mem.eql(u8, provided_token, auth_token)) {
            return true;
        }
    }

    sendJsonError(request, "Unauthorized: Invalid or missing token", .unauthorized);
    return false;
}

/// 发送 JSON 错误响应
fn sendJsonError(request: *http.Server.Request, message: []const u8, status: http.Status) void {
    const json_err = std.fmt.allocPrint(getAllocator(), "{{\"err\":\"{s}\"}}", .{message}) catch {
        _ = request.respond("", .{ .status = status }) catch {};
        return;
    };
    defer getAllocator().free(json_err);
    _ = request.respond(json_err, .{
        .status = status,
        .extra_headers = &.{
            .{ .name = "Content-Type", .value = "application/json" },
        },
    }) catch {};
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
        const html = "<html><body><h1>Little Timer</h1><p>请先构建前端: cd assets && pnpm run build</p></body></html>";
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
    app.mutex.lock();
    defer app.mutex.unlock();

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

    const session_id = app.createTimerSession(habit_id, mode, work_duration, rest_duration, loop_count) catch |err| {
        logger.global_logger.err("创建计时会话失败: {any}", .{err});
        try request.respond("{\"err\":\"Failed to create timer session\"}", .{
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
    app.mutex.lock();
    defer app.mutex.unlock();
    app.clock_manager.handleEvent(.user_pause_timer);
    app.saveTimerProgress();

    try request.respond("{\"status\":\"paused\"}", .{
        .extra_headers = &.{.{ .name = "Content-Type", .value = "application/json" }},
    });
}

fn handleReset(request: *http.Server.Request) !void {
    logger.global_logger.info("[API] POST /api/reset", .{});
    const app = getApp();
    app.mutex.lock();
    defer app.mutex.unlock();
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
    app.mutex.lock();
    defer app.mutex.unlock();

    const habit_id = app.current_habit_id;
    const session_id = app.current_timer_session_id;

    const elapsed = app.finishTimerSession() catch |err| {
        logger.global_logger.err("完成计时会话失败: {any}", .{err});
        app.clock_manager.handleEvent(.user_finish_timer);
        const clock_state = app.clock_manager.update();
        const elapsed_seconds = clock_state.getElapsedSeconds();

        if (habit_id != null and elapsed_seconds > 0) {
            const timestamp: i64 = @intCast(std.time.timestamp());
            const es = std.time.epoch.EpochSeconds{ .secs = @as(u64, @intCast(timestamp)) };
            const yd = es.getEpochDay().calculateYearDay();
            const md = yd.calculateMonthDay();
            var buffer: [10]u8 = undefined;
            const today = std.fmt.bufPrint(&buffer, "{d:0>4}-{d:0>2}-{d:0>2}", .{ yd.year, md.month.numeric(), md.day_index + 1 }) catch "";

            _ = app.settings_manager.sqlite_db.?.habit_manager.createSession(
                habit_id.?,
                elapsed_seconds,
                1,
                today,
            ) catch |e2| {
                logger.global_logger.err("错误恢复期间创建日统计记录也失败: {any}", .{e2});
            };
        }

        const resp = try std.fmt.allocPrint(getAllocator(), "{{\"status\":\"finished\",\"elapsed_seconds\":{}}}", .{elapsed_seconds});
        defer getAllocator().free(resp);
        try request.respond(resp, .{
            .extra_headers = &.{.{ .name = "Content-Type", .value = "application/json" }},
        });
        app.resetTimerSession();
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

fn handleGetBackupConfig(request: *http.Server.Request) !void {
    logger.global_logger.debug("[API] GET /api/backup/config", .{});
    const app = getApp();
    const backup_config = app.settings_manager.getBackupConfig();

    const target_type_str = switch (backup_config.target_type) {
        .local => "local",
        .webdav => "webdav",
        .s3 => "s3",
    };

    const json_resp = try std.fmt.allocPrint(getAllocator(),
        "{{\"enabled\":{},\"auto_backup\":{},\"auto_backup_interval\":{},\"target_type\":\"{s}\",\"local_path\":\"{s}\",\"webdav_url\":\"{s}\",\"webdav_username\":\"{s}\",\"webdav_password\":\"{s}\",\"s3_endpoint\":\"{s}\",\"s3_bucket\":\"{s}\",\"s3_region\":\"{s}\",\"s3_access_key\":\"{s}\",\"s3_secret_key\":\"{s}\",\"s3_path_prefix\":\"{s}\"}}",
        .{
            @intFromBool(backup_config.enabled),
            @intFromBool(backup_config.auto_backup),
            backup_config.auto_backup_interval,
            target_type_str,
            backup_config.local_path,
            backup_config.webdav_url,
            backup_config.webdav_username,
            if (backup_config.webdav_password.len > 0) "******" else "",
            backup_config.s3_endpoint,
            backup_config.s3_bucket,
            backup_config.s3_region,
            if (backup_config.s3_access_key.len > 0) "******" else "",
            if (backup_config.s3_secret_key.len > 0) "******" else "",
            backup_config.s3_path_prefix,
        }
    );
    defer getAllocator().free(json_resp);
    try request.respond(json_resp, .{
        .extra_headers = &.{.{ .name = "Content-Type", .value = "application/json" }},
    });
}

fn handleUpdateBackupConfig(request: *http.Server.Request) !void {
    logger.global_logger.info("[API] POST /api/backup/config", .{});
    const app = getApp();
    const allocator = getAllocator();
    const body = try readRequestBody(request, allocator);
    defer allocator.free(body);

    const body_json = std.json.parseFromSlice(json.Value, allocator, body, .{}) catch {
        try request.respond("{\"success\":false,\"error\":\"invalid json\"}", .{
            .extra_headers = &.{.{ .name = "Content-Type", .value = "application/json" }},
        });
        return;
    };
    defer body_json.deinit();

    if (body_json.value.object.get("target_type")) |target_type_val| {
        const target_type_str = target_type_val.string;
        const is_webdav = std.mem.eql(u8, target_type_str, "webdav");
        const is_s3 = std.mem.eql(u8, target_type_str, "s3");
        const is_cloud = is_webdav or is_s3;

        if (is_cloud) {
            const current_config = app.settings_manager.getBackupConfig();
            const current_has_creds = if (is_webdav)
                current_config.webdav_password.len > 0
            else
                current_config.s3_access_key.len > 0 and current_config.s3_secret_key.len > 0;

            if (!current_has_creds) {
                if (!app.settings_manager.hasMasterPassword()) {
                    const err_msg = try std.fmt.allocPrint(allocator, "{{\"success\":false,\"error\":\"master_password_required\",\"message\":\"请先设置主密码才能使用云端备份\",\"action\":{{\"type\":\"show_modal\",\"target\":\"master_password\",\"params\":{{\"mode\":\"setup\"}}}}}}", .{});
                    defer allocator.free(err_msg);
                    try request.respond(err_msg, .{
                        .extra_headers = &.{.{ .name = "Content-Type", .value = "application/json" }},
                    });
                    return;
                }

                if (!app.settings_manager.isUnlocked()) {
                    const err_msg = try std.fmt.allocPrint(allocator, "{{\"success\":false,\"error\":\"master_password_not_unlocked\",\"message\":\"凭证已过期，请重新解锁\",\"action\":{{\"type\":\"show_modal\",\"target\":\"master_password\",\"params\":{{\"mode\":\"unlock\"}}}}}}", .{});
                    defer allocator.free(err_msg);
                    try request.respond(err_msg, .{
                        .extra_headers = &.{.{ .name = "Content-Type", .value = "application/json" }},
                    });
                    return;
                }
            }
        }
    }

    const body_copy: [:0]u8 = try allocator.allocSentinel(u8, body.len, 0);
    @memcpy(body_copy[0..body.len], body);

    try app.settings_manager.updateBackupConfig(body_copy);
    allocator.free(body_copy);

    const resp = "{\"success\":true}";
    try request.respond(resp, .{
        .extra_headers = &.{.{ .name = "Content-Type", .value = "application/json" }},
    });
}

fn handleBackupCreate(request: *http.Server.Request) !void {
    logger.global_logger.info("[API] POST /api/backup/create", .{});
    const allocator = getAllocator();
    const app = getApp();
    const db = app.settings_manager.sqlite_db orelse {
        try request.respond("{\"success\":false,\"error\":\"database not open\"}", .{
            .extra_headers = &.{.{ .name = "Content-Type", .value = "application/json" }},
        });
        return;
    };

    const backup_config = app.settings_manager.getBackupConfig();
    if (!backup_config.enabled) {
        try request.respond("{\"success\":false,\"error\":\"backup not enabled\"}", .{
            .extra_headers = &.{.{ .name = "Content-Type", .value = "application/json" }},
        });
        return;
    }

    if (backup_config.target_type == .webdav or backup_config.target_type == .s3) {
        const credentials_available = switch (backup_config.target_type) {
            .webdav => backup_config.webdav_password.len > 0,
            .s3 => backup_config.s3_access_key.len > 0 and backup_config.s3_secret_key.len > 0,
            else => true,
        };

        if (!credentials_available) {
            const has_pwd = app.settings_manager.hasMasterPassword();
            const action_target = if (has_pwd) "unlock" else "setup";
            const err_msg = try createMasterPasswordError(allocator, "credentials_not_available", "凭证不可用，请先设置主密码", action_target);
            defer allocator.free(err_msg);
            try request.respond(err_msg, .{
                .extra_headers = &.{.{ .name = "Content-Type", .value = "application/json" }},
            });
            return;
        }

        if (!app.settings_manager.isUnlocked()) {
            const err_msg = try createMasterPasswordError(allocator, "master_password_not_unlocked", "凭证已过期，请重新解锁", "unlock");
            defer allocator.free(err_msg);
            try request.respond(err_msg, .{
                .extra_headers = &.{.{ .name = "Content-Type", .value = "application/json" }},
            });
            return;
        }
    }

    const timestamp = std.time.timestamp();
    var backup_buf: [512]u8 = undefined;
    const backup_name = try std.fmt.bufPrint(&backup_buf, "presets_backup_{}.db", .{timestamp});

    const adapter: backup_mod.BackupAdapter = switch (backup_config.target_type) {
        .local => blk: {
            const local_path = if (backup_config.local_path.len > 0) backup_config.local_path else "";
            break :blk backup_mod.createLocalAdapter(allocator, .{ .path = local_path });
        },
        .webdav => backup_mod.createWebDAVAdapter(allocator, .{
            .url = backup_config.webdav_url,
            .username = backup_config.webdav_username,
            .password = backup_config.webdav_password,
            .base_path = "/",
        }, &logWebdavErr),
        .s3 => backup_mod.createS3Adapter(allocator, .{
            .endpoint = backup_config.s3_endpoint,
            .bucket = backup_config.s3_bucket,
            .region = backup_config.s3_region,
            .access_key = backup_config.s3_access_key,
            .secret_key = backup_config.s3_secret_key,
            .path_prefix = if (backup_config.s3_path_prefix.len > 0) backup_config.s3_path_prefix else "little_timer/",
        }),
    };
    defer {
        adapter.vtable.freeList(adapter.ptr, &[_]backup_mod.BackupInfo{});
    }

    const db_file_path = db.db_path;

    logger.global_logger.info("[Backup] Starting backup: db_path={s}, backup_name={s}", .{ db_file_path, backup_name });

    db.backup_manager.closeDbForBackup() catch |err| {
        logger.global_logger.err("[Backup] Failed to close DB: {any}", .{err});
    };
    errdefer {
        db.backup_manager.reopenDb() catch |err| {
            logger.global_logger.err("[Backup] Failed to reopen DB: {any}", .{err});
        };
    }

    adapter.push(db_file_path, backup_name) catch |err| {
        logger.global_logger.err("[Backup] Push failed: {any}", .{err});
        db.backup_manager.reopenDb() catch {};
        const err_msg = std.fmt.allocPrint(allocator, "{{\"success\":false,\"error\":\"{s}\"}}", .{@errorName(err)}) catch unreachable;
        defer allocator.free(err_msg);
        try request.respond(err_msg, .{
            .extra_headers = &.{.{ .name = "Content-Type", .value = "application/json" }},
        });
        return;
    };

    db.backup_manager.reopenDb() catch |err| {
        logger.global_logger.err("[Backup] Failed to reopen DB: {any}", .{err});
    };

    logger.global_logger.info("[Backup] Backup created successfully: {s}", .{backup_name});
    const resp = try std.fmt.allocPrint(allocator, "{{\"success\":true,\"backup_path\":\"{s}\"}}", .{backup_name});
    defer allocator.free(resp);
    try request.respond(resp, .{
        .extra_headers = &.{.{ .name = "Content-Type", .value = "application/json" }},
    });
}

fn handleBackupRestore(request: *http.Server.Request) !void {
    logger.global_logger.info("[API] POST /api/backup/restore", .{});
    const app = getApp();
    const allocator = getAllocator();
    const body = try readRequestBody(request, allocator);
    defer allocator.free(body);

    const parsed = std.json.parseFromSlice(std.json.Value, allocator, body, .{}) catch null;
    defer if (parsed) |p| p.deinit();

    if (parsed == null) {
        try request.respond("{\"success\":false,\"error\":\"invalid json\"}", .{
            .extra_headers = &.{.{ .name = "Content-Type", .value = "application/json" }},
        });
        return;
    }

    const name_val = parsed.?.value.object.get("name") orelse null;
    if (name_val == null or name_val.? != .string) {
        try request.respond("{\"success\":false,\"error\":\"missing name\"}", .{
            .extra_headers = &.{.{ .name = "Content-Type", .value = "application/json" }},
        });
        return;
    }

    const backup_name = name_val.?.string;

    const backup_config = app.settings_manager.getBackupConfig();
    if (backup_config.target_type == .webdav or backup_config.target_type == .s3) {
        const credentials_available = switch (backup_config.target_type) {
            .webdav => backup_config.webdav_password.len > 0,
            .s3 => backup_config.s3_access_key.len > 0 and backup_config.s3_secret_key.len > 0,
            else => true,
        };

        if (!credentials_available) {
            const has_pwd = app.settings_manager.hasMasterPassword();
            const action_mode: []const u8 = if (has_pwd) "unlock" else "setup";
            const err_msg = try createMasterPasswordError(allocator, "credentials_not_available", "凭证不可用，请先设置主密码", action_mode);
            defer allocator.free(err_msg);
            try request.respond(err_msg, .{
                .extra_headers = &.{.{ .name = "Content-Type", .value = "application/json" }},
            });
            return;
        }

        if (!app.settings_manager.isUnlocked()) {
            const err_msg = try createMasterPasswordError(allocator, "master_password_not_unlocked", "凭证已过期，请重新解锁", "unlock");
            defer allocator.free(err_msg);
            try request.respond(err_msg, .{
                .extra_headers = &.{.{ .name = "Content-Type", .value = "application/json" }},
            });
            return;
        }
    }

    const db = app.settings_manager.sqlite_db orelse {
        try request.respond("{\"success\":false,\"error\":\"database not open\"}", .{
            .extra_headers = &.{.{ .name = "Content-Type", .value = "application/json" }},
        });
        return;
    };

    db.backup_manager.restoreFromBackup(backup_name) catch |err| {
        const err_msg = std.fmt.allocPrint(getAllocator(), "{{\"success\":false,\"error\":\"{s}\"}}", .{@errorName(err)}) catch unreachable;
        defer getAllocator().free(err_msg);
        try request.respond(err_msg, .{
            .extra_headers = &.{.{ .name = "Content-Type", .value = "application/json" }},
        });
        return;
    };

    const resp = "{\"success\":true}";
    try request.respond(resp, .{
        .extra_headers = &.{.{ .name = "Content-Type", .value = "application/json" }},
    });
}

fn handleBackupList(request: *http.Server.Request) !void {
    logger.global_logger.info("[API] GET /api/backup/list", .{});
    const app = getApp();
    const db = app.settings_manager.sqlite_db orelse {
        try request.respond("{\"success\":false,\"error\":\"database not open\"}", .{
            .extra_headers = &.{.{ .name = "Content-Type", .value = "application/json" }},
        });
        return;
    };

    const backups = db.backup_manager.listBackups() catch |err| {
        logger.global_logger.warn("[Backup] listBackups failed: {any}, returning empty list", .{err});
        try request.respond("{\"success\":true,\"backups\":[]}", .{
            .extra_headers = &.{.{ .name = "Content-Type", .value = "application/json" }},
        });
        return;
    };
    defer db.backup_manager.freeBackupList(backups);

    const allocator = getAllocator();
    var json_parts: std.ArrayList(u8) = .{};
    defer json_parts.deinit(allocator);

    try json_parts.appendSlice(allocator, "{\"success\":true,\"backups\":[");
    for (backups, 0..) |b, i| {
        if (i > 0) try json_parts.appendSlice(allocator, ",");
        const entry = try std.fmt.allocPrint(allocator,
            "{{\"name\":\"{s}\",\"timestamp\":{d},\"size_bytes\":{d}}}",
            .{ b.name, b.timestamp, b.size_bytes }
        );
        defer allocator.free(entry);
        try json_parts.appendSlice(allocator, entry);
    }
    try json_parts.appendSlice(allocator, "]}");

    try request.respond(json_parts.items, .{
        .extra_headers = &.{.{ .name = "Content-Type", .value = "application/json" }},
    });
}

fn handleBackupDelete(request: *http.Server.Request) !void {
    logger.global_logger.info("[API] DELETE /api/backup/*", .{});
    const allocator = getAllocator();
    const target = request.head.target;

    const name_start = std.mem.lastIndexOfScalar(u8, target, '/') orelse target.len;
    const backup_name = target[name_start + 1..];

    // 防止路径遍历攻击
    if (backup_name.len == 0 or
        std.mem.indexOf(u8, backup_name, "..") != null or
        std.mem.indexOfAny(u8, backup_name, "/\\") != null) {
        try request.respond("{\"success\":false,\"error\":\"invalid backup name\"}", .{
            .extra_headers = &.{.{ .name = "Content-Type", .value = "application/json" }},
        });
        return;
    }

    const app = getApp();
    const db = app.settings_manager.sqlite_db orelse {
        try request.respond("{\"success\":false,\"error\":\"database not open\"}", .{
            .extra_headers = &.{.{ .name = "Content-Type", .value = "application/json" }},
        });
        return;
    };

    db.backup_manager.deleteBackup(backup_name) catch |err| {
        const err_msg = std.fmt.allocPrint(allocator, "{{\"success\":false,\"error\":\"{s}\"}}", .{@errorName(err)}) catch unreachable;
        defer allocator.free(err_msg);
        try request.respond(err_msg, .{
            .extra_headers = &.{.{ .name = "Content-Type", .value = "application/json" }},
        });
        return;
    };

    const resp = "{\"success\":true}";
    try request.respond(resp, .{
        .extra_headers = &.{.{ .name = "Content-Type", .value = "application/json" }},
    });
}

fn handleBackupVerify(request: *http.Server.Request) !void {
    logger.global_logger.info("[API] POST /api/backup/verify", .{});
    const app = getApp();
    const allocator = getAllocator();

    const backup_config = app.settings_manager.getBackupConfig();
    if (!backup_config.enabled) {
        try request.respond("{\"success\":false,\"error\":\"backup not enabled\"}", .{
            .extra_headers = &.{.{ .name = "Content-Type", .value = "application/json" }},
        });
        return;
    }

    if (backup_config.target_type == .webdav or backup_config.target_type == .s3) {
        const credentials_available = switch (backup_config.target_type) {
            .webdav => backup_config.webdav_password.len > 0,
            .s3 => backup_config.s3_access_key.len > 0 and backup_config.s3_secret_key.len > 0,
            else => true,
        };

        if (!credentials_available) {
            const has_pwd = app.settings_manager.hasMasterPassword();
            const action_mode: []const u8 = if (has_pwd) "unlock" else "setup";
            const err_msg = try createMasterPasswordError(allocator, "credentials_not_available", "凭证不可用，请先设置主密码", action_mode);
            defer allocator.free(err_msg);
            try request.respond(err_msg, .{
                .extra_headers = &.{.{ .name = "Content-Type", .value = "application/json" }},
            });
            return;
        }

        if (!app.settings_manager.isUnlocked()) {
            const err_msg = try createMasterPasswordError(allocator, "master_password_not_unlocked", "凭证已过期，请重新解锁", "unlock");
            defer allocator.free(err_msg);
            try request.respond(err_msg, .{
                .extra_headers = &.{.{ .name = "Content-Type", .value = "application/json" }},
            });
            return;
        }
    }

    const adapter: backup_mod.BackupAdapter = switch (backup_config.target_type) {
        .local => backup_mod.createLocalAdapter(allocator, .{ .path = if (backup_config.local_path.len > 0) backup_config.local_path else "" }),
        .webdav => backup_mod.createWebDAVAdapter(allocator, .{
            .url = backup_config.webdav_url,
            .username = backup_config.webdav_username,
            .password = backup_config.webdav_password,
            .base_path = "/",
        }, &logWebdavErr),
        .s3 => backup_mod.createS3Adapter(allocator, .{
            .endpoint = backup_config.s3_endpoint,
            .bucket = backup_config.s3_bucket,
            .region = backup_config.s3_region,
            .access_key = backup_config.s3_access_key,
            .secret_key = backup_config.s3_secret_key,
            .path_prefix = if (backup_config.s3_path_prefix.len > 0) backup_config.s3_path_prefix else "little_timer/",
        }),
    };
    defer {
        adapter.vtable.freeList(adapter.ptr, &[_]backup_mod.BackupInfo{});
    }

    const test_result = adapter.list() catch |err| {
        const err_msg = std.fmt.allocPrint(allocator, "{{\"success\":false,\"error\":\"{s}\"}}", .{@errorName(err)}) catch unreachable;
        defer allocator.free(err_msg);
        try request.respond(err_msg, .{
            .extra_headers = &.{.{ .name = "Content-Type", .value = "application/json" }},
        });
        return;
    };
    adapter.freeList(test_result);

    try request.respond("{\"success\":true}", .{
        .extra_headers = &.{.{ .name = "Content-Type", .value = "application/json" }},
    });
}

fn handleBackupUnlock(request: *http.Server.Request) !void {
    logger.global_logger.info("[API] POST /api/backup/unlock", .{});
    const app = getApp();
    const allocator = getAllocator();

    const body_bytes = readRequestBody(request, allocator) catch |err| {
        const err_msg = switch (err) {
            error.RequestBodyTooLarge => "body too large",
            else => "read error",
        };
        try request.respond(try std.fmt.allocPrint(allocator, "{{\"success\":false,\"error\":\"{s}\"}}", .{err_msg}), .{
            .extra_headers = &.{.{ .name = "Content-Type", .value = "application/json" }},
        });
        return;
    };
    defer allocator.free(body_bytes);

    const body_json = std.json.parseFromSlice(json.Value, allocator, body_bytes, .{}) catch {
        try request.respond("{\"success\":false,\"error\":\"invalid json\"}", .{
            .extra_headers = &.{.{ .name = "Content-Type", .value = "application/json" }},
        });
        return;
    };
    defer body_json.deinit();

    const password = body_json.value.object.get("password") orelse {
        try request.respond("{\"success\":false,\"error\":\"missing password\"}", .{
            .extra_headers = &.{.{ .name = "Content-Type", .value = "application/json" }},
        });
        return;
    };

    const password_str = password.string;
    const unlock_result = app.settings_manager.unlockCredentials(password_str);

    const resp = try std.fmt.allocPrint(allocator, "{{\"success\":{s},\"locked_until\":{d}}}", .{
        if (unlock_result.success) "true" else "false",
        @as(f64, @floatFromInt(unlock_result.locked_until)),
    });
    defer allocator.free(resp);

    try request.respond(resp, .{
        .extra_headers = &.{.{ .name = "Content-Type", .value = "application/json" }},
    });
}

fn handleBackupLock(request: *http.Server.Request) !void {
    logger.global_logger.info("[API] POST /api/backup/lock", .{});
    const app = getApp();

    const now = std.time.timestamp();
    app.settings_manager.backup_config.credential_locked_until = now + 1;
    app.settings_manager.setCredentialPassword("");

    app.settings_manager.saveUnlockState() catch |err| {
        logger.global_logger.err("保存锁定状态失败: {any}", .{err});
    };

    try request.respond("{\"success\":true}", .{
        .extra_headers = &.{.{ .name = "Content-Type", .value = "application/json" }},
    });
}

fn handleGetMasterPasswordStatus(request: *http.Server.Request) !void {
    logger.global_logger.info("[API] GET /api/backup/master-password", .{});
    const app = getApp();
    const allocator = getAllocator();

    const status = app.settings_manager.getMasterPasswordStatus();

    const response = try std.fmt.allocPrint(allocator,
        \\{{"has_password":{}, "unlocked":{}, "locked_until":{d}, "unlock_time":{d}}}
    , .{
        status.has_password,
        status.unlocked,
        @as(f64, @floatFromInt(status.locked_until)),
        @as(f64, @floatFromInt(status.unlock_time)),
    });
    defer allocator.free(response);

    try request.respond(response, .{
        .extra_headers = &.{.{ .name = "Content-Type", .value = "application/json" }},
    });
}

fn handleSetMasterPassword(request: *http.Server.Request) !void {
    logger.global_logger.info("[API] POST /api/backup/master-password", .{});
    const app = getApp();
    const allocator = getAllocator();

    const body_bytes = readRequestBody(request, allocator) catch |err| {
        const err_msg = switch (err) {
            error.RequestBodyTooLarge => "body too large",
            else => "read error",
        };
        try request.respond(try std.fmt.allocPrint(allocator, "{{\"success\":false,\"error\":\"{s}\"}}", .{err_msg}), .{
            .extra_headers = &.{.{ .name = "Content-Type", .value = "application/json" }},
        });
        return;
    };
    defer allocator.free(body_bytes);

    const body_json = std.json.parseFromSlice(json.Value, allocator, body_bytes, .{}) catch {
        try request.respond("{\"success\":false,\"error\":\"invalid json\"}", .{
            .extra_headers = &.{.{ .name = "Content-Type", .value = "application/json" }},
        });
        return;
    };
    defer body_json.deinit();

    const password = body_json.value.object.get("password") orelse {
        try request.respond("{\"success\":false,\"error\":\"missing password\"}", .{
            .extra_headers = &.{.{ .name = "Content-Type", .value = "application/json" }},
        });
        return;
    };

    const password_str = password.string;

    if (password_str.len < 4) {
        try request.respond("{\"success\":false,\"error\":\"password too short (minimum 4 characters)\"}", .{
            .extra_headers = &.{.{ .name = "Content-Type", .value = "application/json" }},
        });
        return;
    }

    app.settings_manager.setMasterPassword(password_str) catch |err| {
        try request.respond(try std.fmt.allocPrint(allocator, "{{\"success\":false,\"error\":\"{s}\"}}", .{@errorName(err)}), .{
            .extra_headers = &.{.{ .name = "Content-Type", .value = "application/json" }},
        });
        return;
    };

    try request.respond("{\"success\":true}", .{
        .extra_headers = &.{.{ .name = "Content-Type", .value = "application/json" }},
    });
}

fn handleAuthStatus(request: *http.Server.Request) !void {
    logger.global_logger.info("[API] GET /api/auth/status", .{});
    const app = getApp();
    const allocator = getAllocator();

    const auth_config = app.settings_manager.getConfig().auth;

    const response = std.fmt.allocPrint(allocator,
        \\{{"auth_enabled":{}, "has_token":{}}}
    , .{
        auth_config.auth_enabled,
        auth_config.auth_token.len > 0,
    }) catch {
        try request.respond("{\"err\":\"encoding error\"}", .{
            .extra_headers = &.{.{ .name = "Content-Type", .value = "application/json" }},
        });
        return;
    };
    defer allocator.free(response);

    try request.respond(response, .{
        .extra_headers = &.{.{ .name = "Content-Type", .value = "application/json" }},
    });
}

fn handleAuthEnable(request: *http.Server.Request) !void {
    logger.global_logger.info("[API] POST /api/auth/enable", .{});
    const allocator = getAllocator();

    const body = try readRequestBody(request, allocator);
    defer allocator.free(body);

    const app = getApp();
    const token = crypto.generateToken(allocator) catch {
        try request.respond("{\"err\":\"token generation failed\"}", .{
            .extra_headers = &.{.{ .name = "Content-Type", .value = "application/json" }},
        });
        return;
    };
    defer allocator.free(token);

    var new_config = app.settings_manager.getConfig();
    new_config.auth.auth_enabled = true;
    new_config.auth.auth_token = token;

    app.settings_manager.updateAuth(new_config.*) catch {
        try request.respond("{\"err\":\"save failed\"}", .{
            .extra_headers = &.{.{ .name = "Content-Type", .value = "application/json" }},
        });
        return;
    };

    const response = std.fmt.allocPrint(allocator, "{{\"success\":true,\"token\":\"{s}\"}}", .{token}) catch {
        try request.respond("{\"err\":\"encoding error\"}", .{
            .extra_headers = &.{.{ .name = "Content-Type", .value = "application/json" }},
        });
        return;
    };
    defer allocator.free(response);

    try request.respond(response, .{
        .extra_headers = &.{.{ .name = "Content-Type", .value = "application/json" }},
    });
}

fn handleAuthDisable(request: *http.Server.Request) !void {
    logger.global_logger.info("[API] POST /api/auth/disable", .{});
    const allocator = getAllocator();

    const body = try readRequestBody(request, allocator);
    defer allocator.free(body);

    const app = getApp();

    var new_config = app.settings_manager.getConfig();
    new_config.auth.auth_enabled = false;

    app.settings_manager.updateAuth(new_config.*) catch {
        try request.respond("{\"err\":\"save failed\"}", .{
            .extra_headers = &.{.{ .name = "Content-Type", .value = "application/json" }},
        });
        return;
    };

    try request.respond("{\"success\":true}", .{
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
    defer allocator.free(body_copy);

    try app.settings_manager.handleSettingsEvent(.{ .change_settings = body_copy });

    try request.respond("{\"status\":\"settings_updated\"}", .{
        .extra_headers = &.{.{ .name = "Content-Type", .value = "application/json" }},
    });
}

fn handleSSE(request: *http.Server.Request) !void {
    logger.global_logger.info("SSE 客户端连接建立", .{});

    const app = getApp();
    var buffer: [8192]u8 = undefined;
    var last_heartbeat_ts = std.time.timestamp();

    // SSE 安全限制
    const max_session_seconds: i64 = 3600; // 最大连接时间 1 小时
    const max_heartbeat_gap_seconds: i64 = 30; // 最大心跳间隔 30 秒
    const session_start_ts = std.time.timestamp();

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

        // 检查最大连接时间
        if (now - session_start_ts > max_session_seconds) {
            logger.global_logger.info("SSE 连接超时 (1小时)，关闭连接", .{});
            break;
        }

        // 检查心跳超时
        if (now - last_heartbeat_ts > max_heartbeat_gap_seconds * 3) {
            logger.global_logger.warn("SSE 心跳超时，关闭连接", .{});
            break;
        }

        // 不再调用 clock.tick()——状态由用户事件驱动，SSE 只推送状态
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

    const app = getApp();

    var sessions: []habit_crud.SessionRow = &.{};
    var owns_sessions = false;
    defer if (owns_sessions) {
        app.settings_manager.sqlite_db.?.*.habit_manager.freeSessions(sessions);
    };

    if (start_date != null and end_date != null) {
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

    const serialized = try std.fmt.allocPrint(getAllocator(), "{f}", .{std.json.fmt(sessions, .{})});
    defer getAllocator().free(serialized);
    try request.respond(serialized, .{
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

    const progress_percent: i64 = if (h_row.goal_seconds > 0) @divTrunc(today_seconds * 100, h_row.goal_seconds) else 0;

    const resp = try std.fmt.allocPrint(getAllocator(), "{{\"id\":{},\"name\":\"{s}\",\"goal_seconds\":{},\"color\":\"{s}\",\"today_seconds\":{},\"progress_percent\":{}}}", .{
        h_row.id, h_row.name, h_row.goal_seconds, h_row.color, today_seconds, progress_percent,
    });
    defer getAllocator().free(resp);
    try request.respond(resp, .{
        .extra_headers = &.{.{ .name = "Content-Type", .value = "application/json" }},
    });
}

// ============================================
// Wallpaper 辅助函数
// ============================================

fn getWallpapersDir() ![]const u8 {
    const allocator = getAllocator();
    const app = getApp();
    const db_path = app.settings_manager.sqlite_db.?.*.db_path;
    const db_dir = std.fs.path.dirname(db_path) orelse ".";
    const wallpapers_dir = try std.fs.path.join(allocator, &[_][]const u8{ db_dir, "wallpapers" });
    std.fs.cwd().makeDir(wallpapers_dir) catch |err| {
        if (err != error.PathAlreadyExists) {
            logger.global_logger.err("创建wallpapers目录失败: {any}", .{err});
            return error.WallpapersDirCreateFailed;
        }
    };
    return wallpapers_dir;
}

fn sanitizeFilename(name: []const u8, allocator: std.mem.Allocator) ![]const u8 {
    var buf = try allocator.alloc(u8, name.len);
    var out_len: usize = 0;
    for (name) |c| {
        if ((c >= 'a' and c <= 'z') or (c >= 'A' and c <= 'Z') or
            (c >= '0' and c <= '9') or c == '-' or c == '_' or c == '.')
        {
            buf[out_len] = c;
            out_len += 1;
        } else {
            buf[out_len] = '_';
            out_len += 1;
        }
    }
    return allocator.realloc(buf, out_len);
}

fn extractMultipartBoundary(request: *http.Server.Request) ?[]const u8 {
    var it = request.iterateHeaders();
    while (it.next()) |header| {
        if (std.ascii.eqlIgnoreCase(header.name, "content-type")) {
            const prefix = "multipart/form-data; boundary=";
            if (std.mem.startsWith(u8, header.value, prefix)) {
                return header.value[prefix.len..];
            }
        }
    }
    return null;
}

fn parseMultipartFile(
    body: []const u8,
    boundary: []const u8,
    allocator: std.mem.Allocator,
) !struct { filename: []const u8, data: []const u8 } {
    const dash = "--";
    const crlf = "\r\n";
    const crlfcrlf = "\r\n\r\n";

    const opening = try std.mem.concat(allocator, u8, &[_][]const u8{ dash, boundary, crlf });
    defer allocator.free(opening);
    const closing = try std.mem.concat(allocator, u8, &[_][]const u8{ crlf, dash, boundary, dash, crlf });
    defer allocator.free(closing);

    if (!std.mem.startsWith(u8, body, opening)) {
        return error.InvalidMultipartFormat;
    }

    var remaining = body[opening.len..];

    if (std.mem.endsWith(u8, remaining, closing)) {
        remaining = remaining[0 .. remaining.len - closing.len];
    } else {
        const end_dash = try std.mem.concat(allocator, u8, &[_][]const u8{ crlf, dash, boundary, dash });
        defer allocator.free(end_dash);
        if (std.mem.endsWith(u8, remaining, end_dash)) {
            remaining = remaining[0 .. remaining.len - end_dash.len];
        }
    }

    const header_end = std.mem.indexOf(u8, remaining, crlfcrlf) orelse return error.InvalidMultipartFormat;

    const header_section = remaining[0..header_end];
    remaining = remaining[header_end + crlfcrlf.len ..];

    var filename: []const u8 = "upload";
    var lines = std.mem.splitScalar(u8, header_section, '\n');
    while (lines.next()) |line| {
        const trimmed = std.mem.trimRight(u8, line, "\r");
        if (std.mem.startsWith(u8, trimmed, "Content-Disposition:")) {
            const needle = "filename=\"";
            if (std.mem.indexOf(u8, trimmed, needle)) |idx| {
                const start = idx + needle.len;
                if (std.mem.indexOfScalarPos(u8, trimmed, start, '"')) |end_idx| {
                    filename = trimmed[start..end_idx];
                    if (filename.len == 0) filename = "upload";
                }
            }
        }
    }

    return .{ .filename = filename, .data = remaining };
}

// ============================================
// Wallpaper 处理函数
// ============================================

fn handleUploadWallpaper(request: *http.Server.Request) !void {
    const allocator = getAllocator();
    const boundary = extractMultipartBoundary(request) orelse {
        try request.respond("{\"err\":\"Missing boundary\"}", .{
            .extra_headers = &.{.{ .name = "Content-Type", .value = "application/json" }},
        });
        return;
    };

    const body = try readRequestBody(request, allocator);
    defer allocator.free(body);

    const parsed = parseMultipartFile(body, boundary, allocator) catch |err| {
        logger.global_logger.err("解析multipart失败: {any}", .{err});
        try request.respond("{\"err\":\"Failed to parse upload\"}", .{
            .extra_headers = &.{.{ .name = "Content-Type", .value = "application/json" }},
        });
        return;
    };

    const wallpapers_dir = try getWallpapersDir();
    const safe_name = try sanitizeFilename(parsed.filename, allocator);
    defer allocator.free(safe_name);

    const basename = std.fs.path.basename(safe_name);
    const ext = std.fs.path.extension(basename);
    const stem = if (ext.len > 0 and ext.len < basename.len) basename[0 .. basename.len - ext.len] else basename;

    const timestamp = @as(u64, @intCast(std.time.timestamp()));
    const unique_name = try std.fmt.allocPrint(allocator, "{d}_{s}{s}", .{ timestamp, stem, ext });
    defer allocator.free(unique_name);

    const file_path = try std.fs.path.join(allocator, &[_][]const u8{ wallpapers_dir, unique_name });
    defer allocator.free(file_path);

    const file = try std.fs.cwd().createFile(file_path, .{});
    defer file.close();
    try file.writeAll(parsed.data);

    logger.global_logger.info("壁纸上载成功: {s}", .{unique_name});

    const resp = try std.fmt.allocPrint(allocator, "{{\"filename\":\"{s}\"}}", .{unique_name});
    defer allocator.free(resp);
    try request.respond(resp, .{
        .extra_headers = &.{.{ .name = "Content-Type", .value = "application/json" }},
    });
}

fn handleListWallpapers(request: *http.Server.Request) !void {
    const allocator = getAllocator();
    const wallpapers_dir = getWallpapersDir() catch {
        try request.respond("[]", .{
            .extra_headers = &.{.{ .name = "Content-Type", .value = "application/json" }},
        });
        return;
    };

    var dir = std.fs.cwd().openDir(wallpapers_dir, .{ .iterate = true }) catch {
        try request.respond("[]", .{
            .extra_headers = &.{.{ .name = "Content-Type", .value = "application/json" }},
        });
        return;
    };
    defer dir.close();

    // Build JSON manually to avoid ArrayList API issues
    var json_buf = std.ArrayList(u8).initCapacity(allocator, 4096) catch {
        try request.respond("[]", .{
            .extra_headers = &.{.{ .name = "Content-Type", .value = "application/json" }},
        });
        return;
    };
    defer json_buf.deinit(allocator);

    try json_buf.append(allocator, '[');

    var first = true;
    var it = dir.iterate();
    while (it.next() catch null) |entry| {
        if (entry.kind != .file) continue;

        if (!first) try json_buf.append(allocator, ',');
        first = false;

        const entry_json = try std.fmt.allocPrint(allocator, "{{\"name\":\"{s}\"}}", .{entry.name});
        defer allocator.free(entry_json);
        try json_buf.appendSlice(allocator, entry_json);
    }

    try json_buf.append(allocator, ']');

    try request.respond(json_buf.items, .{
        .extra_headers = &.{.{ .name = "Content-Type", .value = "application/json" }},
    });
}

fn handleServeWallpaper(request: *http.Server.Request) !void {
    const target = request.head.target;
    const path = if (std.mem.indexOfScalar(u8, target, '?')) |q_idx| target[0..q_idx] else target;

    const filename = path["/api/wallpapers/".len..];
    if (filename.len == 0 or std.mem.indexOfScalar(u8, filename, '/') != null) {
        try request.respond("{\"err\":\"Invalid filename\"}", .{
            .extra_headers = &.{.{ .name = "Content-Type", .value = "application/json" }},
        });
        return;
    }

    const allocator = getAllocator();
    const wallpapers_dir = getWallpapersDir() catch {
        try request.respond("{\"err\":\"Wallpapers dir not found\"}", .{
            .extra_headers = &.{.{ .name = "Content-Type", .value = "application/json" }},
        });
        return;
    };
    defer allocator.free(wallpapers_dir);

    const file_path = try std.fs.path.join(allocator, &[_][]const u8{ wallpapers_dir, filename });
    defer allocator.free(file_path);

    const file = std.fs.cwd().openFile(file_path, .{}) catch {
        try request.respond("{\"err\":\"File not found\"}", .{
            .extra_headers = &.{.{ .name = "Content-Type", .value = "application/json" }},
        });
        return;
    };
    defer file.close();

    const file_size = (try file.stat()).size;
    if (file_size > 50 * 1024 * 1024) {
        try request.respond("{\"err\":\"File too large\"}", .{
            .extra_headers = &.{.{ .name = "Content-Type", .value = "application/json" }},
        });
        return;
    }

    const file_data = try allocator.alloc(u8, @intCast(file_size));
    defer allocator.free(file_data);
    _ = try file.readAll(file_data);

    const ext = std.fs.path.extension(filename);
    const content_type = if (std.ascii.eqlIgnoreCase(ext, ".jpg") or std.ascii.eqlIgnoreCase(ext, ".jpeg"))
        "image/jpeg"
    else if (std.ascii.eqlIgnoreCase(ext, ".png"))
        "image/png"
    else if (std.ascii.eqlIgnoreCase(ext, ".gif"))
        "image/gif"
    else if (std.ascii.eqlIgnoreCase(ext, ".webp"))
        "image/webp"
    else if (std.ascii.eqlIgnoreCase(ext, ".svg"))
        "image/svg+xml"
    else if (std.ascii.eqlIgnoreCase(ext, ".bmp"))
        "image/bmp"
    else
        "application/octet-stream";

    const content_type_header = try std.fmt.allocPrint(allocator, "{s}", .{content_type});
    defer allocator.free(content_type_header);

    try request.respond(file_data, .{
        .extra_headers = &.{
            .{ .name = "Content-Type", .value = content_type_header },
            .{ .name = "Cache-Control", .value = "public, max-age=86400" },
        },
    });
}

fn handleDeleteWallpaper(request: *http.Server.Request) !void {
    const target = request.head.target;
    const path = if (std.mem.indexOfScalar(u8, target, '?')) |q_idx| target[0..q_idx] else target;

    const filename = path["/api/wallpapers/".len..];
    if (filename.len == 0 or std.mem.indexOfScalar(u8, filename, '/') != null) {
        try request.respond("{\"err\":\"Invalid filename\"}", .{
            .extra_headers = &.{.{ .name = "Content-Type", .value = "application/json" }},
        });
        return;
    }

    const allocator = getAllocator();
    const wallpapers_dir = getWallpapersDir() catch {
        try request.respond("{\"err\":\"Wallpapers dir not found\"}", .{
            .extra_headers = &.{.{ .name = "Content-Type", .value = "application/json" }},
        });
        return;
    };
    defer allocator.free(wallpapers_dir);

    const file_path = try std.fs.path.join(allocator, &[_][]const u8{ wallpapers_dir, filename });
    defer allocator.free(file_path);

    std.fs.cwd().deleteFile(file_path) catch {
        try request.respond("{\"err\":\"Failed to delete file\"}", .{
            .extra_headers = &.{.{ .name = "Content-Type", .value = "application/json" }},
        });
        return;
    };

    logger.global_logger.info("壁纸已删除: {s}", .{filename});

    try request.respond("{\"success\":true}", .{
        .extra_headers = &.{.{ .name = "Content-Type", .value = "application/json" }},
    });
}
