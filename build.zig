const std = @import("std");

pub fn build(b: *std.Build) void {
    const root_target = b.standardTargetOptions(.{});
    const optimize = b.standardOptimizeOption(.{});

    const mod = b.addModule("little_timer", .{
        .root_source_file = b.path("src/root.zig"),
        .target = root_target,
    });

    // 导入 toml 依赖
    const toml_dep = b.dependency("toml", .{
        .target = root_target,
        .optimize = optimize,
    });

    mod.addImport("toml", toml_dep.module("toml"));

    // 创建桌面应用模块（仅当不跨编译到 Android 时）
    if (!root_target.result.abi.isAndroid()) {
        const app_module = b.createModule(.{
            .root_source_file = b.path("src/main.zig"),
            .target = root_target,
            .optimize = optimize,
            .imports = &.{
                .{ .name = "little_timer", .module = mod },
                .{ .name = "toml", .module = toml_dep.module("toml") },
            },
        });

        // 桌面: 导入 webui 依赖
        const webui_dep = b.dependency("webui", .{
            .target = root_target,
            .optimize = optimize,
        });
        app_module.addImport("webui", webui_dep.module("webui"));

        // 构建桌面可执行文件
        const exe = b.addExecutable(.{
            .name = "little_timer",
            .root_module = app_module,
        });

        exe.linkLibC();
        b.installArtifact(exe);

        // 添加 run 步骤
        const run_step = b.step("run", "Run the app");
        const run_cmd = b.addRunArtifact(exe);
        run_step.dependOn(&run_cmd.step);
        if (b.args) |args| {
            run_cmd.addArgs(args);
        }

        // 添加测试步骤
        const test_step = b.step("test", "Run tests");
        const mod_tests = b.addTest(.{
            .root_module = mod,
        });
        const run_mod_tests = b.addRunArtifact(mod_tests);
        test_step.dependOn(&run_mod_tests.step);
    } else {
        // Android: 提供 lib 目标，生成 .so
        const ndk_home = std.process.getEnvVarOwned(b.allocator, "ANDROID_NDK_HOME") catch |err| {
            std.debug.print("❌ 未设置 ANDROID_NDK_HOME: {}\n", .{err});
            @panic("ANDROID_NDK_HOME not set");
        };

        const sysroot_path = b.fmt("{s}/toolchains/llvm/prebuilt/linux-x86_64/sysroot", .{ndk_home});
        b.sysroot = sysroot_path; // 设置全局 Sysroot

        const webui_dep = b.dependency("webui", .{
            .target = root_target,
            .optimize = optimize,
        });

        // ✅ 关键改动 1：手动创建一个只包含 Zig 绑定的模块，不触发依赖包的 C 编译逻辑
        const webui_module = b.createModule(.{
            .root_source_file = webui_dep.path("src/webui.zig"), // 直接指向源码中的 Zig 绑定文件
        });

        const app_module = b.createModule(.{
            .root_source_file = b.path("src/main.zig"),
            .target = root_target,
            .optimize = optimize,
            .imports = &.{
                .{ .name = "little_timer", .module = mod },
                .{ .name = "toml", .module = toml_dep.module("toml") },
                .{ .name = "webui", .module = webui_module }, // 使用我们手动定义的模块
            },
        });

        const lib = b.addLibrary(.{
            .name = "little_timer",
            .root_module = app_module,
            .linkage = .dynamic, // 生成 liblittle_timer.so
        });

        // 注意：zig_webui-2.5.0-beta.4 只包含 Zig 绑定，不包含 C 源码
        // WebUI C 依赖应通过 Zig 绑定的 @cImport 机制自动处理

        // ✅ 关键改动 3：补齐所有 Include 路径
        lib.addIncludePath(webui_dep.path("include"));
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
