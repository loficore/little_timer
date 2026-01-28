const std = @import("std");
const validator = @import("../settings_validator.zig");

// 验证范围测试
test "validate timezone and language" {
    try validator.validateTimezone(0);
    try validator.validateTimezone(-12);
    try validator.validateTimezone(14);
    try std.testing.expectError(error.InvalidTimezone, validator.validateTimezone(-13));
    try std.testing.expectError(error.InvalidTimezone, validator.validateTimezone(15));

    try validator.validateLanguage("ZH");
    try std.testing.expectError(error.InvalidLanguage, validator.validateLanguage(""));
    try std.testing.expectError(error.InvalidLanguage, validator.validateLanguage("ABCDEFGHIJK"));
}

// 倒计时与循环参数测试
test "validate countdown params" {
    try validator.validateDuration(1);
    try validator.validateDuration(86400);
    try std.testing.expectError(error.InvalidDuration, validator.validateDuration(0));
    try std.testing.expectError(error.InvalidDuration, validator.validateDuration(86401));

    try validator.validateLoopCount(0);
    try validator.validateLoopCount(1000);
    try std.testing.expectError(error.InvalidLoopCount, validator.validateLoopCount(1001));

    try validator.validateLoopInterval(0);
    try validator.validateLoopInterval(3600);
    try std.testing.expectError(error.InvalidLoopInterval, validator.validateLoopInterval(3601));
}

// 秒表与 tick 参数测试
test "validate stopwatch and tick" {
    try validator.validateMaxSeconds(1);
    try validator.validateMaxSeconds(86400 * 365);
    try std.testing.expectError(error.InvalidMaxSeconds, validator.validateMaxSeconds(0));
    try std.testing.expectError(error.InvalidMaxSeconds, validator.validateMaxSeconds(86400 * 365 + 1));

    try validator.validateTickInterval(100);
    try validator.validateTickInterval(5000);
    try std.testing.expectError(error.InvalidTickInterval, validator.validateTickInterval(99));
    try std.testing.expectError(error.InvalidTickInterval, validator.validateTickInterval(5001));
}

// 预设数量与名称测试
test "validate preset name and count" {
    try validator.validatePresetName("focus");
    try std.testing.expectError(error.InvalidPresetName, validator.validatePresetName(""));
    try std.testing.expectError(error.InvalidPresetName, validator.validatePresetName("ABCDEFGHIJKLMNOPQRSTUVWXYZABCDEFGHIJKLMNOPQRSTUVWXYZABCDEFGHIJKLMNOPQRSTUVWXYZ"));

    try validator.validatePresetCount(0);
    try validator.validatePresetCount(999);
    try std.testing.expectError(error.PresetLimitExceeded, validator.validatePresetCount(1000));
}
