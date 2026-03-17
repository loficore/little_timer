//! 日志系统模块 - 支持多个日志等级、时间戳、等级过滤、文件日志、轮转和压缩
const std = @import("std");
const builtin = @import("builtin");

/// 日志等级枚举，由低到高：DEBUG < INFO < WARN < ERROR
pub const LogLevel = enum(u8) {
    DEBUG = 0,
    INFO = 1,
    WARN = 2,
    ERROR = 3,

    pub fn toString(self: LogLevel) []const u8 {
        return switch (self) {
            .DEBUG => "DEBUG",
            .INFO => "INFO",
            .WARN => "WARN",
            .ERROR => "ERROR",
        };
    }

    pub fn emoji(self: LogLevel) []const u8 {
        return switch (self) {
            .DEBUG => "🐛",
            .INFO => "ℹ️",
            .WARN => "⚠️",
            .ERROR => "❌",
        };
    }

    pub fn fromString(str: []const u8) ?LogLevel {
        const trimmed = std.mem.trim(u8, str, " ");

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

/// 日志配置结构体
pub const LogConfig = struct {
    /// 日志目录路径（空字符串表示使用默认目录）
    log_dir: []const u8 = "",
    /// 日志文件名（不含路径和扩展名）
    log_filename: []const u8 = "little_timer",
    /// 单个日志文件最大大小（字节），超过后轮转
    max_file_size: u64 = 10 * 1024 * 1024,
    /// 最多保留的日志文件数量（包括当前日志和压缩后的日志）
    max_file_count: u8 = 5,
    /// 压缩间隔（秒），每到这个间隔会压缩未压缩的历史日志
    compress_interval: u64 = 3600,
    /// 日志输出等级
    level: LogLevel = .INFO,
    /// 是否启用时间戳
    enable_timestamp: bool = true,

    /// 获取默认日志目录（跨平台）
    pub fn getDefaultLogDir(allocator: std.mem.Allocator) ![]const u8 {
        if (builtin.os.tag == .windows) {
            const dir = try std.fs.getAppDataDir(allocator, "LittleTimer");
            errdefer allocator.free(dir);
            return try std.fs.path.join(allocator, &[_][]const u8{ dir, "logs" });
        } else if (builtin.target.abi == .android) {
            return try allocator.dupe(u8, "/data/local/tmp/little_timer/logs");
        } else if (builtin.os.tag == .macos) {
            const dir = try std.fs.getAppDataDir(allocator, "LittleTimer");
            errdefer allocator.free(dir);
            return try std.fs.path.join(allocator, &[_][]const u8{ dir, "logs" });
        } else {
            // Linux 和其他 Unix-like 系统
            const dir = try std.fs.getAppDataDir(allocator, "little_timer");
            errdefer allocator.free(dir);
            return try std.fs.path.join(allocator, &[_][]const u8{ dir, "logs" });
        }
    }
};

/// 日志系统结构体 - 全局使用
pub const Logger = struct {
    /// 当前日志输出等级（>=此等级的日志才会输出）
    current_level: LogLevel = .INFO,

    /// 是否启用时间戳
    enable_timestamp: bool = true,

    /// 日志配置（用于文件日志）
    config: LogConfig = .{},

    /// 日志文件路径（为空时不输出到文件）
    log_file_path: ?[]const u8 = null,

    /// 日志文件句柄（内部使用）
    log_file: ?std.fs.File = null,

    /// 上次压缩检查的时间戳
    last_compress_ts: i64 = 0,

    /// 分配器（用于动态内存分配）
    allocator: ?std.mem.Allocator = null,

    /// 打开日志文件
    pub fn openLogFile(self: *Logger) !void {
        // 获取日志目录
        const log_dir = if (self.config.log_dir.len > 0)
            self.config.log_dir
        else
            try LogConfig.getDefaultLogDir(self.allocator.?);

        // 确保日志目录存在
        try std.fs.makeDirAbsolute(log_dir);

        // 构建日志文件完整路径
        const full_path = try std.fmt.allocPrint(
            self.allocator.?,
            "{s}/{s}.log",
            .{ log_dir, self.config.log_filename },
        );
        errdefer self.allocator.?.free(full_path);

        self.log_file_path = full_path;

        // 打开或创建日志文件
        self.log_file = try std.fs.createFileAbsolute(full_path, .{});
        errdefer {
            if (self.log_file) |*f| f.close();
            self.log_file = null;
        }

        // 获取当前文件大小并检查是否需要轮转
        const file_stat = try self.log_file.?.stat();
        const current_size: u64 = file_stat.size;

        if (current_size >= self.config.max_file_size) {
            self.log_file.?.close();
            self.log_file = null;
            try self.rotateLogFile(full_path);
            self.log_file = try std.fs.createFileAbsolute(full_path, .{});
        }

        // 跳到文件末尾
        try self.log_file.?.seekToEnd();

        self.info("日志文件已打开: {s}", .{full_path});

        // 尝试压缩旧日志
        try self.compressOldLogs();
    }

    /// 轮转日志文件
    fn rotateLogFile(self: *Logger, full_path: []const u8) !void {
        // 从完整路径中提取目录和文件名
        const dir_path = std.fs.path.dirname(full_path) orelse ".";
        const filename = std.fs.path.basename(full_path);

        var dir = try std.fs.openDirAbsolute(dir_path, .{});
        defer dir.close();

        // 构建带编号的日志文件名
        const rotated_name = try std.fmt.allocPrint(
            self.allocator.?,
            "{s}.log.1",
            .{self.config.log_filename},
        );
        errdefer self.allocator.?.free(rotated_name);

        // 如果 .1 文件已存在，递归移动
        if (dir.access(rotated_name, .{})) |_| {
            try self.shiftLogFiles(dir, 1);
        } else |_| {}

        // 重命名当前日志为 .1
        try dir.rename(filename, rotated_name);
        self.allocator.?.free(rotated_name);

        // 清理超过 max_file_count 的文件
        try self.cleanOldFiles(dir);

        self.info("日志文件已轮转", .{});
    }

    /// 移动编号的日志文件（.1 -> .2, .2 -> .3 等）
    fn shiftLogFiles(self: *Logger, dir: std.fs.Dir, from_index: u8) !void {
        // 从最大的编号开始移动，避免覆盖
        var i: u8 = self.config.max_file_count;
        while (i > from_index) : (i -= 1) {
            const src = try std.fmt.allocPrint(
                self.allocator.?,
                "{s}.log.{d}",
                .{ self.config.log_filename, i },
            );
            errdefer self.allocator.?.free(src);
            const dst = try std.fmt.allocPrint(
                self.allocator.?,
                "{s}.log.{d}",
                .{ self.config.log_filename, i + 1 },
            );
            errdefer self.allocator.?.free(dst);

            // 尝试移动文件，如果不存在则忽略
            dir.rename(src, dst) catch {};

            self.allocator.?.free(src);
            self.allocator.?.free(dst);
        }
    }

    /// 清理超过保留数量的旧日志文件
    fn cleanOldFiles(self: *Logger, dir: std.fs.Dir) !void {
        var i: u8 = self.config.max_file_count + 1;
        while (i < 20) : (i += 1) {
            const path = try std.fmt.allocPrint(
                self.allocator.?,
                "{s}.log.{d}",
                .{ self.config.log_filename, i },
            );
            // 尝试删除文件，不存在则忽略
            dir.deleteFile(path) catch {};
            self.allocator.?.free(path);
        }
    }

    /// 压缩旧的未压缩日志文件
    fn compressOldLogs(self: *Logger) !void {
        const now = std.time.timestamp();
        if (now - self.last_compress_ts < @as(i64, @intCast(self.config.compress_interval))) {
            return;
        }
        self.last_compress_ts = now;

        // 获取日志目录
        const log_dir = if (self.config.log_dir.len > 0)
            self.config.log_dir
        else
            try LogConfig.getDefaultLogDir(self.allocator.?);

        var dir = try std.fs.openDirAbsolute(log_dir, .{});
        defer dir.close();

        // 遍历 .log.N 文件，压缩未压缩的
        var i: u8 = 1;
        while (i <= self.config.max_file_count) : (i += 1) {
            const src = try std.fmt.allocPrint(
                self.allocator.?,
                "{s}.log.{d}",
                .{ self.config.log_filename, i },
            );
            errdefer self.allocator.?.free(src);

            // 如果源文件不存在，跳过
            if (dir.access(src, .{})) |_| {} else |_| {
                self.allocator.?.free(src);
                continue;
            }

            const dst = try std.fmt.allocPrint(
                self.allocator.?,
                "{s}.log.{d}.gz",
                .{ self.config.log_filename, i },
            );
            errdefer self.allocator.?.free(dst);

            // 如果目标文件已存在，跳过
            if (dir.access(dst, .{})) |_| {
                // dst exists, skip
            } else |_| {
                try self.compressFile(dir, src, dst);
                try dir.deleteFile(src);
                self.info("日志文件已压缩: {s} -> {s}", .{ src, dst });
            }

            self.allocator.?.free(src);
            self.allocator.?.free(dst);
        }
    }

    /// 压缩单个文件为 gzip
    fn compressFile(self: *Logger, dir: std.fs.Dir, src_name: []const u8, dst_name: []const u8) !void {
        const src_file = try dir.openFile(src_name, .{});
        defer src_file.close();

        const src_stat = try src_file.stat();
        const src_size = src_stat.size;

        // 读取源文件内容
        const src_content = try self.allocator.?.alloc(u8, src_size);
        defer self.allocator.?.free(src_content);
        _ = try src_file.readAll(src_content);

        // 创建目标文件
        const dst_file = try dir.createFile(dst_name, .{});
        defer dst_file.close();

        // 使用 gzip 压缩
        var gzip_buffer = try std.ArrayList(u8).initCapacity(self.allocator.?, src_size);
        defer gzip_buffer.deinit();

        var compressor = std.compress.gzip.compressor(gzip_buffer.writer());
        try compressor.writeAll(src_content);
        try compressor.flush();

        try dst_file.writeAll(gzip_buffer.items);
    }

    /// 关闭日志文件
    pub fn closeLogFile(self: *Logger) void {
        if (self.log_file) |*file| {
            file.close();
            self.log_file = null;
        }
        if (self.log_file_path) |path| {
            if (self.allocator) |alloc| {
                alloc.free(path);
            }
            self.log_file_path = null;
        }
    }

    /// 获取当前Unix时间戳（秒）并格式化为HH:MM:SS
    pub fn formatTimestamp(self: Logger, buf: []u8) []const u8 {
        if (!self.enable_timestamp) {
            return "";
        }

        const now_s = std.time.timestamp();
        if (now_s < 0) return "";

        const es = std.time.epoch.EpochSeconds{ .secs = @as(u64, @intCast(now_s)) };
        const yd = es.getEpochDay().calculateYearDay();
        const md = yd.calculateMonthDay();
        const ds = es.getDaySeconds();

        const year: u16 = yd.year;
        const month: u4 = md.month.numeric();
        const day: u5 = md.day_index + 1;
        const hour: u5 = ds.getHoursIntoDay();
        const minute: u6 = ds.getMinutesIntoHour();
        const second: u6 = ds.getSecondsIntoMinute();

        const timestamp_str = std.fmt.bufPrint(
            buf,
            "[{d:0>4}-{:0>2}-{:0>2}T{:0>2}:{:0>2}:{:0>2}.000Z] ",
            .{ year, month, day, hour, minute, second },
        ) catch return "";

        return timestamp_str;
    }

    /// 内部日志输出函数，检查等级过滤后再输出
    fn logInternal(self: *Logger, level: LogLevel, comptime fmt: []const u8, args: anytype) void {
        // 等级过滤：只输出>=current_level的日志
        if (@intFromEnum(level) < @intFromEnum(self.current_level)) {
            return;
        }

        // 检查是否需要轮转
        if (self.log_file) |*file| {
            const stat = file.stat() catch return;
            if (stat.size >= self.config.max_file_size) {
                self.closeLogFile();
                const log_dir = if (self.config.log_dir.len > 0)
                    self.config.log_dir
                else
                    LogConfig.getDefaultLogDir(self.allocator.?) catch return;
                const full_path = std.fmt.allocPrint(
                    self.allocator.?,
                    "{s}/{s}.log",
                    .{ log_dir, self.config.log_filename },
                ) catch return;
                self.rotateLogFile(full_path) catch return;
                self.log_file = std.fs.createFileAbsolute(full_path, .{}) catch return;
            }
        }

        // 计算时间戳字符串
        var timestamp_buf: [32]u8 = undefined;
        const timestamp_str = self.formatTimestamp(&timestamp_buf);

        // 构建日志内容
        var log_buf: [1024]u8 = undefined;
        const log_content = std.fmt.bufPrint(&log_buf, "[{s}] {s} ", .{ level.toString(), level.emoji() }) catch {
            return;
        };

        // 输出到控制台
        if (builtin.is_test) {
            const stderr_file = std.fs.File.stderr();
            if (self.enable_timestamp and timestamp_str.len > 0) {
                stderr_file.writeAll(timestamp_str) catch {};
            }
            stderr_file.writeAll(log_content) catch {};
            var msg_buf: [1024]u8 = undefined;
            const msg = std.fmt.bufPrint(&msg_buf, fmt ++ "\n", args) catch "格式错误";
            stderr_file.writeAll(msg) catch {};
        } else {
            if (self.enable_timestamp and timestamp_str.len > 0) {
                std.debug.print("{s}", .{timestamp_str});
            }
            std.debug.print("{s}", .{log_content});
            std.debug.print(fmt ++ "\n", args);
        }

        // 输出到日志文件
        if (self.log_file) |file| {
            if (self.enable_timestamp and timestamp_str.len > 0) {
                file.writeAll(timestamp_str) catch {};
            }
            file.writeAll(log_content) catch {};
            var msg_buf: [1024]u8 = undefined;
            const msg = std.fmt.bufPrint(&msg_buf, fmt ++ "\n", args) catch "格式错误";
            file.writeAll(msg) catch {};
        }
    }

    /// DEBUG等级日志（最详细的调试信息）
    pub fn debug(self: *Logger, comptime fmt: []const u8, args: anytype) void {
        self.logInternal(.DEBUG, fmt, args);
    }

    /// INFO等级日志（一般信息）
    pub fn info(self: *Logger, comptime fmt: []const u8, args: anytype) void {
        self.logInternal(.INFO, fmt, args);
    }

    /// WARN等级日志（警告信息，可能有问题）
    pub fn warn(self: *Logger, comptime fmt: []const u8, args: anytype) void {
        self.logInternal(.WARN, fmt, args);
    }

    /// ERROR等级日志（错误信息，需要处理）
    pub fn err(self: *Logger, comptime fmt: []const u8, args: anytype) void {
        self.logInternal(.ERROR, fmt, args);
    }

    /// 改变日志输出等级
    pub fn setLevel(self: *Logger, level: LogLevel) void {
        self.current_level = level;
        self.info("日志等级已改变为: {s}", .{level.toString()});
    }

    /// 改变是否启用时间戳
    pub fn setTimestamp(self: *Logger, enable: bool) void {
        self.enable_timestamp = enable;
    }

    /// 手动触发日志轮转（可用于外部调用）
    pub fn rotate(self: *Logger) !void {
        if (self.log_file_path) |path| {
            self.closeLogFile();
            try self.rotateLogFile(path);
            self.log_file = try std.fs.createFileAbsolute(path, .{});
        }
    }

    /// 手动触发压缩（可用于外部调用）
    pub fn compress(self: *Logger) !void {
        try self.compressOldLogs();
    }

    /// 初始化文件日志（带配置）
    pub fn initFileLogging(self: *Logger, allocator: std.mem.Allocator, config: LogConfig) !void {
        self.allocator = allocator;
        self.config = config;
        try self.openLogFile();
    }
};

/// 全局日志实例（需手动初始化）
pub var global_logger: Logger = .{
    .current_level = .INFO,
    .enable_timestamp = true,
};

/// 初始化全局日志文件（便捷函数）
pub fn initGlobalLoggerFile(allocator: std.mem.Allocator, config: LogConfig) !void {
    global_logger.allocator = allocator;
    global_logger.config = config;
    try global_logger.openLogFile();
}
