const std = @import("std");
const clock = @import("clock.zig");

// 为了支持不同的UI后端，我们使用一个通用接口
// 这里我们暂时使用GTK作为默认后端，但保留扩展性
// const windows = @import("windows.zig");

/// 全局 App 实例指针，用于回调函数访问
var global_app: ?*MainApplication = null;

/// 主应用程序结构体 - 单向数据流的协调中心
///
/// 数据流向：
/// 1. Windows 发送事件 → handleUserEvent → ClockManager
/// 2. ClockManager 处理逻辑 → update → ClockInterfaceT
/// 3. ClockInterfaceT → Windows.updateDisplay → UI 显示
///
/// 注意：该结构体设计为与窗口管理器实现无关
/// 可以使用GTK、WebUI或其他UI后端
pub const MainApplication = struct {
    clock_manager: clock.ClockManager,

    // 窗口管理器 - 只使用WebUI后端
    windows_manager: union(enum) {
        webui: @import("webui_windows.zig").WebUIManager,
    },

    allocator: std.mem.Allocator,

    /// 初始化应用程序（在指针上原地初始化）
    ///
    /// 参数:
    /// - self: MainApplication实例指针
    /// - allocator: 内存分配器
    /// - clock_config_param: 时钟配置参数
    ///
    /// 返回:
    /// - !void: 如果初始化失败则返回错误
    pub fn init(self: *MainApplication, allocator: std.mem.Allocator, clock_config_param: clock.ClockTaskConfigT) !void {
        // 1. 创建时钟配置
        const clock_config = clock_config_param;

        // 2. 初始化时钟管理器
        self.clock_manager = clock.ClockManager.init(clock_config);

        // 3. 初始化窗口管理器（只使用WebUI）
        self.windows_manager = .{ .webui = undefined };
        try self.windows_manager.webui.init(handleUserEventWrapper, .{ .ctx = self, .tick_handler = MainApplication.tick });

        self.allocator = allocator;

        // 4. 设置全局 App 指针，供回调函数访问
        self.setGlobalApp();

        // 5. 立即更新显示，显示初始时间
        const initial_display = self.clock_manager.update();
        self.updateDisplay(initial_display);
    }

    /// 设置全局 App 指针供回调函数使用
    /// 必须在 init() 后，run() 前调用
    pub fn setGlobalApp(self: *MainApplication) void {
        global_app = self;
    }

    /// 运行应用程序主循环
    ///
    /// 参数:
    /// - self: MainApplication实例指针
    ///
    /// 返回:
    /// - !void: 如果运行失败则返回错误
    ///
    /// 注意：目前WebUI会接管主循环
    /// 未来可以考虑使用WebUI的定时器来实现tick
    pub fn run(self: *MainApplication) !void {
        // 启动WebUI主循环
        return self.windows_manager.webui.run();
    }

    /// 应用程序 tick - 更新逻辑并刷新显示
    /// 应该每帧调用一次（例如 60 FPS = 16ms）
    ///
    /// 参数:
    /// - ctx: 应用程序上下文指针
    /// - delta_ms: 自上次tick以来经过的毫秒数
    pub fn tick(ctx: ?*anyopaque, delta_ms: i64) void {
        // 1. 获取应用程序实例
        const self: *MainApplication = @ptrCast(@alignCast(ctx));

        // 2. 发送 tick 事件给时钟
        self.clock_manager.handleEvent(.{ .tick = delta_ms });

        // 3. 获取时钟的显示数据（无需分配，返回内部指针）
        const display_data = self.clock_manager.update();

        // 4. 更新窗口显示
        self.updateDisplay(display_data);
    }

    /// 更新显示 - 调用WebUI更新方法
    ///
    /// 参数:
    /// - self: MainApplication实例指针
    /// - display_data: 时钟显示数据
    fn updateDisplay(self: *MainApplication, display_data: *clock.ClockInterfaceT) void {
        self.windows_manager.webui.updateDisplay(display_data);
    }

    /// 处理来自用户界面的事件（回调函数）
    /// 这是事件向上冒泡的入口点
    ///
    /// 参数:
    /// - self: MainApplication实例指针（未使用，通过全局变量访问）
    /// - event: 时钟事件
    fn handleUserEvent(_: *MainApplication, event: clock.ClockEvent) void {
        if (global_app) |app| {
            // 将用户事件转发给时钟管理器
            app.clock_manager.handleEvent(event);

            // 立即更新显示
            const display_data = app.clock_manager.update();
            app.updateDisplay(display_data);
        }
    }

    /// 事件处理器包装函数 - 适配窗口管理器期望的函数签名
    ///
    /// 参数:
    /// - event: 时钟事件
    fn handleUserEventWrapper(event: clock.ClockEvent) void {
        if (global_app) |app| {
            app.handleUserEvent(event);
        }
    }
};
