const std = @import("std");
const app = @import("app.zig");
const logger = @import("logger.zig");
const builtin = @import("builtin");

const webui_windows = @import("webui_windows.zig");
const clock = @import("clock.zig");
const settings = @import("settings.zig");
const interface = @import("interface.zig");
const error_recovery = @import("error_recovery.zig");

const EventThrottle = struct {
    last_event_time: i64 = 0,
    last_event_type: ?interface.EventType = null,
    debounce_ms: i64 = 100,
};

/// 全局 App 实例指针，用于回调函数访问
var global_app: ?*MainApplication = null;

pub const MainApplication = struct {
    clock_manager: clock.ClockManager,
    settings_manager: settings.SettingsManager,
    mutex: std.Thread.Mutex = .{},
    webui: webui_windows.WebUIManager,
    allocator: std.mem.Allocator,
    /// 错误恢复管理器 - 用于处理和追踪错误
    error_recovery: error_recovery.ErrorRecoveryManager,
    /// 事件去抖限流器 - 防止快速连续点击
    event_throttle: EventThrottle = .{},

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

        // 初始化和加载设置
        self.settings_manager = try settings.SettingsManager.init(allocator, "settings.toml");
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
        self.configureLogger();

        // 根据设置构建时钟配置
        const clock_config = self.settings_manager.buildClockConfig();

        self.clock_manager = clock.ClockManager.init(
            clock_config,
        );

        // 初始化 WebUI - 现在需要传入 allocator
        try self.webui.init(handleUserEventWrapper, .{ .ctx = self, .tick_handler = MainApplication.tick }, allocator);

        // 从设置中配置 tick 间隔（默认 1000ms，范围 100-5000ms）
        const tick_interval = self.settings_manager.config.logging.tick_interval_ms;
        if (tick_interval >= 100 and tick_interval <= 5000) {
            self.webui.tick_interval_ms = tick_interval;
            logger.global_logger.info("✓ Tick 间隔已配置为 {}ms (从 settings.toml 读取)", .{tick_interval});
        } else {
            logger.global_logger.warn("⚠️ Tick 间隔配置无效 {}ms，使用默认值 1000ms", .{tick_interval});
            self.webui.tick_interval_ms = 1000;
        }

        // 读取设置中的时区（用于前端世界时钟和状态同步）
        self.webui.setTimezone(self.settings_manager.config.basic.timezone);

        self.setGlobalApp();

        // 更新初始显示
        const initial_display = self.clock_manager.update();
        self.updateDisplay(initial_display) catch |err| {
            logger.global_logger.err("初始化时更新显示失败: {any}", .{err});
            self.error_recovery.recordError("初始化时更新显示失败", "DISPLAY_UPDATE");
        };
    }

    /// 设置全局 App 指针供回调函数使用
    pub fn setGlobalApp(self: *MainApplication) void {
        global_app = self;
        webui_windows.setGlobalApp(@ptrCast(self));
    }

    /// 配置日志系统
    ///
    /// 参数:
    /// - **self**: MainApplication实例指针
    fn configureLogger(self: *MainApplication) void {
        // 从设置中读取日志配置
        const log_config = &self.settings_manager.config.logging;

        // 解析日志等级
        const log_level = logger.LogLevel.fromString(log_config.level) orelse .INFO;

        // 配置全局logger
        logger.global_logger.current_level = log_level;
        logger.global_logger.enable_timestamp = log_config.enable_timestamp;

        logger.global_logger.info("日志系统已初始化 (等级: {s}, 时间戳: {})", .{
            log_level.toString(),
            log_config.enable_timestamp,
        });
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
    }

    /// 更新显示
    ///
    /// 参数:
    /// - **self**: MainApplication实例指针
    /// - **display_data**: 时钟显示数据
    fn updateDisplay(self: *MainApplication, display_data: *clock.ClockState) !void {
        try self.webui.updateDisplay(display_data);
    }

    /// 处理来自用户界面的事件
    ///
    /// 参数:
    /// - **self**: MainApplication实例指针
    /// - **event**: 事件类型
    fn handleUserEvent(self: *MainApplication, event: interface.EventType) void {
        if (global_app) |global_app_instance| {
            self.mutex.lock();
            defer self.mutex.unlock();

            const app_instance = @as(*MainApplication, @ptrCast(@alignCast(global_app_instance)));
            switch (event) {
                .clock_event => |clock_event| {
                    logger.global_logger.debug("处理时钟事件: {}", .{clock_event});
                    app_instance.clockHandle(clock_event) catch |err| {
                        logger.global_logger.err("处理时钟事件失败: {any}", .{err});
                        self.error_recovery.recordError("处理时钟事件失败", "CLOCK_EVENT");
                    };
                },
                .settings_event => |settings_event| {
                    logger.global_logger.debug("处理设置事件", .{});
                    app_instance.settingsHandle(settings_event) catch |err| {
                        logger.global_logger.err("处理设置事件失败: {any}", .{err});
                        self.error_recovery.recordError("处理设置事件失败", "SETTINGS_EVENT");
                    };
                },
            }

            const display_data = app_instance.clock_manager.update();
            app_instance.updateDisplay(display_data) catch |err| {
                logger.global_logger.err("更新显示失败: {any}", .{err});
                app_instance.error_recovery.recordError("更新显示失败", "DISPLAY_UPDATE");
            };
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

            // 更新前端的默认时区，确保世界时钟模式立即反映设置变化
            self.webui.setTimezone(self.settings_manager.config.basic.timezone);
            // 设置变更后立即推送最新配置到前端
            self.webui.pushSettings();
            logger.global_logger.info("✓ 设置已更新，时钟已重新初始化", .{});
        }
    }

    /// 清理应用程序资源
    ///
    /// 参数:
    /// - **self**: MainApplication实例指针
    pub fn deinit(self: *MainApplication) void {
        logger.global_logger.info("MainApplication.deinit() 开始清理...", .{});

        // 1. 清理 WebUI 资源
        self.webui.deinit();

        self.clock_manager.deinit();

        // 2. 清理错误恢复管理器
        self.error_recovery.deinit();

        // 3. 清空全局指针
        global_app = null;

        logger.global_logger.info("MainApplication.deinit() 完成", .{});
    }

    /// 事件处理器包装函数
    ///
    /// 参数:
    /// - **event**: 时钟事件
    fn handleUserEventWrapper(event: interface.EventType) void {
        logger.global_logger.debug("handleUserEventWrapper 被调用，事件类型: {}", .{event});
        if (global_app) |app_instance| {
            logger.global_logger.debug("  调用 app.handleUserEvent", .{});
            @as(*MainApplication, @ptrCast(@alignCast(app_instance))).handleUserEvent(event);
        } else {
            logger.global_logger.err("  错误: global_app 为 null", .{});
        }
    }

    /// 获取当前设置的 JSON 表示
    ///
    /// 参数:
    /// - **self**: MainApplication实例指针
    /// - **buffer**: 用于存储 JSON 的缓冲区
    ///
    /// 返回:
    /// - ![]const u8: JSON 字符串（指向传入的缓冲区）
    pub fn getSettingsJSON(self: *MainApplication, buffer: []u8) ![]const u8 {
        return self.settings_manager.toJson(buffer);
    }

    /// 更新设置配置
    ///
    /// 参数:
    /// - **self**: MainApplication实例指针
    /// - **new_settings**: 新的设置配置
    ///
    /// 返回:
    /// - !void: 如果保存失败则返回错误
    pub fn updateSettings(self: *MainApplication, new_settings: interface.SettingsConfig) !void {
        self.settings_manager.config = new_settings;
        self.settings_manager.is_dirty = true;
        try self.settings_manager.save();
    }

    /// 从 JSON 字符串更新设置
    ///
    /// 参数:
    /// - **self**: MainApplication实例指针
    /// - **json_str**: JSON 格式的设置字符串
    ///
    /// 返回:
    /// - !void: 如果解析或保存失败则返回错误
    pub fn updateSettingsFromJson(self: *MainApplication, json_str: []const u8) !void {
        try self.settings_manager.jsonToSettings(json_str);
        try self.settings_manager.save();

        // 如果默认模式改变了，需要重新初始化时钟
        const new_clock_config = self.settings_manager.buildClockConfig();
        self.clock_manager = clock.ClockManager.init(new_clock_config);

        logger.global_logger.info("✓ 设置已更新，时钟已重新初始化", .{});
    }
};
