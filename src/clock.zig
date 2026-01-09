//! 时钟模块
const std = @import("std");
const interface = @import("interface.zig");

// 重新导出接口模块的公共类型
pub const ClockInterfaceT = interface.ClockInterfaceT;
pub const ClockEvent = interface.ClockEvent;
pub const ModeEnumT = interface.ModeEnumT;
pub const ClockInterfaceUnion = interface.ClockInterfaceUnion;
pub const ClockInterfaceData = interface.ClockInterfaceData;

// 用于 tick 计数的全局变量（每隔10个 tick 打印一次日志）
var tick_count: usize = 0;

// 导出配置类型
pub const ClockTaskConfigT = union(enum) {
    countdown: struct {
        duration_seconds: u64 = 25 * 60, //倒计时秒
        loop: bool = false, //是否循环倒计时
        loop_interval_break_seconds: u64 = 0, //循环间隔休息秒数,暂时不实现
        loop_count: u32 = 0, //循环次数,0表示无限循环，暂时不实现
    },

    stopwatch: struct {
        max_seconds: u64, //正计时秒数
    },

    world_clock: struct {
        timezone: i8 = 8, // 默认东八区
    },
};

const CountdownState = struct {
    remaining_ms: i64, // 剩余毫秒数
    is_paused: bool = true, // 是否暂停
    is_finished: bool = false,

    // 逻辑：更新时间
    pub fn tick(self: *CountdownState, delta_ms: i64) void {
        if (self.is_paused or self.is_finished) return;
        self.remaining_ms -= delta_ms;
        if (self.remaining_ms <= 0) {
            self.remaining_ms = 0;
            self.is_finished = true;
        }
    }
};

const StopwatchState = struct {
    esplased_ms: i64,
    max_ms: i64,
    is_paused: bool = true, // 是否暂停
    is_finished: bool = false,

    pub fn tick(self: *StopwatchState, delta_ms: i64) void {
        if (self.is_paused or self.is_finished) return;
        self.esplased_ms += delta_ms;
        if (self.esplased_ms > self.max_ms) {
            self.esplased_ms = self.max_ms;
            self.is_finished = true;
        }
    }
};

// 以上类型从 interface.zig 重新导出
const ClockState = union(ModeEnumT) {
    COUNTDOWN_MODE: CountdownState,
    STOPWATCH_MODE: StopwatchState,
    WORLD_CLOCK_MODE: void,
};

pub const ClockManager = struct {
    state: ClockState,
    last_tick_time: i64 = 0,
    display_data: ClockInterfaceData = undefined, // 复用的显示数据
    initial_config: ClockTaskConfigT, // 保存初始配置用于重置

    pub fn init(clock_config: ClockTaskConfigT) ClockManager {
        const state = switch (clock_config) {
            .countdown => ClockState{
                .COUNTDOWN_MODE = CountdownState{
                    .remaining_ms = @as(i64, @intCast(clock_config.countdown.duration_seconds * 1000)),
                    .is_paused = true,
                },
            },
            .stopwatch => ClockState{
                .STOPWATCH_MODE = StopwatchState{
                    .esplased_ms = @as(i64, @intCast(clock_config.stopwatch.max_seconds * 1000)),
                    .max_ms = @as(i64, @intCast(clock_config.stopwatch.max_seconds * 1000)),
                },
            },
            .world_clock => ClockState{
                .WORLD_CLOCK_MODE = {},
            },
        };
        return .{
            .state = state,
            .initial_config = clock_config,
        };
    }

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
            .user_set_duration => |duration| {
                // 处理设置时长事件
                // 如果duration为0，则重置到初始配置
                switch (self.state) {
                    .COUNTDOWN_MODE => {
                        if (duration == 0) {
                            // 重置到初始配置 - 暂停状态，完整时长
                            self.state.COUNTDOWN_MODE.remaining_ms = @as(i64, @intCast(self.initial_config.countdown.duration_seconds * 1000));
                            self.state.COUNTDOWN_MODE.is_paused = true;
                            self.state.COUNTDOWN_MODE.is_finished = false;
                        } else {
                            // 设置新的倒计时时间
                            self.state.COUNTDOWN_MODE.remaining_ms = @as(i64, @intCast(duration * 1000));
                            self.state.COUNTDOWN_MODE.is_paused = true; // 设置新时间后暂停
                            self.state.COUNTDOWN_MODE.is_finished = false;
                        }
                    },
                    .STOPWATCH_MODE => {
                        // 对于秒表，设置最大时间
                        if (duration == 0) {
                            // 重置秒表
                            self.state.STOPWATCH_MODE.esplased_ms = 0;
                            self.state.STOPWATCH_MODE.max_ms = @as(i64, @intCast(self.initial_config.stopwatch.max_seconds * 1000));
                            self.state.STOPWATCH_MODE.is_paused = true;
                            self.state.STOPWATCH_MODE.is_finished = false;
                        } else {
                            // 设置新的最大时间
                            self.state.STOPWATCH_MODE.max_ms = @as(i64, @intCast(duration * 1000));
                            self.state.STOPWATCH_MODE.is_paused = true; // 设置新时间后暂停
                            self.state.STOPWATCH_MODE.is_finished = false;
                        }
                    },
                    .WORLD_CLOCK_MODE => {
                        // 世界时钟不支持设置时长
                    },
                }
            },
            .system_low_battery => {
                // TODO: 处理电量低事件
            },
        }
    }

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

    /// 对外部更新数据的函数
    /// 返回内部数据的指针，无需分配和释放
    pub fn update(self: *ClockManager) *ClockInterfaceT {
        switch (self.state) {
            .COUNTDOWN_MODE => {
                self.display_data.mode = ModeEnumT.COUNTDOWN_MODE;
                self.display_data.info = ClockInterfaceUnion{
                    .countdown = .{
                        .remaining_ms = self.state.COUNTDOWN_MODE.remaining_ms,
                        .is_paused = self.state.COUNTDOWN_MODE.is_paused,
                        .is_finished = self.state.COUNTDOWN_MODE.is_finished,
                    },
                };
            },
            .STOPWATCH_MODE => {
                self.display_data.mode = ModeEnumT.STOPWATCH_MODE;
                self.display_data.info = ClockInterfaceUnion{
                    .stopwatch = .{
                        .esplased_ms = self.state.STOPWATCH_MODE.esplased_ms,
                        .max_ms = self.state.STOPWATCH_MODE.max_ms,
                        .is_paused = self.state.STOPWATCH_MODE.is_paused,
                        .is_finished = self.state.STOPWATCH_MODE.is_finished,
                    },
                };
            },
            .WORLD_CLOCK_MODE => {
                self.display_data.mode = ModeEnumT.WORLD_CLOCK_MODE;
                self.display_data.info = ClockInterfaceUnion{
                    .worldclock = .{
                        .timezone = 8,
                        .time = 0,
                    },
                };
            },
        }
        return @ptrCast(&self.display_data);
    }
};
