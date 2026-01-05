const std = @import("std");
const app = @import("app.zig");
const clock = @import("clock.zig");

// 定义USE_WEBUI为true
pub const USE_WEBUI = true;

pub fn main() !void {
    // 1. 创建内存分配器
    var gpa = std.heap.GeneralPurposeAllocator(.{}){};
    defer _ = gpa.deinit();
    const allocator = gpa.allocator();

    // 2. 在堆上分配应用程序实例
    const main_app = try allocator.create(app.MainApplication);
    defer allocator.destroy(main_app);

    const clock_config = clock.ClockTaskConfigT{
        .countdown = .{
            .duration_seconds = 25 * 60, // 25分钟倒计时
            .loop = false,
        },
    };

    // 3. 初始化应用程序（在指针上原地初始化）
    try main_app.init(allocator, clock_config);

    // 4. 设置全局指针（这样回调函数才能访问 app 实例）
    main_app.setGlobalApp();

    // 5. 运行应用程序
    // 根据编译选项，这将启动GTK或WebUI主循环
    try main_app.run();
}