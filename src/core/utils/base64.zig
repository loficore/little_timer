const std = @import("std");

const encode_table = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/";

const decode_table = [256]u8{
    0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
    0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
    0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0x3e, 0xff, 0xff, 0xff, 0x3f,
    0x34, 0x35, 0x36, 0x37, 0x38, 0x39, 0x3a, 0x3b, 0x3c, 0x3d, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
    0xff, 0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e,
    0x0f, 0x10, 0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18, 0x19, 0xff, 0xff, 0xff, 0xff, 0xff,
    0xff, 0x1a, 0x1b, 0x1c, 0x1d, 0x1e, 0x1f, 0x20, 0x21, 0x22, 0x23, 0x24, 0x25, 0x26, 0x27, 0x28,
    0x29, 0x2a, 0x2b, 0x2c, 0x2d, 0x2e, 0x2f, 0x30, 0x31, 0x32, 0x33, 0xff, 0xff, 0xff, 0xff, 0xff,
    0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
    0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
    0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
    0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
    0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
    0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
    0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
    0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
};

pub fn encode(allocator: std.mem.Allocator, data: []const u8) std.mem.Allocator.Error![]u8 {
    const out_len = 4 * ((data.len + 2) / 3);
    const buf = try allocator.alloc(u8, out_len);
    var i: usize = 0;
    var out_idx: usize = 0;
    while (i + 2 < data.len) : (i += 3) {
        const v = @as(u24, data[i]) << 16 | @as(u24, data[i + 1]) << 8 | @as(u24, data[i + 2]);
        buf[out_idx] = encode_table[(v >> 18) & 0x3F];
        buf[out_idx + 1] = encode_table[(v >> 12) & 0x3F];
        buf[out_idx + 2] = encode_table[(v >> 6) & 0x3F];
        buf[out_idx + 3] = encode_table[v & 0x3F];
        out_idx += 4;
    }
    if (i + 1 < data.len) {
        const v = @as(u24, data[i]) << 16 | @as(u24, data[i + 1]) << 8;
        buf[out_idx] = encode_table[(v >> 18) & 0x3F];
        buf[out_idx + 1] = encode_table[(v >> 12) & 0x3F];
        buf[out_idx + 2] = encode_table[(v >> 6) & 0x3F];
        buf[out_idx + 3] = '=';
    } else if (i < data.len) {
        const v = @as(u24, data[i]) << 16;
        buf[out_idx] = encode_table[(v >> 18) & 0x3F];
        buf[out_idx + 1] = encode_table[(v >> 12) & 0x3F];
        buf[out_idx + 2] = '=';
        buf[out_idx + 3] = '=';
    }
    return buf;
}

pub fn encodedLength(data_len: usize) usize {
    return 4 * ((data_len + 2) / 3);
}

pub const Base64Error = error{ InvalidFormat, OutOfMemory };

pub fn decode(allocator: std.mem.Allocator, data: []const u8) Base64Error![]u8 {
    if (data.len % 4 != 0) return error.InvalidFormat;
    if (data.len == 0) return allocator.alloc(u8, 0);

    var padding: usize = 0;
    if (data[data.len - 1] == '=') padding += 1;
    if (data[data.len - 2] == '=') padding += 1;
    const out_len = (data.len / 4) * 3 - padding;
    const buf = try allocator.alloc(u8, out_len);

    var out_idx: usize = 0;
    var i: usize = 0;
    while (i < data.len) : (i += 4) {
        const a = decode_table[data[i]];
        const b = decode_table[data[i + 1]];
        const c = if (data[i + 2] == '=') @as(u8, 0) else decode_table[data[i + 2]];
        const d = if (data[i + 3] == '=') @as(u8, 0) else decode_table[data[i + 3]];

        if (a == 0xff or b == 0xff) return error.InvalidFormat;
        if (data[i + 2] != '=' and c == 0xff) return error.InvalidFormat;
        if (data[i + 3] != '=' and d == 0xff) return error.InvalidFormat;

        const v = @as(u24, a) << 18 | @as(u24, b) << 12 | @as(u24, c) << 6 | @as(u24, d);

        if (out_idx < out_len) {
            buf[out_idx] = @truncate(v >> 16);
            out_idx += 1;
        }
        if (out_idx < out_len) {
            buf[out_idx] = @truncate(v >> 8);
            out_idx += 1;
        }
        if (out_idx < out_len) {
            buf[out_idx] = @truncate(v);
            out_idx += 1;
        }
    }
    return buf[0..out_len];
}

pub fn isBase64(data: []const u8) bool {
    if (data.len % 4 != 0) return false;
    if (data.len == 0) return true;
    for (data[0 .. data.len - 2]) |c| {
        if (decode_table[c] == 0xff) return false;
    }
    if (data[data.len - 2] != '=' and decode_table[data[data.len - 2]] == 0xff) return false;
    if (data[data.len - 1] != '=' and decode_table[data[data.len - 1]] == 0xff) return false;
    return true;
}

test "base64 encode/decode roundtrip" {
    const test_cases = &[_][]const u8{
        "",
        "hello",
        "\x00\x01\x02\xff\xfe\xfd",
        "a",
        "ab",
        "abc",
        "abcd",
    };
    const allocator = std.testing.allocator;
    for (test_cases) |tc| {
        const encoded = try encode(allocator, tc);
        defer allocator.free(encoded);
        const decoded = try decode(allocator, encoded);
        defer allocator.free(decoded);
        try std.testing.expectEqualSlices(u8, tc, decoded);
    }
}

test "base64 known values" {
    const allocator = std.testing.allocator;

    const e1 = try encode(allocator, "hello");
    defer allocator.free(e1);
    try std.testing.expectEqualStrings("aGVsbG8=", e1);

    const e2 = try encode(allocator, "\x00\x01\x02\xff\xfe\xfd");
    defer allocator.free(e2);
    try std.testing.expectEqualStrings("AAEC//7/", e2);
}

test "base64 decode rejects invalid input" {
    const allocator = std.testing.allocator;
    const result = decode(allocator, "!!!invalid!!!");
    try std.testing.expectError(error.InvalidFormat, result);
}

test "isBase64 detection" {
    try std.testing.expect(isBase64("aGVsbG8="));
    try std.testing.expect(isBase64("AAEC//7/"));
    try std.testing.expect(!isBase64("hello"));
    try std.testing.expect(!isBase64(""));
    try std.testing.expect(isBase64("abcd"));
}