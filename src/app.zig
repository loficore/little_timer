const std = @import("std");
const clock = @import("clock.zig");

/// 全局 App 实例指针，用于回调函数访问
var global_app: ?*MainApplication = null;

/// 主应用程序结构体 - 单向数据流的协调中心
pub const MainApplication = struct {
    clock_manager: clock.ClockManager,
    mutex: std.Thread.Mutex = .{},
    webui: @import("webui_windows.zig").WebUIManager,
    allocator: std.mem.Allocator,

    /// 初始化应用程序（在指针上原地初始化）
    ///
    /// 参数:
    /// - **self**: MainApplication实例指针
    /// - **allocator**: 内存分配器
    /// - **clock_config_param**: 时钟配置参数
    ///
    /// 返回:
    /// - !void: 如果初始化失败则返回错误
    pub fn init(self: *MainApplication, allocator: std.mem.Allocator, clock_config_param: clock.ClockTaskConfig) !void {
        self.clock_manager = clock.ClockManager.init(clock_config_param);
        try self.webui.init(handleUserEventWrapper, .{ .ctx = self, .tick_handler = MainApplication.tick });
        self.allocator = allocator;
        self.setGlobalApp();

        const initial_display = self.clock_manager.update();
        self.updateDisplay(initial_display) catch |err| {
            std.debug.print("更新显示失败: {any}\n", .{err});
        };
    }

    /// 设置全局 App 指针供回调函数使用
    pub fn setGlobalApp(self: *MainApplication) void {
        global_app = self;
    }

    /// 运行应用程序主循环
    pub fn run(self: *MainApplication) !void {
        return self.webui.run();
    }

    /// 应用程序 tick - 更新逻辑并刷新显示
    ///
    /// 参数:
    /// - **ctx**: 应用程序上下文指针
    /// - **delta_ms**: 自上次tick以来经过的毫秒数
    pub fn tick(ctx: ?*anyopaque, delta_ms: i64) void {
        const self: *MainApplication = @ptrCast(@alignCast(ctx));

        self.mutex.lock();
        defer self.mutex.unlock();

        std.debug.print("Tick: 增量 {} ms\n", .{delta_ms});

        self.clock_manager.handleEvent(.{ .tick = delta_ms });
        const display_data = self.clock_manager.update();
        self.updateDisplay(display_data) catch |err| {
            std.debug.print("更新显示失败: {any}\n", .{err});
        };
    }

    /// 更新显示
    ///
    /// 参数:
    /// - **self**: MainApplication实例指针
    /// - **display_data**: 时钟显示数据
    fn updateDisplay(self: *MainApplication, display_data: *clock.ClockInterface) !void {
        try self.webui.updateDisplay(display_data);
    }

    /// 处理来自用户界面的事件
    ///
    /// 参数:
    /// - **self**: MainApplication实例指针
    /// - **event**: 时钟事件
    fn handleUserEvent(self: *MainApplication, event: clock.ClockEvent) void {
        if (global_app) |app| {
            self.mutex.lock();
            defer self.mutex.unlock();

            app.clock_manager.handleEvent(event);
            const display_data = app.clock_manager.update();
            app.updateDisplay(display_data) catch |err| {
                std.debug.print("更新显示失败: {any}\n", .{err});
            };
        }
    }

    /// 事件处理器包装函数
    ///
    /// 参数:
    /// - **event**: 时钟事件
    fn handleUserEventWrapper(event: clock.ClockEvent) void {
        std.debug.print("handleUserEventWrapper 被调用，事件类型: {}\n", .{event});
        if (global_app) |app| {
            std.debug.print("  调用 app.handleUserEvent\n", .{});
            app.handleUserEvent(event);
        } else {
            std.debug.print("  错误: global_app 为 null\n", .{});
        }
    }
};
