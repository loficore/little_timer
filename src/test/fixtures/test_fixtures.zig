//! 测试数据 fixtures
//! 职责：提供预设和设置的测试样本数据
const std = @import("std");
const interface = @import("../../core/interface.zig");

/// 预设 fixture 样本
pub const PresetFixtures = struct {
    /// 倒计时预设样本
    pub const countdown_preset = TimerPresetFixture{
        .name = "测试倒计时",
        .mode = "countdown",
        .config = .{
            .duration_seconds = 300,
            .loop = false,
            .loop_count = 0,
            .loop_interval_seconds = 60,
        },
    };

    /// 循环倒计时预设样本
    pub const looping_countdown_preset = TimerPresetFixture{
        .name = "番茄工作法",
        .mode = "countdown",
        .config = .{
            .duration_seconds = 1500,
            .loop = true,
            .loop_count = 4,
            .loop_interval_seconds = 300,
        },
    };

    /// 正计时预设样本
    pub const stopwatch_preset = TimerPresetFixture{
        .name = "测试正计时",
        .mode = "stopwatch",
        .config = .{
            .duration_seconds = 0,
            .loop = false,
            .loop_count = 0,
            .loop_interval_seconds = 0,
            .max_seconds = 3600,
        },
    };

    /// 世界时钟预设样本
    pub const world_clock_preset = TimerPresetFixture{
        .name = "世界时钟",
        .mode = "world_clock",
        .config = .{
            .duration_seconds = 0,
            .loop = false,
            .loop_count = 0,
            .loop_interval_seconds = 0,
            .timezone = 8,
        },
    };

    /// 包含特殊字符的预设名称
    pub const special_char_preset = TimerPresetFixture{
        .name = "测试-带横杠_下划线",
        .mode = "countdown",
        .config = .{
            .duration_seconds = 60,
            .loop = false,
            .loop_count = 0,
            .loop_interval_seconds = 0,
        },
    };

    /// 空名称测试（应该被拒绝）
    pub const empty_name_preset = TimerPresetFixture{
        .name = "",
        .mode = "countdown",
        .config = .{
            .duration_seconds = 60,
            .loop = false,
            .loop_count = 0,
            .loop_interval_seconds = 0,
        },
    };
};

/// 预设 fixture 结构
pub const TimerPresetFixture = struct {
    name: []const u8,
    mode: []const u8,
    config: PresetConfig,
};

/// 预设配置结构
pub const PresetConfig = struct {
    duration_seconds: u64 = 60,
    loop: bool = false,
    loop_count: u32 = 0,
    loop_interval_seconds: u64 = 60,
    max_seconds: u64 = 3600,
    timezone: i8 = 8,
};

/// 设置 fixture 样本
pub const SettingsFixtures = struct {
    /// 默认设置
    pub const default_settings = SettingsFixture{
        .timezone = 8,
        .language = "ZH",
        .default_mode = "countdown",
        .theme_mode = "system",
        .duration_seconds = 300,
        .countdown_loop = false,
        .countdown_loop_count = 0,
        .countdown_loop_interval = 60,
        .stopwatch_max_seconds = 3600,
        .log_level = "info",
        .log_enable_timestamp = true,
        .log_tick_interval = 100,
    };

    /// 英文环境设置
    pub const english_settings = SettingsFixture{
        .timezone = 0,
        .language = "EN",
        .default_mode = "stopwatch",
        .theme_mode = "dark",
        .duration_seconds = 600,
        .countdown_loop = true,
        .countdown_loop_count = 3,
        .countdown_loop_interval = 120,
        .stopwatch_max_seconds = 7200,
        .log_level = "debug",
        .log_enable_timestamp = false,
        .log_tick_interval = 50,
    };

    /// 边界时区设置
    pub const timezone_min = SettingsFixture{
        .timezone = -12,
        .language = "ZH",
        .default_mode = "countdown",
        .theme_mode = "light",
        .duration_seconds = 60,
        .countdown_loop = false,
        .countdown_loop_count = 0,
        .countdown_loop_interval = 0,
        .stopwatch_max_seconds = 1800,
        .log_level = "warn",
        .log_enable_timestamp = true,
        .log_tick_interval = 200,
    };

    pub const timezone_max = SettingsFixture{
        .timezone = 14,
        .language = "EN",
        .default_mode = "world_clock",
        .theme_mode = "system",
        .duration_seconds = 0,
        .countdown_loop = false,
        .countdown_loop_count = 0,
        .countdown_loop_interval = 0,
        .stopwatch_max_seconds = 0,
        .log_level = "error",
        .log_enable_timestamp = true,
        .log_tick_interval = 500,
    };
};

/// 设置 fixture 结构
pub const SettingsFixture = struct {
    timezone: i8,
    language: []const u8,
    default_mode: []const u8,
    theme_mode: []const u8,
    duration_seconds: u64,
    countdown_loop: bool,
    countdown_loop_count: u32,
    countdown_loop_interval: u64,
    stopwatch_max_seconds: u64,
    log_level: []const u8,
    log_enable_timestamp: bool,
    log_tick_interval: i64,
};

/// 生成预设 JSON 字符串
pub fn generatePresetJson(allocator: std.mem.Allocator, fixture: TimerPresetFixture) ![]const u8 {
    var list = try std.ArrayListAligned(u8, null).initCapacity(allocator, 0);
    const writer = list.writer(allocator);

    try writer.writeByte('{');
    try writer.print("{f}", .{std.json.fmt(fixture.name, .{})});
    try writer.writeByte(':');
    try writer.print("{f}", .{std.json.fmt(fixture.name, .{})});
    try writer.writeByte(',');

    try writer.writeAll("\"mode\":");
    try writer.print("{f}", .{std.json.fmt(fixture.mode, .{})});
    try writer.writeByte(',');

    try writer.writeAll("\"duration_seconds\":");
    try writer.print("{f}", .{std.json.fmt(fixture.config.duration_seconds, .{})});

    if (fixture.config.loop) {
        try writer.writeByte(',');
        try writer.writeAll("\"loop\":true");
    }

    if (fixture.config.loop_count > 0) {
        try writer.writeByte(',');
        try writer.print("\"loop_count\":{}", .{fixture.config.loop_count});
    }

    if (fixture.config.loop_interval_seconds > 0) {
        try writer.writeByte(',');
        try writer.print("\"loop_interval_seconds\":{}", .{fixture.config.loop_interval_seconds});
    }

    if (fixture.config.max_seconds > 0) {
        try writer.writeByte(',');
        try writer.print("\"max_seconds\":{}", .{fixture.config.max_seconds});
    }

    if (fixture.mode[0] == 'w') {
        try writer.writeByte(',');
        try writer.print("\"timezone\":{}", .{fixture.config.timezone});
    }

    try writer.writeByte('}');

    return list.toOwnedSlice(allocator);
}

/// 生成设置 JSON 字符串
pub fn generateSettingsJson(allocator: std.mem.Allocator, fixture: SettingsFixture) ![]const u8 {
    var list = try std.ArrayListAligned(u8, null).initCapacity(allocator, 0);
    const writer = list.writer(allocator);

    try writer.writeByte('{');

    try writer.writeAll("\"timezone\":");
    try writer.print("{},", .{fixture.timezone});

    try writer.writeAll("\"language\":");
    try writer.print("{f}", .{std.json.fmt(fixture.language, .{})});
    try writer.writeByte(',');

    try writer.writeAll("\"default_mode\":");
    try writer.print("{f}", .{std.json.fmt(fixture.default_mode, .{})});
    try writer.writeByte(',');

    try writer.writeAll("\"theme_mode\":");
    try writer.print("{f}", .{std.json.fmt(fixture.theme_mode, .{})});
    try writer.writeByte(',');

    try writer.writeAll("\"duration_seconds\":");
    try writer.print("{},", .{fixture.duration_seconds});

    try writer.writeAll("\"countdown_loop\":");
    try writer.writeByte(if (fixture.countdown_loop) 't' else 'f');
    try writer.writeByte(',');

    try writer.writeAll("\"countdown_loop_count\":");
    try writer.print("{},", .{fixture.countdown_loop_count});

    try writer.writeAll("\"countdown_loop_interval\":");
    try writer.print("{},", .{fixture.countdown_loop_interval});

    try writer.writeAll("\"stopwatch_max_seconds\":");
    try writer.print("{},", .{fixture.stopwatch_max_seconds});

    try writer.writeAll("\"log_level\":");
    try writer.print("{f}", .{std.json.fmt(fixture.log_level, .{})});
    try writer.writeByte(',');

    try writer.writeAll("\"log_enable_timestamp\":");
    try writer.writeByte(if (fixture.log_enable_timestamp) 't' else 'f');
    try writer.writeByte(',');

    try writer.writeAll("\"log_tick_interval\":");
    try writer.print("{}", .{fixture.log_tick_interval});

    try writer.writeByte('}');

    return list.toOwnedSlice(allocator);
}

/// 将字符串模式转换为 ModeEnumT 枚举
fn stringToModeEnum(mode_str: []const u8) interface.ModeEnumT {
    if (std.mem.eql(u8, mode_str, "countdown")) return .COUNTDOWN_MODE;
    if (std.mem.eql(u8, mode_str, "stopwatch")) return .STOPWATCH_MODE;
    if (std.mem.eql(u8, mode_str, "world_clock")) return .WORLD_CLOCK_MODE;
    return .COUNTDOWN_MODE;
}

/// 将 TimerPresetFixture 转换为 interface.TimerPreset
pub fn fixtureToTimerPreset(allocator: std.mem.Allocator, fixture: TimerPresetFixture) !interface.TimerPreset {
    return interface.TimerPreset{
        .name = try allocator.dupe(u8, fixture.name),
        .mode = stringToModeEnum(fixture.mode),
        .config = .{
            .countdown = .{
                .duration_seconds = fixture.config.duration_seconds,
                .loop = fixture.config.loop,
                .loop_count = fixture.config.loop_count,
                .loop_interval_seconds = fixture.config.loop_interval_seconds,
            },
            .stopwatch = .{
                .max_seconds = fixture.config.max_seconds,
            },
            .world_clock = .{
                .timezone = fixture.config.timezone,
            },
        },
    };
}
