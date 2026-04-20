const std = @import("std");

fn pkgConfigModuleExists(allocator: std.mem.Allocator, module: []const u8) bool {
    const result = std.process.Child.run(.{
        .allocator = allocator,
        .argv = &.{ "pkg-config", "--exists", module },
    }) catch return false;
    defer allocator.free(result.stdout);
    defer allocator.free(result.stderr);

    return switch (result.term) {
        .Exited => |code| code == 0,
        else => false,
    };
}

fn linkWebviewDesktopDeps(b: *std.Build, exe: *std.Build.Step.Compile, target: std.Build.ResolvedTarget) void {
    exe.addIncludePath(b.path("third_party/webview/core/include"));
    exe.addCSourceFile(.{
        .file = b.path("third_party/webview/core/src/webview.cc"),
        .flags = &.{ "-std=c++17", "-DWEBVIEW_STATIC" },
    });
    exe.linkLibCpp();

    switch (target.result.os.tag) {
        .linux => {
            const Candidate = struct { gtk: []const u8, webkit: []const u8 };
            const candidates = [_]Candidate{
                .{ .gtk = "gtk4", .webkit = "webkitgtk-6.0" },
                .{ .gtk = "gtk+-3.0", .webkit = "webkit2gtk-4.1" },
                .{ .gtk = "gtk+-3.0", .webkit = "webkit2gtk-4.0" },
            };

            var selected: ?Candidate = null;
            for (candidates) |candidate| {
                if (pkgConfigModuleExists(b.allocator, candidate.gtk) and pkgConfigModuleExists(b.allocator, candidate.webkit)) {
                    selected = candidate;
                    break;
                }
            }

            if (selected) |s| {
                exe.root_module.linkSystemLibrary(s.gtk, .{ .use_pkg_config = .force });
                exe.root_module.linkSystemLibrary(s.webkit, .{ .use_pkg_config = .force });
            } else {
                std.debug.print(
                    "❌ 未检测到可用 GTK/WebKit 组合。请安装任一组合：\n" ++
                        "  1) gtk4 + webkitgtk-6.0\n" ++
                        "  2) gtk+-3.0 + webkit2gtk-4.1\n" ++
                        "  3) gtk+-3.0 + webkit2gtk-4.0\n",
                    .{},
                );
                @panic("missing Linux webview dependencies");
            }

            exe.root_module.linkSystemLibrary("dl", .{ .use_pkg_config = .no });
        },
        .macos => {
            exe.root_module.linkFramework("WebKit", .{});
            exe.root_module.linkSystemLibrary("dl", .{ .use_pkg_config = .no });
        },
        .windows => {
            exe.root_module.linkSystemLibrary("advapi32", .{ .use_pkg_config = .no });
            exe.root_module.linkSystemLibrary("ole32", .{ .use_pkg_config = .no });
            exe.root_module.linkSystemLibrary("shell32", .{ .use_pkg_config = .no });
            exe.root_module.linkSystemLibrary("shlwapi", .{ .use_pkg_config = .no });
            exe.root_module.linkSystemLibrary("user32", .{ .use_pkg_config = .no });
            exe.root_module.linkSystemLibrary("version", .{ .use_pkg_config = .no });
        },
        else => {},
    }
}

pub fn build(b: *std.Build) void {
    const root_target = b.standardTargetOptions(.{});
    const optimize = b.standardOptimizeOption(.{});

    // 构建选项：是否将前端 HTML 内嵌到可执行文件
    const embed_ui = b.option(bool, "embed_ui", "Embed UI HTML into binary") orelse false;
    const build_options = b.addOptions();
    build_options.addOption(bool, "embed_ui", embed_ui);
    // 当内嵌 UI 时，把 HTML 内容注入到 build_options 中
    // 这样避免 @embedFile 访问包外路径的问题
    if (embed_ui) {
        const html_path = b.pathFromRoot("assets/dist/index.html");
        const html = std.fs.cwd().readFileAlloc(b.allocator, html_path, 16 * 1024 * 1024) catch |err| {
            std.debug.print("❌ 无法读取 assets/dist/index.html: {any}\n", .{err});
            @panic("missing or too-large assets/dist/index.html, please run assets build first");
        };
        build_options.addOption([]const u8, "embedded_html", html);
    } else {
        build_options.addOption([]const u8, "embedded_html", "");
    }

    const mod = b.addModule("little_timer", .{
        .root_source_file = b.path("src/main_entry.zig"),
        .target = root_target,
    });

    // 添加 build_options 到模块（供 HTTP Server 使用）
    mod.addOptions("build_options", build_options);

    // 创建桌面应用模块（仅当不跨编译到 Android 时）
    if (!root_target.result.abi.isAndroid()) {
        // 导入 zqlite 依赖
        const zqlite_dep = b.dependency("zqlite", .{
            .target = root_target,
            .optimize = optimize,
        });
        mod.addImport("zqlite", zqlite_dep.module("zqlite"));

        const app_module = mod;

        // 构建桌面可执行文件
        const run_step = b.step("run", "Run the app");

        if (root_target.result.os.tag == .windows) {
            // GUI 版：双击启动，不弹控制台窗口。
            const exe_gui = b.addExecutable(.{
                .name = "little_timer",
                .root_module = app_module,
            });
            linkWebviewDesktopDeps(b, exe_gui, root_target);
            exe_gui.linkLibC();
            exe_gui.subsystem = .Windows;
            b.installArtifact(exe_gui);

            // CLI 版：保留命令行实时日志，便于诊断。
            const exe_cli = b.addExecutable(.{
                .name = "little_timer_cli",
                .root_module = app_module,
            });
            linkWebviewDesktopDeps(b, exe_cli, root_target);
            exe_cli.linkLibC();
            b.installArtifact(exe_cli);

            const run_cmd = b.addRunArtifact(exe_cli);
            run_step.dependOn(&run_cmd.step);
            if (b.args) |args| {
                run_cmd.addArgs(args);
            }
        } else {
            const exe = b.addExecutable(.{
                .name = "little_timer",
                .root_module = app_module,
            });

            linkWebviewDesktopDeps(b, exe, root_target);

            exe.linkLibC();
            b.installArtifact(exe);

            const run_cmd = b.addRunArtifact(exe);
            run_step.dependOn(&run_cmd.step);
            if (b.args) |args| {
                run_cmd.addArgs(args);
            }
        }

        // 添加测试步骤
        const test_step = b.step("test", "Run tests");

        // 测试前清理测试数据库
        const cleanup_db = b.addSystemCommand(&.{
            "rm", "-f", "presets.db", "test_tmp/presets.db",
        });
        cleanup_db.step.name = "cleanup_test_db";

        const mod_tests = b.addTest(.{
            .root_module = mod,
        });
        const run_mod_tests = b.addRunArtifact(mod_tests);

        // 测试完成后清理
        const cleanup_after = b.addSystemCommand(&.{
            "rm", "-f", "presets.db", "test_tmp/presets.db",
        });
        cleanup_after.step.name = "cleanup_after_test";

        test_step.dependOn(&cleanup_db.step);
        run_mod_tests.step.dependOn(&cleanup_after.step);
    } else {
        // Android: 提供 lib 目标，生成 .so
        const ndk_home = std.process.getEnvVarOwned(b.allocator, "ANDROID_NDK_HOME") catch |err| {
            std.debug.print("❌ 未设置 ANDROID_NDK_HOME: {}\n", .{err});
            @panic("ANDROID_NDK_HOME not set");
        };

        const sysroot_path = b.fmt("{s}/toolchains/llvm/prebuilt/linux-x86_64/sysroot", .{ndk_home});
        b.sysroot = sysroot_path; // 设置全局 Sysroot

        const app_module = mod;

        const lib = b.addLibrary(.{
            .name = "little_timer",
            .root_module = app_module,
            .linkage = .dynamic, // 生成 liblittle_timer.so
        });

        lib.addIncludePath(.{ .cwd_relative = b.fmt("{s}/usr/include", .{sysroot_path}) });
        lib.addIncludePath(.{ .cwd_relative = b.fmt("{s}/usr/include/aarch64-linux-android", .{sysroot_path}) });

        // 核心修正：显式添加 Android 系统库搜索路径，避免链接阶段找不到 liblog/libandroid
        const api_level = "26"; // 与目标 Android API Level 保持一致
        // 使用以 "/" 开头的路径，使 Zig 在有 sysroot 时自动前缀 sysroot
        const lib_path = b.fmt("/usr/lib/aarch64-linux-android/{s}", .{api_level});
        lib.addLibraryPath(.{ .cwd_relative = lib_path });

        lib.linkLibC();
        lib.linkSystemLibrary("log"); // 链接 Android 日志库
        lib.linkSystemLibrary("android"); // 链接 Android 系统库

        const install_lib = b.addInstallArtifact(lib, .{
            .dest_dir = .{ .override = .lib },
        });

        const lib_build = b.step("lib", "Build Android shared library");
        lib_build.dependOn(&install_lib.step);
    }
}
