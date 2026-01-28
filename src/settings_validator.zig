//! 设置验证模块 - 集中所有配置参数的范围和格式验证
const std = @import("std");

/// 验证错误类型
pub const ValidationError = error{
    InvalidTimezone, // 时区超出范围 (-12 到 14)
    InvalidLanguage, // 语言代码格式不正确
    InvalidDuration, // 倒计时时长超出范围
    InvalidLoopCount, // 循环次数超出范围
    InvalidLoopInterval, // 循环间隔超出范围
    InvalidMaxSeconds, // 正计时上限超出范围
    InvalidTickInterval, // Tick 间隔超出范围
    InvalidPresetName, // 预设名称无效
    PresetLimitExceeded, // 预设数量超过上限
};

/// 验证时区范围 [-12, 14]
///
/// 参数:
/// - **tz**: 时区偏移（小时）
///
/// 返回:
/// - ValidationError!void: 如果时区超出范围则返回错误
pub fn validateTimezone(tz: i8) ValidationError!void {
    if (tz < -12 or tz > 14) {
        return error.InvalidTimezone;
    }
}

/// 验证语言代码 [1-10 字符]
///
/// 参数:
/// - **lang**: 语言代码字符串
///
/// 返回:
/// - ValidationError!void: 如果语言代码格式无效则返回错误
pub fn validateLanguage(lang: []const u8) ValidationError!void {
    if (lang.len == 0 or lang.len > 10) {
        return error.InvalidLanguage;
    }
}

/// 验证倒计时时长 [1, 86400] 秒（最多24小时）
///
/// 参数:
/// - **duration**: 倒计时时长（秒）
///
/// 返回:
/// - ValidationError!void: 如果时长超出范围则返回错误
pub fn validateDuration(duration: u64) ValidationError!void {
    if (duration < 1 or duration > 86400) {
        return error.InvalidDuration;
    }
}

/// 验证循环次数 [0, 1000]（0 表示无限循环）
///
/// 参数:
/// - **count**: 循环次数
///
/// 返回:
/// - ValidationError!void: 如果循环次数超出范围则返回错误
pub fn validateLoopCount(count: u32) ValidationError!void {
    if (count > 1000) {
        return error.InvalidLoopCount;
    }
}

/// 验证循环间隔 [0, 3600] 秒（最多1小时休息）
///
/// 参数:
/// - **interval**: 循环间隔（秒）
///
/// 返回:
/// - ValidationError!void: 如果间隔超出范围则返回错误
pub fn validateLoopInterval(interval: u64) ValidationError!void {
    if (interval > 3600) {
        return error.InvalidLoopInterval;
    }
}

/// 验证正计时上限 (0, 31536000] 秒（最多365天）
///
/// 参数:
/// - **max_seconds**: 正计时上限（秒）
///
/// 返回:
/// - ValidationError!void: 如果上限超出范围则返回错误
pub fn validateMaxSeconds(max_seconds: u64) ValidationError!void {
    if (max_seconds == 0 or max_seconds > 86400 * 365) {
        return error.InvalidMaxSeconds;
    }
}

/// 验证 Tick 间隔 [100, 5000] 毫秒
///
/// 参数:
/// - **interval_ms**: Tick 间隔（毫秒）
///
/// 返回:
/// - ValidationError!void: 如果间隔超出范围则返回错误
pub fn validateTickInterval(interval_ms: i64) ValidationError!void {
    if (interval_ms < 100 or interval_ms > 5000) {
        return error.InvalidTickInterval;
    }
}

/// 验证预设名称（非空，最多64字符）
///
/// 参数:
/// - **name**: 预设名称
///
/// 返回:
/// - ValidationError!void: 如果名称无效则返回错误
pub fn validatePresetName(name: []const u8) ValidationError!void {
    if (name.len == 0 or name.len > 64) {
        return error.InvalidPresetName;
    }
}

/// 验证预设数量 [0, 999]
///
/// 参数:
/// - **count**: 预设数量
///
/// 返回:
/// - ValidationError!void: 如果数量超过上限则返回错误
pub fn validatePresetCount(count: usize) ValidationError!void {
    if (count > 999) {
        return error.PresetLimitExceeded;
    }
}

/// 从 JSON 整数值安全转换为 i8（带范围验证）
///
/// 参数:
/// - **json_int**: JSON 整数值
/// - **min**: 最小值
/// - **max**: 最大值
///
/// 返回:
/// - ?i8: 如果值在范围内则返回转换结果，否则返回 null
pub fn safeI8FromJson(json_int: i64, min: i8, max: i8) ?i8 {
    if (json_int < min or json_int > max) {
        return null;
    }
    return @intCast(json_int);
}

/// 从 JSON 整数值安全转换为 u32（带范围验证）
///
/// 参数:
/// - **json_int**: JSON 整数值
/// - **max**: 最大值
///
/// 返回:
/// - ?u32: 如果值在范围内则返回转换结果，否则返回 null
pub fn safeU32FromJson(json_int: i64, max: u32) ?u32 {
    if (json_int < 0 or json_int > max) {
        return null;
    }
    return @intCast(json_int);
}

/// 从 JSON 整数值安全转换为 u64（带范围验证）
///
/// 参数:
/// - **json_int**: JSON 整数值
/// - **min**: 最小值
/// - **max**: 最大值
///
/// 返回:
/// - ?u64: 如果值在范围内则返回转换结果，否则返回 null
pub fn safeU64FromJson(json_int: i64, min: u64, max: u64) ?u64 {
    if (json_int < 0) return null;
    const val: u64 = @intCast(json_int);
    if (val < min or val > max) {
        return null;
    }
    return val;
}

/// 从 JSON 整数值安全转换为 i64（带范围验证）
///
/// 参数:
/// - **json_int**: JSON 整数值
/// - **min**: 最小值
/// - **max**: 最大值
///
/// 返回:
/// - ?i64: 如果值在范围内则返回转换结果，否则返回 null
pub fn safeI64FromJson(json_int: i64, min: i64, max: i64) ?i64 {
    if (json_int < min or json_int > max) {
        return null;
    }
    return json_int;
}
