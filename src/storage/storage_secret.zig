const builtin = @import("builtin");
const std = @import("std");

pub fn storeSecret(allocator: std.mem.Allocator, secret: []const u8) !void {
    _ = allocator;
    _ = secret;
}

pub fn retrieveSecret(allocator: std.mem.Allocator) ![]const u8 {
    _ = allocator;
}

pub fn deleteSecret(allocator: std.mem.Allocator) !void {
    _ = allocator;
}

pub fn freeSecretBuffer(allocator: std.mem.Allocator) !void {
    _ = allocator;
}
