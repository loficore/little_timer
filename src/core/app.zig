const std = @import("std");
const app = @import("app.zig");
const logger = @import("logger.zig");
const builtin = @import("builtin");

const build_options = @import("build_options");
const http_server = if (build_options.use_std_http) @import("http/std_server.zig") else @import("http/http_server.zig");
const clock = @import("clock.zig");
const settings = @import("../settings/settings_manager.zig");
const interface = @import("interface.zig");
const error_recovery = @import("utils/error_recovery.zig");
const habit_crud = @import("../storage/habit_crud.zig");

const EventThrottle = struct {
    last_event_time: i64 = 0,
    last_event_type: ?interface.EventType = null,
    debounce_ms: i64 = 100,
};

fn computeSessionElapsedSeconds(session: *const habit_crud.TimerSessionRow, now_ts: i64) i64 {
    var effective_paused_seconds = session.paused_total_seconds;

    if (session.pause_started_at) |pause_started_at| {
        if (now_ts > pause_started_at) {
            effective_paused_seconds += now_ts - pause_started_at;
        }
    }

    if (now_ts <= session.started_at or now_ts <= effective_paused_seconds) {
        return 0;
    }

    return now_ts - session.started_at - effective_paused_seconds;
}

fn updatePauseAccounting(session: *const habit_crud.TimerSessionRow, now_ts: i64) struct {
    paused_total_seconds: i64,
    pause_started_at: ?i64,
} {
    var paused_total_seconds = session.paused_total_seconds;
    var pause_started_at = session.pause_started_at;

    if (session.is_paused) {
        if (pause_started_at == null) {
            pause_started_at = now_ts;
        }
    } else if (pause_started_at) |started_at| {
        if (now_ts > started_at) {
            paused_total_seconds += now_ts - started_at;
        }
        pause_started_at = null;
    }

    return .{
        .paused_total_seconds = paused_total_seconds,
        .pause_started_at = pause_started_at,
    };
}

fn computeElapsedFromAccounting(
    started_at: i64,
    paused_total_seconds: i64,
    pause_started_at: ?i64,
    now_ts: i64,
) i64 {
    var effective_paused_seconds = paused_total_seconds;

    if (pause_started_at) |started_pause_at| {
        if (now_ts > started_pause_at) {
            effective_paused_seconds += now_ts - started_pause_at;
        }
    }

    if (now_ts <= started_at or now_ts <= effective_paused_seconds) {
        return 0;
    }

    return now_ts - started_at - effective_paused_seconds;
}

/// 全局 App 实例指针，用于回调函数访问
var global_app: ?*MainApplication = null;

pub const MainApplication = struct {
    clock_manager: clock.ClockManager,
    settings_manager: settings.SettingsManager,
    http_server: http_server.HttpServerManager,
    mutex: std.Thread.Mutex = .{},
    allocator: std.mem.Allocator,
    /// 错误恢复管理器 - 用于处理和追踪错误
    error_recovery: error_recovery.ErrorRecoveryManager,
    /// 事件去抖限流器 - 防止快速连续点击
    event_throttle: EventThrottle = .{},
    /// 当前正在计时的习惯 ID
    current_habit_id: ?i64 = null,
    /// 当前计时会话 ID（用于持久化）
    current_timer_session_id: ?i64 = null,
    /// 当前计时会话开始时间（秒）
    current_timer_session_started_at: ?i64 = null,
    /// 当前计时会话累计暂停时长（秒）
    current_timer_session_paused_total_seconds: i64 = 0,
    /// 当前暂停起始时间（秒）
    current_timer_session_pause_started_at: ?i64 = null,
    /// 上次保存进度的时间（用于节流）
    last_save_time: i64 = 0,
    /// 退出标志 - 用于通知 HTTP 服务器线程退出
    should_exit: std.atomic.Value(bool) = std.atomic.Value(bool).init(false),
    /// 停止标志 - 防止重复调用 stop()
    stopped: bool = false,

    /// 初始化应用程序（在指针上原地初始化）
    ///
    /// 参数:
    /// - **self**: MainApplication实例指针
    /// - **allocator**: 内存分配器
    /// - **settings_manager_param**: 设置管理器（已初始化）
    /// - **clock_config_param**: 时钟配置参数
    ///
    /// 返回:
    /// - !void: 如果初始化失败则返回错误
    pub fn init(self: *MainApplication, allocator: std.mem.Allocator) !void {
        // 第一步：初始化所有必要的字段（确保 mutex 被初始化）
        self.allocator = allocator;
        self.mutex = .{}; // 显式初始化 mutex

        // 初始化错误恢复管理器
        self.error_recovery = error_recovery.ErrorRecoveryManager.init(allocator);

        // 初始化和加载设置（纯 SQLite 版本，不再需要 settings.toml）
        self.settings_manager = try settings.SettingsManager.init(allocator, "");
        errdefer self.settings_manager.deinit();
        self.settings_manager.load() catch |err| {
            logger.global_logger.warn("⚠️ 加载设置失败: {any}", .{err});

            // 方案3: 备份 + 自动重置
            // 1. 备份损坏的文件（如果存在）
            self.settings_manager.backupCorruptedFile() catch |backup_err| {
                logger.global_logger.debug("备份失败（文件可能不存在）: {any}", .{backup_err});
            };

            // 2. 重置为默认配置
            self.settings_manager.resetToDefaults() catch |reset_err| {
                logger.global_logger.err("❌ 重置默认配置失败: {any}", .{reset_err});
                return reset_err; // 致命错误，无法继续
            };

            // 3. 保存默认配置到文件
            self.settings_manager.save() catch |save_err| {
                logger.global_logger.err("❌ 保存默认配置失败: {any}", .{save_err});
                self.error_recovery.recordError("保存默认配置失败", "SETTINGS_SAVE");
            };

            logger.global_logger.info("✅ 已重置为默认配置并保存", .{});
        };

        // 根据设置配置日志系统
        try self.configureLogger();

        // 根据设置构建时钟配置
        const clock_config = self.settings_manager.buildClockConfig();

        self.clock_manager = clock.ClockManager.init(
            clock_config,
        );

        // 初始化 HTTP 服务器
        self.http_server = try http_server.HttpServerManager.init(allocator, 8080, self);
    }
    /// 配置日志系统
    ///
    /// 参数:
    /// - **self**: MainApplication实例指针
    fn configureLogger(self: *MainApplication) !void {
        // 从设置中读取日志配置
        const log_config = &self.settings_manager.config.logging;

        // 解析日志等级
        const log_level = logger.LogLevel.fromString(log_config.level) orelse .INFO;

        // 配置全局logger
        logger.global_logger.current_level = log_level;
        logger.global_logger.enable_timestamp = log_config.enable_timestamp;

        // 启用文件日志
        if (log_config.enable_file_logging) {
            const file_config = logger.LogConfig{
                .log_dir = if (log_config.log_dir.len > 0) log_config.log_dir else "",
                .use_date_filename = true,
                .max_file_size = log_config.max_file_size,
                .max_file_count = log_config.max_file_count,
                .level = log_level,
                .enable_timestamp = log_config.enable_timestamp,
            };
            try logger.initGlobalLoggerFile(self.allocator, file_config);
            logger.global_logger.info("日志文件已启用，使用日期文件名: {}", .{file_config.use_date_filename});
        }

        logger.global_logger.info("日志系统已初始化 (等级: {s}, 时间戳: {})", .{
            log_level.toString(),
            log_config.enable_timestamp,
        });
    }

    /// 运行应用程序主循环
    pub fn run(self: *MainApplication) !void {
        return self.http_server.start();
    }

    /// 停止应用程序主循环
    pub fn stop(self: *MainApplication) !void {
        // 防止重复调用
        if (self.stopped) return;
        self.stopped = true;

        // 设置退出标志，通知 HTTP 服务器线程退出
        self.should_exit.store(true, .release);
        return self.http_server.stop();
    }

    /// 应用程序 tick - 更新逻辑并刷新显示
    ///
    /// 参数:
    /// - **ctx**: 应用程序上下文指针
    /// - **delta_ms**: 自上次tick以来经过的毫秒数
    pub fn tick(ctx: ?*anyopaque, delta_ms: i64) void {
        if (ctx == null) {
            logger.global_logger.err("Tick: ctx 为 null", .{});
            return;
        }

        const self: *MainApplication = @ptrCast(@alignCast(ctx));

        self.mutex.lock();
        defer self.mutex.unlock();

        logger.global_logger.debug("Tick: 增量 {} ms", .{delta_ms});

        self.clock_manager.handleEvent(.{ .tick = delta_ms });
        const display_data = self.clock_manager.update();
        self.updateDisplay(display_data) catch |err| {
            logger.global_logger.err("更新显示失败: {any}", .{err});
            self.error_recovery.recordError("更新显示失败", "DISPLAY_UPDATE");
        };

        // 每隔几秒自动保存计时进度（仅当正在运行时）
        const now_ns = std.time.nanoTimestamp();
        const now_ms: i64 = @intCast(@divFloor(now_ns, 1_000_000));
        if (now_ms - self.last_save_time > 5000) { // 5 秒保存一次
            self.last_save_time = now_ms;
            self.saveTimerProgress();
        }
    }

    /// 时钟事件处理器
    ///
    /// 参数:
    /// - **self**: MainApplication实例指针
    /// - **event**: 时钟事件
    ///
    /// 返回:
    /// - !void: 如果处理失败则返回错误
    fn clockHandle(self: *MainApplication, event: interface.ClockEvent) !void {
        // 去抖检查：只有 tick 事件或超过阈值时间的其他事件才处理
        const should_process = switch (event) {
            .tick => true, // tick 事件不受限制（需要频繁更新时间）
            else => blk: {
                const now_ns = std.time.nanoTimestamp();
                const now_ms: i64 = @intCast(@divFloor(now_ns, 1_000_000));
                const time_since_last = now_ms - self.event_throttle.last_event_time;

                if (time_since_last > self.event_throttle.debounce_ms) {
                    self.event_throttle.last_event_time = now_ms;
                    self.event_throttle.last_event_type = .{ .clock_event = event };
                    break :blk true;
                } else {
                    logger.global_logger.debug("事件被去抖忽略，距上次事件仅 {}ms", .{time_since_last});
                    break :blk false;
                }
            },
        };

        if (!should_process) {
            return;
        }

        self.clock_manager.handleEvent(event);
    }

    /// 设置事件处理器
    ///
    /// 参数:
    /// - **self**: MainApplication实例指针
    /// - **event**: 设置事件
    ///
    /// 返回:
    /// - !void: 如果处理失败则返回错误
    fn settingsHandle(self: *MainApplication, event: interface.SettingsEvent) !void {
        // 委托给 SettingsManager 处理（内部会自动管理内存）
        try self.settings_manager.handleSettingsEvent(event);

        // 如果是更改设置，需要重新初始化时钟
        if (event == .change_settings) {
            const new_clock_config = self.settings_manager.buildClockConfig();

            self.clock_manager = clock.ClockManager.init(
                new_clock_config,
            );

            // HTTP 服务器会通过 SSE 推送更新，前端会自动同步
            logger.global_logger.info("✓ 设置已更新，时钟已重新初始化", .{});
        }
    }

    /// 保存当前计时进度到数据库
    pub fn saveTimerProgress(self: *MainApplication) void {
        const session_id = self.current_timer_session_id orelse return;
        const db = self.settings_manager.sqlite_db orelse return;

        const session = db.habit_manager.getTimerSessionById(session_id) catch null;
        if (session) |s| {
            defer self.allocator.free(s.mode);
        }

        const clock_state = self.clock_manager.update();
        const remaining = clock_state.getRemainingSeconds();
        const is_running = !clock_state.isPaused();
        const is_finished = clock_state.isFinished();
        const is_paused = clock_state.isPaused();
        const current_round = clock_state.getCurrentRound();
        const in_rest = clock_state.inRest();

        const now_ts: i64 = @intCast(std.time.timestamp());
        var paused_total_seconds: i64 = 0;
        var pause_started_at: ?i64 = null;

        if (session) |s| {
            const pause_state = updatePauseAccounting(&s, now_ts);
            paused_total_seconds = pause_state.paused_total_seconds;
            pause_started_at = pause_state.pause_started_at;
        }

        if (self.current_timer_session_started_at == null) {
            if (session) |s| {
                self.current_timer_session_started_at = s.started_at;
            }
        }

        self.current_timer_session_paused_total_seconds = paused_total_seconds;
        self.current_timer_session_pause_started_at = pause_started_at;

        const elapsed_seconds = if (session) |s|
            computeSessionElapsedSeconds(&s, now_ts)
        else
            clock_state.getElapsedSeconds();

        db.habit_manager.updateTimerSession(
            session_id,
            elapsed_seconds,
            remaining,
            paused_total_seconds,
            pause_started_at,
            now_ts,
            is_running,
            is_paused,
            is_finished,
            current_round,
            in_rest,
        ) catch |err| {
            logger.global_logger.err("保存 timer_sessions 进度失败: {any}", .{err});
        };
    }

    /// 加载保存的计时进度（用于页面刷新恢复）
    pub fn loadTimerProgress(self: *MainApplication) void {
        const db = self.settings_manager.sqlite_db orelse return;

        const session = db.habit_manager.getActiveTimerSession() catch null;
        if (session == null) {
            logger.global_logger.debug("当前没有可恢复的计时会话", .{});
            return;
        }

        const s = session.?;
        defer self.allocator.free(s.mode);
        self.current_timer_session_id = s.id;
        self.current_habit_id = s.habit_id;
        self.current_timer_session_started_at = s.started_at;
        self.current_timer_session_paused_total_seconds = s.paused_total_seconds;
        self.current_timer_session_pause_started_at = s.pause_started_at;

        // 根据保存的状态恢复 clock
        const mode: interface.ModeEnumT = if (std.mem.eql(u8, s.mode, "countdown"))
            .COUNTDOWN_MODE
        else
            .STOPWATCH_MODE;

        const config: interface.ClockTaskConfig = if (mode == .COUNTDOWN_MODE)
            .{
                .default_mode = .COUNTDOWN_MODE,
                .countdown = .{
                    .duration_seconds = @intCast(s.work_duration),
                    .loop = s.loop_count > 0,
                    .loop_interval_seconds = @intCast(s.rest_duration),
                    .loop_count = @intCast(s.loop_count),
                },
                .stopwatch = .{ .max_seconds = 24 * 60 * 60 },
            }
        else
            .{
                .default_mode = .STOPWATCH_MODE,
                .countdown = .{ .duration_seconds = 25 * 60, .loop = false, .loop_count = 0, .loop_interval_seconds = 0 },
                .stopwatch = .{ .max_seconds = 24 * 60 * 60 },
            };

        self.clock_manager = clock.ClockManager.init(config);

        // 恢复状态
        const display = self.clock_manager.update();
        switch (display.getMode()) {
            .COUNTDOWN_MODE => {
                display.COUNTDOWN_MODE.remaining_ms = if (s.remaining_seconds) |r| r * 1000 else s.work_duration * 1000;
                display.COUNTDOWN_MODE.is_paused = s.is_paused;
                display.COUNTDOWN_MODE.is_finished = s.is_finished;
                display.COUNTDOWN_MODE.in_rest = s.in_rest;
            },
            .STOPWATCH_MODE => {
                display.STOPWATCH_MODE.esplased_ms = s.elapsed_seconds * 1000;
                display.STOPWATCH_MODE.is_paused = s.is_paused;
                display.STOPWATCH_MODE.is_finished = s.is_finished;
            },
        }

        logger.global_logger.info("✓ 已恢复计时会话记录 ID: {}", .{s.id});
    }

    /// 创建新的计时会话
    pub fn createTimerSession(self: *MainApplication, habit_id: ?i64, mode: []const u8, work_duration: i64, rest_duration: i64, loop_count: i64) !i64 {
        const db = self.settings_manager.sqlite_db orelse return error.DatabaseNotOpen;

        const session_id = try db.habit_manager.createTimerSession(
            habit_id,
            mode,
            work_duration,
            rest_duration,
            loop_count,
        );

        self.current_timer_session_id = session_id;
        self.current_habit_id = habit_id;
        self.current_timer_session_started_at = @intCast(std.time.timestamp());
        self.current_timer_session_paused_total_seconds = 0;
        self.current_timer_session_pause_started_at = null;

        logger.global_logger.info("✓ 已创建计时会话记录 ID: {}", .{session_id});
        return session_id;
    }

    /// 完成并记录计时会话
    pub fn finishTimerSession(self: *MainApplication) !i64 {
        const session_id = self.current_timer_session_id orelse return 0;
        const db = self.settings_manager.sqlite_db orelse return error.DatabaseNotOpen;

        const session = db.habit_manager.getTimerSessionById(session_id) catch null;
        const clock_state = self.clock_manager.update();
        const now_ts: i64 = @intCast(std.time.timestamp());

        // 触发 clock 结束事件
        self.clock_manager.handleEvent(.user_finish_timer);

        // 标记 session 为完成
        try db.habit_manager.finishTimerSession(session_id);

        // 以会话账本为准计算累计运行时间，避免依赖 SSE tick。
        var elapsed: i64 = if (self.current_timer_session_started_at) |started_at|
            computeElapsedFromAccounting(
                started_at,
                self.current_timer_session_paused_total_seconds,
                self.current_timer_session_pause_started_at,
                now_ts,
            )
        else
            clock_state.getElapsedSeconds();

        if (session) |s| {
            defer self.allocator.free(s.mode);

            const db_elapsed = computeSessionElapsedSeconds(&s, now_ts);

            if (db_elapsed > elapsed) {
                elapsed = db_elapsed;
            }

            if (s.elapsed_seconds > elapsed) {
                elapsed = s.elapsed_seconds;
            }
        }

        if (elapsed <= 0) {
            elapsed = clock_state.getElapsedSeconds();
        }

        logger.global_logger.info("✓ 会话统计已完成，累计运行时间: {} 秒", .{elapsed});

        return elapsed;
    }

    /// 重置并删除计时会话
    pub fn resetTimerSession(self: *MainApplication) void {
        if (self.current_timer_session_id) |session_id| {
            const db = self.settings_manager.sqlite_db;
            if (db) |d| {
                d.habit_manager.deleteTimerSession(session_id) catch |err| {
                    logger.global_logger.err("删除计时会话失败: {any}", .{err});
                };
            }
        }
        self.current_timer_session_id = null;
        self.current_timer_session_started_at = null;
        self.current_timer_session_paused_total_seconds = 0;
        self.current_timer_session_pause_started_at = null;
        self.current_habit_id = null;
    }

    /// 清理应用程序资源
    ///
    /// 参数:
    /// - **self**: MainApplication实例指针
    pub fn deinit(self: *MainApplication) void {
        logger.global_logger.info("MainApplication.deinit() 开始清理...", .{});

        // 1. 停止并释放 HTTP 服务器
        // stop() 可能已在主流程中调用，这里通过 MainApplication.stop() 的幂等保护避免重复 stop。
        self.stop() catch |err| {
            logger.global_logger.warn("deinit 阶段 stop 失败（继续释放资源）: {any}", .{err});
        };
        self.http_server.deinit();

        // 2. 释放设置管理器
        self.settings_manager.deinit();

        self.clock_manager.deinit();

        // 3. 清理错误恢复管理器
        self.error_recovery.deinit();

        // 4. 清空全局指针
        global_app = null;

        logger.global_logger.info("MainApplication.deinit() 完成", .{});

        // 5. 关闭全局日志文件，释放日志路径内存
        logger.global_logger.closeLogFile();
    }
};
