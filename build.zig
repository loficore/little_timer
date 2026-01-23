const std = @import("std");
const builtin = @import("builtin");

// 在编译时导入 android 构建工具
const androidbuild = @import("android");

pub fn build(b: *std.Build) void {
    const root_target = b.standardTargetOptions(.{});
    const optimize = b.standardOptimizeOption(.{});

    // 使用 standardTargets 获取 Android 目标列表
    const android_targets = androidbuild.standardTargets(b, root_target);

    // 确定实际要编译的目标列表
    var root_target_single = [_]std.Build.ResolvedTarget{root_target};
    const targets: []std.Build.ResolvedTarget = if (android_targets.len == 0)
        root_target_single[0..]
    else
        android_targets;

    const mod = b.addModule("little_timer", .{
        .root_source_file = b.path("src/root.zig"),
        .target = root_target,
    });

    // 导入 toml 依赖（桌面和 Android 都需要）
    const toml_dep = b.dependency("toml", .{
        .target = root_target,
        .optimize = optimize,
    });

    // 为 mod 添加 toml 导入
    mod.addImport("toml", toml_dep.module("toml"));

    // 如果有 Android 目标，创建 APK
    const android_apk: ?*androidbuild.Apk = blk: {
        if (android_targets.len == 0) break :blk null;

        const android_sdk = androidbuild.Sdk.create(b, .{});
        const apk = android_sdk.createApk(.{
            .api_level = .android8, // 调整为 API 26，提升设备覆盖率
            .build_tools_version = "35.0.1",
            .ndk_version = "26.1.10909125",
        });

        // TODO: 需要创建这些文件
        apk.setAndroidManifest(b.path("android/AndroidManifest.xml"));
        apk.addResourceDirectory(b.path("android/res"));

        // 创建测试用的 keystore
        const key_store_file = android_sdk.createKeyStore(.example);
        apk.setKeyStore(key_store_file);

        break :blk apk;
    };

    // 遍历所有目标（桌面或多个 Android 架构）
    for (targets) |target_item| {
        const app_module = b.createModule(.{
            .root_source_file = b.path("src/main.zig"),
            .target = target_item,
            .optimize = optimize,
            .imports = &.{
                .{ .name = "little_timer", .module = mod },
                .{ .name = "toml", .module = toml_dep.module("toml") },
            },
        });

        // 如果是 Android 目标，构建共享库并添加到 APK
        if (target_item.result.abi.isAndroid()) {
            const apk: *androidbuild.Apk = android_apk orelse @panic("Android APK should be initialized");

            // 获取 android 依赖并添加到模块
            const android_dep = b.dependency("android", .{
                .target = target_item,
                .optimize = optimize,
            });
            app_module.addImport("android", android_dep.module("android"));

            const lib = b.addLibrary(.{
                .name = "little_timer",
                .root_module = app_module,
                .linkage = .dynamic,
            });

            // 添加库到 APK
            apk.addArtifact(lib);
        } else {
            // 桌面构建 - 导入 webui
            const webui_dep = b.dependency("webui", .{
                .target = target_item,
                .optimize = optimize,
            });
            app_module.addImport("webui", webui_dep.module("webui"));

            const exe = b.addExecutable(.{
                .name = "little_timer",
                .root_module = app_module,
            });

            exe.linkLibC();
            b.installArtifact(exe);

            // 如果只有一个目标，添加 "run" 步骤
            if (targets.len == 1) {
                const run_step = b.step("run", "Run the app");
                const run_cmd = b.addRunArtifact(exe);
                run_step.dependOn(&run_cmd.step);
                if (b.args) |args| {
                    run_cmd.addArgs(args);
                }
            }
        }
    }

    // 如果有 Android APK，安装它
    if (android_apk) |apk| {
        const installed_apk = apk.addInstallApk();
        b.getInstallStep().dependOn(&installed_apk.step);

        const android_sdk = apk.sdk;
        const run_step = b.step("run", "Install and run the application on an Android device");
        const adb_install = android_sdk.addAdbInstall(installed_apk.source);
        const adb_start = android_sdk.addAdbStart("com.zig.little_timer/android.app.NativeActivity");
        adb_start.step.dependOn(&adb_install.step);
        run_step.dependOn(&adb_start.step);
    } else {
        // 桌面测试
        const test_step = b.step("test", "Run tests");
        const mod_tests = b.addTest(.{
            .root_module = mod,
        });
        const run_mod_tests = b.addRunArtifact(mod_tests);
        test_step.dependOn(&run_mod_tests.step);
    }
}
