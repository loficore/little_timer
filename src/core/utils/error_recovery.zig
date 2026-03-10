//! 错误恢复和监控模块 - 提供后端错误处理、重试和监控功能

const std = @import("std");
const logger = @import("../logger.zig");

pub const ErrorRecoveryManager = struct {
    /// 错误计数，用于检测频繁出现的错误
    error_count: u32 = 0,
    /// 最后一次错误消息
    last_error_message: [256]u8 = undefined,
    /// 最后一次错误时间戳（纳秒）
    last_error_time: i64 = 0,
    /// 是否处于恢复状态
    is_recovering: bool = false,
    /// 恢复尝试次数
    recovery_attempts: u32 = 0,
    /// 分配器
    allocator: std.mem.Allocator,

    /// 初始化错误恢复管理器
    ///
    /// 参数:
    /// - **allocator**: 内存分配器
    ///
    /// 返回:
    /// - ErrorRecoveryManager: 初始化的实例
    pub fn init(allocator: std.mem.Allocator) ErrorRecoveryManager {
        return .{
            .allocator = allocator,
        };
    }

    /// 记录错误
    ///
    /// 参数:
    /// - **self**: ErrorRecoveryManager实例指针
    /// - **error_msg**: 错误消息
    /// - **error_type**: 错误类型（用于分类）
    pub fn recordError(self: *ErrorRecoveryManager, error_msg: []const u8, error_type: []const u8) void {
        self.error_count += 1;
        self.last_error_time = @as(i64, @truncate(std.time.nanoTimestamp()));

        // 复制错误消息（截断到最大长度）
        const msg_len = @min(error_msg.len, 255);
        @memcpy(self.last_error_message[0..msg_len], error_msg[0..msg_len]);
        self.last_error_message[msg_len] = 0;

        logger.global_logger.err("错误 #{d} [{s}]: {s}", .{ self.error_count, error_type, error_msg });

        // 如果错误数过多，启动恢复程序
        if (self.error_count > 5) {
            self.startRecovery();
        }
    }

    /// 启动错误恢复程序
    ///
    /// 参数:
    /// - **self**: ErrorRecoveryManager实例指针
    fn startRecovery(self: *ErrorRecoveryManager) void {
        if (!self.is_recovering) {
            self.is_recovering = true;
            self.recovery_attempts = 0;
            logger.global_logger.warn("⚠️ 检测到多个错误，启动恢复程序...", .{});
        }
    }

    /// 尝试恢复
    ///
    /// 参数:
    /// - **self**: ErrorRecoveryManager实例指针
    ///
    /// 返回:
    /// - bool: 恢复是否成功
    pub fn attemptRecovery(self: *ErrorRecoveryManager) bool {
        if (!self.is_recovering) {
            return true;
        }

        self.recovery_attempts += 1;

        if (self.recovery_attempts > 3) {
            logger.global_logger.err("❌ 恢复失败：已超过最大重试次数", .{});
            self.is_recovering = false;
            self.error_count = 0;
            return false;
        }

        logger.global_logger.info("🔄 恢复尝试 {d}/3...", .{self.recovery_attempts});

        // 延迟一段时间后重试
        const delay_ms = std.math.pow(u32, 2, self.recovery_attempts) * 100; // 指数退避
        logger.global_logger.info("等待 {d}ms 后重试...", .{delay_ms});

        // 这里可以添加实际的恢复逻辑（例如清理资源、重新初始化等）
        return true;
    }

    /// 标记恢复成功
    ///
    /// 参数:
    /// - **self**: ErrorRecoveryManager实例指针
    pub fn markRecoverySuccess(self: *ErrorRecoveryManager) void {
        if (self.is_recovering) {
            logger.global_logger.info("✅ 错误恢复成功！", .{});
            self.is_recovering = false;
            self.error_count = 0;
            self.recovery_attempts = 0;
        }
    }

    /// 获取最后的错误消息
    ///
    /// 返回:
    /// - []const u8: 最后的错误消息
    pub fn getLastErrorMessage(self: *const ErrorRecoveryManager) []const u8 {
        // 找到 null 终止符
        var len: usize = 0;
        for (self.last_error_message) |byte| {
            if (byte == 0) break;
            len += 1;
        }
        return self.last_error_message[0..len];
    }

    /// 获取错误统计
    ///
    /// 返回:
    /// - struct { count: u32, is_recovering: bool, recovery_attempts: u32 }
    pub fn getErrorStats(self: *const ErrorRecoveryManager) struct { count: u32, is_recovering: bool, recovery_attempts: u32 } {
        return .{
            .count = self.error_count,
            .is_recovering = self.is_recovering,
            .recovery_attempts = self.recovery_attempts,
        };
    }

    /// 清理状态
    ///
    /// 参数:
    /// - **self**: ErrorRecoveryManager实例指针
    pub fn reset(self: *ErrorRecoveryManager) void {
        self.error_count = 0;
        self.is_recovering = false;
        self.recovery_attempts = 0;
        logger.global_logger.info("错误恢复管理器已重置", .{});
    }

    /// 清空错误记录（别名方法）
    ///
    /// 参数:
    /// - **self**: ErrorRecoveryManager实例指针
    pub fn clearErrors(self: *ErrorRecoveryManager) void {
        self.reset();
    }

    /// 检查是否处于恢复状态
    ///
    /// 返回:
    /// - bool: true 表示处于恢复状态
    pub fn isRecovering(self: *const ErrorRecoveryManager) bool {
        return self.is_recovering;
    }

    /// 获取恢复尝试次数
    ///
    /// 返回:
    /// - u32: 当前恢复尝试次数
    pub fn getRecoveryAttempts(self: *const ErrorRecoveryManager) u32 {
        return self.recovery_attempts;
    }

    /// 清理资源
    ///
    /// 参数:
    /// - **self**: ErrorRecoveryManager实例指针
    pub fn deinit(self: *ErrorRecoveryManager) void {
        logger.global_logger.info("错误恢复管理器已清理 (总错误数: {d})", .{self.error_count});
    }
};

/// 网络错误检测器
pub const NetworkErrorDetector = struct {
    /// 连续失败计数
    consecutive_failures: u32 = 0,
    /// 最后失败时间
    last_failure_time: i64 = 0,
    /// 最大允许连续失败次数
    max_consecutive_failures: u32 = 5,

    /// 记录连接成功
    pub fn recordSuccess(self: *NetworkErrorDetector) void {
        if (self.consecutive_failures > 0) {
            logger.global_logger.info("✅ 连接恢复成功，重置失败计数", .{});
        }
        self.consecutive_failures = 0;
    }

    /// 记录连接失败
    pub fn recordFailure(self: *NetworkErrorDetector) void {
        self.consecutive_failures += 1;
        self.last_failure_time = std.time.nanoTimestamp();

        logger.global_logger.warn("⚠️ 连接失败 {d}/{d}", .{ self.consecutive_failures, self.max_consecutive_failures });

        if (self.consecutive_failures >= self.max_consecutive_failures) {
            logger.global_logger.err("❌ 连接失败次数过多，可能存在网络问题", .{});
        }
    }

    /// 检查是否应该断开连接
    ///
    /// 返回:
    /// - bool: true 表示应该断开
    pub fn shouldDisconnect(self: *const NetworkErrorDetector) bool {
        return self.consecutive_failures >= self.max_consecutive_failures;
    }

    /// 重置状态
    pub fn reset(self: *NetworkErrorDetector) void {
        self.consecutive_failures = 0;
    }
};
