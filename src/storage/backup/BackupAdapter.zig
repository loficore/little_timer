const std = @import("std");

pub const BackupError = error{
    BackupFailed,
    RestoreFailed,
    InvalidBackupPath,
    DatabaseOpenFailed,
    ConnectionFailed,
    AuthenticationFailed,
    FileNotFound,
    PermissionDenied,
    NetworkError,
    OutOfMemory,
};

pub const BackupTarget = enum {
    local,
    webdav,
    s3,
};

pub const BackupInfo = struct {
    name: []const u8,
    timestamp: i64,
    size_bytes: u64,
};

pub const WebDAVConfig = struct {
    url: []const u8,
    username: []const u8,
    password: []const u8,
    base_path: []const u8 = "/",
};

pub const S3Config = struct {
    endpoint: []const u8,
    bucket: []const u8,
    region: []const u8,
    access_key: []const u8,
    secret_key: []const u8,
    path_prefix: []const u8 = "little_timer/",
};

pub const BackupAdapter = struct {
    ptr: *anyopaque,
    vtable: *const VTable,

    pub const VTable = struct {
        push: *const fn (*anyopaque, []const u8, []const u8) BackupError!void,
        pull: *const fn (*anyopaque, []const u8, []const u8) BackupError!void,
        list: *const fn (*anyopaque) BackupError![]BackupInfo,
        delete: *const fn (*anyopaque, []const u8) BackupError!void,
        freeList: *const fn (*anyopaque, []BackupInfo) void,
    };

    pub fn push(self: BackupAdapter, db_path: []const u8, backup_name: []const u8) BackupError!void {
        return self.vtable.push(self.ptr, db_path, backup_name);
    }

    pub fn pull(self: BackupAdapter, backup_name: []const u8, dest_path: []const u8) BackupError!void {
        return self.vtable.pull(self.ptr, backup_name, dest_path);
    }

    pub fn list(self: BackupAdapter) BackupError![]BackupInfo {
        return self.vtable.list(self.ptr);
    }

    pub fn delete(self: BackupAdapter, backup_name: []const u8) BackupError!void {
        return self.vtable.delete(self.ptr, backup_name);
    }

    pub fn freeList(self: BackupAdapter, items: []BackupInfo) void {
        return self.vtable.freeList(self.ptr, items);
    }
};

const LocalAdapterState = struct {
    allocator: std.mem.Allocator,
    backup_path: []const u8,

    fn pushImpl(ptr: *anyopaque, db_path: []const u8, backup_name: []const u8) BackupError!void {
        const self: *LocalAdapterState = @ptrCast(@alignCast(ptr));

        std.fs.cwd().access(self.backup_path, .{}) catch {
            std.fs.cwd().makeDir(self.backup_path) catch return BackupError.BackupFailed;
        };

        const full_path = std.fs.path.join(self.allocator, &[_][]const u8{ self.backup_path, backup_name }) catch return BackupError.BackupFailed;
        defer self.allocator.free(full_path);

        std.fs.cwd().copyFile(db_path, std.fs.cwd(), full_path, .{}) catch return BackupError.BackupFailed;
    }

    fn pullImpl(ptr: *anyopaque, backup_name: []const u8, dest_path: []const u8) BackupError!void {
        const self: *LocalAdapterState = @ptrCast(@alignCast(ptr));

        const full_path = std.fs.path.join(self.allocator, &[_][]const u8{ self.backup_path, backup_name }) catch return BackupError.RestoreFailed;
        defer self.allocator.free(full_path);

        std.fs.cwd().access(full_path, .{}) catch return BackupError.FileNotFound;
        std.fs.cwd().copyFile(full_path, std.fs.cwd(), dest_path, .{}) catch return BackupError.RestoreFailed;
    }

    fn listImpl(ptr: *anyopaque) BackupError![]BackupInfo {
        const self: *LocalAdapterState = @ptrCast(@alignCast(ptr));

        var dir = std.fs.cwd().openDir(self.backup_path, .{ .iterate = true }) catch return BackupError.BackupFailed;
        defer dir.close();

        var list = std.ArrayListUnmanaged(BackupInfo){};
        errdefer {
            for (list.items) |item| self.allocator.free(item.name);
            list.deinit(self.allocator);
        }

        var iter = dir.iterate();
        while (true) {
            const entry = iter.next() catch break;
            if (entry == null) break;
            const e = entry.?;
            if (e.kind == .file and std.mem.startsWith(u8, e.name, "presets_backup_") and std.mem.endsWith(u8, e.name, ".db")) {
                const full_path = std.fs.path.join(self.allocator, &[_][]const u8{ self.backup_path, e.name }) catch continue;
                defer self.allocator.free(full_path);

                const file = std.fs.cwd().openFile(full_path, .{}) catch continue;
                defer file.close();

                const stat = file.stat() catch continue;

                const name_prefix = "presets_backup_";
                const name_suffix = ".db";
                const timestamp_start = name_prefix.len;
                const timestamp_end = e.name.len - name_suffix.len;

                if (timestamp_end > timestamp_start) {
                    const timestamp_str = e.name[timestamp_start..timestamp_end];
                    const timestamp = std.fmt.parseInt(i64, timestamp_str, 10) catch continue;

                    const info = BackupInfo{
                        .name = try self.allocator.dupe(u8, e.name),
                        .timestamp = timestamp,
                        .size_bytes = @intCast(stat.size),
                    };
                    try list.append(self.allocator, info);
                }
            }
        }

        return try list.toOwnedSlice(self.allocator);
    }

    fn deleteImpl(ptr: *anyopaque, backup_name: []const u8) BackupError!void {
        const self: *LocalAdapterState = @ptrCast(@alignCast(ptr));

        const full_path = std.fs.path.join(self.allocator, &[_][]const u8{ self.backup_path, backup_name }) catch return BackupError.BackupFailed;
        defer self.allocator.free(full_path);

        std.fs.cwd().deleteFile(full_path) catch return BackupError.BackupFailed;
    }

    fn freeListImpl(ptr: *anyopaque, list: []BackupInfo) void {
        const self: *LocalAdapterState = @ptrCast(@alignCast(ptr));
        for (list) |info| {
            self.allocator.free(info.name);
        }
        self.allocator.free(list);
    }

    fn createVTable() BackupAdapter.VTable {
        return .{
            .push = @as(*const fn (*anyopaque, []const u8, []const u8) BackupError!void, pushImpl),
            .pull = @as(*const fn (*anyopaque, []const u8, []const u8) BackupError!void, pullImpl),
            .list = @as(*const fn (*anyopaque) BackupError![]BackupInfo, listImpl),
            .delete = @as(*const fn (*anyopaque, []const u8) BackupError!void, deleteImpl),
            .freeList = @as(*const fn (*anyopaque, []BackupInfo) void, freeListImpl),
        };
    }
};

var local_vtable_storage: ?BackupAdapter.VTable = null;

pub fn createLocalAdapter(allocator: std.mem.Allocator, config: struct { path: []const u8 }) BackupAdapter {
    const state = allocator.create(LocalAdapterState) catch unreachable;
    state.* = .{
        .allocator = allocator,
        .backup_path = config.path,
    };
    if (local_vtable_storage == null) {
        local_vtable_storage = LocalAdapterState.createVTable();
    }
    return .{
        .ptr = state,
        .vtable = &local_vtable_storage.?,
    };
}

const WebDAVAdapterState = struct {
    allocator: std.mem.Allocator,
    config: WebDAVConfig,

    fn pushImpl(ptr: *anyopaque, db_path: []const u8, backup_name: []const u8) BackupError!void {
        const self: *WebDAVAdapterState = @ptrCast(@alignCast(ptr));

        const remote_path = std.fs.path.join(self.allocator, &[_][]const u8{ self.config.base_path, backup_name }) catch return BackupError.BackupFailed;
        defer self.allocator.free(remote_path);

        const file_data = std.fs.cwd().readFileAlloc(self.allocator, db_path, 100 * 1024 * 1024) catch return BackupError.BackupFailed;
        defer self.allocator.free(file_data);

        var client = std.http.Client{ .allocator = self.allocator };
        defer client.deinit();

        const uri = std.Uri.parse(self.config.url) catch return BackupError.ConnectionFailed;

        var request = client.request(.PUT, uri, .{
            .extra_headers = &.{
                .{ .name = "Content-Type", .value = "application/octet-stream" },
                .{ .name = "Destination", .value = remote_path },
            },
        }) catch return BackupError.NetworkError;
        defer request.deinit();

        request.sendBodyComplete(file_data) catch return BackupError.NetworkError;

        var redirect_buffer: [8192]u8 = undefined;
        const response = request.receiveHead(&redirect_buffer) catch return BackupError.NetworkError;

        if (response.head.status != .ok and response.head.status != .created and response.head.status != .no_content) {
            return BackupError.BackupFailed;
        }
    }

    fn pullImpl(ptr: *anyopaque, backup_name: []const u8, dest_path: []const u8) BackupError!void {
        const self: *WebDAVAdapterState = @ptrCast(@alignCast(ptr));

        const remote_path = std.fs.path.join(self.allocator, &[_][]const u8{ self.config.base_path, backup_name }) catch return BackupError.RestoreFailed;
        defer self.allocator.free(remote_path);

        var client = std.http.Client{ .allocator = self.allocator };
        defer client.deinit();

        const uri = std.Uri.parse(self.config.url) catch return BackupError.ConnectionFailed;

        var request = client.request(.GET, uri, .{}) catch return BackupError.NetworkError;
        defer request.deinit();

        var redirect_buffer: [8192]u8 = undefined;
        var response = request.receiveHead(&redirect_buffer) catch return BackupError.NetworkError;

        if (response.head.status == .not_found) return BackupError.FileNotFound;
        if (response.head.status != .ok) return BackupError.RestoreFailed;

        const body = response.reader(&.{});

        const file = std.fs.cwd().createFile(dest_path, .{}) catch return BackupError.RestoreFailed;
        defer file.close();

        while (true) {
            var buf: [8192]u8 = undefined;
            const n = body.readSliceShort(buf[0..]) catch |err| {
                if (err == error.EndOfStream) break;
                return BackupError.RestoreFailed;
            };
            if (n == 0) break;
            file.writeAll(buf[0..n]) catch return BackupError.RestoreFailed;
        }
    }

    fn listImpl(ptr: *anyopaque) BackupError![]BackupInfo {
        const self: *WebDAVAdapterState = @ptrCast(@alignCast(ptr));

        var client = std.http.Client{ .allocator = self.allocator };
        defer client.deinit();

        const uri = std.Uri.parse(self.config.url) catch return BackupError.ConnectionFailed;

        const propfind_body = "<?xml version=\"1.0\" encoding=\"utf-8\"?><D:propfind xmlns:D=\"DAV:\"><D:prop><D:getlastmodified/><D:getcontentlength/></D:prop></D:propfind>";
        const body_copy = try self.allocator.dupe(u8, propfind_body);
        defer self.allocator.free(body_copy);

        var request = client.request(.POST, uri, .{
            .extra_headers = &.{
                .{ .name = "Depth", .value = "1" },
                .{ .name = "Content-Type", .value = "application/xml" },
            },
        }) catch return BackupError.NetworkError;
        defer request.deinit();

        request.sendBodyComplete(body_copy) catch return BackupError.NetworkError;

        var redirect_buffer: [8192]u8 = undefined;
        var response = request.receiveHead(&redirect_buffer) catch return BackupError.NetworkError;

        if (response.head.status != .multi_status) return BackupError.BackupFailed;

        var list = std.ArrayListUnmanaged(BackupInfo){};
        errdefer {
            for (list.items) |item| self.allocator.free(item.name);
            list.deinit(self.allocator);
        }

        const body = response.reader(&.{});
        const body_data = body.readAlloc(self.allocator, 10 * 1024 * 1024) catch return BackupError.BackupFailed;
        defer self.allocator.free(body_data);

        var parser = WebDAVXmlParser.init(self.allocator, body_data);

        while (parser.next()) {
            if (parser.isBackupEntry()) {
                const info = BackupInfo{
                    .name = parser.getFilename() orelse continue,
                    .timestamp = parser.getTimestamp() orelse 0,
                    .size_bytes = parser.getSize() orelse 0,
                };
                if (info.name.len > 0) {
                    const name_copy = try self.allocator.dupe(u8, info.name);
                    errdefer {
                        self.allocator.free(name_copy);
                        for (list.items) |item| self.allocator.free(item.name);
                        list.deinit(self.allocator);
                    }
                    try list.append(self.allocator, .{ .name = name_copy, .timestamp = info.timestamp, .size_bytes = info.size_bytes });
                }
            }
        }

        return try list.toOwnedSlice(self.allocator);
    }

    fn deleteImpl(ptr: *anyopaque, backup_name: []const u8) BackupError!void {
        const self: *WebDAVAdapterState = @ptrCast(@alignCast(ptr));

        const remote_path = std.fs.path.join(self.allocator, &[_][]const u8{ self.config.base_path, backup_name }) catch return BackupError.BackupFailed;
        defer self.allocator.free(remote_path);

        const delete_url = std.fmt.allocPrint(self.allocator, "{s}{s}", .{ self.config.url, remote_path }) catch return BackupError.BackupFailed;
        defer self.allocator.free(delete_url);

        const delete_uri = std.Uri.parse(delete_url) catch return BackupError.ConnectionFailed;

        var client = std.http.Client{ .allocator = self.allocator };
        defer client.deinit();

        var request = client.request(.DELETE, delete_uri, .{}) catch return BackupError.NetworkError;
        defer request.deinit();

        var redirect_buffer: [8192]u8 = undefined;
        const response = request.receiveHead(&redirect_buffer) catch return BackupError.NetworkError;

        if (response.head.status != .ok and response.head.status != .no_content and response.head.status != .not_found) {
            return BackupError.BackupFailed;
        }
    }

    fn freeListImpl(ptr: *anyopaque, list: []BackupInfo) void {
        const self: *WebDAVAdapterState = @ptrCast(@alignCast(ptr));
        for (list) |info| {
            self.allocator.free(info.name);
        }
        self.allocator.free(list);
    }

    fn createVTable() BackupAdapter.VTable {
        return .{
            .push = @as(*const fn (*anyopaque, []const u8, []const u8) BackupError!void, pushImpl),
            .pull = @as(*const fn (*anyopaque, []const u8, []const u8) BackupError!void, pullImpl),
            .list = @as(*const fn (*anyopaque) BackupError![]BackupInfo, listImpl),
            .delete = @as(*const fn (*anyopaque, []const u8) BackupError!void, deleteImpl),
            .freeList = @as(*const fn (*anyopaque, []BackupInfo) void, freeListImpl),
        };
    }
};

var webdav_vtable_storage: ?BackupAdapter.VTable = null;

pub fn createWebDAVAdapter(allocator: std.mem.Allocator, config: WebDAVConfig) BackupAdapter {
    const state = allocator.create(WebDAVAdapterState) catch unreachable;
    state.* = .{
        .allocator = allocator,
        .config = config,
    };
    if (webdav_vtable_storage == null) {
        webdav_vtable_storage = WebDAVAdapterState.createVTable();
    }
    return .{
        .ptr = state,
        .vtable = &webdav_vtable_storage.?,
    };
}

const WebDAVXmlParser = struct {
    allocator: std.mem.Allocator,
    data: []const u8,
    pos: usize,
    in_response: bool = false,
    current_href: ?[]u8 = null,
    current_size: ?u64 = null,
    current_modified: ?i64 = null,
    in_entry: bool = false,

    fn init(allocator: std.mem.Allocator, data: []const u8) WebDAVXmlParser {
        return .{
            .allocator = allocator,
            .data = data,
            .pos = 0,
        };
    }

    fn next(parser: *WebDAVXmlParser) bool {
        while (parser.pos < parser.data.len) {
            const remaining = parser.data[parser.pos..];
            if (remaining.len == 0) return false;

            if (remaining[0] == '<') {
                if (remaining.len >= 4 and std.mem.eql(u8, remaining[0..4], "<!--")) {
                    const end = std.mem.indexOf(u8, remaining, "-->") orelse remaining.len;
                    parser.pos += end + 3;
                    continue;
                }

                if (remaining.len >= 2 and remaining[1] == '?') {
                    const end = std.mem.indexOfScalar(u8, remaining, '>') orelse remaining.len;
                    parser.pos = parser.pos + end + 1;
                    continue;
                }

                if (remaining.len >= 2 and remaining[1] == '/') {
                    const end = std.mem.indexOfScalar(u8, remaining, '>') orelse remaining.len;
                    const name = remaining[2..end];
                    parser.pos += end + 1;

                    if (std.mem.eql(u8, name, "D:response") or std.mem.eql(u8, name, "response")) {
                        parser.in_response = false;
                        parser.current_href = null;
                        parser.current_size = null;
                        parser.current_modified = null;
                    }
                    continue;
                }

                var i: usize = 1;
                while (i < remaining.len and remaining[i] != '>' and remaining[i] != ' ') i += 1;
                const name = remaining[1..i];
                parser.pos += i + 1;

                while (parser.pos < parser.data.len and parser.data[parser.pos] != '>') parser.pos += 1;
                if (parser.pos < parser.data.len) parser.pos += 1;

                if (std.mem.eql(u8, name, "D:response") or std.mem.eql(u8, name, "response")) {
                    parser.in_response = true;
                    parser.in_entry = true;
                } else if (parser.in_entry) {
                    if (std.mem.eql(u8, name, "D:href") or std.mem.eql(u8, name, "href")) {
                        const text_start = parser.pos;
                        while (parser.pos < parser.data.len and parser.data[parser.pos] != '<') parser.pos += 1;
                        if (parser.pos > text_start) {
                            parser.current_href = parser.allocator.dupe(u8, parser.data[text_start..parser.pos]) catch null;
                        }
                    } else if (std.mem.eql(u8, name, "D:getcontentlength") or std.mem.eql(u8, name, "getcontentlength")) {
                        const text_start = parser.pos;
                        while (parser.pos < parser.data.len and parser.data[parser.pos] != '<') parser.pos += 1;
                        if (parser.pos > text_start) {
                            const text = parser.data[text_start..parser.pos];
                            parser.current_size = std.fmt.parseInt(u64, text, 10) catch null;
                        }
                    } else if (std.mem.eql(u8, name, "D:getlastmodified") or std.mem.eql(u8, name, "getlastmodified")) {
                        const text_start = parser.pos;
                        while (parser.pos < parser.data.len and parser.data[parser.pos] != '<') parser.pos += 1;
                        if (parser.pos > text_start) {
                            const text = parser.data[text_start..parser.pos];
                            _ = text;
                        }
                    }
                }
                return true;
            } else {
                while (parser.pos < parser.data.len and parser.data[parser.pos] != '<') parser.pos += 1;
            }
        }
        return false;
    }

    fn isBackupEntry(parser: *WebDAVXmlParser) bool {
        if (!parser.in_entry or parser.current_href == null) return false;
        const href = parser.current_href.?;
        const name_start = std.mem.lastIndexOfScalar(u8, href, '/') orelse 0;
        const filename = href[name_start + 1..];
        return filename.len > 0 and std.mem.startsWith(u8, filename, "presets_backup_") and std.mem.endsWith(u8, filename, ".db");
    }

    fn getFilename(parser: *WebDAVXmlParser) ?[]u8 {
        if (parser.current_href) |href| {
            const name_start = std.mem.lastIndexOfScalar(u8, href, '/') orelse 0;
            const filename = href[name_start + 1..];
            if (filename.len > 0 and std.mem.startsWith(u8, filename, "presets_backup_") and std.mem.endsWith(u8, filename, ".db")) {
                return filename;
            }
        }
        return null;
    }

    fn getTimestamp(parser: *WebDAVXmlParser) ?i64 {
        return parser.current_modified;
    }

    fn getSize(parser: *WebDAVXmlParser) ?u64 {
        return parser.current_size;
    }
};

const S3AdapterState = struct {
    allocator: std.mem.Allocator,
    config: S3Config,

    fn hmacSHA256(key: []const u8, data: []const u8) [32]u8 {
        var out: [32]u8 = undefined;
        std.crypto.auth.hmac.sha2.HmacSha256.create(out[0..], data, key);
        return out;
    }

    fn sha256Hash(data: []const u8) [32]u8 {
        var out: [32]u8 = undefined;
        std.crypto.hash.sha2.Sha256.hash(data, &out, .{});
        return out;
    }

    fn hexEncode(data: []const u8, allocator: std.mem.Allocator) []u8 {
        const hex_chars = "0123456789abcdef";
        var result = allocator.alloc(u8, data.len * 2) catch return "";
        for (data, 0..) |byte, i| {
            result[i * 2] = hex_chars[byte >> 4];
            result[i * 2 + 1] = hex_chars[byte & 0x0f];
        }
        return result;
    }

    fn signRequest(self: *S3AdapterState, _: *const std.Uri, headers: *std.http.Client.Request.Headers, timestamp: i64, method: []const u8) !void {
        const now = @as(u64, @intCast(@divTrunc(timestamp, 1000)));
        var date_buf: [32]u8 = undefined;
        const date_str = std.fmt.bufPrint(&date_buf, "{d}", .{now}) catch return error.OutOfMemory;

        const amz_date = std.fmt.allocPrint(std.heap.page_allocator, "{s}T000000Z", .{date_str}) catch return error.OutOfMemory;
        defer std.heap.page_allocator.free(amz_date);

        const date_scope = std.fmt.allocPrint(std.heap.page_allocator, "{s}/{s}/s3/aws4_request", .{ date_str, self.config.region }) catch return error.OutOfMemory;
        defer std.heap.page_allocator.free(date_scope);

        const canonical_uri = std.fmt.allocPrint(std.heap.page_allocator, "/{s}/", .{ self.config.bucket }) catch return error.OutOfMemory;
        defer std.heap.page_allocator.free(canonical_uri);

        const canonical_querystring = "";

        const signed_headers_str = "host";

        const payload_hash = "UNSIGNED-PAYLOAD";
        const canonical_request = std.fmt.allocPrint(std.heap.page_allocator,
            "{s}\n{s}\n{s}\nhost:\n\n{s}\n{s}",
            .{ method, canonical_uri, canonical_querystring, signed_headers_str, payload_hash }) catch return error.OutOfMemory;
        defer std.heap.page_allocator.free(canonical_request);

        const canonical_request_hash = hexEncode(&sha256Hash(canonical_request), std.heap.page_allocator);

        const string_to_sign = std.fmt.allocPrint(std.heap.page_allocator,
            "AWS4-HMAC-SHA256\n{s}\n{s}\n{s}",
            .{ amz_date, date_scope, canonical_request_hash }) catch return error.OutOfMemory;
        defer std.heap.page_allocator.free(string_to_sign);

        const k_date = hmacSHA256(&[_]u8{ 'A', 'W', 'S', '4', 'H', 'M', 'A', 'C', 'S', 'H', 'A', '2', '5', '6' }, date_str);
        const k_region = hmacSHA256(&k_date, self.config.region);
        const k_service = hmacSHA256(&k_region, "s3");
        const k_signing = hmacSHA256(&k_service, "aws4_request");

        const signature = hexEncode(&hmacSHA256(&k_signing, string_to_sign), std.heap.page_allocator);

        const auth_header = std.fmt.allocPrint(std.heap.page_allocator,
            "AWS4-HMAC-SHA256 Credential={s}/{s}, SignedHeaders={s}, Signature={s}",
            .{ self.config.access_key, date_scope, signed_headers_str, signature }) catch return error.OutOfMemory;
        defer std.heap.page_allocator.free(auth_header);

        headers.authorization = .{ .override = auth_header };
    }

    fn pushImpl(ptr: *anyopaque, db_path: []const u8, backup_name: []const u8) BackupError!void {
        const self: *S3AdapterState = @ptrCast(@alignCast(ptr));

        const file_data = std.fs.cwd().readFileAlloc(self.allocator, db_path, 100 * 1024 * 1024) catch return BackupError.BackupFailed;
        defer self.allocator.free(file_data);

        const object_key = std.fs.path.join(self.allocator, &[_][]const u8{ self.config.path_prefix, backup_name }) catch return BackupError.BackupFailed;
        defer self.allocator.free(object_key);

        var client = std.http.Client{ .allocator = self.allocator };
        defer client.deinit();

        const uri_str = std.fmt.allocPrint(self.allocator, "{s}/{s}/{s}", .{
            self.config.endpoint, self.config.bucket, object_key,
        }) catch return BackupError.BackupFailed;
        defer self.allocator.free(uri_str);

        const uri = std.Uri.parse(uri_str) catch return BackupError.ConnectionFailed;

        var request = client.request(.PUT, uri, .{
            .extra_headers = &.{ .{ .name = "Content-Type", .value = "application/octet-stream" } },
        }) catch return BackupError.NetworkError;
        defer request.deinit();

        const timestamp = std.time.timestamp();
        self.signRequest(&uri, &request.headers, timestamp * 1000, "PUT") catch return BackupError.BackupFailed;

        request.sendBodyComplete(file_data) catch return BackupError.NetworkError;

        var redirect_buffer: [8192]u8 = undefined;
        const response = request.receiveHead(&redirect_buffer) catch return BackupError.NetworkError;

        if (response.head.status != .ok and response.head.status != .created) return BackupError.BackupFailed;
    }

    fn pullImpl(ptr: *anyopaque, backup_name: []const u8, dest_path: []const u8) BackupError!void {
        const self: *S3AdapterState = @ptrCast(@alignCast(ptr));

        const object_key = std.fs.path.join(self.allocator, &[_][]const u8{ self.config.path_prefix, backup_name }) catch return BackupError.RestoreFailed;
        defer self.allocator.free(object_key);

        var client = std.http.Client{ .allocator = self.allocator };
        defer client.deinit();

        const uri_str = std.fmt.allocPrint(self.allocator, "{s}/{s}/{s}", .{
            self.config.endpoint, self.config.bucket, object_key,
        }) catch return BackupError.RestoreFailed;
        defer self.allocator.free(uri_str);

        const uri = std.Uri.parse(uri_str) catch return BackupError.ConnectionFailed;

        var request = client.request(.GET, uri, .{}) catch return BackupError.NetworkError;
        defer request.deinit();

        const timestamp = std.time.timestamp();
        self.signRequest(&uri, &request.headers, timestamp * 1000, "GET") catch return BackupError.RestoreFailed;

        var redirect_buffer: [8192]u8 = undefined;
        var response = request.receiveHead(&redirect_buffer) catch return BackupError.NetworkError;

        if (response.head.status == .not_found) return BackupError.FileNotFound;
        if (response.head.status != .ok) return BackupError.RestoreFailed;

        const body = response.reader(&.{});

        const file = std.fs.cwd().createFile(dest_path, .{}) catch return BackupError.RestoreFailed;
        defer file.close();

        while (true) {
            var buf: [8192]u8 = undefined;
            const n = body.readSliceShort(buf[0..]) catch |err| {
                if (err == error.EndOfStream) break;
                return BackupError.RestoreFailed;
            };
            if (n == 0) break;
            file.writeAll(buf[0..n]) catch return BackupError.RestoreFailed;
        }
    }

    fn listImpl(ptr: *anyopaque) BackupError![]BackupInfo {
        const self: *S3AdapterState = @ptrCast(@alignCast(ptr));

        var client = std.http.Client{ .allocator = self.allocator };
        defer client.deinit();

        const uri_str = std.fmt.allocPrint(self.allocator, "{s}/{s}/?list-type=2&prefix={s}", .{
            self.config.endpoint, self.config.bucket, self.config.path_prefix,
        }) catch return BackupError.BackupFailed;
        defer self.allocator.free(uri_str);

        const uri = std.Uri.parse(uri_str) catch return BackupError.ConnectionFailed;

        var request = client.request(.GET, uri, .{}) catch return BackupError.NetworkError;
        defer request.deinit();

        const timestamp = std.time.timestamp();
        self.signRequest(&uri, &request.headers, timestamp * 1000, "GET") catch return BackupError.BackupFailed;

        var redirect_buffer: [8192]u8 = undefined;
        var response = request.receiveHead(&redirect_buffer) catch return BackupError.NetworkError;

        if (response.head.status != .ok) return BackupError.BackupFailed;

        var list = std.ArrayListUnmanaged(BackupInfo){};
        errdefer {
            for (list.items) |item| self.allocator.free(item.name);
            list.deinit(self.allocator);
        }

        const body = response.reader(&.{});
        const body_data = body.readAlloc(self.allocator, 10 * 1024 * 1024) catch return BackupError.BackupFailed;
        defer self.allocator.free(body_data);

        var parser = S3XmlParser.init(self.allocator, body_data);

        while (parser.next()) {
            if (parser.isContents()) {
                const info = parser.getEntryInfo();
                if (info) |entry| {
                    if (entry.name.len > 0) {
                        const name_copy = try self.allocator.dupe(u8, entry.name);
                        errdefer {
                            self.allocator.free(name_copy);
                            for (list.items) |item| self.allocator.free(item.name);
                            list.deinit(self.allocator);
                        }
                        try list.append(self.allocator, .{ .name = name_copy, .timestamp = entry.timestamp, .size_bytes = entry.size_bytes });
                    }
                }
            }
        }

        return try list.toOwnedSlice(self.allocator);
    }

    fn deleteImpl(ptr: *anyopaque, backup_name: []const u8) BackupError!void {
        const self: *S3AdapterState = @ptrCast(@alignCast(ptr));

        const object_key = std.fs.path.join(self.allocator, &[_][]const u8{ self.config.path_prefix, backup_name }) catch return BackupError.BackupFailed;
        defer self.allocator.free(object_key);

        var client = std.http.Client{ .allocator = self.allocator };
        defer client.deinit();

        const uri_str = std.fmt.allocPrint(self.allocator, "{s}/{s}/{s}", .{
            self.config.endpoint, self.config.bucket, object_key,
        }) catch return BackupError.BackupFailed;
        defer self.allocator.free(uri_str);

        const uri = std.Uri.parse(uri_str) catch return BackupError.ConnectionFailed;

        var request = client.request(.DELETE, uri, .{}) catch return BackupError.NetworkError;
        defer request.deinit();

        const timestamp = std.time.timestamp();
        self.signRequest(&uri, &request.headers, timestamp * 1000, "DELETE") catch return BackupError.BackupFailed;

        var redirect_buffer: [8192]u8 = undefined;
        const response = request.receiveHead(&redirect_buffer) catch return BackupError.NetworkError;

        if (response.head.status != .ok and response.head.status != .no_content and response.head.status != .not_found) return BackupError.BackupFailed;
    }

    fn freeListImpl(ptr: *anyopaque, list: []BackupInfo) void {
        const self: *S3AdapterState = @ptrCast(@alignCast(ptr));
        for (list) |info| {
            self.allocator.free(info.name);
        }
        self.allocator.free(list);
    }

    fn createVTable() BackupAdapter.VTable {
        return .{
            .push = @as(*const fn (*anyopaque, []const u8, []const u8) BackupError!void, pushImpl),
            .pull = @as(*const fn (*anyopaque, []const u8, []const u8) BackupError!void, pullImpl),
            .list = @as(*const fn (*anyopaque) BackupError![]BackupInfo, listImpl),
            .delete = @as(*const fn (*anyopaque, []const u8) BackupError!void, deleteImpl),
            .freeList = @as(*const fn (*anyopaque, []BackupInfo) void, freeListImpl),
        };
    }
};

var s3_vtable_storage: ?BackupAdapter.VTable = null;

pub fn createS3Adapter(allocator: std.mem.Allocator, config: S3Config) BackupAdapter {
    const state = allocator.create(S3AdapterState) catch unreachable;
    state.* = .{
        .allocator = allocator,
        .config = config,
    };
    if (s3_vtable_storage == null) {
        s3_vtable_storage = S3AdapterState.createVTable();
    }
    return .{
        .ptr = state,
        .vtable = &s3_vtable_storage.?,
    };
}

const S3XmlParser = struct {
    allocator: std.mem.Allocator,
    data: []const u8,
    pos: usize,
    current_key: ?[]u8 = null,
    current_size: ?u64 = null,
    current_last_modified: ?i64 = null,
    in_contents: bool = false,

    fn init(allocator: std.mem.Allocator, data: []const u8) S3XmlParser {
        return .{
            .allocator = allocator,
            .data = data,
            .pos = 0,
        };
    }

    fn next(parser: *S3XmlParser) bool {
        while (parser.pos < parser.data.len) {
            if (parser.data[parser.pos] == '<') {
                if (parser.pos + 2 < parser.data.len and parser.data[parser.pos + 1] == '?') {
                    const end = std.mem.indexOfScalar(u8, parser.data[parser.pos..], '>') orelse parser.data.len;
                    parser.pos += end + 1;
                    continue;
                }

                if (parser.pos + 2 < parser.data.len and parser.data[parser.pos + 1] == '/') {
                    const end = std.mem.indexOfScalar(u8, parser.data[parser.pos..], '>') orelse parser.data.len;
                    const name = parser.data[parser.pos + 2 .. parser.pos + end];
                    parser.pos += end + 1;

                    if (std.mem.eql(u8, name, "Contents")) {
                        parser.in_contents = false;
                        parser.current_key = null;
                        parser.current_size = null;
                        parser.current_last_modified = null;
                    }
                    continue;
                }

                var i = parser.pos + 1;
                while (i < parser.data.len and parser.data[i] != '>' and parser.data[i] != ' ') i += 1;
                const name = parser.data[parser.pos + 1 .. i];
                parser.pos = i + 1;

                while (parser.pos < parser.data.len and parser.data[parser.pos] != '>') parser.pos += 1;
                if (parser.pos < parser.data.len) parser.pos += 1;

                if (std.mem.eql(u8, name, "Contents")) {
                    parser.in_contents = true;
                } else if (parser.in_contents) {
                    if (std.mem.eql(u8, name, "Key")) {
                        const text_start = parser.pos;
                        while (parser.pos < parser.data.len and parser.data[parser.pos] != '<') parser.pos += 1;
                        if (parser.pos > text_start) {
                            parser.current_key = parser.allocator.dupe(u8, parser.data[text_start..parser.pos]) catch null;
                        }
                    } else if (std.mem.eql(u8, name, "Size")) {
                        const text_start = parser.pos;
                        while (parser.pos < parser.data.len and parser.data[parser.pos] != '<') parser.pos += 1;
                        if (parser.pos > text_start) {
                            const text = parser.data[text_start..parser.pos];
                            parser.current_size = std.fmt.parseInt(u64, text, 10) catch null;
                        }
                    } else if (std.mem.eql(u8, name, "LastModified")) {
                        const text_start = parser.pos;
                        while (parser.pos < parser.data.len and parser.data[parser.pos] != '<') parser.pos += 1;
                        if (parser.pos > text_start) {
                            const text = parser.data[text_start..parser.pos];
                            _ = text;
                        }
                    }
                }
                return true;
            } else {
                while (parser.pos < parser.data.len and parser.data[parser.pos] != '<') parser.pos += 1;
            }
        }
        return false;
    }

    fn isContents(parser: *S3XmlParser) bool {
        return parser.in_contents and parser.current_key != null;
    }

    fn getEntryInfo(parser: *S3XmlParser) ?BackupInfo {
        if (parser.current_key == null) return null;
        const name = parser.current_key.?;
        if (!std.mem.startsWith(u8, name, "little_timer/")) return null;
        const filename = name["little_timer/".len..];
        if (!std.mem.startsWith(u8, filename, "presets_backup_") or !std.mem.endsWith(u8, filename, ".db")) return null;
        return BackupInfo{
            .name = filename,
            .timestamp = parser.current_last_modified orelse 0,
            .size_bytes = parser.current_size orelse 0,
        };
    }
};