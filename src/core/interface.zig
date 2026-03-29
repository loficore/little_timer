//! 公共接口模块
const std = @import("std");

pub const ModeEnumT = enum {
    COUNTDOWN_MODE,
    STOPWATCH_MODE,
};

pub const EventType = union(enum) {
    clock_event: ClockEvent,
    settings_event: SettingsEvent,
};

/// 时钟任务配置，供设置/持久化等模块共享
/// 这是一个结构体，包含所有三种模式的配置
pub const ClockTaskConfig = struct {
    /// 默认启动模式
    default_mode: ModeEnumT = .COUNTDOWN_MODE,

    /// 倒计时配置
    countdown: struct {
        duration_seconds: u64 = 25 * 60, // 倒计时秒
        loop: bool = false, // 是否循环倒计时
        loop_interval_seconds: u64 = 0, // 循环间隔休息秒数
        loop_count: u32 = 0, // 循环次数，0 表示无限循环
    } = .{},

    /// 正计时配置
    stopwatch: struct {
        max_seconds: u64 = 24 * 60 * 60, // 正计时上限（秒），默认一天
    } = .{},
};

/// 时钟事件 - 从 windows 发送给 app/clock
pub const ClockEvent = union(enum) {
    tick: i64, // 毫秒增量
    user_start_timer: void, // 用户开始计时
    user_pause_timer: void, // 用户暂停计时
    user_reset_timer: void, // 用户重置计时
    user_change_mode: ModeEnumT, // 用户更改模式
    user_change_config: ClockTaskConfig, // 用户更改配置
};

/// 定时器预设配置项
pub const TimerPreset = struct {
    name: []const u8, // 预设名称
    mode: ModeEnumT, // 预设模式
    config: ClockTaskConfig, // 预设配置
};

/// 应用设置配置
pub const SettingsConfig = struct {
    /// 基本设置
    basic: struct {
        timezone: i8 = 8, // 时区，默认东八区
        language: []const u8 = "ZH", // 语言代码
        default_mode: DefaultMode = .countdown, // 默认启动模式
        theme_mode: []const u8 = "dark", // 主题模式
    } = .{},

    /// 默认时钟设置
    clock_defaults: ClockTaskConfig = .{ .countdown = .{} },

    /// 日志和性能设置
    logging: struct {
        level: []const u8 = "INFO", // 日志等级
        enable_timestamp: bool = true, // 是否启用时间戳
        tick_interval_ms: i64 = 1000, // Tick 间隔（毫秒），默认 1000ms (1秒)
        enable_file_logging: bool = false, // 是否启用文件日志
        log_dir: []const u8 = "", // 日志目录（空则用默认）
        max_file_size: u64 = 10 * 1024 * 1024, // 单个日志文件最大大小（字节）
        max_file_count: u8 = 5, // 最多保留的日志文件数量
    } = .{},
};

/// 默认模式枚举（替代字符串比较）
pub const DefaultMode = enum {
    countdown,
    stopwatch,
};

pub const SettingsEvent = union(enum) {
    get_settings: [:0]u8, // 用于获取设置的缓冲区（哨兵切片）
    change_settings: [:0]u8, // JSON 字符串（可变，便于调用方释放）
};
