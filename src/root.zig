//! By convention, root.zig is the root source file when making a library.
const std = @import("std");

/// 使用缓冲输出，输出测试信息
///
/// 返回:
/// - !void: 如果输出失败则返回错误
pub fn bufferedPrint() !void {
    // Stdout is for the actual output of your application, for example if you
    // are implementing gzip, then only the compressed bytes should be sent to
    // stdout, not any debugging messages.
    var stdout_buffer: [1024]u8 = undefined;
    var stdout_writer = std.fs.File.stdout().writer(&stdout_buffer);
    const stdout = &stdout_writer.interface;

    try stdout.print("Run `zig build test` to run the tests.\n", .{});

    try stdout.flush(); // Don't forget to flush!
}

/// 算数加法
///
/// 参数:
/// - **a**: 第一个整数
/// - **b**: 第二个整数
///
/// 返回:
/// - i32: a 和 b 的和
pub fn add(a: i32, b: i32) i32 {
    return a + b;
}

test "basic add functionality" {
    try std.testing.expect(add(3, 7) == 10);
}
