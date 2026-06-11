//! Base64 边界情况测试
const std = @import("std");
const base64 = @import("../core/utils/base64.zig");

test "Base64 空字符串编码" {
    const allocator = std.testing.allocator;
    const result = try base64.encode(allocator, "");
    defer allocator.free(result);
    try std.testing.expectEqualStrings("", result);
}

test "Base64 单字符编码" {
    const allocator = std.testing.allocator;
    const result = try base64.encode(allocator, "a");
    defer allocator.free(result);
    try std.testing.expectEqualStrings("YQ==", result);
}

test "Base64 双字符编码" {
    const allocator = std.testing.allocator;
    const result = try base64.encode(allocator, "ab");
    defer allocator.free(result);
    try std.testing.expectEqualStrings("YWI=", result);
}

test "Base64 三字符编码" {
    const allocator = std.testing.allocator;
    const result = try base64.encode(allocator, "abc");
    defer allocator.free(result);
    try std.testing.expectEqualStrings("YWJj", result);
}

test "Base64 四字符编码" {
    const allocator = std.testing.allocator;
    const result = try base64.encode(allocator, "abcd");
    defer allocator.free(result);
    try std.testing.expectEqualStrings("YWJjZA==", result);
}

test "Base64 解码空字符串" {
    const allocator = std.testing.allocator;
    const result = try base64.decode(allocator, "");
    defer allocator.free(result);
    try std.testing.expectEqualStrings("", result);
}

test "Base64 解码带填充" {
    const allocator = std.testing.allocator;
    const result = try base64.decode(allocator, "YQ==");
    defer allocator.free(result);
    try std.testing.expectEqualStrings("a", result);
}

test "Base64 非 Base64 字符检测" {
    try std.testing.expect(!base64.isBase64("!@#$%^&*()"));
    try std.testing.expect(!base64.isBase64("abc def"));
    try std.testing.expect(!base64.isBase64("abc\tdef"));
}

test "Base64 有效字符检测" {
    try std.testing.expect(base64.isBase64("ABCDEFGHIJKLMNOPQRSTUVWXYZ"));
    try std.testing.expect(base64.isBase64("abcdefghijklmnopqrstuvwxyz"));
    try std.testing.expect(base64.isBase64("0123456789+/"));
    try std.testing.expect(base64.isBase64(""));
}

test "Base64 错误填充检测" {
    const allocator = std.testing.allocator;
    const result = base64.decode(allocator, "YQ=");
    try std.testing.expectError(base64.Base64Error.InvalidPadding, result);
}

test "Base64 错误填充检测2" {
    const allocator = std.testing.allocator;
    const result = base64.decode(allocator, "YQ===");
    try std.testing.expectError(base64.Base64Error.InvalidPadding, result);
}

test "Base64 错误字符检测" {
    const allocator = std.testing.allocator;
    const result = base64.decode(allocator, "YQ!==");
    try std.testing.expectError(base64.Base64Error.InvalidCharacter, result);
}

test "Base64 编码长度计算" {
    try std.testing.expectEqual(@as(usize, 0), base64.encodedLength(0));
    try std.testing.expectEqual(@as(usize, 4), base64.encodedLength(1));
    try std.testing.expectEqual(@as(usize, 4), base64.encodedLength(2));
    try std.testing.expectEqual(@as(usize, 4), base64.encodedLength(3));
    try std.testing.expectEqual(@as(usize, 8), base64.encodedLength(4));
}

test "Base64 标准测试向量" {
    const allocator = std.testing.allocator;

    const test1 = try base64.encode(allocator, "");
    defer allocator.free(test1);
    try std.testing.expectEqualStrings("", test1);

    const test2 = try base64.encode(allocator, "f");
    defer allocator.free(test2);
    try std.testing.expectEqualStrings("Zg==", test2);

    const test3 = try base64.encode(allocator, "fo");
    defer allocator.free(test3);
    try std.testing.expectEqualStrings("Zm8=", test3);

    const test4 = try base64.encode(allocator, "foo");
    defer allocator.free(test4);
    try std.testing.expectEqualStrings("Zm9v", test4);

    const test5 = try base64.encode(allocator, "foob");
    defer allocator.free(test5);
    try std.testing.expectEqualStrings("Zm9vYg==", test5);

    const test6 = try base64.encode(allocator, "fooba");
    defer allocator.free(test6);
    try std.testing.expectEqualStrings("Zm9vYmE=", test6);

    const test7 = try base64.encode(allocator, "foobar");
    defer allocator.free(test7);
    try std.testing.expectEqualStrings("Zm9vYmFy", test7);
}