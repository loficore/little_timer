//! 时钟模块
const std = @import("std");
const interface = @import("interface.zig");
const logger = @import("logger.zig");

pub const ClockEvent = interface.ClockEvent;
pub const ModeEnumT = interface.ModeEnumT;
pub const ClockTaskConfig = interface.ClockTaskConfig;

var tick_count: usize = 0;

const CountdownState = struct {
    duration_ms: u64, // 配置的总时长
    remaining_ms: i64,
    loop: bool,
    loop_interval_seconds: u64,
    loop_count: u32,
    loop_remaining: u32, // 0 表示无限循环
    loop_completed: bool = false, // 标记循环是否全部完成
    in_rest: bool = false,
    rest_remaining_ms: i64 = 0,
    is_paused: bool = true,
    is_finished: bool = false,

    /// 更新倒计时状态
    ///
    /// 参数:
    /// - **self**: CountdownState 实例指针
    /// - **delta_ms**: 增加的毫秒数
    pub fn tick(self: *CountdownState, delta_ms: i64) void {
        if (self.is_paused or self.is_finished) return;

        // 休息阶段倒计时
        if (self.in_rest) {
            self.rest_remaining_ms -= delta_ms;
            if (self.rest_remaining_ms <= 0) {
                self.in_rest = false;
                self.is_finished = false;
                self.remaining_ms = @as(i64, @intCast(self.duration_ms));
            }
            return;
        }

        self.remaining_ms -= delta_ms;
        if (self.remaining_ms <= 0) {
            self.remaining_ms = 0;
            self.is_finished = true;

            // 循环逻辑
            if (self.loop) {
                // 只在有限循环模式下扣减次数
                // loop_count > 0 时为有限循环，loop_count == 0 时为无限循环
                if (self.loop_count > 0) {
                    // 有限循环：扣减剩余次数
                    if (self.loop_remaining > 0) self.loop_remaining -= 1;
                    // 如果剩余次数为 0，停止循环
                    if (self.loop_remaining == 0) {
                        self.loop_completed = true; // 标记循环已全部完成
                        return;
                    }
                }
                // 无限循环（loop_count == 0）或还有剩余次数，继续循环

                if (self.loop_interval_seconds > 0) {
                    self.in_rest = true;
                    self.rest_remaining_ms = @as(i64, @intCast(self.loop_interval_seconds * 1000));
                    self.is_finished = false; // 休息中视为未结束，继续流转
                } else {
                    // 无休息直接进入下一轮
                    self.is_finished = false;
                    self.remaining_ms = @as(i64, @intCast(self.duration_ms));
                }
            }
        }
    }
};

const StopwatchState = struct {
    esplased_ms: i64 = 0,
    max_ms: i64,
    is_paused: bool = true,
    is_finished: bool = false,

    /// 更新秒表状态
    ///
    /// 参数:
    /// - **self**: StopwatchState 实例指针
    /// - **delta_ms**: 增加的毫秒数
    pub fn tick(self: *StopwatchState, delta_ms: i64) void {
        if (self.is_paused or self.is_finished) return;

        self.esplased_ms += delta_ms;
        if (self.esplased_ms >= self.max_ms) {
            self.esplased_ms = self.max_ms;
            self.is_finished = true;
        }
    }
};

const WorldClockState = struct {
    timezone: i8 = 8,

    pub fn time(self: *const WorldClockState) i64 {
        // 获取当前时间戳（纳秒）并转换为秒
        const now_ns = std.time.nanoTimestamp();
        const now_s = @as(i64, @intCast(@divFloor(now_ns, 1_000_000_000)));
        // 计算时区偏移（秒）
        const offset_seconds = @as(i64, @intCast(self.timezone)) * 3600;
        return now_s + offset_seconds;
    }
};

pub const ClockState = union(ModeEnumT) {
    COUNTDOWN_MODE: CountdownState,
    STOPWATCH_MODE: StopwatchState,
    WORLD_CLOCK_MODE: WorldClockState,

    /// 获取时间信息（秒数）
    pub fn getTimeInfo(self: *const ClockState) i64 {
        return switch (self.*) {
            .COUNTDOWN_MODE => |*countdown| @divTrunc(countdown.remaining_ms, 1000),
            .STOPWATCH_MODE => |*stopwatch| @divTrunc(stopwatch.esplased_ms, 1000),
            .WORLD_CLOCK_MODE => |*worldclock| worldclock.time(),
        };
    }

    /// 获取时钟模式
    pub fn getMode(self: *const ClockState) ModeEnumT {
        return self.*;
    }

    /// 检查时钟是否暂停
    pub fn isPaused(self: *const ClockState) bool {
        return switch (self.*) {
            .COUNTDOWN_MODE => |*countdown| countdown.is_paused,
            .STOPWATCH_MODE => |*stopwatch| stopwatch.is_paused,
            .WORLD_CLOCK_MODE => false, // 世界时钟不会暂停
        };
    }

    /// 检查时钟是否结束
    pub fn isFinished(self: *const ClockState) bool {
        return switch (self.*) {
            .COUNTDOWN_MODE => |*countdown| countdown.is_finished,
            .STOPWATCH_MODE => |*stopwatch| stopwatch.is_finished,
            .WORLD_CLOCK_MODE => false, // 世界时钟不会结束
        };
    }

    /// 检查时钟是否在休息中（仅倒计时支持）
    pub fn inRest(self: *const ClockState) bool {
        return switch (self.*) {
            .COUNTDOWN_MODE => |*countdown| countdown.in_rest,
            .STOPWATCH_MODE => false, // 正计时不支持休息
            .WORLD_CLOCK_MODE => false,
        };
    }

    /// 获取剩余休息时间（秒数），仅倒计时支持
    pub fn getRestRemainingTime(self: *const ClockState) i64 {
        return switch (self.*) {
            .COUNTDOWN_MODE => |*countdown| @divTrunc(countdown.rest_remaining_ms, 1000),
            .STOPWATCH_MODE => 0, // 正计时不支持休息
            .WORLD_CLOCK_MODE => 0,
        };
    }

    /// 获取循环剩余次数，仅倒计时支持
    pub fn getLoopRemaining(self: *const ClockState) u32 {
        return switch (self.*) {
            .COUNTDOWN_MODE => |*countdown| countdown.loop_remaining,
            .STOPWATCH_MODE => 0,
            .WORLD_CLOCK_MODE => 0,
        };
    }

    /// 获取配置的循环总次数，仅倒计时支持
    pub fn getLoopTotal(self: *const ClockState) u32 {
        return switch (self.*) {
            .COUNTDOWN_MODE => |*countdown| countdown.loop_count,
            .STOPWATCH_MODE => 0,
            .WORLD_CLOCK_MODE => 0,
        };
    }
};

pub const ClockManager = struct {
    state: ClockState,
    last_tick_time: i64 = 0,
    initial_config: ClockTaskConfig, // 保存初始配置用于重置

    /// 初始化时钟管理器
    ///
    /// 参数:
    /// - **clock_config**: 初始时钟配置参数（包含 default_mode）
    ///
    /// 返回:
    /// - ClockManager: 初始化后的时钟管理器实例
    pub fn init(
        clock_config: ClockTaskConfig,
    ) ClockManager {
        // 根据 clock_config.default_mode 决定初始化哪种模式
        const state = switch (clock_config.default_mode) {
            .COUNTDOWN_MODE => ClockState{
                .COUNTDOWN_MODE = CountdownState{
                    .duration_ms = clock_config.countdown.duration_seconds * 1000,
                    .remaining_ms = @as(i64, @intCast(clock_config.countdown.duration_seconds * 1000)),
                    .loop = clock_config.countdown.loop,
                    .loop_interval_seconds = clock_config.countdown.loop_interval_seconds,
                    .loop_count = clock_config.countdown.loop_count,
                    .loop_remaining = clock_config.countdown.loop_count,
                },
            },
            .STOPWATCH_MODE => ClockState{
                .STOPWATCH_MODE = StopwatchState{
                    .esplased_ms = 0,
                    .max_ms = @as(i64, @intCast(clock_config.stopwatch.max_seconds * 1000)),
                    .is_paused = true,
                },
            },
            .WORLD_CLOCK_MODE => ClockState{
                .WORLD_CLOCK_MODE = WorldClockState{
                    .timezone = clock_config.world_clock.timezone,
                },
            },
        };
        return .{
            .state = state,
            .initial_config = clock_config,
        };
    }

    /// 处理时钟事件
    ///
    /// 参数:
    /// - **self**: ClockManager 实例指针
    /// - **event**: 时钟事件
    pub fn handleEvent(self: *ClockManager, event: ClockEvent) void {
        switch (event) {
            .tick => {
                // 每隔几个 tick 打印一次剩余时间（减少日志量）
                tick_count += 1;
                if (tick_count % 10 == 0) {
                    // 获取当前时间用于调试
                    const display = self.update();
                    const remaining_ms = display.getTimeInfo() * 1000;
                    switch (self.state) {
                        .COUNTDOWN_MODE => logger.global_logger.debug("[Tick] 剩余时间: {d:.1} 秒，暂停状态: COUNTDOWN is_paused={}", .{
                            @as(f64, @floatFromInt(remaining_ms)) / 1000.0,
                            self.state.COUNTDOWN_MODE.is_paused,
                        }),
                        .STOPWATCH_MODE => logger.global_logger.debug("[Tick] 已经过时间: {d:.1} 秒，暂停状态: STOPWATCH is_paused={}", .{
                            @as(f64, @floatFromInt(remaining_ms)) / 1000.0,
                            self.state.STOPWATCH_MODE.is_paused,
                        }),
                        .WORLD_CLOCK_MODE => logger.global_logger.debug("[Tick] 世界时钟模式", .{}),
                    }
                }
                self.OnTick(event);
            },
            .user_start_timer => {
                switch (self.state) {
                    .COUNTDOWN_MODE => {
                        if (self.state.COUNTDOWN_MODE.is_paused) {
                            self.state.COUNTDOWN_MODE.is_paused = false;
                            logger.global_logger.info("Clock: 倒计时已启动", .{});
                        } else {
                            logger.global_logger.debug("Clock: 倒计时已在运行，忽略重复启动事件", .{});
                        }
                    },
                    .STOPWATCH_MODE => {
                        if (self.state.STOPWATCH_MODE.is_paused) {
                            self.state.STOPWATCH_MODE.is_paused = false;
                            logger.global_logger.info("Clock: 秒表已启动", .{});
                        } else {
                            logger.global_logger.debug("Clock: 秒表已在运行，忽略重复启动事件", .{});
                        }
                    },
                    .WORLD_CLOCK_MODE => {
                        // 世界时钟不支持开始/暂停
                    },
                }
            },
            .user_pause_timer => {
                switch (self.state) {
                    .COUNTDOWN_MODE => {
                        if (!self.state.COUNTDOWN_MODE.is_paused) {
                            self.state.COUNTDOWN_MODE.is_paused = true;
                            logger.global_logger.info("Clock: 倒计时已暂停", .{});
                        } else {
                            logger.global_logger.debug("Clock: 倒计时已暂停，忽略重复暂停事件", .{});
                        }
                    },
                    .STOPWATCH_MODE => {
                        if (!self.state.STOPWATCH_MODE.is_paused) {
                            self.state.STOPWATCH_MODE.is_paused = true;
                            logger.global_logger.info("Clock: 秒表已暂停", .{});
                        } else {
                            logger.global_logger.debug("Clock: 秒表已暂停，忽略重复暂停事件", .{});
                        }
                    },
                    .WORLD_CLOCK_MODE => {
                        // 世界时钟不支持暂停
                    },
                }
            },
            .user_reset_timer => {
                logger.global_logger.info("Clock: 收到重置事件", .{});
                // 重置计时器
                switch (self.state) {
                    .COUNTDOWN_MODE => {
                        self.state.COUNTDOWN_MODE.remaining_ms = @as(i64, @intCast(self.initial_config.countdown.duration_seconds * 1000));
                        self.state.COUNTDOWN_MODE.loop_remaining = self.initial_config.countdown.loop_count;
                        self.state.COUNTDOWN_MODE.loop_completed = false;
                        self.state.COUNTDOWN_MODE.in_rest = false;
                        self.state.COUNTDOWN_MODE.rest_remaining_ms = 0;
                        self.state.COUNTDOWN_MODE.is_paused = true;
                        self.state.COUNTDOWN_MODE.is_finished = false;
                    },
                    .STOPWATCH_MODE => {
                        self.state.STOPWATCH_MODE.esplased_ms = 0;
                        self.state.STOPWATCH_MODE.is_paused = true;
                        self.state.STOPWATCH_MODE.is_finished = false;
                    },
                    .WORLD_CLOCK_MODE => {
                        // 世界时钟不支持重置
                    },
                }
            },
            .user_change_mode => |new_mode| {
                logger.global_logger.info("Clock: 切换模式到 {}", .{new_mode});
                // 根据新模式创建默认状态
                switch (new_mode) {
                    .COUNTDOWN_MODE => {
                        // 切换到倒计时模式：使用默认配置（25分钟）
                        const default_duration_seconds: u64 = 25 * 60;
                        self.state = ClockState{
                            .COUNTDOWN_MODE = CountdownState{
                                .duration_ms = default_duration_seconds * 1000,
                                .remaining_ms = @as(i64, @intCast(default_duration_seconds * 1000)),
                                .loop = false,
                                .loop_interval_seconds = 0,
                                .loop_count = 0,
                                .loop_remaining = 0,
                                .loop_completed = false,
                                .is_paused = true,
                            },
                        };
                        self.initial_config = .{
                            .countdown = .{
                                .duration_seconds = default_duration_seconds,
                                .loop = false,
                                .loop_interval_seconds = 0,
                                .loop_count = 0,
                            },
                            .stopwatch = .{ .max_seconds = 24 * 60 * 60 },
                            .world_clock = .{ .timezone = 8 },
                        };
                    },
                    .STOPWATCH_MODE => {
                        // 切换到秒表模式：使用默认配置（24小时上限）
                        const default_max_seconds: u64 = 24 * 60 * 60;
                        self.state = ClockState{
                            .STOPWATCH_MODE = StopwatchState{
                                .esplased_ms = 0,
                                .max_ms = @as(i64, @intCast(default_max_seconds * 1000)),
                                .is_paused = true,
                            },
                        };
                        self.initial_config = .{
                            .countdown = .{ .duration_seconds = 25 * 60, .loop = false, .loop_interval_seconds = 0, .loop_count = 0 },
                            .stopwatch = .{
                                .max_seconds = default_max_seconds,
                            },
                            .world_clock = .{ .timezone = 8 },
                        };
                    },
                    .WORLD_CLOCK_MODE => {
                        // 切换到世界时钟模式：使用默认时区（东八区）
                        const default_timezone: i8 = 8;
                        self.state = ClockState{
                            .WORLD_CLOCK_MODE = WorldClockState{
                                .timezone = default_timezone,
                            },
                        };
                        self.initial_config = .{
                            .countdown = .{ .duration_seconds = 25 * 60, .loop = false, .loop_interval_seconds = 0, .loop_count = 0 },
                            .stopwatch = .{ .max_seconds = 24 * 60 * 60 },
                            .world_clock = .{
                                .timezone = default_timezone,
                            },
                        };
                    },
                }
            },
            .user_change_config => {
                logger.global_logger.info("Clock: 收到更改配置事件", .{});
                // 根据 new_config 的 default_mode 切换到相应的模式
                const new_config = event.user_change_config;
                self.state = switch (new_config.default_mode) {
                    .COUNTDOWN_MODE => ClockState{
                        .COUNTDOWN_MODE = CountdownState{
                            .duration_ms = new_config.countdown.duration_seconds * 1000,
                            .remaining_ms = @as(i64, @intCast(new_config.countdown.duration_seconds * 1000)),
                            .loop = new_config.countdown.loop,
                            .loop_interval_seconds = new_config.countdown.loop_interval_seconds,
                            .loop_count = new_config.countdown.loop_count,
                            .loop_remaining = new_config.countdown.loop_count,
                            .loop_completed = false,
                            .is_paused = true,
                        },
                    },
                    .STOPWATCH_MODE => ClockState{
                        .STOPWATCH_MODE = StopwatchState{
                            .esplased_ms = 0,
                            .max_ms = @as(i64, @intCast(new_config.stopwatch.max_seconds * 1000)),
                            .is_paused = true,
                        },
                    },
                    .WORLD_CLOCK_MODE => ClockState{
                        .WORLD_CLOCK_MODE = WorldClockState{
                            .timezone = new_config.world_clock.timezone,
                        },
                    },
                };
                self.initial_config = event.user_change_config;
            },
        }
    }

    /// 处理 tick 事件
    ///
    /// 参数:
    /// - **self**: ClockManager 实例指针
    /// - **tick**: tick 事件
    fn OnTick(self: *ClockManager, tick: ClockEvent) void {
        switch (self.state) {
            .COUNTDOWN_MODE => {
                self.state.COUNTDOWN_MODE.tick(tick.tick);
            },
            .STOPWATCH_MODE => {
                // TODO: 实现正计时逻辑
                self.state.STOPWATCH_MODE.tick(tick.tick);
            },
            .WORLD_CLOCK_MODE => {
                // 世界时钟不需要 tick
            },
        }
    }

    /// 获取时钟显示数据
    ///
    /// 参数:
    /// - **self**: ClockManager 实例指针
    ///
    /// 返回:
    /// - *ClockState: 时钟状态指针
    pub fn update(self: *ClockManager) *ClockState {
        return &self.state;
    }

    /// 销毁时钟管理器
    ///
    /// 参数:
    /// - **self**: ClockManager 实例指针
    ///
    /// Note: 目前无动态资源需要释放，仅作为占位符
    pub fn deinit(self: *ClockManager) void {
        _ = self;
    }
};
