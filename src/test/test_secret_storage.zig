const std = @import("std");
const builtin = @import("builtin");
const secret_storage = @import("../core/utils/secret_storage.zig");

const allocator = std.testing.allocator;

test "SecretError enum has expected variants" {
    try std.testing.expectError(error.NotFound, @as(anyerror, error.NotFound));
    try std.testing.expectError(error.AlreadyExists, @as(anyerror, error.AlreadyExists));
    try std.testing.expectError(error.InvalidValue, @as(anyerror, error.InvalidValue));
    try std.testing.expectError(error.NoAccess, @as(anyerror, error.NoAccess));
    try std.testing.expectError(error.OutOfMemory, @as(anyerror, error.OutOfMemory));
    try std.testing.expectError(error.NotImplemented, @as(anyerror, error.NotImplemented));
    try std.testing.expectError(error.PlatformError, @as(anyerror, error.PlatformError));
    try std.testing.expectError(error.Base64Error, @as(anyerror, error.Base64Error));
}

test "SecretService.create succeeds on Linux" {
    if (builtin.os.tag != .linux) return error.SkipZigTest;

    const svc = try secret_storage.SecretService.create(allocator);
    try std.testing.expect(svc.ptr != null);
    try std.testing.expect(svc.vtable != null);
}

test "vtable functions are non-null" {
    if (builtin.os.tag != .linux) return error.SkipZigTest;

    const svc = try secret_storage.SecretService.create(allocator);
    try std.testing.expect(svc.vtable.store != null);
    try std.testing.expect(svc.vtable.retrieve != null);
    try std.testing.expect(svc.vtable.delete != null);
}

test "store and retrieve roundtrip" {
    if (builtin.os.tag != .linux) return error.SkipZigTest;

    const svc = try secret_storage.SecretService.create(allocator);
    const service = "little_timer_test";
    const key = "test_roundtrip";
    const value = "secret_value_123";

    try svc.store(service, key, value);
    defer _ = svc.delete(service, key) catch {};

    const result = try svc.retrieve(allocator, service, key);
    defer allocator.free(result);
    try std.testing.expectEqualStrings(value, result);

    _ = svc.delete(service, key) catch {};
}

test "retrieve nonexistent key returns NotFound" {
    if (builtin.os.tag != .linux) return error.SkipZigTest;

    const svc = try secret_storage.SecretService.create(allocator);
    const result = svc.retrieve(allocator, "little_timer_test_nonexist", "no_such_key");
    try std.testing.expectError(error.NotFound, result);
}

test "retrieve after delete returns NotFound" {
    if (builtin.os.tag != .linux) return error.SkipZigTest;

    const svc = try secret_storage.SecretService.create(allocator);
    const service = "little_timer_test";
    const key = "test_delete_retrieve";
    const value = "will_be_deleted";

    try svc.store(service, key, value);
    try svc.delete(service, key);

    const result = svc.retrieve(allocator, service, key);
    try std.testing.expectError(error.NotFound, result);
}

test "delete nonexistent key does not crash" {
    if (builtin.os.tag != .linux) return error.SkipZigTest;

    const svc = try secret_storage.SecretService.create(allocator);
    const result = svc.delete("little_timer_test_ghost", "never_existed");
    try std.testing.expectError(error.NotFound, result);
}

test "store empty value" {
    if (builtin.os.tag != .linux) return error.SkipZigTest;

    const svc = try secret_storage.SecretService.create(allocator);
    const service = "little_timer_test";
    const key = "test_empty";
    const value = "";

    try svc.store(service, key, value);
    defer _ = svc.delete(service, key) catch {};

    const result = try svc.retrieve(allocator, service, key);
    defer allocator.free(result);
    try std.testing.expectEqualStrings("", result);

    _ = svc.delete(service, key) catch {};
}

test "store binary data" {
    if (builtin.os.tag != .linux) return error.SkipZigTest;

    const svc = try secret_storage.SecretService.create(allocator);
    const service = "little_timer_test";
    const key = "test_binary";
    const value = "\x00\x01\x02\xff\xfe\xfd";

    try svc.store(service, key, value);
    defer _ = svc.delete(service, key) catch {};

    const result = try svc.retrieve(allocator, service, key);
    defer allocator.free(result);
    try std.testing.expectEqualSlices(u8, value, result);

    _ = svc.delete(service, key) catch {};
}

test "different service same key are independent" {
    if (builtin.os.tag != .linux) return error.SkipZigTest;

    const svc = try secret_storage.SecretService.create(allocator);
    const service_a = "little_timer_test_svc_a";
    const service_b = "little_timer_test_svc_b";
    const key = "same_key";
    const value_a = "value_from_a";
    const value_b = "value_from_b";

    try svc.store(service_a, key, value_a);
    defer _ = svc.delete(service_a, key) catch {};
    try svc.store(service_b, key, value_b);
    defer _ = svc.delete(service_b, key) catch {};

    const result_a = try svc.retrieve(allocator, service_a, key);
    defer allocator.free(result_a);
    const result_b = try svc.retrieve(allocator, service_b, key);
    defer allocator.free(result_b);
    try std.testing.expectEqualStrings(value_a, result_a);
    try std.testing.expectEqualStrings(value_b, result_b);

    _ = svc.delete(service_a, key) catch {};
    _ = svc.delete(service_b, key) catch {};
}

test "same service different keys are independent" {
    if (builtin.os.tag != .linux) return error.SkipZigTest;

    const svc = try secret_storage.SecretService.create(allocator);
    const service = "little_timer_test";
    const key_x = "test_key_x";
    const key_y = "test_key_y";
    const value_x = "value_x_456";
    const value_y = "value_y_789";

    try svc.store(service, key_x, value_x);
    defer _ = svc.delete(service, key_x) catch {};
    try svc.store(service, key_y, value_y);
    defer _ = svc.delete(service, key_y) catch {};

    const result_x = try svc.retrieve(allocator, service, key_x);
    defer allocator.free(result_x);
    const result_y = try svc.retrieve(allocator, service, key_y);
    defer allocator.free(result_y);
    try std.testing.expectEqualStrings(value_x, result_x);
    try std.testing.expectEqualStrings(value_y, result_y);

    _ = svc.delete(service, key_x) catch {};
    _ = svc.delete(service, key_y) catch {};
}

test "store overwrites existing value" {
    if (builtin.os.tag != .linux) return error.SkipZigTest;

    const svc = try secret_storage.SecretService.create(allocator);
    const service = "little_timer_test";
    const key = "test_overwrite";
    const value_v1 = "first_value";
    const value_v2 = "second_value";

    try svc.store(service, key, value_v1);
    defer _ = svc.delete(service, key) catch {};

    const result1 = try svc.retrieve(allocator, service, key);
    defer allocator.free(result1);
    try std.testing.expectEqualStrings(value_v1, result1);

    try svc.store(service, key, value_v2);

    const result2 = try svc.retrieve(allocator, service, key);
    defer allocator.free(result2);
    try std.testing.expectEqualStrings(value_v2, result2);

    _ = svc.delete(service, key) catch {};
}

test "storeMasterKey/retrieveMasterKey roundtrip" {
    if (builtin.os.tag != .linux) return error.SkipZigTest;

    const test_key = "test_master_key_0123456789abcdef";
    try secret_storage.storeMasterKey(test_key);
    defer secret_storage.deleteMasterKey();

    const result = try secret_storage.retrieveMasterKey(allocator);
    defer allocator.free(result);
    try std.testing.expectEqualStrings(test_key, result);

    secret_storage.deleteMasterKey();
}

test "deleteMasterKey clears stored key" {
    if (builtin.os.tag != .linux) return error.SkipZigTest;

    const test_key = "test_master_key_to_delete";
    try secret_storage.storeMasterKey(test_key);

    secret_storage.deleteMasterKey();

    const result = secret_storage.retrieveMasterKey(allocator);
    try std.testing.expectError(error.NotFound, result);
}