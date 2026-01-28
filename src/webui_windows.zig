const std = @import("std");
const builtin = @import("builtin");
const interface = @import("interface.zig");
const logger = @import("logger.zig");
const Thread = std.Thread;
const webui_module = @import("webui");

// 全局 App 实例指针，用于回调函数访问
var global_app: ?*anyopaque = null;

/// 设置全局 App 指针
pub fn setGlobalApp(app: ?*anyopaque) void {
    global_app = app;
}

// Tick函数类型定义
const TickFn = *const fn (ctx: ?*anyopaque, delta_ms: i64) void;

const UserEventT = interface.EventType;

const ExternParam = struct {
    ctx: ?*anyopaque,
    tick_handler: TickFn,
};

const ModeEnumT = interface.ModeEnumT;

const DisplaySnapshot = struct {
    seconds: i64,
    mode: ModeEnumT,
    is_running: bool,
    is_finished: bool,
    in_rest: bool,
    loop_remaining: u32,
    loop_total: u32,
    rest_remaining: i64,
    timezone: i8, // 世界时钟使用，单位小时
};

const Constants = struct {
    pub const APP_TITLE = "Little Timer - WebUI";
    pub const WINDOW_WIDTH = 400;
    pub const WINDOW_HEIGHT = 300;
};

/// WebUI窗口管理器 - 负责WebUI界面显示和用户事件收集
///
/// - **note** : 该结构体替代了原来的GTK WindowsManager，提供相同的接口
/// 但使用WebUI作为UI后端，实现跨平台的Web界面
pub const WebUIManager = struct {
    // WebUI窗口实例
    window: webui_module,

    // 事件回调函数指针 - 用于将用户事件传递给应用程序
    on_user_event: ?*const fn (interface.EventType) void,

    // 外部参数 - 保存应用程序上下文和tick处理器
    extern_param: ExternParam,

    // 内存分配器 - 用于html_content的释放
    allocator: std.mem.Allocator,

    // HTML内容缓冲 - 用于deinit时释放
    html_content: ?[]u8 = null,

    // 显示快照缓存 - 避免重复更新
    snap_shot: DisplaySnapshot,

    // 线程控制 - 用于停止 tick 线程
    is_running: std.atomic.Value(bool) = std.atomic.Value(bool).init(false),

    // Tick线程句柄 - 用于管理线程生命周期
    tick_thread: ?std.Thread = null,

    // Tick 间隔（毫秒），默认 1000ms (1秒) - 可在运行时动态配置
    tick_interval_ms: i64 = 1000,

    /// 初始化WebUI窗口管理器
    ///
    /// 参数:
    /// - **self**: WebUIManager实例指针
    /// - **on_user_event_param**: 用户事件回调函数
    /// - **extern_param**: 外部参数（包含应用程序上下文和tick处理器）
    ///
    /// 返回:
    /// - !void: 如果初始化失败则返回错误
    pub fn init(self: *WebUIManager, on_user_event_param: ?*const fn (UserEventT) void, extern_param: ExternParam, allocator: std.mem.Allocator) !void {
        self.window = webui_module.newWindow();
        self.on_user_event = on_user_event_param;
        self.extern_param = extern_param;
        self.allocator = allocator;

        // 在运行时读取 HTML 文件内容
        // 尝试多个可能的路径
        const possible_paths = [_][]const u8{
            "assets/dist/index.html", // 从项目根目录运行
        };

        var selected_path: []const u8 = "";

        var html_content_raw: []u8 = undefined;
        var html_found = false;

        for (possible_paths) |path| {
            html_content_raw = std.fs.cwd().readFileAlloc(self.allocator, path, 1024 * 1024) catch |err| {
                logger.global_logger.warn("无法从 {s} 读取: {any}", .{ path, err });
                continue;
            };
            logger.global_logger.info("✓ 从 {s} 读取 HTML ({} 字节)", .{ path, html_content_raw.len });
            selected_path = path;
            html_found = true;
            break;
        }

        if (!html_found) {
            logger.global_logger.err("错误: 无法找到 HTML 文件", .{});
            return error.HTMLFileNotFound;
        }

        // 将内容转换为 null 终止的字符串，show() 方法需要这个类型
        const html_content_zdup = try self.allocator.dupeZ(u8, html_content_raw);
        defer self.allocator.free(html_content_raw); // 释放原始内容
        self.html_content = html_content_zdup; // 保存指针供deinit释放

        // 关闭 WebUI 的文件夹监控线程，避免在 Android 上使用 pthread_cancel
        webui_module.setConfig(.folder_monitor, false);

        // 优先设置固定端口（需在 show/startServer 之前）
        const default_port: usize = 12889;
        self.window.setPort(default_port) catch |err| {
            logger.global_logger.warn("设置端口 {d} 失败，可能已被占用: {any}", .{ default_port, err });
        };

        // 在不同平台采用不同的启动方式：Android 使用 startServer，其它平台使用 show
        if (builtin.target.abi == .android) {
            const path_z = try self.allocator.dupeZ(u8, selected_path);
            defer self.allocator.free(path_z);
            _ = self.window.startServer(path_z) catch |err| {
                logger.global_logger.err("startServer() 失败: {any}", .{err});
            };
        } else {
            // 使用 show() 方法显示 HTML 内容（桌面平台）
            // 这会自动启动服务器并提供 /webui.js
            self.window.show(html_content_zdup) catch |err| {
                logger.global_logger.warn("show() 失败: {any}", .{err});
                logger.global_logger.warn("注意: show() 方法在某些环境下可能不可用", .{});
                logger.global_logger.warn("但服务器应该已经启动，请尝试访问下面的地址", .{});
            };
        }

        // 获取服务器信息
        const port = self.window.getPort() catch |err| blk: {
            logger.global_logger.warn("获取端口失败: {any}", .{err});
            break :blk 0;
        };

        // getUrl() 可能返回错误，需要特殊处理
        const url_str = self.window.getUrl() catch |err| blk: {
            logger.global_logger.warn("获取URL失败: {any}", .{err});
            break :blk "unknown";
        };

        logger.global_logger.info("", .{});
        logger.global_logger.info("========================================", .{});
        logger.global_logger.info("WebUI 服务器已启动！", .{});
        logger.global_logger.info("========================================", .{});
        logger.global_logger.info("访问地址: http://localhost:{d}", .{port});
        logger.global_logger.info("完整URL: {s}", .{url_str});
        logger.global_logger.info("========================================", .{});
        logger.global_logger.info("请在浏览器中打开上面的地址", .{});
        logger.global_logger.info("按 Ctrl+C 停止服务器", .{});
        logger.global_logger.info("========================================", .{});
        logger.global_logger.info("", .{});

        // 设置JavaScript回调函数，用于处理来自UI的事件
        self.setupEventHandlers();

        // 初始化显示快照（用于被动更新检测）
        self.snap_shot = DisplaySnapshot{
            .seconds = 0,
            .mode = .COUNTDOWN_MODE,
            .is_running = false,
            .is_finished = false,
            .in_rest = false,
            .loop_remaining = 0,
            .loop_total = 0,
            .rest_remaining = 0,
            .timezone = 8,
        };

        // 初始化时主动推送设置到前端，避免等待用户请求
        self.pushSettings();

        logger.global_logger.info("WebUI窗口管理器初始化完成", .{});
    }

    /// 获取HTML内容
    ///
    /// 返回:
    /// - **![]const u8**: HTML内容字符串，如果读取失败则返回错误
    fn getHTMLContent(self: *WebUIManager) ![]const u8 {
        _ = self;

        // 使用一个持久的分配器（这里为了简化，使用全局分配器）
        // 在实际应用中，应该从应用程序传入分配器
        var gpa = std.heap.GeneralPurposeAllocator(.{}){};
        // 注意：不 defer deinit，因为内存需要在应用程序生命周期内保留
        const allocator = gpa.allocator();

        // 尝试获取可执行文件路径，然后构建HTML文件的路径
        // 如果无法获取可执行文件路径，则尝试使用相对路径
        const html_content = std.fs.cwd().readFileAlloc(allocator, "assets/index.html", 1024 * 1024) catch {
            // 如果相对路径失败，尝试从当前工作目录的父目录读取
            return std.fs.cwd().readFileAlloc(allocator, "../assets/index.html", 1024 * 1024) catch |err| {
                logger.global_logger.err("无法读取HTML文件: {any}", .{err});
                // 如果文件读取失败，返回一个简单的错误页面
                return allocator.dupe(u8,
                    \\<!DOCTYPE html>
                    \\<html>
                    \\<head><meta charset="UTF-8"><title>错误</title></head>
                    \\<body><h1>无法加载应用程序界面</h1><p>assets/index.html 文件未找到</p></body>
                    \\</html>
                ) catch {
                    // 如果连错误页面都无法分配，返回错误
                    return error.HTMLFileReadError;
                };
            };
        };

        // 替换HTML中的动态内容
        // 在这个应用中，我们可能需要替换标题和窗口宽度
        // 但目前这些值是固定的，所以不需要替换
        // 如果将来需要动态内容，可以在这里添加替换逻辑

        return html_content;
    }

    /// 设置事件处理器
    ///
    /// 参数:
    /// - **self**: WebUIManager实例指针
    ///
    /// 配置JavaScript回调函数以处理来自UI的用户事件
    fn setupEventHandlers(self: *WebUIManager) void {
        // 注册JavaScript函数，用于处理UI事件
        // 这些函数将被HTML中的按钮点击事件调用
        logger.global_logger.debug("开始绑定事件处理器...", .{});

        _ = self.window.bind("start", handleStart) catch |err| {
            logger.global_logger.warn("绑定 start 失败: {any}", .{err});
            return;
        };
        logger.global_logger.debug("✓ start 绑定成功", .{});

        _ = self.window.bind("pause", handlePause) catch |err| {
            logger.global_logger.warn("绑定 pause 失败: {any}", .{err});
            return;
        };
        logger.global_logger.debug("✓ pause 绑定成功", .{});

        _ = self.window.bind("reset", handleReset) catch |err| {
            logger.global_logger.warn("绑定 reset 失败: {any}", .{err});
            return;
        };
        logger.global_logger.debug("✓ reset 绑定成功", .{});

        _ = self.window.bind("tick", handleTick) catch |err| {
            logger.global_logger.warn("绑定 tick 失败: {any}", .{err});
            return;
        };
        logger.global_logger.debug("✓ tick 绑定成功", .{});

        _ = self.window.bind("mode_change", handleModeChange) catch |err| {
            logger.global_logger.warn("绑定 mode_change 失败: {any}", .{err});
            return;
        };
        logger.global_logger.debug("✓ mode_change 绑定成功", .{});

        // 为兼容前端，同时绑定 change_mode
        _ = self.window.bind("change_mode", handleModeChange) catch |err| {
            logger.global_logger.warn("绑定 change_mode 失败: {any}", .{err});
            return;
        };
        logger.global_logger.debug("✓ change_mode 绑定成功", .{});

        _ = self.window.bind("get_settings", handleGetSettings) catch |err| {
            logger.global_logger.warn("绑定 get_settings 失败: {any}", .{err});
            return;
        };
        logger.global_logger.debug("✓ get_settings 绑定成功", .{});

        _ = self.window.bind("change_settings", handleChangeSettings) catch |err| {
            logger.global_logger.warn("绑定 change_settings 失败: {any}", .{err});
            return;
        };
        logger.global_logger.debug("✓ change_settings 绑定成功", .{});

        // 为每个绑定设置上下文，这样回调函数可以访问WebUIManager实例
        self.window.setContext("start", self);
        logger.global_logger.debug("✓ start 上下文设置成功", .{});

        self.window.setContext("pause", self);
        logger.global_logger.debug("✓ pause 上下文设置成功", .{});

        self.window.setContext("reset", self);
        logger.global_logger.debug("✓ reset 上下文设置成功", .{});

        self.window.setContext("tick", self);
        logger.global_logger.debug("✓ tick 上下文设置成功", .{});

        self.window.setContext("mode_change", self);
        logger.global_logger.debug("✓ mode_change 上下文设置成功", .{});

        self.window.setContext("change_mode", self);
        logger.global_logger.debug("✓ change_mode 上下文设置成功", .{});

        self.window.setContext("get_settings", self);
        logger.global_logger.debug("✓ get_settings 上下文设置成功", .{});

        self.window.setContext("change_settings", self);
        logger.global_logger.debug("✓ change_settings 上下文设置成功", .{});

        logger.global_logger.debug("事件绑定已注册", .{});
    }

    /// 更新显示
    ///
    /// 参数:
    /// - **self**: WebUIManager实例指针
    /// - **display_data**: 时钟显示数据
    ///
    /// 返回:
    /// - !void: 如果更新失败则返回错误
    pub fn updateDisplay(self: *WebUIManager, display_data: *const @import("clock.zig").ClockState) !void {
        // 1. 构造当前快照（包含所有显示相关状态）
        const current_snapshot = DisplaySnapshot{
            .seconds = display_data.getTimeInfo(),
            .mode = display_data.getMode(),
            .is_running = !display_data.isPaused(),
            .is_finished = display_data.isFinished(),
            .in_rest = display_data.inRest(),
            .loop_remaining = display_data.getLoopRemaining(),
            .loop_total = display_data.getLoopTotal(),
            .rest_remaining = display_data.getRestRemainingTime(),
            .timezone = switch (display_data.*) {
                .WORLD_CLOCK_MODE => |worldclock| worldclock.timezone,
                else => self.snap_shot.timezone,
            },
        };

        // 2. 被动更新：仅在状态改变时才执行 JS（而非无条件推送）
        // 关键修复：逐字段比较而非字节比较，避免内存对齐问题
        const snapshot_changed = (self.snap_shot.seconds != current_snapshot.seconds) or
            (self.snap_shot.mode != current_snapshot.mode) or
            (self.snap_shot.is_running != current_snapshot.is_running) or
            (self.snap_shot.is_finished != current_snapshot.is_finished) or
            (self.snap_shot.in_rest != current_snapshot.in_rest) or
            (self.snap_shot.loop_remaining != current_snapshot.loop_remaining) or
            (self.snap_shot.loop_total != current_snapshot.loop_total) or
            (self.snap_shot.rest_remaining != current_snapshot.rest_remaining) or
            (self.snap_shot.timezone != current_snapshot.timezone);

        if (!snapshot_changed) {
            // 状态未变化，无需推送任何 JS
            return;
        }

        logger.global_logger.debug("显示状态已改变，准备推送更新 (old_mode={}, new_mode={}, mode_changed={})", .{
            self.snap_shot.mode,
            current_snapshot.mode,
            self.snap_shot.mode != current_snapshot.mode,
        });

        // 3. 状态已改变，构造并执行 JavaScript 更新
        var js_code_buffer: [512:0]u8 = undefined;

        // 模式键（使用稳定的后端键，前端负责本地化）
        const mode_key = switch (current_snapshot.mode) {
            .COUNTDOWN_MODE => "countdown",
            .STOPWATCH_MODE => "stopwatch",
            .WORLD_CLOCK_MODE => "world_clock",
        };

        // 发送时间更新事件（从快照取值）
        const time_event_js = try std.fmt.bufPrintZ(
            &js_code_buffer,
            "if(window.webuiEvent){{window.webuiEvent({{function:'update_time',data:{}}});}}",
            .{current_snapshot.seconds},
        );
        self.window.run(time_event_js);

        // 发送模式更新事件
        const mode_event_js = try std.fmt.bufPrintZ(
            &js_code_buffer,
            "if(window.webuiEvent){{window.webuiEvent({{function:'update_mode',data:'{s}'}});}}",
            .{mode_key},
        );
        self.window.run(mode_event_js);

        // 发送完整状态更新事件（从快照取所有字段）
        const state_event_js = try std.fmt.bufPrintZ(
            &js_code_buffer,
            "if(window.webuiEvent){{window.webuiEvent({{function:'update_state',data:{{isRunning:{},isFinished:{},inRest:{},loopRemaining:{},loopTotal:{},restRemaining:{},timezone:{}}}}});}}",
            .{
                current_snapshot.is_running,
                current_snapshot.is_finished,
                current_snapshot.in_rest,
                current_snapshot.loop_remaining,
                current_snapshot.loop_total,
                current_snapshot.rest_remaining,
                current_snapshot.timezone,
            },
        );
        self.window.run(state_event_js);

        // 4. 缓存当前快照，作为下次比较的基准
        self.snap_shot = current_snapshot;
    }

    /// 配置前端使用的默认时区（非世界时钟模式也会携带此字段）
    pub fn setTimezone(self: *WebUIManager, timezone: i8) void {
        self.snap_shot.timezone = timezone;
        logger.global_logger.info("✓ WebUI 时区已更新为 {}", .{timezone});
    }

    /// 主动推送设置 JSON 到前端（用于初始化或设置变更后）
    pub fn pushSettings(self: *WebUIManager) void {
        if (global_app) |app_ptr| {
            const app = @import("app.zig").MainApplication;
            const main_app = @as(*app, @ptrCast(@alignCast(app_ptr)));
            const allocator = main_app.settings_manager.allocator;

            const json_str = main_app.settings_manager.toJsonAlloc() catch |err| {
                logger.global_logger.err("生成设置 JSON 失败: {any}", .{err});
                return;
            };
            defer allocator.free(json_str);

            var js_list = std.ArrayList(u8){};
            defer js_list.deinit(allocator);
            const js_writer = js_list.writer(allocator);

            js_writer.writeAll("updateSettingsDisplay(`") catch return;
            js_writer.writeAll(json_str) catch return;
            js_writer.writeAll("`)") catch return;

            const js_code = js_list.toOwnedSliceSentinel(allocator, 0) catch return;
            defer allocator.free(js_code);

            logger.global_logger.debug("pushSettings: 执行 JS (长度: {})", .{js_code.len});
            self.window.run(js_code);
            logger.global_logger.info("✓ 设置已推送到前端", .{});
        } else {
            logger.global_logger.warn("pushSettings: global_app 为 null，无法推送设置", .{});
        }
    }

    /// 处理用户事件
    ///
    /// 参数:
    /// - **self**: WebUIManager实例指针
    /// - **event**: 用户事件
    fn handleUserEvent(self: *WebUIManager, event: interface.EventType) void {
        logger.global_logger.debug("WebUIManager.handleUserEvent 被调用，on_user_event 是否为 null: {}", .{self.on_user_event == null});
        if (self.on_user_event) |handler| {
            logger.global_logger.debug("  调用用户事件处理器", .{});
            handler(event);
        } else {
            logger.global_logger.warn("未设置用户事件处理器", .{});
        }
    }

    /// Tick线程函数 - 在子线程中周期性调用tick处理器
    ///
    /// 参数:
    /// - **manager_ptr**: WebUIManager实例指针（作为*anyopaque传入）
    fn tickThreadFn(manager_ptr: *anyopaque) void {
        const self = @as(*WebUIManager, @ptrCast(@alignCast(manager_ptr)));
        // 使用可配置的 tick 间隔（从 WebUIManager 中读取，默认 1000ms）
        const tick_interval_ms = self.tick_interval_ms;
        const sleep_duration: u64 = @intCast(std.time.ns_per_ms * tick_interval_ms);

        logger.global_logger.info("Tick线程已启动，每 {}ms 触发一次 tick", .{tick_interval_ms});

        while (self.is_running.load(.acquire)) {
            // 调用 tick 处理器
            self.extern_param.tick_handler(self.extern_param.ctx, tick_interval_ms);

            // 休眠指定时间
            Thread.sleep(sleep_duration);
        }

        logger.global_logger.info("Tick线程已退出", .{});
    }

    /// 运行应用程序主循环
    ///
    /// 参数:
    /// - **self**: WebUIManager实例指针
    ///
    /// 返回:
    /// - !void: 如果运行失败则返回错误
    ///
    /// 该函数启动WebUI主循环和Tick线程，会阻塞直到收到中断信号
    pub fn run(self: *WebUIManager) !void {
        logger.global_logger.info("启动WebUI主循环...", .{});
        logger.global_logger.info("服务器正在运行，前端将通过子线程驱动 tick 更新", .{});

        // 1. 启动 Tick 线程
        self.is_running.store(true, .release);
        self.tick_thread = try Thread.spawn(
            .{},
            tickThreadFn,
            .{self},
        );
        logger.global_logger.info("Tick线程已生成", .{});

        // 2. 尝试等待 WebUI 事件
        // 由于 show() 失败，这个函数可能会在十几秒后自动返回
        webui_module.wait();

        logger.global_logger.info("WebUI wait() 已返回 (检测到无活跃窗口)", .{});
        logger.global_logger.info(">>> 进入服务器保活模式 <<<", .{});
        logger.global_logger.info("程序将保持运行，直到你按下 Ctrl+C 强制退出", .{});

        // 3. 【核心修复】强制阻塞主线程
        // 只要 tick 线程还在跑（is_running 为 true），join 就会一直卡住，程序就不会退出
        if (self.tick_thread) |thread| {
            thread.join();
            self.tick_thread = null;
        }

        logger.global_logger.info("主循环结束", .{});
    }

    pub fn deinit(self: *WebUIManager) void {
        logger.global_logger.info("WebUIManager.deinit() 开始清理...", .{});

        // 1. 停止Tick线程
        self.is_running.store(false, .release);

        // 2. 等待线程退出
        if (self.tick_thread) |thread| {
            thread.join();
            logger.global_logger.info("Tick线程已回收", .{});
        }

        // 3. 清理WebUI资源
        webui_module.clean();
        logger.global_logger.info("WebUI资源已清理", .{});

        // 4. 释放 html_content
        if (self.html_content) |content| {
            self.allocator.free(content);
            self.html_content = null;
            logger.global_logger.info("HTML内容缓冲已释放", .{});
        }

        logger.global_logger.info("WebUIManager.deinit() 完成", .{});
    }
};

/// "开始"按钮事件处理器
///
/// 参数:
/// - **e**: JavaScript事件
fn handleStart(e: *webui_module.Event) void {
    logger.global_logger.debug("handleStart 被调用", .{});
    // 从事件中获取上下文（WebUIManager实例）
    const manager_ptr = e.getContext() catch {
        logger.global_logger.warn("无法获取上下文", .{});
        return;
    };

    const manager = @as(*WebUIManager, @ptrCast(@alignCast(manager_ptr)));
    // 发送用户开始计时事件到应用程序
    logger.global_logger.debug("发送 user_start_timer 事件", .{});
    manager.handleUserEvent(.{ .clock_event = .{ .user_start_timer = {} } });
}

/// "暂停"按钮事件处理器
///
/// 参数:
/// - **e**: JavaScript事件
fn handlePause(e: *webui_module.Event) void {
    logger.global_logger.debug("handlePause 被调用", .{});
    // 从事件中获取上下文（WebUIManager实例）
    const manager_ptr = e.getContext() catch {
        logger.global_logger.warn("无法获取上下文", .{});
        return;
    };

    const manager = @as(*WebUIManager, @ptrCast(@alignCast(manager_ptr)));
    // 发送用户暂停事件到应用程序
    logger.global_logger.debug("发送 user_pause_timer 事件", .{});
    manager.handleUserEvent(.{ .clock_event = .{ .user_pause_timer = {} } });
}

/// "重置"按钮事件处理器
///
/// 参数:
/// - **e**: JavaScript事件
fn handleReset(e: *webui_module.Event) void {
    logger.global_logger.debug("handleReset 被调用", .{});
    // 从事件中获取上下文（WebUIManager实例）
    const manager_ptr = e.getContext() catch {
        logger.global_logger.warn("无法获取上下文", .{});
        return;
    };

    const manager = @as(*WebUIManager, @ptrCast(@alignCast(manager_ptr)));
    // 发送用户重置事件到应用程序
    logger.global_logger.debug("发送 user_reset_timer 事件", .{});
    manager.handleUserEvent(.{ .clock_event = .{ .user_reset_timer = {} } });
}

/// Tick 事件处理器
///
/// 参数:
/// - **e**: JavaScript事件
fn handleTick(e: *webui_module.Event) void {
    // 从事件中获取上下文（WebUIManager实例）
    const manager_ptr = e.getContext() catch {
        logger.global_logger.warn("Tick: 无法获取上下文", .{});
        return;
    };

    const manager = @as(*WebUIManager, @ptrCast(@alignCast(manager_ptr)));

    // 调用应用程序的 tick 处理器，增量固定为 500ms
    const delta_ms: i64 = 500;
    manager.extern_param.tick_handler(manager.extern_param.ctx, delta_ms);
}

/// "模式切换"事件处理器
///
/// 参数:
/// - **e**: JavaScript事件
fn handleModeChange(e: *webui_module.Event) void {
    logger.global_logger.debug("handleModeChange 被调用", .{});
    // 从事件中获取上下文（WebUIManager实例）
    const manager_ptr = e.getContext() catch {
        logger.global_logger.warn("无法获取上下文", .{});
        return;
    };

    // 从事件参数中获取模式字符串（前端传递的英文名称）
    const new_mode = e.getString();
    logger.global_logger.info("收到模式切换请求: {s}", .{new_mode});

    var mode_enum: interface.ModeEnumT = undefined;
    var new_config: interface.ClockTaskConfig = undefined;

    // 从 global_app 获取设置管理器，以构建默认配置
    if (global_app) |app_ptr| {
        const app = @import("app.zig").MainApplication;
        const main_app = @as(*app, @ptrCast(@alignCast(app_ptr)));
        const settings = &main_app.settings_manager.config;

        // 根据前端传递的英文模式名称匹配
        if (std.mem.eql(u8, new_mode, "countdown")) {
            mode_enum = .COUNTDOWN_MODE;
            new_config = .{
                .default_mode = .COUNTDOWN_MODE,
                .countdown = .{
                    .duration_seconds = settings.clock_defaults.countdown.duration_seconds,
                    .loop = settings.clock_defaults.countdown.loop,
                    .loop_count = settings.clock_defaults.countdown.loop_count,
                    .loop_interval_seconds = settings.clock_defaults.countdown.loop_interval_seconds,
                },
                .stopwatch = settings.clock_defaults.stopwatch,
                .world_clock = .{ .timezone = settings.basic.timezone },
            };
        } else if (std.mem.eql(u8, new_mode, "stopwatch")) {
            mode_enum = .STOPWATCH_MODE;
            new_config = .{
                .default_mode = .STOPWATCH_MODE,
                .countdown = settings.clock_defaults.countdown,
                .stopwatch = .{
                    .max_seconds = settings.clock_defaults.stopwatch.max_seconds,
                },
                .world_clock = .{ .timezone = settings.basic.timezone },
            };
        } else if (std.mem.eql(u8, new_mode, "world_clock")) {
            mode_enum = .WORLD_CLOCK_MODE;
            new_config = .{
                .default_mode = .WORLD_CLOCK_MODE,
                .countdown = settings.clock_defaults.countdown,
                .stopwatch = settings.clock_defaults.stopwatch,
                .world_clock = .{
                    .timezone = settings.basic.timezone,
                },
            };
        } else {
            logger.global_logger.warn("未知的模式: {s}", .{new_mode});
            return;
        }

        const manager = @as(*WebUIManager, @ptrCast(@alignCast(manager_ptr)));
        // 发送模式切换事件
        logger.global_logger.debug("发送 user_change_mode 事件: {}", .{mode_enum});
        manager.handleUserEvent(.{ .clock_event = .{ .user_change_mode = mode_enum } });

        // 发送配置更新事件（真正切换模式）
        logger.global_logger.debug("发送 user_change_config 事件", .{});
        manager.handleUserEvent(.{ .clock_event = .{ .user_change_config = new_config } });

        logger.global_logger.info("✓ 模式已切换到: {}", .{mode_enum});
    } else {
        logger.global_logger.err("无法获取 global_app，模式切换失败", .{});
    }
}

/// "获取设置"事件处理器
///
/// 参数:
/// - **e**: JavaScript事件
fn handleGetSettings(e: *webui_module.Event) void {
    logger.global_logger.debug("handleGetSettings 被调用", .{});

    const manager_ptr = e.getContext() catch {
        logger.global_logger.warn("无法获取上下文", .{});
        return;
    };
    const manager = @as(*WebUIManager, @ptrCast(@alignCast(manager_ptr)));

    // 直接复用 pushSettings 方法，避免重复逻辑validator + presets 可行后再拆 JSON？
    manager.pushSettings();
}

fn handleChangeSettings(e: *webui_module.Event) void {
    logger.global_logger.debug("handleChangeSettings 被调用", .{});
    // 从事件中获取上下文（WebUIManager实例）
    const manager_ptr = e.getContext() catch {
        logger.global_logger.warn("无法获取上下文", .{});
        return;
    };

    const manager = @as(*WebUIManager, @ptrCast(@alignCast(manager_ptr)));
    // 从事件参数中获取新的设置 JSON 字符串
    const new_settings_json = e.getString();

    // 从 global_app 获取应用的 allocator，确保所有权一致
    if (global_app) |app_ptr| {
        const app = @import("app.zig").MainApplication;
        const main_app = @as(*app, @ptrCast(@alignCast(app_ptr)));
        const allocator = main_app.settings_manager.allocator;

        // 使用应用的 allocator 分配（便于应用层后续释放）
        const new_settings_json_buffer = allocator.dupeZ(u8, new_settings_json) catch |err| {
            logger.global_logger.err("复制新设置 JSON 失败: {any}", .{err});
            return;
        };

        logger.global_logger.debug("已分配设置 JSON，长度: {}", .{new_settings_json.len});
        manager.handleUserEvent(.{ .settings_event = .{ .change_settings = new_settings_json_buffer } });
    } else {
        logger.global_logger.err("无法获取 global_app，无法分配内存", .{});
    }
}
