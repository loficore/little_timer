//! 公共接口模块
const std = @import("std");

pub const ModeEnumT = enum {
    COUNTDOWN_MODE,
    STOPWATCH_MODE,
    WORLD_CLOCK_MODE,
};

pub const ClockInterfaceUnion = union(enum) {
    countdown: struct {
        // UI 显示信息
        remaining_ms: i64,
        // 配置项（可被修改）
        duration_ms: u64,
        loop: bool,
        loop_count: u32, // 0 表示无限循环
        loop_interval_seconds: u64,
    },
    stopwatch: struct {
        esplased_ms: i64,
        max_ms: i64,
    },
    worldclock: struct {
        timezone: i8 = 8,
        time: i64,
    },
};

/// 私有的接口数据结构,仅允许用于Clock和interface模块内部
pub const _ClockInterfaceData = struct {
    mode: ModeEnumT,
    info: ClockInterfaceUnion,
};

/// 时钟任务配置，供设置/持久化等模块共享
pub const ClockTaskConfig = union(enum) {
    countdown: struct {
        duration_seconds: u64 = 25 * 60, // 倒计时秒
        loop: bool = false, // 是否循环倒计时
        loop_interval_seconds: u64 = 0, // 循环间隔休息秒数，暂不实现
        loop_count: u32 = 0, // 循环次数，0 表示无限循环，暂不实现
    },

    stopwatch: struct {
        max_seconds: u64 = 24 * 60 * 60, // 正计时上限（秒），默认一天
    },

    world_clock: struct {
        timezone: i8 = 8, // 默认东八区
    },
};

/// 时钟接口 - 隐藏实现细节
pub const ClockInterface = opaque {
    /// 获取时间信息（秒数）
    ///
    /// 参数:
    /// - **self**: ClockInterface 实例指针
    ///
    /// 返回:
    /// - i64: 时间信息（秒数）
    pub fn getTimeInfo(self: *ClockInterface) i64 {
        const data: *_ClockInterfaceData = @ptrCast(@alignCast(self));
        return switch (data.mode) {
            .COUNTDOWN_MODE => @divTrunc(data.info.countdown.remaining_ms, 1000),
            .STOPWATCH_MODE => @divTrunc(data.info.stopwatch.esplased_ms, 1000),
            .WORLD_CLOCK_MODE => data.info.worldclock.time,
        };
    }

    /// 获取时钟模式
    ///
    /// 参数:
    /// - **self**: ClockInterface 实例指针
    ///
    /// 返回:
    /// - ModeEnumT: 时钟模式
    pub fn getMode(self: *ClockInterface) ModeEnumT {
        const data: *_ClockInterfaceData = @ptrCast(@alignCast(self));
        return data.mode;
    }

    /// 检查时钟是否暂停
    ///
    /// 参数:
    /// - **self**: ClockInterface 实例指针
    ///
    /// 返回:
    /// - bool: 是否暂停（true 表示暂停）
    pub fn isPaused(self: *ClockInterface) bool {
        const data: *_ClockInterfaceData = @ptrCast(@alignCast(self));
        return switch (data.mode) {
            .COUNTDOWN_MODE => data.info.countdown.is_paused,
            .STOPWATCH_MODE => data.info.stopwatch.is_paused,
            .WORLD_CLOCK_MODE => false, // 世界时钟不会暂停
        };
    }

    /// 检查时钟是否结束
    ///
    /// 参数:
    /// - **self**: ClockInterface 实例指针
    ///
    /// 返回:
    /// - bool: 是否结束（true 表示结束）
    pub fn isFinished(self: *ClockInterface) bool {
        const data: *_ClockInterfaceData = @ptrCast(@alignCast(self));
        return switch (data.mode) {
            .COUNTDOWN_MODE => data.info.countdown.is_finished,
            .STOPWATCH_MODE => data.info.stopwatch.is_finished,
            .WORLD_CLOCK_MODE => false, // 世界时钟不会结束
        };
    }

    /// 检查时钟是否在休息中（仅倒计时支持）
    ///
    /// 参数:
    /// - **self**: ClockInterface 实例指针
    ///
    /// 返回:
    /// - bool: 是否在休息（true 表示休息中）
    pub fn inRest(self: *ClockInterface) bool {
        const data: *_ClockInterfaceData = @ptrCast(@alignCast(self));
        return switch (data.mode) {
            .COUNTDOWN_MODE => data.info.countdown.in_rest,
            .STOPWATCH_MODE => false, // 正计时不支持休息
            .WORLD_CLOCK_MODE => false,
        };
    }

    /// 获取剩余休息时间（秒数），仅倒计时支持
    ///
    /// 参数:
    /// - **self**: ClockInterface 实例指针
    ///
    /// 返回:
    /// - i64: 剩余休息时间（秒数），未在休息时返回 0
    pub fn getRestRemainingTime(self: *ClockInterface) i64 {
        const data: *_ClockInterfaceData = @ptrCast(@alignCast(self));
        return switch (data.mode) {
            .COUNTDOWN_MODE => @divTrunc(data.info.countdown.rest_remaining_ms, 1000),
            .STOPWATCH_MODE => 0, // 正计时不支持休息
            .WORLD_CLOCK_MODE => 0,
        };
    }

    /// 获取循环剩余次数，仅倒计时支持
    ///
    /// 参数:
    /// - **self**: ClockInterface 实例指针
    ///
    /// 返回:
    /// - u32: 剩余循环次数，0 表示无限循环
    pub fn getLoopRemaining(self: *ClockInterface) u32 {
        const data: *_ClockInterfaceData = @ptrCast(@alignCast(self));
        return switch (data.mode) {
            .COUNTDOWN_MODE => data.info.countdown.loop_remaining,
            .STOPWATCH_MODE => 0,
            .WORLD_CLOCK_MODE => 0,
        };
    }

    /// 获取配置的循环总次数，仅倒计时支持
    ///
    /// 参数:
    /// - **self**: ClockInterface 实例指针
    ///
    /// 返回:
    /// - u32: 循环总次数，0 表示无限循环
    pub fn getLoopTotal(self: *ClockInterface) u32 {
        const data: *_ClockInterfaceData = @ptrCast(@alignCast(self));
        return switch (data.mode) {
            .COUNTDOWN_MODE => data.info.countdown.loop_count,
            .STOPWATCH_MODE => 0,
            .WORLD_CLOCK_MODE => 0,
        };
    }
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
    config: ClockTaskConfig, // 预设配置
};

/// 应用设置配置
pub const SettingsConfig = struct {
    /// 基本设置
    basic: struct {
        timezone: i8 = 8, // 时区，默认东八区
        language: []const u8 = "ZH", // 语言，默认中文
    } = .{},

    /// 默认时钟设置（与 ClockInterfaceUnion 中的可配置项对齐）
    clock_defaults: struct {
        countdown: struct {
            duration_ms: u64 = 25 * 60 * 1000, // 默认 25 分钟（毫秒）
            loop: bool = false,
            loop_count: u32 = 0, // 0 表示无限循环
            loop_interval_seconds: u64 = 0,
        } = .{},
        stopwatch: struct {
            max_ms: u64 = 24 * 60 * 60 * 1000, // 默认一天（毫秒）
        } = .{},
    } = .{},

    /// 定时器预设列表
    timer_presets: []TimerPreset = &[_]TimerPreset{},
};

pub const SettingsEvent = union(enum) {
    load_settings: void, // 加载设置
    save_settings: SettingsConfig, // 保存设置
    update_basic: struct {
        timezone: ?i8 = null,
        language: ?[]const u8 = null,
    }, // 更新基本设置
    update_clock_defaults: ClockTaskConfig, // 更新默认时钟配置
    add_preset: TimerPreset, // 添加预设
    remove_preset: []const u8, // 删除预设（按名称）
};

pub const SettingsInterface = opaque {};
