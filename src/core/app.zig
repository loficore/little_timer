const std = @import("std");
const app = @import("app.zig");
const logger = @import("logger.zig");
const builtin = @import("builtin");

const webui_windows = @import("ui/webui_windows.zig");
const clock = @import("clock.zig");
const settings = @import("../settings/settings_manager.zig");
const interface = @import("interface.zig");
const error_recovery = @import("utils/error_recovery.zig");

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

        // 初始化和加载设置（纯 SQLite 版本，不再需要 settings.toml）
        self.settings_manager = try settings.SettingsManager.init(allocator, "");
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
};
