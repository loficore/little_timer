const std = @import("std");
const interface = @import("interface.zig");

// 检查是否定义了USE_WEBUI
const use_webui = @hasDecl(@import("root"), "USE_WEBUI") and @import("root").USE_WEBUI;

// 根据是否使用WebUI导入不同的模块
const webui_module = if (use_webui) @import("webui") else struct {};

// Tick函数类型定义 - 用于处理时钟tick事件
const TickFn = *const fn (ctx: ?*anyopaque, delta_ms: i64) void;

// 用户事件类型别名 - 与interface.zig中的ClockEvent一致
const UserEventT = interface.ClockEvent;

// 外部参数结构体 - 用于传递上下文和tick处理器
const ExternParam = struct {
    ctx: ?*anyopaque, // 应用程序上下文指针
    tick_handler: TickFn, // tick事件处理器
};

// 常量定义
const Constants = struct {
    pub const APP_TITLE = "Little Timer - WebUI";
    pub const WINDOW_WIDTH = 400;
    pub const WINDOW_HEIGHT = 300;
};

/// WebUI窗口管理器 - 负责WebUI界面显示和用户事件收集
///
/// - **note** : 该结构体替代了原来的GTK WindowsManager，提供相同的接口
/// 但使用WebUI作为UI后端，实现跨平台的Web界面
pub const WebUIManager = if (use_webui) struct {
    // WebUI窗口实例
    window: webui_module,

    // 事件回调函数指针 - 用于将用户事件传递给应用程序
    on_user_event: ?*const fn (UserEventT) void,

    // 外部参数 - 保存应用程序上下文和tick处理器
    extern_param: ExternParam,

    // 窗口是否已初始化
    is_initialized: bool = false,

    /// 初始化WebUI窗口管理器
    ///
    /// 参数:
    /// - self: WebUIManager实例指针
    /// - on_user_event_param: 用户事件回调函数
    /// - extern_param: 外部参数（包含应用程序上下文和tick处理器）
    ///
    /// 返回:
    /// - !void: 如果初始化失败则返回错误
    pub fn init(self: *WebUIManager, on_user_event_param: ?*const fn (UserEventT) void, extern_param: ExternParam) !void {
        // 创建新的WebUI窗口
        self.window = webui_module.newWindow();

        // 保存用户事件回调函数
        self.on_user_event = on_user_event_param;

        // 保存外部参数
        self.extern_param = extern_param;

        // 设置HTML内容
        const html_content = try self.getHTMLContent();

        // 显示窗口
        // 使用全局分配器
        var gpa = std.heap.GeneralPurposeAllocator(.{}){};
        defer _ = gpa.deinit();
        const temp_allocator = gpa.allocator();

        // 将html_content转换为null终止字符串
        const null_terminated_html = try std.mem.concat(temp_allocator, u8, &.{ html_content, "" });
        defer temp_allocator.free(null_terminated_html);

        _ = try self.window.show(null_terminated_html);

        // 设置JavaScript回调函数，用于处理来自UI的事件
        self.setupEventHandlers();

        // 标记为已初始化
        self.is_initialized = true;

        std.debug.print("WebUI窗口管理器初始化完成\n", .{});
    }

    /// 获取HTML内容
    /// - **param** : *self** 暂时不使用
    /// - **return** : **![]const u8**: HTML内容字符串，如果读取失败则返回错误
    fn getHTMLContent(self: *WebUIManager) ![]const u8 {
        _ = self;

        // 获取当前可执行文件的路径，然后构建HTML文件的路径
        var gpa = std.heap.GeneralPurposeAllocator(.{}){};
        defer _ = gpa.deinit();
        const allocator = gpa.allocator();

        // 尝试获取可执行文件路径，然后构建assets路径
        // 如果无法获取可执行文件路径，则尝试使用相对路径
        const html_content = std.fs.cwd().readFileAlloc(allocator, "assets/index.html", 1024 * 1024) catch {
            // 如果相对路径失败，尝试从当前工作目录的父目录读取
            return std.fs.cwd().readFileAlloc(allocator, "../assets/index.html", 1024 * 1024) catch |err| {
                std.debug.print("无法读取HTML文件: {any}\n", .{err});
                // 如果文件读取失败，返回一个简单的错误页面
                return allocator.dupe(u8,
                    \\<!DOCTYPE html>
                    \\<html>
                    \\<head><title>错误</title></head>
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
    /// 配置JavaScript回调函数，用于处理来自UI的用户事件
    fn setupEventHandlers(self: *WebUIManager) void {
        // 注册JavaScript函数，用于处理UI事件
        // 这些函数将被HTML中的按钮点击事件调用
        _ = self.window.bind("start", handleStart);
        _ = self.window.bind("pause", handlePause);
        _ = self.window.bind("reset", handleReset);

        // 为每个绑定设置上下文，这样回调函数可以访问WebUIManager实例
        self.window.setContext("start", self);
        self.window.setContext("pause", self);
        self.window.setContext("reset", self);
    }

    /// 更新显示（应用程序每帧调用）
    ///
    /// 参数:
    /// - self: WebUIManager实例指针
    /// - display_data: 时钟显示数据
    ///
    /// 该函数接收来自ClockManager的显示数据，并更新UI显示
    pub fn updateDisplay(self: *WebUIManager, display_data: *interface.ClockInterfaceT) void {
        // 从显示数据中获取剩余时间（秒）
        const remaining_seconds = display_data.getTimeInfo();

        // 根据显示数据更新时间字符串
        const hours = @divTrunc(remaining_seconds, 3600);
        const minutes = @divTrunc(@rem(remaining_seconds, 3600), 60);
        const seconds = @rem(remaining_seconds, 60);

        // 格式化为 "HH:MM:SS" 格式
        var time_string_buffer: [9]u8 = undefined;
        _ = std.fmt.bufPrint(&time_string_buffer, "{:0>2}:{:0>2}:{:0>2}", .{ hours, minutes, seconds }) catch {
            // 如果格式化失败，使用默认值
            @memcpy(&time_string_buffer, "00:00:00\x00");
        };

        // 通过JavaScript更新UI显示
        // 将时间字符串发送到前端进行显示更新
        // 使用临时分配器
        var gpa = std.heap.GeneralPurposeAllocator(.{}){};
        defer _ = gpa.deinit();
        const temp_allocator = gpa.allocator();

        const js_code_content = try std.fmt.allocPrint(temp_allocator, "document.getElementById('time').textContent = '{s}';", .{time_string_buffer});
        const js_code = try std.mem.concat(temp_allocator, u8, &.{ js_code_content, "" });
        defer temp_allocator.free(js_code);
        defer temp_allocator.free(js_code_content);

        self.window.run(js_code);

        // 根据模式更新模式指示器
        const mode_text = switch (display_data.getMode()) {
            .COUNTDOWN_MODE => "倒计时模式",
            .STOPWATCH_MODE => "正计时模式",
            .WORLD_CLOCK_MODE => "世界时钟模式",
        };

        const mode_js_content = try std.fmt.allocPrint(temp_allocator, "document.getElementById('mode').textContent = '{s}';", .{mode_text});
        const mode_js_code = try std.mem.concat(temp_allocator, u8, &.{ mode_js_content, "" });
        defer temp_allocator.free(mode_js_code);
        defer temp_allocator.free(mode_js_content);

        self.window.run(mode_js_code);
    }

    /// 处理用户事件
    ///
    /// - **param**: **self**   WebUIManager实例指针
    /// - **param**: **event**  用户事件
    ///
    /// - **note** : 该函数处理来自UI的用户事件，并将其转发给应用程序
    fn handleUserEvent(self: *WebUIManager, event: UserEventT) void {
        if (self.on_user_event) |handler| {
            handler(event);
        } else {
            std.debug.print("警告: 未设置用户事件处理器\n", .{});
        }
    }

    /// 运行应用程序主循环
    ///
    /// 参数:
    /// - **self**: WebUIManager实例指针
    ///
    /// 返回:
    /// - !void: 如果运行失败则返回错误
    ///
    /// 该函数启动WebUI主循环，会阻塞直到所有窗口关闭
    pub fn run(self: *WebUIManager) !void {
        if (!self.is_initialized) {
            return error.WebUIManagerNotInitialized;
        }

        std.debug.print("启动WebUI主循环...\n", .{});

        // 阻塞直到所有窗口关闭
        webui_module.wait();

        std.debug.print("WebUI主循环结束\n", .{});
    }

    /// 清理资源
    ///
    /// 参数:
    /// - self: WebUIManager实例指针
    ///
    /// 该函数清理WebUI相关资源
    pub fn deinit(self: *WebUIManager) void {
        // 清理WebUI资源
        webui_module.clean();
        self.is_initialized = false;
    }
} else struct {
    // 当不使用WebUI时，提供空实现
    _dummy: u8 = 0,

    pub fn init(self: *WebUIManager, on_user_event_param: ?*const fn (UserEventT) void, extern_param: ExternParam) !void {
        _ = self;
        _ = on_user_event_param;
        _ = extern_param;
        @panic("WebUI未启用");
    }

    pub fn updateDisplay(self: *WebUIManager, display_data: *interface.ClockInterfaceT) void {
        _ = self;
        _ = display_data;
    }

    pub fn run(self: *WebUIManager) !void {
        _ = self;
        @panic("WebUI未启用");
    }

    pub fn deinit(self: *WebUIManager) void {
        _ = self;
    }
};

// 定义事件处理器函数 - 这些函数在编译时总是存在，但只有在使用WebUI时才被调用
/// "开始"按钮事件处理器
///
/// 这是一个静态函数，用于处理来自JavaScript的"开始"事件
/// 它会将用户事件转发给WebUIManager实例
fn handleStart(e: if (use_webui) *webui_module.Event else void) void {
    if (use_webui) {
        // 从事件中获取上下文（WebUIManager实例）
        const manager_ptr = e.getContext() catch {
            std.debug.print("无法获取上下文\n", .{});
            return;
        };

        const manager = @as(*WebUIManager, @ptrCast(manager_ptr));

        // 发送用户开始计时事件到应用程序
        manager.handleUserEvent(.{ .user_start_timer = {} });
    }
}

/// "暂停"按钮事件处理器
///
/// 这是一个静态函数，用于处理来自JavaScript的"暂停"事件
fn handlePause(e: if (use_webui) *webui_module.Event else void) void {
    if (use_webui) {
        // 从事件中获取上下文（WebUIManager实例）
        const manager_ptr = e.getContext() catch {
            std.debug.print("无法获取上下文\n", .{});
            return;
        };

        const manager = @as(*WebUIManager, @ptrCast(manager_ptr));

        // 发送用户暂停事件到应用程序
        manager.handleUserEvent(.{ .user_pause_timer = {} });
    }
}

/// "重置"按钮事件处理器
///
/// 这是一个静态函数，用于处理来自JavaScript的"重置"事件
fn handleReset(e: if (use_webui) *webui_module.Event else void) void {
    if (use_webui) {
        // 从事件中获取上下文（WebUIManager实例）
        const manager_ptr = e.getContext() catch {
            std.debug.print("无法获取上下文\n", .{});
            return;
        };

        const manager = @as(*WebUIManager, @ptrCast(manager_ptr));

        // 发送用户重置事件到应用程序
        manager.handleUserEvent(.{ .user_reset_timer = {} });
    }
}
