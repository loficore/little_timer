//! By convention, root.zig is the root source file when making a library.
const std = @import("std");

// 导入所有模块以使其测试可以被执行
const clock = @import("core/clock.zig");
const settings = @import("settings/settings_manager.zig");
const logger_module = @import("core/logger.zig");
const interface = @import("core/interface.zig");
const error_recovery = @import("core/utils/error_recovery.zig");

// 在测试时导入单独的测试文件
comptime {
    if (@import("builtin").is_test) {
        _ = @import("test/test_clock.zig");
        _ = @import("test/test_settings.zig");
        _ = @import("test/test_logger.zig");
        _ = @import("test/test_error_recovery.zig");
        _ = @import("test/test_boundary_conditions.zig");
        _ = @import("test/test_settings_validator.zig");
        _ = @import("test/test_settings_presets.zig");
        _ = @import("test/test_sqlite.zig");
    }
}

// 通过编译这些模块，使其中的 test 块被包含
comptime {
    _ = clock;
    _ = settings;
    _ = logger_module;
    _ = interface;
    _ = error_recovery;
}
