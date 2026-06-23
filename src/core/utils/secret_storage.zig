//! 密钥存储模块 - 提供跨平台的安全凭证存储
//! Linux: libsecret (D-Bus org.freedesktop.secrets)
//! macOS: Keychain Services (stub)
//! Windows: wincred API
//! Android: AccountManager (stub)

const std = @import("std");
const builtin = @import("builtin");
const logger = @import("../logger.zig");

const libsecret = @cImport({
    @cInclude("libsecret/secret.h");
});

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

pub const SecretError = error{
    NotFound,
    AlreadyExists,
    InvalidValue,
    NoAccess,
    OutOfMemory,
    NotImplemented,
    PlatformError,
    Base64Error,
};

pub const SecretSchema = struct { name: []const u8, attributes: []const Attribute };
pub const Attribute = struct { key: []const u8, value: []const u8 };

pub const SecretService = struct {
    ptr: *anyopaque,
    vtable: *const VTable,
    allocator: std.mem.Allocator,

    pub const VTable = struct {
        store: *const fn (*anyopaque, []const u8, []const u8, []const u8) SecretError!void,
        retrieve: *const fn (*anyopaque, std.mem.Allocator, []const u8, []const u8) SecretError![]u8,
        delete: *const fn (*anyopaque, []const u8, []const u8) SecretError!void,
    };

    pub fn store(self: *SecretService, service: []const u8, key: []const u8, value: []const u8) SecretError!void {
        return self.vtable.store(self.ptr, service, key, value);
    }
    pub fn retrieve(self: *SecretService, allocator: std.mem.Allocator, service: []const u8, key: []const u8) SecretError![]u8 {
        return self.vtable.retrieve(self.ptr, allocator, service, key);
    }
    pub fn delete(self: *SecretService, service: []const u8, key: []const u8) SecretError!void {
        return self.vtable.delete(self.ptr, service, key);
    }
    pub fn create(allocator: std.mem.Allocator) !SecretService {
        return switch (builtin.os.tag) {
            .linux => createLinux(allocator),
            .macos => createMac(allocator),
            .windows => createWindows(allocator),
            else => return SecretError.NotImplemented,
        };
    }
};

const LINUX_SERVICE_NAME = "little_timer";
const MASTER_KEY_KEY = "master_key";

// Linux implementation - libsecret D-Bus
fn createLinux(allocator: std.mem.Allocator) !SecretService {
    const impl = try allocator.create(LinuxSecretImpl);
    impl.* = .{ .allocator = allocator };

    libsecret.g_type_init();

    return SecretService{
        .ptr = @ptrCast(impl),
        .allocator = allocator,
        .vtable = &.{ .store = storeLinux, .retrieve = retrieveLinux, .delete = deleteLinux },
    };
}

const LinuxSecretImpl = struct { allocator: std.mem.Allocator };

fn createLinuxSchema() ?*libsecret.SecretSchema {
    const schema_attrs = libsecret.g_hash_table_new_full(
        libsecret.g_str_hash,
        libsecret.g_str_equal,
        null,
        null,
    ) orelse return null;
    defer _ = libsecret.g_hash_table_destroy(schema_attrs);

    _ = libsecret.g_hash_table_insert(schema_attrs, @constCast(@ptrCast(@as([*c]const u8, @ptrCast("service")))), @as(?*anyopaque, @ptrFromInt(@as(usize, @intCast(libsecret.SECRET_SCHEMA_ATTRIBUTE_STRING)))));
    _ = libsecret.g_hash_table_insert(schema_attrs, @constCast(@ptrCast(@as([*c]const u8, @ptrCast("account")))), @as(?*anyopaque, @ptrFromInt(@as(usize, @intCast(libsecret.SECRET_SCHEMA_ATTRIBUTE_STRING)))));

    return libsecret.secret_schema_newv("little_timer", libsecret.SECRET_SCHEMA_DONT_MATCH_NAME, schema_attrs);
}

fn storeLinux(ptr: *anyopaque, service: []const u8, key: []const u8, value: []const u8) SecretError!void {
    const impl: *LinuxSecretImpl = @ptrCast(@alignCast(ptr));
    var err: ?*libsecret.GError = null;

    const schema = createLinuxSchema() orelse return error.InvalidValue;
    defer _ = libsecret.secret_schema_unref(schema);

    const attrs = libsecret.g_hash_table_new_full(
        libsecret.g_str_hash,
        libsecret.g_str_equal,
        null,
        null,
    );
    if (attrs == null) return error.InvalidValue;
    defer _ = libsecret.g_hash_table_destroy(attrs);

    const svc_key = libsecret.g_strdup(@ptrCast(service));
    const acct_key = libsecret.g_strdup(@ptrCast(key));
    if (svc_key == null or acct_key == null) return error.InvalidValue;
    _ = libsecret.g_hash_table_insert(attrs, @constCast(@ptrCast(@as([*c]const u8, @ptrCast("service")))), svc_key);
    _ = libsecret.g_hash_table_insert(attrs, @constCast(@ptrCast(@as([*c]const u8, @ptrCast("account")))), acct_key);

    const label_bytes = std.mem.concat(impl.allocator, u8, &.{ service, key }) catch {
        return error.InvalidValue;
    };
    defer impl.allocator.free(label_bytes);
    const label_null = impl.allocator.alloc(u8, label_bytes.len + 1) catch { return error.InvalidValue; };
    @memcpy(label_null.ptr, label_bytes);
    label_null[label_bytes.len] = 0;
    defer impl.allocator.free(label_null);

    const encoded_len = std.base64.standard.Encoder.calcSize(value.len);
    const encoded = try impl.allocator.alloc(u8, encoded_len);
    errdefer impl.allocator.free(encoded);
    const encoded_slice = std.base64.standard.Encoder.encode(encoded, value);
    const encoded_copy = impl.allocator.alloc(u8, encoded_slice.len + 1) catch { return error.InvalidValue; };
    @memcpy(encoded_copy.ptr, encoded_slice);
    encoded_copy[encoded_slice.len] = 0;
    defer impl.allocator.free(encoded_copy);

    const res = libsecret.secret_password_storev_sync(schema, attrs, null, @ptrCast(label_null.ptr), @ptrCast(encoded_copy.ptr), null, @ptrCast(&err));
    if (res == 0) {
        if (err) |e| libsecret.g_error_free(e);
        return error.NoAccess;
    }
    if (err) |e| libsecret.g_error_free(e);
}

fn retrieveLinux(ptr: *anyopaque, allocator: std.mem.Allocator, service: []const u8, key: []const u8) SecretError![]u8 {
    const impl: *LinuxSecretImpl = @ptrCast(@alignCast(ptr));
    _ = impl;
    var err: ?*libsecret.GError = null;

    const schema = createLinuxSchema() orelse return error.InvalidValue;
    defer _ = libsecret.secret_schema_unref(schema);

    const attrs = libsecret.g_hash_table_new_full(
        libsecret.g_str_hash,
        libsecret.g_str_equal,
        null,
        null,
    );
    if (attrs == null) return error.InvalidValue;
    defer _ = libsecret.g_hash_table_destroy(attrs);

    const svc_key = libsecret.g_strdup(@ptrCast(service));
    const acct_key = libsecret.g_strdup(@ptrCast(key));
    if (svc_key == null or acct_key == null) return error.InvalidValue;
    _ = libsecret.g_hash_table_insert(attrs, @constCast(@ptrCast(@as([*c]const u8, @ptrCast("service")))), svc_key);
    _ = libsecret.g_hash_table_insert(attrs, @constCast(@ptrCast(@as([*c]const u8, @ptrCast("account")))), acct_key);

    const password = libsecret.secret_password_lookupv_sync(schema, attrs, null, @ptrCast(&err));
    if (err != null) {
        if (err) |e| libsecret.g_error_free(e);
        return error.NotFound;
    }
    if (password == null) {
        return error.NotFound;
    }
    defer libsecret.secret_password_free(password);

    const len = std.mem.len(password);
    const raw = password[0..len];

    if (isBase64(raw)) {
        const decoder = std.base64.standard.Decoder;
        const decoded_len = decoder.calcSizeForSlice(raw) catch return error.Base64Error;
        const decoded_buf = try allocator.alloc(u8, decoded_len);
        errdefer allocator.free(decoded_buf);
        decoder.decode(decoded_buf, raw) catch return error.Base64Error;
        return decoded_buf;
    } else {
        return allocator.dupe(u8, raw) catch return error.OutOfMemory;
    }
}

fn deleteLinux(ptr: *anyopaque, service: []const u8, key: []const u8) SecretError!void {
    const impl: *LinuxSecretImpl = @ptrCast(@alignCast(ptr));
    _ = impl;
    var err: ?*libsecret.GError = null;

    const schema = createLinuxSchema() orelse return error.InvalidValue;
    defer _ = libsecret.secret_schema_unref(schema);

    const attrs = libsecret.g_hash_table_new_full(
        libsecret.g_str_hash,
        libsecret.g_str_equal,
        null,
        null,
    );
    if (attrs == null) return error.InvalidValue;
    defer _ = libsecret.g_hash_table_destroy(attrs);

    const svc_key = libsecret.g_strdup(@ptrCast(service));
    const acct_key = libsecret.g_strdup(@ptrCast(key));
    if (svc_key == null or acct_key == null) return error.InvalidValue;
    _ = libsecret.g_hash_table_insert(attrs, @constCast(@ptrCast(@as([*c]const u8, @ptrCast("service")))), svc_key);
    _ = libsecret.g_hash_table_insert(attrs, @constCast(@ptrCast(@as([*c]const u8, @ptrCast("account")))), acct_key);

    const res = libsecret.secret_password_clearv_sync(schema, attrs, null, @ptrCast(&err));
    if (res == 0) {
        if (err) |e| libsecret.g_error_free(e);
        return error.NoAccess;
    }
    if (err) |e| libsecret.g_error_free(e);
}

// macOS
fn createMac(allocator: std.mem.Allocator) !SecretService {
    const impl = try allocator.create(MacSecretImpl);
    impl.* = .{ .allocator = allocator };
    return SecretService{
        .ptr = @ptrCast(impl),
        .allocator = allocator,
        .vtable = &.{ .store = storeMac, .retrieve = retrieveMac, .delete = deleteMac },
    };
}

const MacSecretImpl = struct { allocator: std.mem.Allocator };

fn storeMac(_: *anyopaque, _: []const u8, _: []const u8, _: []const u8) SecretError!void { return SecretError.NotImplemented; }
fn retrieveMac(_: *anyopaque, _: std.mem.Allocator, _: []const u8, _: []const u8) SecretError![]u8 { return SecretError.NotImplemented; }
fn deleteMac(_: *anyopaque, _: []const u8, _: []const u8) SecretError!void { return SecretError.NotImplemented; }

// Windows
fn createWindows(allocator: std.mem.Allocator) !SecretService {
    const impl = try allocator.create(WindowsSecretImpl);
    impl.* = .{ .allocator = allocator };
    return SecretService{
        .ptr = @ptrCast(impl),
        .allocator = allocator,
        .vtable = &.{ .store = storeWindows, .retrieve = retrieveWindows, .delete = deleteWindows },
    };
}

const WindowsSecretImpl = struct { allocator: std.mem.Allocator };

const windows = struct {
    const DWORD = u32;
    const WCHAR = u16;
    const LPWSTR = ?[*:0]WCHAR;
    const BOOL = c_int;
    const FILETIME = extern struct { dwLowDateTime: DWORD, dwHighDateTime: DWORD };
    const CREDENTIALW = extern struct {
        Flags: DWORD, Type: DWORD, TargetName: LPWSTR, Comment: LPWSTR, LastWritten: FILETIME,
        CredentialBlobSize: DWORD, CredentialBlob: ?[*]u8, Persist: DWORD, AttributeCount: DWORD,
        Attributes: ?*anyopaque, TargetAlias: LPWSTR, UserName: LPWSTR,
    };
    const CRED_PERSIST_LOCAL_MACHINE: DWORD = 2;
    const CRED_TYPE_GENERIC: DWORD = 1;
    extern "advapi32" fn CredWriteW(*const CREDENTIALW, DWORD) callconv(.Stdcall) BOOL;
    extern "advapi32" fn CredReadW(LPWSTR, DWORD, DWORD, *?*CREDENTIALW) callconv(.Stdcall) BOOL;
    extern "advapi32" fn CredDeleteW(LPWSTR, DWORD, DWORD) callconv(.Stdcall) BOOL;
    extern "advapi32" fn CredFree(*anyopaque) callconv(.Stdcall) void;
};

fn storeWindows(ptr: *anyopaque, service: []const u8, key: []const u8, value: []const u8) SecretError!void {
    if (builtin.os.tag != .windows) return SecretError.NotImplemented;
    const impl: *WindowsSecretImpl = @ptrCast(@alignCast(ptr));
    const target = std.fmt.allocPrintZ(impl.allocator, "little_timer:{s}:{s}", .{service, key}) catch return SecretError.OutOfMemory;
    defer impl.allocator.free(target);
    const utf16 = std.unicode.utf8ToUtf16LeAllocZ(impl.allocator, target) catch return SecretError.OutOfMemory;
    defer impl.allocator.free(utf16);

    const encoded_len = std.base64.standard.Encoder.calcSize(value.len);
    const encoded = try impl.allocator.alloc(u8, encoded_len);
    errdefer impl.allocator.free(encoded);
    const encoded_slice = std.base64.standard.Encoder.encode(encoded, value);
    const utf16_value = std.unicode.utf8ToUtf16LeAllocZ(impl.allocator, encoded_slice) catch return SecretError.OutOfMemory;
    defer impl.allocator.free(utf16_value);

    const cred = windows.CREDENTIALW{
        .TargetName = utf16.ptr,
        .Type = windows.CRED_TYPE_GENERIC,
        .CredentialBlobSize = @intCast(utf16_value.len * 2),
        .CredentialBlob = @ptrCast(utf16_value.ptr),
        .Persist = windows.CRED_PERSIST_LOCAL_MACHINE,
    };
    if (windows.CredWriteW(&cred, 0) == 0) return SecretError.PlatformError;
}

fn retrieveWindows(ptr: *anyopaque, allocator: std.mem.Allocator, service: []const u8, key: []const u8) SecretError![]u8 {
    if (builtin.os.tag != .windows) return SecretError.NotImplemented;
    const impl: *WindowsSecretImpl = @ptrCast(@alignCast(ptr));
    _ = impl;
    const target = std.fmt.allocPrintZ(allocator, "little_timer:{s}:{s}", .{service, key}) catch return SecretError.OutOfMemory;
    defer allocator.free(target);
    const utf16 = std.unicode.utf8ToUtf16LeAllocZ(allocator, target) catch return SecretError.OutOfMemory;
    defer allocator.free(utf16);
    var cred: ?*windows.CREDENTIALW = null;
    if (windows.CredReadW(utf16.ptr, windows.CRED_TYPE_GENERIC, 0, &cred) == 0) return SecretError.NotFound;
    defer if (cred) |c| windows.CredFree(c);
    const c = cred.?;
    if (c.CredentialBlobSize == 0 or c.CredentialBlob == null) return SecretError.NotFound;
    const blob_len = c.CredentialBlobSize / 2;
    const utf16_slice: [:0]u16 = @ptrCast(@alignCast(c.CredentialBlob.?[0 .. blob_len * 2]));
    const encoded = std.unicode.utf16LeToUtf8Alloc(allocator, utf16_slice) catch return SecretError.InvalidValue;
    defer allocator.free(encoded);

    const decoder = std.base64.standard.Decoder;
    const decoded_len = try decoder.calcSizeForSlice(encoded);
    const decoded_buf = try allocator.alloc(u8, decoded_len);
    errdefer allocator.free(decoded_buf);
    decoder.decode(decoded_buf, encoded);
    return decoded_buf;
}

fn deleteWindows(ptr: *anyopaque, service: []const u8, key: []const u8) SecretError!void {
    if (builtin.os.tag != .windows) return SecretError.NotImplemented;
    const impl: *WindowsSecretImpl = @ptrCast(@alignCast(ptr));
    const target = std.fmt.allocPrintZ(impl.allocator, "little_timer:{s}:{s}", .{service, key}) catch return SecretError.OutOfMemory;
    defer impl.allocator.free(target);
    const utf16 = std.unicode.utf8ToUtf16LeAllocZ(impl.allocator, target) catch return SecretError.OutOfMemory;
    defer impl.allocator.free(utf16);
    if (windows.CredDeleteW(utf16.ptr, windows.CRED_TYPE_GENERIC, 0) == 0) return SecretError.NotFound;
}

// Android
fn createAndroid(allocator: std.mem.Allocator) !SecretService {
    const impl = try allocator.create(AndroidSecretImpl);
    impl.* = .{ .allocator = allocator };
    return SecretService{
        .ptr = @ptrCast(impl),
        .allocator = allocator,
        .vtable = &.{ .store = storeAndroid, .retrieve = retrieveAndroid, .delete = deleteAndroid },
    };
}

const AndroidSecretImpl = struct { allocator: std.mem.Allocator };

fn storeAndroid(_: *anyopaque, _: []const u8, _: []const u8, _: []const u8) SecretError!void { return SecretError.NotImplemented; }
fn retrieveAndroid(_: *anyopaque, _: std.mem.Allocator, _: []const u8, _: []const u8) SecretError![]u8 { return SecretError.NotImplemented; }
fn deleteAndroid(_: *anyopaque, _: []const u8, _: []const u8) SecretError!void { return SecretError.NotImplemented; }

// Master key functions
var global_service: SecretService = undefined;
var global_service_init = false;

fn getGlobalSecretService() !*SecretService {
    if (global_service_init) return &global_service;
    global_service = try SecretService.create(std.heap.page_allocator);
    global_service_init = true;
    return &global_service;
}

pub fn storeMasterKey(key: []const u8) SecretError!void {
    logger.global_logger.info("storeMasterKey: key_len={d}", .{key.len});
    const svc = try getGlobalSecretService();
    try svc.store(LINUX_SERVICE_NAME, MASTER_KEY_KEY, key);
    logger.global_logger.info("storeMasterKey: stored", .{});
}

pub fn retrieveMasterKey(allocator: std.mem.Allocator) SecretError![]u8 {
    const svc = try getGlobalSecretService();
    return try svc.retrieve(allocator, LINUX_SERVICE_NAME, MASTER_KEY_KEY);
}

pub fn deleteMasterKey() void {
    if (global_service_init) {
        _ = global_service.delete(LINUX_SERVICE_NAME, MASTER_KEY_KEY) catch {};
    }
}
