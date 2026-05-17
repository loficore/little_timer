//! 密钥存储模块 - 提供跨平台的安全凭证存储
//! Linux: Kernel Keyring (with in-memory fallback)
//! macOS: Keychain Services (stub)
//! Windows: wincred API
//! Android: AccountManager (stub)

const std = @import("std");
const builtin = @import("builtin");
const logger = @import("../logger.zig");
const keyring = @import("keyring.zig");

pub const SecretError = error{
    NotFound,
    AlreadyExists,
    InvalidValue,
    NoAccess,
    OutOfMemory,
    NotImplemented,
    PlatformError,
};

pub const SecretSchema = struct {
    name: []const u8,
    attributes: []const Attribute,
};

pub const Attribute = struct {
    key: []const u8,
    value: []const u8,
};

pub const SecretService = struct {
    ptr: *anyopaque,
    vtable: *const VTable,

    pub const VTable = struct {
        store: *const fn (*anyopaque, []const u8, []const u8, []const u8) SecretError!void,
        retrieve: *const fn (*anyopaque, []const u8, []const u8, *[*]u8, *usize) SecretError!void,
        delete: *const fn (*anyopaque, []const u8, []const u8) SecretError!void,
        free: *const fn (*anyopaque, [*]u8, usize) void,
    };

    pub fn store(self: *SecretService, service: []const u8, key: []const u8, value: []const u8) SecretError!void {
        return self.vtable.store(self.ptr, service, key, value);
    }

    pub fn retrieve(self: *SecretService, service: []const u8, key: []const u8) SecretError![]u8 {
        var ptr: [*]u8 = undefined;
        var len: usize = undefined;
        try self.vtable.retrieve(self.ptr, service, key, &ptr, &len);
        defer self.vtable.free(self.ptr, ptr, len);
        return ptr[0..len];
    }

    pub fn delete(self: *SecretService, service: []const u8, key: []const u8) SecretError!void {
        return self.vtable.delete(self.ptr, service, key);
    }

    pub fn create(allocator: std.mem.Allocator) !SecretService {
        return switch (builtin.os.tag) {
            .linux => try createLinux(allocator),
            .macos => try createMac(allocator),
            .windows => try createWindows(allocator),
            else => return SecretError.NotImplemented,
        };
    }
};

const LINUX_SERVICE_NAME = "little_timer";
const MASTER_KEY_ATTR = "master_key";

var master_key_instance: ?[]u8 = null;
var linux_in_memory_store: std.StringHashMap([]u8) = undefined;
var linux_in_memory_initialized = false;

fn createLinux(allocator: std.mem.Allocator) !SecretService {
    @setRuntimeSafety(false);
    const impl = try allocator.create(LinuxSecretImpl);
    impl.* = .{ .allocator = allocator };
    return SecretService{
        .ptr = @ptrCast(impl),
        .vtable = &.{
            .store = storeLinux,
            .retrieve = retrieveLinux,
            .delete = deleteLinux,
            .free = freeLinux,
        },
    };
}

const LinuxSecretImpl = struct {
    allocator: std.mem.Allocator,
};

const SECRET_SCHEMA_NONE: u32 = 0;
const SECRET_SCHEMA_ATTRIBUTE_STRING: u32 = 0;

fn getLinuxInMemoryStore() *std.StringHashMap([]u8) {
    if (!linux_in_memory_initialized) {
        linux_in_memory_store = std.StringHashMap([]u8).init(std.heap.page_allocator);
        linux_in_memory_initialized = true;
    }
    return &linux_in_memory_store;
}

fn makeLinuxKey(service: []const u8, key: []const u8) ![]u8 {
    return std.fmt.allocPrint(std.heap.page_allocator, "little_timer:{s}:{s}", .{ service, key });
}

fn storeLinux(ptr: *anyopaque, service: []const u8, key: []const u8, value: []const u8) SecretError!void {
    _ = ptr;
    @setRuntimeSafety(false);

    const map = getLinuxInMemoryStore();
    const map_key = makeLinuxKey(service, key) catch return SecretError.OutOfMemory;
    defer std.heap.page_allocator.free(map_key);

    if (map.get(map_key)) |existing| {
        std.heap.page_allocator.free(existing);
    }

    const value_copy = std.heap.page_allocator.dupe(u8, value) catch return SecretError.OutOfMemory;
    map.put(map_key, value_copy) catch return SecretError.OutOfMemory;
}

fn retrieveLinux(ptr: *anyopaque, service: []const u8, key: []const u8, out_ptr: *[*]u8, out_len: *usize) SecretError!void {
    _ = ptr;
    @setRuntimeSafety(false);

    const map = getLinuxInMemoryStore();
    const map_key = makeLinuxKey(service, key) catch return SecretError.OutOfMemory;
    defer std.heap.page_allocator.free(map_key);

    const value_copy = map.get(map_key) orelse return SecretError.NotFound;
    const copy = std.heap.page_allocator.dupe(u8, value_copy) catch return SecretError.OutOfMemory;
    out_ptr.* = copy.ptr;
    out_len.* = copy.len;
}

fn deleteLinux(ptr: *anyopaque, service: []const u8, key: []const u8) SecretError!void {
    _ = ptr;
    @setRuntimeSafety(false);

    const map = getLinuxInMemoryStore();
    const map_key = makeLinuxKey(service, key);
    defer std.heap.page_allocator.free(map_key);

    if (map.fetchRemove(map_key)) |entry| {
        std.heap.page_allocator.free(entry.value);
    }
}

fn freeLinux(ptr: *anyopaque, buffer: [*]u8, len: usize) void {
    _ = ptr;
    @setRuntimeSafety(false);
    if (len > 0) {
        std.heap.page_allocator.free(buffer[0..len]);
    }
}

fn createMac(allocator: std.mem.Allocator) !SecretService {
    @setRuntimeSafety(false);
    const impl = try allocator.create(MacSecretImpl);
    impl.* = .{ .allocator = allocator };
    return SecretService{
        .ptr = @ptrCast(impl),
        .vtable = &.{
            .store = storeMac,
            .retrieve = retrieveMac,
            .delete = deleteMac,
            .free = freeMac,
        },
    };
}

const MacSecretImpl = struct {
    allocator: std.mem.Allocator,
};

fn storeMac(ptr: *anyopaque, service: []const u8, key: []const u8, value: []const u8) SecretError!void {
    _ = ptr;
    _ = service;
    _ = key;
    _ = value;
    @setRuntimeSafety(false);
    return SecretError.NotImplemented;
}

fn retrieveMac(ptr: *anyopaque, service: []const u8, key: []const u8, out_ptr: *[*]u8, out_len: *usize) SecretError!void {
    _ = ptr;
    _ = service;
    _ = key;
    _ = out_ptr;
    _ = out_len;
    @setRuntimeSafety(false);
    return SecretError.NotImplemented;
}

fn deleteMac(ptr: *anyopaque, service: []const u8, key: []const u8) SecretError!void {
    _ = ptr;
    _ = service;
    _ = key;
    @setRuntimeSafety(false);
    return SecretError.NotImplemented;
}

fn freeMac(ptr: *anyopaque, buffer: [*]u8, len: usize) void {
    _ = ptr;
    _ = buffer;
    _ = len;
    @setRuntimeSafety(false);
}

fn createWindows(allocator: std.mem.Allocator) !SecretService {
    @setRuntimeSafety(false);
    const impl = try allocator.create(WindowsSecretImpl);
    impl.* = .{ .allocator = allocator };
    return SecretService{
        .ptr = @ptrCast(impl),
        .vtable = &.{
            .store = storeWindows,
            .retrieve = retrieveWindows,
            .delete = deleteWindows,
            .free = freeWindows,
        },
    };
}

const WindowsSecretImpl = struct {
    allocator: std.mem.Allocator,
};

const windows = struct {
    const DWORD = u32;
    const WCHAR = u16;
    const LPWSTR = ?[*:0]WCHAR;
    const BOOL = c_int;
    const FILETIME = extern struct {
        dwLowDateTime: DWORD,
        dwHighDateTime: DWORD,
    };
    const CREDENTIALW = extern struct {
        Flags: DWORD,
        Type: DWORD,
        TargetName: LPWSTR,
        Comment: LPWSTR,
        LastWritten: FILETIME,
        CredentialBlobSize: DWORD,
        CredentialBlob: ?[*]u8,
        Persist: DWORD,
        AttributeCount: DWORD,
        Attributes: ?*anyopaque,
        TargetAlias: LPWSTR,
        UserName: LPWSTR,
    };
    const CRED_PERSIST_LOCAL_MACHINE: DWORD = 2;
    const CRED_TYPE_GENERIC: DWORD = 1;

    extern "advapi32" fn CredWriteW(Credential: *const CREDENTIALW, Flags: DWORD) callconv(.Stdcall) BOOL;
    extern "advapi32" fn CredReadW(TargetName: LPWSTR, Type: DWORD, Flags: DWORD, Credential: *?*CREDENTIALW) callconv(.Stdcall) BOOL;
    extern "advapi32" fn CredDeleteW(TargetName: LPWSTR, Type: DWORD, Flags: DWORD) callconv(.Stdcall) BOOL;
    extern "advapi32" fn CredFree(BUFFER: *anyopaque) callconv(.Stdcall) void;
};

fn storeWindows(ptr: *anyopaque, service: []const u8, key: []const u8, value: []const u8) SecretError {
    if (builtin.os.tag != .windows) {
        return SecretError.NotImplemented;
    }

    const impl: *WindowsSecretImpl = @ptrCast(@alignCast(ptr));
    @setRuntimeSafety(false);

    const target = std.fmt.allocPrintZ(impl.allocator, "little_timer:{s}:{s}", .{ service, key }) catch return SecretError.OutOfMemory;
    defer impl.allocator.free(target);

    const target_utf16 = std.unicode.utf8ToUtf16LeAllocZ(impl.allocator, target) catch return SecretError.OutOfMemory;
    defer impl.allocator.free(target_utf16);

    const credential = windows.CREDENTIALW{
        .TargetName = target_utf16.ptr,
        .Type = windows.CRED_TYPE_GENERIC,
        .CredentialBlobSize = @intCast(value.len),
        .CredentialBlob = @constCast(value.ptr),
        .Persist = windows.CRED_PERSIST_LOCAL_MACHINE,
    };

    if (windows.CredWriteW(&credential, 0) == 0) {
        return SecretError.PlatformError;
    }

    return;
}

fn retrieveWindows(ptr: *anyopaque, service: []const u8, key: []const u8, out_ptr: *[*]u8, out_len: *usize) SecretError {
    if (builtin.os.tag != .windows) {
        return SecretError.NotImplemented;
    }

    const impl: *WindowsSecretImpl = @ptrCast(@alignCast(ptr));
    @setRuntimeSafety(false);

    const target = std.fmt.allocPrintZ(impl.allocator, "little_timer:{s}:{s}", .{ service, key }) catch return SecretError.OutOfMemory;
    defer impl.allocator.free(target);

    const target_utf16 = std.unicode.utf8ToUtf16LeAllocZ(impl.allocator, target) catch return SecretError.OutOfMemory;
    defer impl.allocator.free(target_utf16);

    var cred: ?*windows.CREDENTIALW = null;
    if (windows.CredReadW(target_utf16.ptr, windows.CRED_TYPE_GENERIC, 0, &cred) == 0) {
        return SecretError.NotFound;
    }
    defer {
        if (cred) |c| {
            windows.CredFree(c);
        }
    }

    const c = cred.?;
    if (c.CredentialBlobSize == 0 or c.CredentialBlob == null) {
        return SecretError.NotFound;
    }

    const copy = impl.allocator.dupe(u8, c.CredentialBlob.?[0..c.CredentialBlobSize]) catch return SecretError.OutOfMemory;
    out_ptr.* = copy.ptr;
    out_len.* = copy.len;

    return;
}

fn deleteWindows(ptr: *anyopaque, service: []const u8, key: []const u8) SecretError {
    if (builtin.os.tag != .windows) {
        return SecretError.NotImplemented;
    }

    const impl: *WindowsSecretImpl = @ptrCast(@alignCast(ptr));
    @setRuntimeSafety(false);

    const target = std.fmt.allocPrintZ(impl.allocator, "little_timer:{s}:{s}", .{ service, key }) catch return SecretError.OutOfMemory;
    defer impl.allocator.free(target);

    const target_utf16 = std.unicode.utf8ToUtf16LeAllocZ(impl.allocator, target) catch return SecretError.OutOfMemory;
    defer impl.allocator.free(target_utf16);

    if (windows.CredDeleteW(target_utf16.ptr, windows.CRED_TYPE_GENERIC, 0) == 0) {
        return SecretError.NotFound;
    }

    return;
}

fn freeWindows(ptr: *anyopaque, buffer: [*]u8, len: usize) void {
    _ = ptr;
    @setRuntimeSafety(false);
    if (len > 0 and buffer != null) {
        std.heap.page_allocator.free(buffer[0..len]);
    }
}

fn createAndroid(allocator: std.mem.Allocator) !SecretService {
    @setRuntimeSafety(false);
    const impl = try allocator.create(AndroidSecretImpl);
    impl.* = .{ .allocator = allocator };
    return SecretService{
        .ptr = @ptrCast(impl),
        .vtable = &.{
            .store = storeAndroid,
            .retrieve = retrieveAndroid,
            .delete = deleteAndroid,
            .free = freeAndroid,
        },
    };
}

const AndroidSecretImpl = struct {
    allocator: std.mem.Allocator,
};

fn storeAndroid(ptr: *anyopaque, service: []const u8, key: []const u8, value: []const u8) SecretError {
    _ = ptr;
    _ = service;
    _ = key;
    _ = value;
    @setRuntimeSafety(false);
    return SecretError.NotImplemented;
}

fn retrieveAndroid(ptr: *anyopaque, service: []const u8, key: []const u8, out_ptr: *[*]u8, out_len: *usize) SecretError {
    _ = ptr;
    _ = service;
    _ = key;
    _ = out_ptr;
    _ = out_len;
    @setRuntimeSafety(false);
    return SecretError.NotImplemented;
}

fn deleteAndroid(ptr: *anyopaque, service: []const u8, key: []const u8) SecretError {
    _ = ptr;
    _ = service;
    _ = key;
    @setRuntimeSafety(false);
    return SecretError.NotImplemented;
}

fn freeAndroid(ptr: *anyopaque, buffer: [*]u8, len: usize) void {
    _ = ptr;
    _ = buffer;
    _ = len;
    @setRuntimeSafety(false);
}

pub fn storeMasterKey(key: []const u8) SecretError!void {
    const key_desc = "little_timer:master_key";
    logger.global_logger.info("storeMasterKey: key_len={d}, desc={s}", .{key.len, key_desc});

    const alloc = std.heap.page_allocator;

    _ = keyring.joinOrCreateSessionKeyring() catch |err| switch (err) {
        error.KeyringNotFound, error.KeyringCreateFailed => {
            logger.global_logger.warn("storeMasterKey: join session keyring failed: {any}", .{err});
        },
        else => {
            logger.global_logger.warn("storeMasterKey: join session keyring failed: {any}", .{err});
        },
    };

    keyring.storeKeyInKeyring(key_desc, key) catch |err| {
        logger.global_logger.warn("storeMasterKey: store key failed, continuing with in-memory cache: {any}", .{err});
    };

    if (master_key_instance) |existing| {
        alloc.free(existing);
        master_key_instance = null;
    }
    const cached = alloc.dupe(u8, key) catch return SecretError.OutOfMemory;
    master_key_instance = cached;
    logger.global_logger.info("storeMasterKey: cached key in memory, len={d}", .{cached.len});
}

pub fn retrieveMasterKey() SecretError![]u8 {
    if (master_key_instance) |key| {
        return key;
    }

    const allocator = std.heap.page_allocator;
    const key_desc = "little_timer:master_key";

    _ = keyring.joinOrCreateSessionKeyring() catch |err| switch (err) {
        error.KeyringNotFound, error.KeyringCreateFailed => return SecretError.NotFound,
        else => return SecretError.PlatformError,
    };

    const key_data = keyring.retrieveKeyFromKeyring(key_desc, allocator) catch |err| switch (err) {
        error.KeyringNotFound => {
            logger.global_logger.warn("retrieveMasterKey: key not found: {any}", .{err});
            return SecretError.NotFound;
        },
        error.KeyringRetrieveFailed => {
            logger.global_logger.warn("retrieveMasterKey: key read failed, regenerate: {any}", .{err});
            return SecretError.NotFound;
        },
        else => {
            logger.global_logger.err("retrieveMasterKey: key retrieval failed: {any}", .{err});
            return SecretError.PlatformError;
        },
    };
    master_key_instance = key_data;
    return key_data;
}

pub fn deleteMasterKey() void {
    const allocator = std.heap.page_allocator;

    if (master_key_instance) |key| {
        allocator.free(key);
        master_key_instance = null;
    }
}
