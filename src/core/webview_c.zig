const build_options = @import("build_options");
const std = @import("std");

pub const c = @cImport({
    @cDefine("WEBVIEW_STATIC", "1");
    @cInclude("webview/api.h");
});

pub const app_url: [:0]const u8 = if (build_options.embed_ui)
    "http://127.0.0.1:8080/?runtime=webview"
else
    "http://localhost:5173/?runtime=webview";

pub const WebviewError = error{
    CreateFailed,
    OutOfMemory,
    SetTitleFailed,
    SetSizeFailed,
    NavigateFailed,
    RunFailed,
    DestroyFailed,
    BindFailed,
};

fn ensureOk(code: c_int, err: WebviewError) WebviewError!void {
    if (code < 0) return err;
}

pub const Window = struct {
    raw: c.webview_t,

    pub fn create(debug: bool) WebviewError!Window {
        const w = c.webview_create(if (debug) 1 else 0, null);
        if (w == null) return WebviewError.CreateFailed;
        return .{ .raw = w };
    }

    pub fn destroy(self: *Window) WebviewError!void {
        if (self.raw == null) return;
        try ensureOk(c.webview_destroy(self.raw), WebviewError.DestroyFailed);
        self.raw = null;
    }

    pub fn setTitle(self: *Window, title: [:0]const u8) WebviewError!void {
        try ensureOk(c.webview_set_title(self.raw, title.ptr), WebviewError.SetTitleFailed);
    }

    pub fn setSize(self: *Window, width: i32, height: i32) WebviewError!void {
        try ensureOk(c.webview_set_size(self.raw, width, height, c.WEBVIEW_HINT_NONE), WebviewError.SetSizeFailed);
    }

    pub fn navigate(self: *Window, url: [:0]const u8) WebviewError!void {
        try ensureOk(c.webview_navigate(self.raw, url.ptr), WebviewError.NavigateFailed);
    }

    pub fn bind(self: *Window, name: [:0]const u8, callback: *const fn (req: [:0]const u8) void) WebviewError!void {
        const CallbackFn = fn (id: [*c]const u8, req: [*c]const u8, arg: ?*anyopaque) callconv(.c) void;
        const UserData = struct {
            cb: *const fn (req: [:0]const u8) void,
        };
        const user_data = try std.heap.c_allocator.create(UserData);
        user_data.* = .{ .cb = callback };
        errdefer std.heap.c_allocator.destroy(user_data);

        const wrapper: CallbackFn = struct {
            fn fn_wrapper(id: [*c]const u8, req: [*c]const u8, arg: ?*anyopaque) callconv(.c) void {
                _ = id;
                if (arg) |ud| {
                    const userdata: *UserData = @ptrCast(@alignCast(ud));
                    if (req != null) {
                        userdata.cb(std.mem.sliceTo(req, 0));
                    }
                }
            }
        }.fn_wrapper;

        try ensureOk(c.webview_bind(self.raw, name.ptr, wrapper, user_data), WebviewError.BindFailed);
    }

    pub fn run(self: *Window) WebviewError!void {
        try ensureOk(c.webview_run(self.raw), WebviewError.RunFailed);
    }

    pub fn openDefault(debug: bool) WebviewError!Window {
        var win = try Window.create(debug);
        errdefer win.destroy() catch {};

        try win.setTitle("Little Timer");
        try win.setSize(1200, 780);
        try win.navigate(app_url);

        return win;
    }
};
