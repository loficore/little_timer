const std = @import("std");

pub fn build(b: *std.Build) void {
    const target = b.standardTargetOptions(.{});
    const optimize = b.standardOptimizeOption(.{});

    const mod = b.addModule("little_timer", .{
        .root_source_file = b.path("src/root.zig"),
        .target = target,
    });

    // 导入 webui 依赖
    const webui_dep = b.dependency("webui", .{
        .target = target,
        .optimize = optimize,
    });

    const toml_dep = b.dependency("toml", .{
        .target = target,
        .optimize = optimize,
    });

    // 主应用程序 - WebUI 版本
    const exe = b.addExecutable(.{
        .name = "little_timer",
        .root_module = b.createModule(.{
            .root_source_file = b.path("src/main.zig"),
            .target = target,
            .optimize = optimize,
            .imports = &.{
                .{ .name = "little_timer", .module = mod },
                .{ .name = "webui", .module = webui_dep.module("webui") },
                .{ .name = "toml", .module = toml_dep.module("toml") },
            },
        }),
    });

    exe.linkLibC();
    exe.addLibraryPath(.{ .cwd_relative = "/usr/local/lib" });
    exe.linkSystemLibrary("webui");

    b.installArtifact(exe);

    // 运行步骤
    const run_step = b.step("run", "Run the app");
    const run_cmd = b.addRunArtifact(exe);
    run_step.dependOn(&run_cmd.step);
    run_cmd.step.dependOn(b.getInstallStep());
    if (b.args) |args| {
        run_cmd.addArgs(args);
    }

    // 测试
    const mod_tests = b.addTest(.{
        .root_module = mod,
    });
    const run_mod_tests = b.addRunArtifact(mod_tests);

    const exe_tests = b.addTest(.{
        .root_module = exe.root_module,
    });
    const run_exe_tests = b.addRunArtifact(exe_tests);

    const test_step = b.step("test", "Run tests");
    test_step.dependOn(&run_mod_tests.step);
    test_step.dependOn(&run_exe_tests.step);
}
