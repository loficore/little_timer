//! 日志系统模块 - 支持多个日志等级、时间戳、等级过滤
const std = @import("std");

/// 日志等级枚举，由低到高：DEBUG < INFO < WARN < ERROR
pub const LogLevel = enum(u8) {
    DEBUG = 0,
    INFO = 1,
    WARN = 2,
    ERROR = 3,

    /// 将LogLevel转换为字符串表示
    pub fn toString(self: LogLevel) []const u8 {
        return switch (self) {
            .DEBUG => "DEBUG",
            .INFO => "INFO",
            .WARN => "WARN",
            .ERROR => "ERROR",
        };
    }

    /// 将LogLevel转换为emoji前缀
    pub fn emoji(self: LogLevel) []const u8 {
        return switch (self) {
            .DEBUG => "🐛",
            .INFO => "ℹ️",
            .WARN => "⚠️",
            .ERROR => "❌",
        };
    }

    /// 从字符串解析LogLevel，不区分大小写
    pub fn fromString(str: []const u8) ?LogLevel {
        const trimmed = std.mem.trim(u8, str, " ");

        // 尝试大写和小写匹配
        if (std.mem.eql(u8, trimmed, "DEBUG") or std.mem.eql(u8, trimmed, "debug")) {
            return .DEBUG;
        } else if (std.mem.eql(u8, trimmed, "INFO") or std.mem.eql(u8, trimmed, "info")) {
            return .INFO;
        } else if (std.mem.eql(u8, trimmed, "WARN") or std.mem.eql(u8, trimmed, "warn")) {
            return .WARN;
        } else if (std.mem.eql(u8, trimmed, "ERROR") or std.mem.eql(u8, trimmed, "error")) {
            return .ERROR;
        } else {
            return null;
        }
    }
};

/// 日志系统结构体 - 全局使用
pub const Logger = struct {
    /// 当前日志输出等级（>=此等级的日志才会输出）
    current_level: LogLevel = .INFO,

    /// 是否启用时间戳
    enable_timestamp: bool = true,

    /// 获取当前Unix时间戳（秒）并格式化为HH:MM:SS
    ///
    /// 参数:
    /// - **self**: Logger实例指针
    /// - **buf**: 用于存储格式化时间戳的缓冲
    ///
    /// 返回:
    /// - []const u8: 格式化后的时间戳字符串切片
    pub fn formatTimestamp(self: Logger, buf: []u8) []const u8 {
        if (!self.enable_timestamp) {
            return "";
        }

        // 获取当前时间戳（纳秒）并转换为秒
        const now_ns = std.time.nanoTimestamp();
        const now_s = @divFloor(now_ns, 1_000_000_000);

        // 简单的时间计算（不考虑日期，只显示时刻）
        const seconds_per_day = 86400;
        const seconds_per_hour = 3600;
        const seconds_per_minute = 60;

        // 获取当天的秒数（假设UTC+0，实际应该根据timezone调整）
        const day_seconds = @mod(now_s, seconds_per_day);
        const hours = @divFloor(day_seconds, seconds_per_hour);
        const minutes = @divFloor(@mod(day_seconds, seconds_per_hour), seconds_per_minute);
        const seconds = @mod(day_seconds, seconds_per_minute);

        const timestamp_str = std.fmt.bufPrint(
            buf,
            "[{:0>2}:{:0>2}:{:0>2}] ",
            .{ hours, minutes, seconds },
        ) catch return "";

        return timestamp_str;
    }

    /// 内部日志输出函数，检查等级过滤后再输出
    ///
    /// 参数:
    /// - **self**: Logger实例指针
    /// - **level**: 日志等级
    /// - **fmt**: 格式化字符串
    /// - **args**: 格式化参数
    ///
    /// 返回:
    /// - void
    fn logInternal(self: Logger, level: LogLevel, comptime fmt: []const u8, args: anytype) void {
        // 等级过滤：只输出>=current_level的日志
        if (@intFromEnum(level) < @intFromEnum(self.current_level)) {
            return;
        }

        // 计算时间戳字符串
        var timestamp_buf: [32]u8 = undefined;
        const timestamp_str = self.formatTimestamp(&timestamp_buf);

        // 输出日志
        if (self.enable_timestamp and timestamp_str.len > 0) {
            std.debug.print("{s}", .{timestamp_str});
        }

        std.debug.print("[{s}] {s} ", .{ level.toString(), level.emoji() });
        std.debug.print(fmt ++ "\n", args);
    }

    /// DEBUG等级日志（最详细的调试信息）
    pub fn debug(self: Logger, comptime fmt: []const u8, args: anytype) void {
        self.logInternal(.DEBUG, fmt, args);
    }

    /// INFO等级日志（一般信息）
    pub fn info(self: Logger, comptime fmt: []const u8, args: anytype) void {
        self.logInternal(.INFO, fmt, args);
    }

    /// WARN等级日志（警告信息，可能有问题）
    pub fn warn(self: Logger, comptime fmt: []const u8, args: anytype) void {
        self.logInternal(.WARN, fmt, args);
    }

    /// ERROR等级日志（错误信息，需要处理）
    pub fn err(self: Logger, comptime fmt: []const u8, args: anytype) void {
        self.logInternal(.ERROR, fmt, args);
    }

    /// 改变日志输出等级
    ///
    /// 参数:
    /// - **self**: Logger实例指针
    /// - **level**: 新的日志等级
    ///
    /// 返回:
    /// - void
    pub fn setLevel(self: *Logger, level: LogLevel) void {
        self.current_level = level;
        self.info("日志等级已改变为: {s}", .{level.toString()});
    }

    /// 改变是否启用时间戳
    ///
    /// 参数:
    /// - **self**: Logger实例指针
    /// - **enable**: 是否启用时间戳
    ///
    /// 返回:
    /// - void
    pub fn setTimestamp(self: *Logger, enable: bool) void {
        self.enable_timestamp = enable;
    }
};

/// 全局日志实例
pub var global_logger: Logger = .{
    .current_level = .INFO,
    .enable_timestamp = true,
};
