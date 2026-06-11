//! 公共接口模块
const std = @import("std");

pub const SECOND = 1;
pub const MINUTE = 60;
pub const HOUR = 3600;
pub const DAY = 86400;
pub const YEAR = 31536000;

pub const DEFAULT_WORK_DURATION_SECONDS = 25 * MINUTE;
pub const DEFAULT_REST_DURATION_SECONDS = 5 * MINUTE;
pub const DEFAULT_MAX_STOPWATCH_SECONDS = 24 * HOUR;
pub const DEFAULT_MAX_DURATION_SECONDS = DAY;
pub const DEFAULT_MAX_YEAR_SECONDS = 365 * YEAR;

pub const DEFAULT_TICK_INTERVAL_MS = 1000;
pub const DEFAULT_AUTO_SAVE_INTERVAL_MS = 5000;
pub const MIN_TICK_INTERVAL_MS = 100;
pub const MAX_TICK_INTERVAL_MS = 5000;
pub const DEFAULT_MAX_LOG_FILE_SIZE = 10 * 1024 * 1024;

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
        duration_seconds: u64 = DEFAULT_WORK_DURATION_SECONDS, // 倒计时秒
        loop: bool = false, // 是否循环倒计时
        loop_interval_seconds: u64 = 0, // 循环间隔休息秒数
        loop_count: u32 = 0, // 循环次数，0 表示无限循环
    } = .{},

    /// 正计时配置
    stopwatch: struct {
        max_seconds: u64 = DEFAULT_MAX_STOPWATCH_SECONDS, // 正计时上限（秒），默认一天
    } = .{},
};

/// 时钟事件 - 从 windows 发送给 app/clock
pub const ClockEvent = union(enum) {
    tick: i64, // 毫秒增量
    user_start_timer: void, // 用户开始计时
    user_pause_timer: void, // 用户暂停计时
    user_reset_timer: void, // 用户重置计时
    user_finish_timer: void, // 用户结束计时（停止并计入统计）
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
        wallpaper: []const u8 = "", // 全局壁纸
    } = .{},

    /// 默认时钟设置
    clock_defaults: ClockTaskConfig = .{ .countdown = .{} },

    /// 日志和性能设置
    logging: struct {
        level: []const u8 = "INFO", // 日志等级
        enable_timestamp: bool = true, // 是否启用时间戳
        tick_interval_ms: i64 = 1000, // Tick 间隔（毫秒），默认 1000ms (1秒)
        enable_file_logging: bool = true, // 是否启用文件日志
        log_dir: []const u8 = "", // 日志目录（空则用默认）
        max_file_size: u64 = 10 * 1024 * 1024, // 单个日志文件最大大小（字节）
        max_file_count: u8 = 5, // 最多保留的日志文件数量
    } = .{},

    /// 认证设置
    auth: struct {
        auth_enabled: bool = false, // 是否启用认证
        auth_token: []const u8 = "", // Bearer Token (空=未设置)
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

/// 备份目标类型
pub const BackupTargetType = enum {
    local,
    webdav,
    s3,
};

/// 解锁结果
pub const UnlockResult = struct {
    success: bool,
    locked_until: i64,
};

/// 主密码状态
pub const MasterPasswordStatus = struct {
    has_password: bool,    // 是否已设置主密码
    unlocked: bool,         // 是否已解锁
    locked_until: i64,     // 锁定截止时间戳
    unlock_time: i64,      // 上次解锁时间戳
};

/// API 动作（用于触发 UI）
pub const ApiAction = union(enum) {
    show_modal: struct {
        target: []const u8,
        params: struct {
            mode: []const u8, // "setup" 或 "unlock"
        },
    },
};

/// 备份配置
pub const BackupConfig = struct {
    enabled: bool = false,
    auto_backup: bool = false,
    auto_backup_interval: u64 = DAY, // 秒，默认1天

    target_type: BackupTargetType = .local,

    // Local 专用
    local_path: []const u8 = "",

    // WebDAV 专用
    webdav_url: []const u8 = "",
    webdav_username: []const u8 = "",
    webdav_password: []const u8 = "", // 加密存储，实际不持久化明文

    // S3 专用
    s3_endpoint: []const u8 = "",
    s3_bucket: []const u8 = "",
    s3_region: []const u8 = "",
    s3_access_key: []const u8 = "",
    s3_secret_key: []const u8 = "", // 加密存储，实际不持久化明文
    s3_path_prefix: []const u8 = "little_timer/",

    // 主密码管理
    has_master_password: bool = false, // 是否已设置主密码

    // 凭证解锁管理
    credentials_unlock_time: i64 = 0, // 上次解锁时间戳
    credential_unlock_attempts: u32 = 0, // 错误尝试次数
    credential_locked_until: i64 = 0, // 锁定截止时间戳
};

/// 备份信息
pub const BackupInfo = struct {
    name: []const u8,
    timestamp: i64,
    size_bytes: u64,
};
