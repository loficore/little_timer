const std = @import("std");
const interface = @import("interface.zig");
const Thread = std.Thread;

// 根据是否使用WebUI导入不同的模块
const webui_module = @import("webui");

// Tick函数类型定义
const TickFn = *const fn (ctx: ?*anyopaque, delta_ms: i64) void;

const UserEventT = interface.ClockEvent;

const ExternParam = struct {
    ctx: ?*anyopaque,
    tick_handler: TickFn,
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
    on_user_event: ?*const fn (UserEventT) void,

    // 外部参数 - 保存应用程序上下文和tick处理器
    extern_param: ExternParam,

    // 窗口是否已初始化
    is_initialized: bool = false,

    // 缓存的上一次显示的时间（秒），用于避免重复更新
    last_displayed_seconds: i64 = -1,

    // 缓存的上一次显示的模式
    last_displayed_mode: interface.ModeEnumT = .COUNTDOWN_MODE,

    // 线程控制 - 用于停止 tick 线程
    is_running: std.atomic.Value(bool) = std.atomic.Value(bool).init(false),

    // Tick线程句柄 - 用于管理线程生命周期
    tick_thread: ?std.Thread = null,

    /// 初始化WebUI窗口管理器
    ///
    /// 参数:
    /// - **self**: WebUIManager实例指针
    /// - **on_user_event_param**: 用户事件回调函数
    /// - **extern_param**: 外部参数（包含应用程序上下文和tick处理器）
    ///
    /// 返回:
    /// - !void: 如果初始化失败则返回错误
    pub fn init(self: *WebUIManager, on_user_event_param: ?*const fn (UserEventT) void, extern_param: ExternParam) !void {
        self.window = webui_module.newWindow();
        self.on_user_event = on_user_event_param;
        self.extern_param = extern_param;

        // 在运行时读取 HTML 文件内容
        // 尝试多个可能的路径
        const possible_paths = [_][]const u8{
            "assets/index.html", // 从项目根目录运行
            "../assets/index.html", // 从 zig-out/bin 运行
        };

        var html_content_raw: []u8 = undefined;
        var html_found = false;
        var gpa = std.heap.GeneralPurposeAllocator(.{}){};
        const allocator = gpa.allocator();

        for (possible_paths) |path| {
            html_content_raw = std.fs.cwd().readFileAlloc(allocator, path, 1024 * 1024) catch |err| {
                std.debug.print("无法从 {s} 读取: {any}\n", .{ path, err });
                continue;
            };
            std.debug.print("✓ 从 {s} 读取 HTML ({} 字节)\n", .{ path, html_content_raw.len });
            html_found = true;
            break;
        }

        if (!html_found) {
            std.debug.print("错误: 无法找到 HTML 文件\n", .{});
            return error.HTMLFileNotFound;
        }

        // 将内容转换为 null 终止的字符串，show() 方法需要这个类型
        const html_content = try allocator.dupeZ(u8, html_content_raw);
        defer allocator.free(html_content_raw); // 释放原始内容

        // 使用 show() 方法显示 HTML 内容
        // 这会自动启动服务器并提供 /webui.js
        self.window.show(html_content) catch |err| {
            std.debug.print("show() 失败: {any}\n", .{err});
            std.debug.print("注意: show() 方法在某些环境下可能不可用\n", .{});
            std.debug.print("但服务器应该已经启动，请尝试访问下面的地址\n", .{});
        };

        // 获取服务器信息
        const port = self.window.getPort() catch |err| blk: {
            std.debug.print("警告: 获取端口失败: {any}\n", .{err});
            break :blk 0;
        };

        // getUrl() 可能返回错误，需要特殊处理
        const url_str = self.window.getUrl() catch |err| blk: {
            std.debug.print("警告: 获取URL失败: {any}\n", .{err});
            break :blk "unknown";
        };

        std.debug.print("\n" ++
            "========================================\n" ++
            "WebUI 服务器已启动！\n" ++
            "========================================\n" ++
            "访问地址: http://localhost:{d}\n" ++
            "完整URL: {s}\n" ++
            "========================================\n" ++
            "请在浏览器中打开上面的地址\n" ++
            "按 Ctrl+C 停止服务器\n" ++
            "========================================\n\n", .{ port, url_str });

        // 设置JavaScript回调函数，用于处理来自UI的事件
        self.setupEventHandlers();

        // 标记为已初始化
        self.is_initialized = true;

        std.debug.print("WebUI窗口管理器初始化完成\n", .{});
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
                std.debug.print("无法读取HTML文件: {any}\n", .{err});
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
        std.debug.print("开始绑定事件处理器...\n", .{});

        _ = self.window.bind("start", handleStart) catch |err| {
            std.debug.print("绑定 start 失败: {any}\n", .{err});
            return;
        };
        std.debug.print("✓ start 绑定成功\n", .{});

        _ = self.window.bind("pause", handlePause) catch |err| {
            std.debug.print("绑定 pause 失败: {any}\n", .{err});
            return;
        };
        std.debug.print("✓ pause 绑定成功\n", .{});

        _ = self.window.bind("reset", handleReset) catch |err| {
            std.debug.print("绑定 reset 失败: {any}\n", .{err});
            return;
        };
        std.debug.print("✓ reset 绑定成功\n", .{});

        _ = self.window.bind("tick", handleTick) catch |err| {
            std.debug.print("绑定 tick 失败: {any}\n", .{err});
            return;
        };
        std.debug.print("✓ tick 绑定成功\n", .{});

        _ = self.window.bind("mode_change", handleModeChange) catch |err| {
            std.debug.print("绑定 mode_change 失败: {any}\n", .{err});
            return;
        };
        std.debug.print("✓ mode_change 绑定成功\n", .{});

        // 为每个绑定设置上下文，这样回调函数可以访问WebUIManager实例
        self.window.setContext("start", self);
        std.debug.print("✓ start 上下文设置成功\n", .{});

        self.window.setContext("pause", self);
        std.debug.print("✓ pause 上下文设置成功\n", .{});

        self.window.setContext("reset", self);
        std.debug.print("✓ reset 上下文设置成功\n", .{});

        self.window.setContext("tick", self);
        std.debug.print("✓ tick 上下文设置成功\n", .{});

        self.window.setContext("mode_change", self);
        std.debug.print("✓ mode_change 上下文设置成功\n", .{});

        std.debug.print("事件绑定已注册\n", .{});
    }

    /// 更新显示
    ///
    /// 参数:
    /// - **self**: WebUIManager实例指针
    /// - **display_data**: 时钟显示数据
    pub fn updateDisplay(self: *WebUIManager, display_data: *interface.ClockInterface) !void {
        // 从显示数据中获取剩余时间（秒）
        const remaining_seconds = display_data.getTimeInfo();
        const current_mode = display_data.getMode();

        // 只有在时间或模式改变时才更新显示，避免过频繁的JavaScript执行
        if (remaining_seconds == self.last_displayed_seconds and current_mode == self.last_displayed_mode) {
            return; // 没有改变，跳过更新
        }

        // 转换为无符号整数以便正确格式化
        const abs_seconds = if (remaining_seconds < 0) 0 else @as(u64, @intCast(remaining_seconds));

        // 根据显示数据更新时间字符串
        const hours = @divTrunc(abs_seconds, 3600);
        const minutes = @divTrunc(@rem(abs_seconds, 3600), 60);
        const seconds = @rem(abs_seconds, 60);

        // 格式化为 "HH:MM:SS" 格式
        // 缓冲区大小：8个字符 + 1个null终止符 = 9字节，留足空间避免溢出
        var time_string_buffer: [10]u8 = undefined;
        const time_string = try std.fmt.bufPrint(&time_string_buffer, "{:0>2}:{:0>2}:{:0>2}", .{ hours, minutes, seconds });

        std.debug.print("updateDisplay: 时间 = {s}\n", .{time_string});

        // 通过JavaScript更新UI显示
        // 使用run()执行JavaScript
        var js_code_buffer: [256:0]u8 = undefined; //JS代码很短，所以直接用栈缓冲区
        // 创建JavaScript代码 - 同时更新时间和模式，在一个JS调用中完成
        const mode_text = switch (current_mode) {
            .COUNTDOWN_MODE => "倒计时模式",
            .STOPWATCH_MODE => "正计时模式",
            .WORLD_CLOCK_MODE => "世界时钟模式",
        };

        const js_code_content = try std.fmt.bufPrintZ(&js_code_buffer, "document.getElementById('time').textContent='{s}';document.getElementById('mode').textContent='{s}';", .{ time_string, mode_text });

        std.debug.print("  执行 JS: {s}\n", .{js_code_content});
        // 使用run()执行JavaScript
        self.window.run(js_code_content);
        // 更新缓存的显示值
        self.last_displayed_seconds = remaining_seconds;
        self.last_displayed_mode = current_mode;
    }

    /// 处理用户事件
    ///
    /// 参数:
    /// - **self**: WebUIManager实例指针
    /// - **event**: 用户事件
    fn handleUserEvent(self: *WebUIManager, event: UserEventT) void {
        std.debug.print("WebUIManager.handleUserEvent 被调用，on_user_event 是否为 null: {}\n", .{self.on_user_event == null});
        if (self.on_user_event) |handler| {
            std.debug.print("  调用用户事件处理器\n", .{});
            handler(event);
        } else {
            std.debug.print("警告: 未设置用户事件处理器\n", .{});
        }
    }

    /// Tick线程函数 - 在子线程中周期性调用tick处理器
    ///
    /// 参数:
    /// - **manager_ptr**: WebUIManager实例指针（作为*anyopaque传入）
    fn tickThreadFn(manager_ptr: *anyopaque) void {
        const self = @as(*WebUIManager, @ptrCast(@alignCast(manager_ptr)));
        const tick_interval_ms = 500; // 500ms 调用一次 tick
        const sleep_duration = std.time.ns_per_ms * tick_interval_ms;

        std.debug.print("Tick线程已启动，每 {}ms 触发一次 tick\n", .{tick_interval_ms});

        while (self.is_running.load(.acquire)) {
            // 调用 tick 处理器
            self.extern_param.tick_handler(self.extern_param.ctx, tick_interval_ms);

            // 休眠指定时间
            Thread.sleep(sleep_duration);
        }

        std.debug.print("Tick线程已退出\n", .{});
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
        if (!self.is_initialized) {
            return error.WebUIManagerNotInitialized;
        }

        std.debug.print("启动WebUI主循环...\n", .{});
        std.debug.print("服务器正在运行，前端将通过子线程驱动 tick 更新\n", .{});

        // 1. 启动 Tick 线程
        self.is_running.store(true, .release);
        self.tick_thread = try Thread.spawn(
            .{},
            tickThreadFn,
            .{self},
        );
        std.debug.print("Tick线程已生成\n", .{});

        // 2. 尝试等待 WebUI 事件
        // 由于 show() 失败，这个函数可能会在十几秒后自动返回
        webui_module.wait();

        std.debug.print("WebUI wait() 已返回 (检测到无活跃窗口)\n", .{});
        std.debug.print(">>> 进入服务器保活模式 <<<\n", .{});
        std.debug.print("程序将保持运行，直到你按下 Ctrl+C 强制退出\n", .{});

        // 3. 【核心修复】强制阻塞主线程
        // 只要 tick 线程还在跑（is_running 为 true），join 就会一直卡住，程序就不会退出
        if (self.tick_thread) |thread| {
            thread.join();
            self.tick_thread = null;
        }

        // 下面的代码通常只有在 is_running 被设为 false 后才会执行
        // 但在这个服务器模式下，通常是用户直接杀掉进程，所以甚至不需要执行到这里
        webui_module.clean();
        std.debug.print("主循环结束\n", .{});
    }

    pub fn deinit(self: *WebUIManager) void {
        // 停止Tick线程
        self.is_running.store(false, .release);

        // 等待线程退出
        if (self.tick_thread) |thread| {
            thread.join();
            std.debug.print("Tick线程已回收\n", .{});
        }
    }
};

/// "开始"按钮事件处理器
///
/// 参数:
/// - **e**: JavaScript事件
fn handleStart(e: *webui_module.Event) void {
    std.debug.print("handleStart 被调用\n", .{});
    // 从事件中获取上下文（WebUIManager实例）
    const manager_ptr = e.getContext() catch {
        std.debug.print("无法获取上下文\n", .{});
        return;
    };

    const manager = @as(*WebUIManager, @ptrCast(@alignCast(manager_ptr)));
    // 发送用户开始计时事件到应用程序
    std.debug.print("发送 user_start_timer 事件\n", .{});
    manager.handleUserEvent(.{ .user_start_timer = {} });
}

/// "暂停"按钮事件处理器
///
/// 参数:
/// - **e**: JavaScript事件
fn handlePause(e: *webui_module.Event) void {
    std.debug.print("handlePause 被调用\n", .{});
    // 从事件中获取上下文（WebUIManager实例）
    const manager_ptr = e.getContext() catch {
        std.debug.print("无法获取上下文\n", .{});
        return;
    };

    const manager = @as(*WebUIManager, @ptrCast(@alignCast(manager_ptr)));
    // 发送用户暂停事件到应用程序
    std.debug.print("发送 user_pause_timer 事件\n", .{});
    manager.handleUserEvent(.{ .user_pause_timer = {} });
}

/// "重置"按钮事件处理器
///
/// 参数:
/// - **e**: JavaScript事件
fn handleReset(e: *webui_module.Event) void {
    std.debug.print("handleReset 被调用\n", .{});
    // 从事件中获取上下文（WebUIManager实例）
    const manager_ptr = e.getContext() catch {
        std.debug.print("无法获取上下文\n", .{});
        return;
    };

    const manager = @as(*WebUIManager, @ptrCast(@alignCast(manager_ptr)));
    // 发送用户重置事件到应用程序
    std.debug.print("发送 user_reset_timer 事件\n", .{});
    manager.handleUserEvent(.{ .user_reset_timer = {} });
}

/// Tick 事件处理器
///
/// 参数:
/// - **e**: JavaScript事件
fn handleTick(e: *webui_module.Event) void {
    // 从事件中获取上下文（WebUIManager实例）
    const manager_ptr = e.getContext() catch {
        std.debug.print("Tick: 无法获取上下文\n", .{});
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
    std.debug.print("handleModeChange 被调用\n", .{});
    // 从事件中获取上下文（WebUIManager实例）
    const manager_ptr = e.getContext() catch {
        std.debug.print("无法获取上下文\n", .{});
        return;
    };

    // 从事件参数中获取模式字符串
    const new_mode = e.getString();
    var mode_enum: interface.ModeEnumT = undefined;

    if (std.mem.eql(u8, new_mode, "倒计时模式")) {
        mode_enum = .COUNTDOWN_MODE;
    } else if (std.mem.eql(u8, new_mode, "正计时模式")) {
        mode_enum = .STOPWATCH_MODE;
    } else if (std.mem.eql(u8, new_mode, "世界时钟模式")) {
        mode_enum = .WORLD_CLOCK_MODE;
    } else {
        std.debug.print("未知的模式: {s}\n", .{new_mode});
        return;
    }

    const manager = @as(*WebUIManager, @ptrCast(@alignCast(manager_ptr)));
    // 发送用户模式切换事件到应用程序
    std.debug.print("发送 user_change_mode 事件\n", .{});
    manager.handleUserEvent(.{ .user_change_mode = mode_enum });
}
