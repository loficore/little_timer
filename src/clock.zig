//! 时钟模块
const std = @import("std");
const interface = @import("interface.zig");

pub const ClockInterface = interface.ClockInterface;
pub const ClockEvent = interface.ClockEvent;
pub const ModeEnumT = interface.ModeEnumT;
pub const ClockInterfaceUnion = interface.ClockInterfaceUnion;
pub const _ClockInterfaceData = interface._ClockInterfaceData;

var tick_count: usize = 0;

pub const ClockTaskConfig = interface.ClockTaskConfig;

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
                if (self.loop_remaining != 0) {
                    // 非零时代表需要扣次数，扣完为 0 时停止循环
                    if (self.loop_remaining > 0) self.loop_remaining -= 1;
                    if (self.loop_remaining == 0) {
                        self.loop_completed = true; // 标记循环已全部完成
                        return;
                    }
                }

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

    pub fn time(self: *WorldClockState) i64 {
        // 获取当前时间戳（纳秒）并转换为秒
        const now_ns = std.time.nanoTimestamp();
        const now_s = @as(i64, @intCast(@divFloor(now_ns, 1_000_000_000)));
        // 计算时区偏移（秒）
        const offset_seconds = @as(i64, @intCast(self.timezone)) * 3600;
        return now_s + offset_seconds;
    }
};

const ClockState = union(ModeEnumT) {
    COUNTDOWN_MODE: CountdownState,
    STOPWATCH_MODE: StopwatchState,
    WORLD_CLOCK_MODE: WorldClockState,
};

pub const ClockManager = struct {
    state: ClockState,
    last_tick_time: i64 = 0,
    display_data: _ClockInterfaceData = undefined, // 复用的显示数据
    initial_config: ClockTaskConfig, // 保存初始配置用于重置
    default_config: ClockTaskConfig,

    /// 初始化时钟管理器
    ///
    /// 参数:
    /// - **clock_config**: 时钟配置参数
    ///
    /// 返回:
    /// - ClockManager: 初始化后的时钟管理器实例
    pub fn init(clock_config: ClockTaskConfig) ClockManager {
        const state = switch (clock_config) {
            .countdown => ClockState{
                .COUNTDOWN_MODE = CountdownState{
                    .duration_ms = clock_config.countdown.duration_seconds * 1000,
                    .remaining_ms = @as(i64, @intCast(clock_config.countdown.duration_seconds * 1000)),
                    .loop = clock_config.countdown.loop,
                    .loop_interval_seconds = clock_config.countdown.loop_interval_seconds,
                    .loop_count = clock_config.countdown.loop_count,
                    .loop_remaining = clock_config.countdown.loop_count,
                },
            },
            .stopwatch => ClockState{
                .STOPWATCH_MODE = StopwatchState{
                    .esplased_ms = 0,
                    .max_ms = @as(i64, @intCast(clock_config.stopwatch.max_seconds * 1000)),
                },
            },
            .world_clock => ClockState{
                .WORLD_CLOCK_MODE = WorldClockState{
                    .timezone = clock_config.world_clock.timezone,
                },
            },
        };
        return .{
            .state = state,
            .initial_config = clock_config,
            .default_config = clock_config,
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
                    std.debug.print("[Tick] 剩余时间: {:.1} 秒，暂停状态: ", .{@as(f64, @floatFromInt(remaining_ms)) / 1000.0});
                    switch (self.state) {
                        .COUNTDOWN_MODE => std.debug.print("COUNTDOWN is_paused={}\n", .{self.state.COUNTDOWN_MODE.is_paused}),
                        .STOPWATCH_MODE => std.debug.print("STOPWATCH is_paused={}\n", .{self.state.STOPWATCH_MODE.is_paused}),
                        .WORLD_CLOCK_MODE => std.debug.print("WORLD_CLOCK\n", .{}),
                    }
                }
                self.OnTick(event);
            },
            .user_start_timer => {
                std.debug.print("Clock: 收到开始事件\n", .{});
                // 开始计时
                switch (self.state) {
                    .COUNTDOWN_MODE => {
                        std.debug.print("  设置暂停=false\n", .{});
                        self.state.COUNTDOWN_MODE.is_paused = false;
                    },
                    .STOPWATCH_MODE => {
                        self.state.STOPWATCH_MODE.is_paused = false;
                    },
                    .WORLD_CLOCK_MODE => {
                        // 世界时钟不支持开始/暂停
                    },
                }
            },
            .user_pause_timer => {
                std.debug.print("Clock: 收到暂停事件\n", .{});
                // 暂停计时
                switch (self.state) {
                    .COUNTDOWN_MODE => {
                        std.debug.print("  设置暂停=true\n", .{});
                        self.state.COUNTDOWN_MODE.is_paused = true;
                    },
                    .STOPWATCH_MODE => {
                        self.state.STOPWATCH_MODE.is_paused = true;
                    },
                    .WORLD_CLOCK_MODE => {
                        // 世界时钟不支持暂停
                    },
                }
            },
            .user_reset_timer => {
                std.debug.print("Clock: 收到重置事件\n", .{});
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
            .user_change_mode => {
                std.debug.print("Clock: 收到更改模式事件\n", .{});
                // 更改模式 - 暂未实现，预留接口
                const new_mode = event.user_change_mode;
                switch (new_mode) {
                    .COUNTDOWN_MODE => {},
                    .STOPWATCH_MODE => {},
                    .WORLD_CLOCK_MODE => {},
                }
            },
            .user_change_config => {
                std.debug.print("Clock: 收到更改配置事件\n", .{});
                // 更改配置
                const new_config = event.user_change_config;
                switch (new_config) {
                    .countdown => {
                        self.state = ClockState{
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
                        };
                        self.initial_config = event.user_change_config;
                    },
                    .stopwatch => {
                        self.state = ClockState{
                            .STOPWATCH_MODE = StopwatchState{
                                .esplased_ms = 0,
                                .max_ms = @as(i64, @intCast(new_config.stopwatch.max_seconds * 1000)),
                                .is_paused = true,
                            },
                        };
                        self.initial_config = event.user_change_config;
                    },
                    .world_clock => {
                        self.state = ClockState{
                            .WORLD_CLOCK_MODE = WorldClockState{
                                .timezone = new_config.world_clock.timezone,
                            },
                        };
                        self.initial_config = event.user_change_config;
                    },
                }
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
    /// - *ClockInterface: 时钟接口指针
    pub fn update(self: *ClockManager) *ClockInterface {
        switch (self.state) {
            .COUNTDOWN_MODE => {
                self.display_data.mode = ModeEnumT.COUNTDOWN_MODE;
                self.display_data.info = ClockInterfaceUnion{
                    .countdown = .{
                        .remaining_ms = self.state.COUNTDOWN_MODE.remaining_ms,
                        .duration_ms = self.state.COUNTDOWN_MODE.duration_ms,
                        .loop = self.state.COUNTDOWN_MODE.loop,
                        .loop_count = self.state.COUNTDOWN_MODE.loop_count,
                        .loop_interval_seconds = self.state.COUNTDOWN_MODE.loop_interval_seconds,
                    },
                };
            },
            .STOPWATCH_MODE => {
                self.display_data.mode = ModeEnumT.STOPWATCH_MODE;
                self.display_data.info = ClockInterfaceUnion{
                    .stopwatch = .{
                        .esplased_ms = self.state.STOPWATCH_MODE.esplased_ms,
                        .max_ms = self.state.STOPWATCH_MODE.max_ms,
                    },
                };
            },
            .WORLD_CLOCK_MODE => {
                self.display_data.mode = ModeEnumT.WORLD_CLOCK_MODE;
                self.display_data.info = ClockInterfaceUnion{
                    .worldclock = .{
                        .timezone = self.state.WORLD_CLOCK_MODE.timezone,
                        .time = self.state.WORLD_CLOCK_MODE.time(),
                    },
                };
            },
        }
        return @ptrCast(&self.display_data);
    }
};
