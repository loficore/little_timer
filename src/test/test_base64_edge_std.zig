//! Base64 stdlib boundary tests
const std = @import("std");

fn isBase64(data: []const u8) bool {
    if (data.len == 0) return true;
    for (data) |c| {
        switch (c) {
            'A'...'Z', 'a'...'z', '0'...'9', '+', '/', '=' => continue,
            else => return false,
        }
    }
    return true;
}

test "Base64 空字符串编码" {
    const result = try std.base64.standard.Encoder.calcSize(0);
    try std.testing.expectEqual(@as(usize, 0), result);
}

test "Base64 单字符编码" {
    const encoded_len = std.base64.standard.Encoder.calcSize(1);
    try std.testing.expectEqual(@as(usize, 4), encoded_len);
}

test "Base64 编码长度计算" {
    try std.testing.expectEqual(@as(usize, 0), std.base64.standard.Encoder.calcSize(0));
    try std.testing.expectEqual(@as(usize, 4), std.base64.standard.Encoder.calcSize(1));
    try std.testing.expectEqual(@as(usize, 4), std.base64.standard.Encoder.calcSize(2));
    try std.testing.expectEqual(@as(usize, 4), std.base64.standard.Encoder.calcSize(3));
    try std.testing.expectEqual(@as(usize, 8), std.base64.standard.Encoder.calcSize(4));
}

test "Base64 解码空字符串" {
    const decoder = std.base64.standard.Decoder;
    const len = try decoder.calcSizeForSlice("");
    try std.testing.expectEqual(@as(usize, 0), len);
}

test "Base64 标准测试向量" {
    const encoder = std.base64.standard.Encoder;
    const decoder = std.base64.standard.Decoder;

    const cases = .{
        .{ "", "" },
        .{ "f", "Zg==" },
        .{ "fo", "Zm8=" },
        .{ "foo", "Zm9v" },
        .{ "foob", "Zm9vYg==" },
        .{ "fooba", "Zm9vYmE=" },
        .{ "foobar", "Zm9vYmFy" },
    };

    const allocator = std.testing.allocator;
    inline for (cases) |c| {
        const input = c[0];
        const expected = c[1];
        const encoded_len = encoder.calcSize(input.len);
        var buf: [16]u8 = undefined;
        const encoded = encoder.encode(buf[0..encoded_len], input);
        try std.testing.expectEqualStrings(expected, encoded);
        const decoded_len = try decoder.calcSizeForSlice(encoded);
        var dec_buf: [16]u8 = undefined;
        decoder.decode(dec_buf[0..decoded_len], encoded);
        try std.testing.expectEqualStrings(input, dec_buf[0..decoded_len]);
        _ = allocator;
    }
}

test "Base64 非 Base64 字符检测" {
    try std.testing.expect(!isBase64("!@#$%^&*()"));
    try std.testing.expect(!isBase64("abc def"));
    try std.testing.expect(!isBase64("abc\tdef"));
}

test "Base64 有效字符检测" {
    try std.testing.expect(isBase64("ABCDEFGHIJKLMNOPQRSTUVWXYZ"));
    try std.testing.expect(isBase64("abcdefghijklmnopqrstuvwxyz"));
    try std.testing.expect(isBase64("0123456789+/"));
    try std.testing.expect(isBase64(""));
}
