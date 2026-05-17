//! Linux Kernel Keyring 密钥存储模块
//! 使用 Linux kernel keyring syscall 实现安全的密钥存储
//!
//! 核心 syscalls:
//! - add_key: 创建/存储密钥
//! - request_key: 按描述查找密钥
//! - keyctl: 密钥操作控制

const std = @import("std");
const logger = @import("../logger.zig");
const linux = std.os.linux;

pub const KeyringError = error{
    KeyringCreateFailed,
    KeyringNotFound,
    KeyringStoreFailed,
    KeyringRetrieveFailed,
    KeyringDeleteFailed,
    KeyringSyscallFailed,
    InvalidArgument,
    OutOfMemory,
};

pub const KeyctlOperation = enum(c_int) {
    GET_KEYRING_ID = 0,
    JOIN_SESSION_KEYRING = 1,
    UPDATE = 2,
    REVOKE = 3,
    CHOWN = 4,
    SETPERM = 5,
    INSTANTIATE = 6,
    NEGATE = 7,
    SET_REQKEY_KEYRING = 8,
    SET_SECURITY = 9,
    ASSCRIBE = 10,
    GET_SECURITY = 11,
    SESSION_TO_PARENT = 12,
    RECALC = 13,
    FREE = 14,
    LINK = 15,
    UNLINK = 16,
    SEARCH = 17,
    READ = 18,
    INSTANTIATE_IOV = 19,
    NEGATE_IOV = 20,
    SET_DEFAULT = 21,
    LOOKUP = 22,
    CLASS_BAD = 23,
    CLASS_OK = 24,
    CLASS_CREATE = 25,
    CLASS_TO_STRING = 26,
    THREAD_KEYRING = 27,
    PROCESS_KEYRING = 28,
    SESSION_KEYRING = 29,
    USER_KEYRING = 30,
    USER_SESSION_KEYRING = 31,
    GET_PERSISTENT = 32,
    GET_INSTIGATOR = 33,
    OVERRIDE_FLAGS = 34,
    GET_ATTRIBUTES = 35,
};

const KEY_POS_ALL: u32 = 0x3F;

fn makeNullTerminated(buffer: []u8, str: []const u8) []u8 {
    @memcpy(buffer[0..str.len], str);
    buffer[str.len] = 0;
    return buffer[0..str.len + 1];
}

var cached_keyring_id: i32 = 0;
var keyring_initialized = false;
var keyring_mutex = std.Thread.Mutex{};

fn syscallErrno(result: usize) std.os.linux.E {
    return std.os.linux.E.init(result);
}

fn syscallSigned(result: usize) isize {
    return @bitCast(result);
}

fn syscallFailed(result: usize) bool {
    return syscallErrno(result) != .SUCCESS;
}

fn logSyscallError(op: []const u8, result: usize) void {
    const errno = syscallErrno(result);
    logger.global_logger.err("keyring {s} failed: errno={any}", .{ op, errno });
}

fn searchKeyId(keyring_id: i32, type_z: []const u8, desc_z: []const u8) KeyringError!i32 {
    logger.global_logger.info("keyring searchKeyId: keyring_id={}", .{keyring_id});
    const result = linux.syscall5(
        .keyctl,
        @intFromEnum(KeyctlOperation.SEARCH),
        @as(usize, @bitCast(@as(isize, keyring_id))),
        @intFromPtr(type_z.ptr),
        @intFromPtr(desc_z.ptr),
        0,
    );
    logger.global_logger.info("keyring searchKeyId: raw_result=0x{x}, errno={any}", .{result, syscallErrno(result)});
    if (syscallFailed(result)) {
        logSyscallError("keyctl_search", result);
        return error.KeyringNotFound;
    }
    const kid: i32 = @intCast(syscallSigned(result));
    logger.global_logger.info("keyring searchKeyId: found key_id={}", .{kid});
    return kid;
}

pub fn joinOrCreateSessionKeyring() KeyringError!i32 {
    keyring_mutex.lock();
    defer keyring_mutex.unlock();
    if (keyring_initialized) {
        logger.global_logger.info("keyring joinOrCreateSessionKeyring: cached keyring_id={d}", .{cached_keyring_id});
        return cached_keyring_id;
    }
    const result = linux.syscall2(.keyctl, @intFromEnum(KeyctlOperation.JOIN_SESSION_KEYRING), 0);
    if (syscallFailed(result)) {
        logSyscallError("join_session_keyring", result);
        return error.KeyringCreateFailed;
    }
    const kid: i32 = @intCast(syscallSigned(result));
    cached_keyring_id = kid;
    keyring_initialized = true;
    logger.global_logger.info("keyring joinOrCreateSessionKeyring: created keyring_id={d}", .{kid});
    return kid;
}

pub fn storeKeyInKeyring(description: []const u8, payload: []const u8) KeyringError!void {
    if (payload.len == 0) return; // 空 payload 不存储
    const keyring_id = try joinOrCreateSessionKeyring();
    logger.global_logger.info("keyring storeKeyInKeyring: keyring_id={d}, desc={s}, payload_len={d}", .{keyring_id, description, payload.len});

    var type_buf: [32]u8 = undefined;
    const type_z = makeNullTerminated(&type_buf, "user");
    var desc_buf: [256]u8 = undefined;
    const desc_z = makeNullTerminated(&desc_buf, description);
    const payload_ptr: [*]const u8 = payload.ptr;

    const key_id_raw = linux.syscall5(.add_key, @intFromPtr(type_z.ptr), @intFromPtr(desc_z.ptr), @intFromPtr(payload_ptr), payload.len, @as(usize, @bitCast(@as(isize, keyring_id))));
    const add_errno = syscallErrno(key_id_raw);
    logger.global_logger.info("keyring add_key raw_result=0x{x}, errno={any}", .{key_id_raw, add_errno});

    var stored_key_id: i32 = 0;
    var key_existed = false;

    if (add_errno == .SUCCESS or add_errno == .EXIST) {
        stored_key_id = @intCast(syscallSigned(key_id_raw));
        logger.global_logger.info("keyring storeKeyInKeyring: key_id={d}, existed={}", .{stored_key_id, add_errno == .EXIST});
        key_existed = add_errno == .EXIST;

        if (key_existed) {
            const read_result = readKeyById(stored_key_id, std.heap.page_allocator);
            if (read_result) |existing| {
                std.heap.page_allocator.free(existing);
                logger.global_logger.info("keyring storeKeyInKeyring: existing key readable, using it directly", .{});
                stored_key_id = try setKeyPermissions(stored_key_id);
                return;
            } else |read_err| {
                logger.global_logger.warn("keyring storeKeyInKeyring: existing key not readable (err={any}), will try update", .{read_err});
                updateKeyPayload(stored_key_id, payload) catch |update_err| {
                    logger.global_logger.warn("keyring storeKeyInKeyring: update failed (err={any}), will overwrite", .{update_err});
                };
            }
        }
    } else {
        logSyscallError("add_key", key_id_raw);
        return error.KeyringStoreFailed;
    }

    stored_key_id = try setKeyPermissions(stored_key_id);
    logger.global_logger.info("keyring storeKeyInKeyring: done key_id={d}", .{stored_key_id});
}

fn setKeyPermissions(key_id: i32) KeyringError!i32 {
    const perm_result = linux.syscall3(.keyctl, @intFromEnum(KeyctlOperation.SETPERM), @intCast(key_id), KEY_POS_ALL);
    logger.global_logger.info("keyring setKeyPermissions: key_id={d}, perm=0x{x}, result=0x{x}, errno={any}", .{ key_id, KEY_POS_ALL, perm_result, syscallErrno(perm_result) });
    if (syscallFailed(perm_result)) {
        logger.global_logger.warn("keyring setKeyPermissions failed for key_id={d}: errno={any}", .{ key_id, syscallErrno(perm_result) });
    }
    return key_id;
}

pub fn retrieveKeyFromKeyring(description: []const u8, allocator: std.mem.Allocator) KeyringError![]u8 {
    const keyring_id = try joinOrCreateSessionKeyring();
    logger.global_logger.info("keyring retrieveKeyFromKeyring: keyring_id={d}, desc={s}", .{keyring_id, description});

    var type_buf: [32]u8 = undefined;
    const type_z = makeNullTerminated(&type_buf, "user");
    var desc_buf: [256]u8 = undefined;
    const desc_z = makeNullTerminated(&desc_buf, description);

    const key_id = try searchKeyId(keyring_id, type_z, desc_z);

    const size_result = linux.syscall5(.keyctl, @intFromEnum(KeyctlOperation.READ), @intCast(key_id), 0, 0, 0);
    logger.global_logger.info("keyring keyctl_read_size: key_id={}, raw_result=0x{x}, errno={any}", .{key_id, size_result, syscallErrno(size_result)});
    if (syscallFailed(size_result)) {
        logSyscallError("keyctl_read_size", size_result);
        return error.KeyringRetrieveFailed;
    }
    const size: usize = @intCast(size_result);

    if (size == 0) {
        return error.KeyringNotFound;
    }

    const buffer = allocator.alloc(u8, size) catch {
        return error.OutOfMemory;
    };
    errdefer allocator.free(buffer);

    const read_result = linux.syscall5(.keyctl, @intFromEnum(KeyctlOperation.READ), @intCast(key_id), @intFromPtr(buffer.ptr), size, 0);
    if (syscallFailed(read_result)) {
        logSyscallError("keyctl_read", read_result);
        allocator.free(buffer);
        return error.KeyringRetrieveFailed;
    }

    return buffer;
}

pub fn deleteKeyFromKeyring(key_id: i32) KeyringError!void {
    const result = linux.syscall3(.keyctl, @intFromEnum(KeyctlOperation.UNLINK), @intCast(key_id), 0);
    if (syscallFailed(result)) {
        logSyscallError("keyctl_unlink", result);
        return error.KeyringDeleteFailed;
    }
}

fn readKeyById(key_id: i32, allocator: std.mem.Allocator) KeyringError![]u8 {
    const size_result = linux.syscall5(.keyctl, @intFromEnum(KeyctlOperation.READ), @intCast(key_id), 0, 0, 0);
    logger.global_logger.info("keyring readKeyById: key_id={}, raw_result=0x{x}, errno={any}", .{ key_id, size_result, syscallErrno(size_result) });
    if (syscallFailed(size_result)) {
        return error.KeyringRetrieveFailed;
    }
    const size: usize = @intCast(size_result);
    if (size == 0) {
        return error.KeyringNotFound;
    }
    const buffer = allocator.alloc(u8, size) catch return error.OutOfMemory;
    errdefer allocator.free(buffer);
    const read_result = linux.syscall5(.keyctl, @intFromEnum(KeyctlOperation.READ), @intCast(key_id), @intFromPtr(buffer.ptr), size, 0);
    if (syscallFailed(read_result)) {
        logSyscallError("keyctl_read", read_result);
        allocator.free(buffer);
        return error.KeyringRetrieveFailed;
    }
    return buffer;
}

pub fn updateKeyPayload(key_id: i32, payload: []const u8) KeyringError!void {
    if (payload.len == 0) return;
    const payload_ptr: [*]const u8 = payload.ptr;
    const result = linux.syscall5(.keyctl, @intFromEnum(KeyctlOperation.UPDATE), @intCast(key_id), @intFromPtr(payload_ptr), payload.len, 0);
    logger.global_logger.info("keyring updateKeyPayload: key_id={d}, raw_result=0x{x}, errno={any}", .{ key_id, result, syscallErrno(result) });
    if (syscallFailed(result)) {
        logSyscallError("keyctl_update", result);
        return error.KeyringStoreFailed;
    }
}

pub fn keyExists(description: []const u8) bool {
    const keyring_id = joinOrCreateSessionKeyring() catch {
        logger.global_logger.warn("keyring keyExists: joinOrCreateSessionKeyring failed", .{});
        return false;
    };
    logger.global_logger.info("keyring keyExists: keyring_id={d}, desc={s}", .{keyring_id, description});
    var type_buf: [32]u8 = undefined;
    const type_z = makeNullTerminated(&type_buf, "user");
    var desc_buf: [256]u8 = undefined;
    const desc_z = makeNullTerminated(&desc_buf, description);
    const key_id = searchKeyId(keyring_id, type_z, desc_z) catch |err| {
        logger.global_logger.warn("keyring keyExists: searchKeyId failed: {any}", .{err});
        return false;
    };
    logger.global_logger.info("keyring keyExists: key_id={} found", .{key_id});
    return true;
}

test "keyring constants" {
    try std.testing.expect(@intFromEnum(KeyctlOperation.GET_KEYRING_ID) == 0);
    try std.testing.expect(@intFromEnum(KeyctlOperation.JOIN_SESSION_KEYRING) == 1);
    try std.testing.expect(@intFromEnum(KeyctlOperation.READ) == 18);
    try std.testing.expect(@intFromEnum(KeyctlOperation.UNLINK) == 16);
}
