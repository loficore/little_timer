//! 设置管理模块单元测试
const std = @import("std");
const settings_module = @import("../settings/settings_manager.zig");
const interface = @import("../core/interface.zig");

test "默认设置配置初始化" {
    const default_config = interface.SettingsConfig{};
    try std.testing.expectEqual(default_config.basic.timezone, 8);
    try std.testing.expect(std.mem.eql(u8, default_config.basic.language, "ZH"));
    try std.testing.expectEqual(default_config.basic.default_mode, .countdown);
}

test "倒计时默认配置" {
    const default_config = interface.SettingsConfig{};
    try std.testing.expectEqual(default_config.clock_defaults.countdown.duration_seconds, 25 * 60);
    try std.testing.expect(!default_config.clock_defaults.countdown.loop);
    try std.testing.expectEqual(default_config.clock_defaults.countdown.loop_count, 0);
}

test "日志默认配置" {
    const default_config = interface.SettingsConfig{};
    try std.testing.expect(std.mem.startsWith(u8, default_config.logging.level, "INFO"));
    try std.testing.expect(default_config.logging.enable_timestamp);
}

test "时区范围校验 - 有效范围 -12 到 14" {
    const allocator = std.testing.allocator;
    var manager = try settings_module.SettingsManager.init(allocator, "");
    defer manager.deinit();

    try manager.updateBasic(.{ .timezone = -12, .language = "ZH", .default_mode = .countdown });
    try std.testing.expectEqual(manager.config.basic.timezone, -12);

    try manager.updateBasic(.{ .timezone = 14, .language = "ZH", .default_mode = .countdown });
    try std.testing.expectEqual(manager.config.basic.timezone, 14);
}

test "时区校验 - 超出范围低于 -12" {
    const allocator = std.testing.allocator;
    var manager = try settings_module.SettingsManager.init(allocator, "");
    defer manager.deinit();

    const result = manager.updateBasic(.{ .timezone = -13, .language = "ZH", .default_mode = .countdown });
    try std.testing.expectError(settings_module.ValidationError.InvalidTimezone, result);
}

test "时区校验 - 超出范围高于 14" {
    const allocator = std.testing.allocator;
    var manager = try settings_module.SettingsManager.init(allocator, "");
    defer manager.deinit();

    const result = manager.updateBasic(.{ .timezone = 15, .language = "ZH", .default_mode = .countdown });
    try std.testing.expectError(settings_module.ValidationError.InvalidTimezone, result);
}

test "时区边界值 0" {
    const allocator = std.testing.allocator;
    var manager = try settings_module.SettingsManager.init(allocator, "");
    defer manager.deinit();

    try manager.updateBasic(.{ .timezone = 0, .language = "ZH", .default_mode = .countdown });
    try std.testing.expectEqual(manager.config.basic.timezone, 0);
}

test "更新基本设置 - 时区和语言" {
    const allocator = std.testing.allocator;
    var manager = try settings_module.SettingsManager.init(allocator, "");
    defer manager.deinit();

    try manager.updateBasic(.{ .timezone = -5, .language = "EN", .default_mode = .stopwatch });

    try std.testing.expectEqual(manager.config.basic.timezone, -5);
    try std.testing.expect(std.mem.eql(u8, manager.config.basic.language, "EN"));
    try std.testing.expectEqual(manager.config.basic.default_mode, .stopwatch);
}

test "更新倒计时默认配置" {
    const allocator = std.testing.allocator;
    var manager = try settings_module.SettingsManager.init(allocator, "");
    defer manager.deinit();

    manager.config.clock_defaults.countdown.duration_seconds = 1800;
    manager.config.clock_defaults.countdown.loop = true;

    try std.testing.expectEqual(manager.config.clock_defaults.countdown.duration_seconds, 1800);
    try std.testing.expect(manager.config.clock_defaults.countdown.loop);
}

test "toJsonAlloc 基本功能" {
    const allocator = std.testing.allocator;
    var manager = try settings_module.SettingsManager.init(allocator, "");
    defer manager.deinit();

    const json_str = try manager.toJsonAlloc();
    defer allocator.free(json_str);

    try std.testing.expect(std.mem.containsAtLeast(u8, json_str, 1, "timezone"));
    try std.testing.expect(std.mem.containsAtLeast(u8, json_str, 1, "language"));
    try std.testing.expect(std.mem.containsAtLeast(u8, json_str, 1, "countdown"));
}

test "toJsonAlloc 时区值正确" {
    const allocator = std.testing.allocator;
    var manager = try settings_module.SettingsManager.init(allocator, "");
    defer manager.deinit();

    manager.config.basic.timezone = -5;

    const json_str = try manager.toJsonAlloc();
    defer allocator.free(json_str);

    try std.testing.expect(std.mem.containsAtLeast(u8, json_str, 1, "timezone"));
    try std.testing.expect(std.mem.containsAtLeast(u8, json_str, 1, "-5"));
}

test "toJsonAlloc 默认模式值" {
    const allocator = std.testing.allocator;
    var manager = try settings_module.SettingsManager.init(allocator, "");
    defer manager.deinit();

    manager.config.basic.default_mode = .stopwatch;

    const json_str = try manager.toJsonAlloc();
    defer allocator.free(json_str);

    try std.testing.expect(std.mem.containsAtLeast(u8, json_str, 1, "stopwatch"));
}

test "buildClockConfig 倒计时模式" {
    const allocator = std.testing.allocator;
    var manager = try settings_module.SettingsManager.init(allocator, "");
    defer manager.deinit();

    manager.config.basic.default_mode = .countdown;
    manager.config.clock_defaults.countdown.duration_seconds = 900;

    const config = manager.buildClockConfig();

    try std.testing.expectEqual(config.countdown.duration_seconds, 900);
}

test "buildClockConfig 正计时模式" {
    const allocator = std.testing.allocator;
    var manager = try settings_module.SettingsManager.init(allocator, "");
    defer manager.deinit();

    manager.config.basic.default_mode = .stopwatch;
    manager.config.clock_defaults.stopwatch.max_seconds = 3600;

    const config = manager.buildClockConfig();

    try std.testing.expectEqual(config.stopwatch.max_seconds, 3600);
}

test "更新基本设置后设置脏标记" {
    const allocator = std.testing.allocator;
    var manager = try settings_module.SettingsManager.init(allocator, "");
    defer manager.deinit();

    manager.is_dirty = false;

    try manager.updateBasic(.{ .timezone = 10, .language = "EN", .default_mode = .stopwatch });

    try std.testing.expect(manager.is_dirty);
}

test "极端时区值 -12 和 14" {
    const allocator = std.testing.allocator;
    var manager = try settings_module.SettingsManager.init(allocator, "");
    defer manager.deinit();

    try manager.updateBasic(.{ .timezone = -12, .language = "ZH", .default_mode = .countdown });
    try std.testing.expectEqual(manager.config.basic.timezone, -12);

    try manager.updateBasic(.{ .timezone = 14, .language = "ZH", .default_mode = .countdown });
    try std.testing.expectEqual(manager.config.basic.timezone, 14);
}

test "零秒倒计时配置" {
    const allocator = std.testing.allocator;
    var manager = try settings_module.SettingsManager.init(allocator, "");
    defer manager.deinit();

    manager.config.clock_defaults.countdown.duration_seconds = 0;
    const config = manager.buildClockConfig();

    try std.testing.expectEqual(config.countdown.duration_seconds, 0);
}

test "resetToDefaults() 重置配置" {
    const allocator = std.testing.allocator;
    var manager = try settings_module.SettingsManager.init(allocator, "");
    defer manager.deinit();

    manager.config.basic.timezone = -10;
    manager.config.clock_defaults.countdown.duration_seconds = 100;

    try manager.resetToDefaults();

    try std.testing.expectEqual(manager.config.basic.timezone, 8);
    try std.testing.expect(std.mem.eql(u8, manager.config.basic.language, "ZH"));
    try std.testing.expect(manager.is_dirty);
}

test "handleSettingsEvent() 通过 get_settings 获取配置" {
    const allocator = std.testing.allocator;
    var manager = try settings_module.SettingsManager.init(allocator, "");
    defer manager.deinit();

    manager.config.basic.timezone = 5;

    var buffer: [1]u8 = .{0};
    const event: interface.SettingsEvent = .{ .get_settings = buffer[0..0 :0] };

    try manager.handleSettingsEvent(event);
}

test "toJsonAlloc() 动态分配 JSON" {
    const allocator = std.testing.allocator;
    var manager = try settings_module.SettingsManager.init(allocator, "");
    defer manager.deinit();

    manager.config.basic.timezone = 7;
    manager.config.clock_defaults.countdown.duration_seconds = 2000;

    const json_str = try manager.toJsonAlloc();
    defer allocator.free(json_str);

    try std.testing.expect(std.mem.containsAtLeast(u8, json_str, 1, "timezone"));
    try std.testing.expect(std.mem.containsAtLeast(u8, json_str, 1, ":7"));
    try std.testing.expect(std.mem.containsAtLeast(u8, json_str, 1, "2000"));
}

test "UnlockResult 默认值" {
    const result: interface.UnlockResult = .{};
    try std.testing.expect(result.success == false);
    try std.testing.expect(result.locked_until == 0);
}

test "UnlockResult 自定义值" {
    const result: interface.UnlockResult = .{
        .success = true,
        .locked_until = 1234567890,
    };
    try std.testing.expect(result.success == true);
    try std.testing.expect(result.locked_until == 1234567890);
}

test "unlockCredentials 首次解锁成功" {
    const allocator = std.testing.allocator;
    var manager = try settings_module.SettingsManager.init(allocator, "");
    defer manager.deinit();

    const before = std.time.timestamp();
    const result = manager.unlockCredentials("password123");
    const after = std.time.timestamp();

    try std.testing.expect(result.success == true);
    try std.testing.expect(result.locked_until == 0);
    try std.testing.expect(manager.backup_config.credentials_unlock_time >= before);
    try std.testing.expect(manager.backup_config.credentials_unlock_time <= after);
    try std.testing.expect(manager.backup_config.credential_unlock_attempts == 0);
}

test "unlockCredentials 5次失败后锁定" {
    const allocator = std.testing.allocator;
    var manager = try settings_module.SettingsManager.init(allocator, "");
    defer manager.deinit();

    manager.backup_config.credential_unlock_attempts = 5;

    const now = std.time.timestamp();
    const result = manager.unlockCredentials("wrong_password");

    try std.testing.expect(result.success == false);
    try std.testing.expect(result.locked_until >= now);
    try std.testing.expect(result.locked_until <= now + 301);
}

test "unlockCredentials 锁定期间拒绝解锁" {
    const allocator = std.testing.allocator;
    var manager = try settings_module.SettingsManager.init(allocator, "");
    defer manager.deinit();

    const future_lock = std.time.timestamp() + 3600;
    manager.backup_config.credential_locked_until = future_lock;

    const result = manager.unlockCredentials("any_password");

    try std.testing.expect(result.success == false);
    try std.testing.expect(result.locked_until == future_lock);
}

test "unlockCredentials 成功解锁重置计数器" {
    const allocator = std.testing.allocator;
    var manager = try settings_module.SettingsManager.init(allocator, "");
    defer manager.deinit();

    manager.backup_config.credential_unlock_attempts = 3;

    const result = manager.unlockCredentials("correct_password");

    try std.testing.expect(result.success == true);
    try std.testing.expect(manager.backup_config.credential_unlock_attempts == 0);
    try std.testing.expect(manager.backup_config.credential_locked_until == 0);
}

test "setCredentialPassword 存储密码" {
    const allocator = std.testing.allocator;
    var manager = try settings_module.SettingsManager.init(allocator, "");
    defer manager.deinit();

    manager.setCredentialPassword("my_secret_password");

    const result = manager.unlockCredentials("my_secret_password");
    try std.testing.expect(result.success == true);
}

test "setCredentialPassword 密码覆盖" {
    const allocator = std.testing.allocator;
    var manager = try settings_module.SettingsManager.init(allocator, "");
    defer manager.deinit();

    manager.setCredentialPassword("password1");
    const result1 = manager.unlockCredentials("password1");
    try std.testing.expect(result1.success == true);

    manager.setCredentialPassword("password2");
    const result2 = manager.unlockCredentials("password2");
    try std.testing.expect(result2.success == true);
}

test "hasMasterPassword 未设置时返回 false" {
    const allocator = std.testing.allocator;
    var manager = try settings_module.SettingsManager.init(allocator, "");
    defer manager.deinit();

    try std.testing.expect(manager.hasMasterPassword() == false);
}

test "hasMasterPassword 设置后返回 true" {
    const allocator = std.testing.allocator;
    var manager = try settings_module.SettingsManager.init(allocator, "");
    defer manager.deinit();

    manager.backup_config.has_master_password = true;

    try std.testing.expect(manager.hasMasterPassword() == true);
}

test "isUnlocked 无主密码时返回 false" {
    const allocator = std.testing.allocator;
    var manager = try settings_module.SettingsManager.init(allocator, "");
    defer manager.deinit();

    try std.testing.expect(manager.isUnlocked() == false);
}

test "isUnlocked 有主密码但未解锁时返回 false" {
    const allocator = std.testing.allocator;
    var manager = try settings_module.SettingsManager.init(allocator, "");
    defer manager.deinit();

    manager.backup_config.has_master_password = true;

    try std.testing.expect(manager.isUnlocked() == false);
}

test "isUnlocked 已解锁时返回 true" {
    const allocator = std.testing.allocator;
    var manager = try settings_module.SettingsManager.init(allocator, "");
    defer manager.deinit();

    manager.backup_config.has_master_password = true;
    manager.backup_config.credentials_unlock_time = std.time.timestamp();
    manager.setCredentialPassword("secret");

    try std.testing.expect(manager.isUnlocked() == true);
}

test "isUnlocked 锁定期间返回 false" {
    const allocator = std.testing.allocator;
    var manager = try settings_module.SettingsManager.init(allocator, "");
    defer manager.deinit();

    manager.backup_config.has_master_password = true;
    manager.backup_config.credentials_unlock_time = std.time.timestamp();
    manager.backup_config.credential_locked_until = std.time.timestamp() + 3600;
    manager.setCredentialPassword("secret");

    try std.testing.expect(manager.isUnlocked() == false);
}

test "isUnlocked 超过60天TTL返回 false" {
    const allocator = std.testing.allocator;
    var manager = try settings_module.SettingsManager.init(allocator, "");
    defer manager.deinit();

    manager.backup_config.has_master_password = true;
    manager.backup_config.credentials_unlock_time = std.time.timestamp() - (61 * 24 * 60 * 60);
    manager.setCredentialPassword("secret");

    try std.testing.expect(manager.isUnlocked() == false);
}

test "getMasterPasswordStatus 默认状态" {
    const allocator = std.testing.allocator;
    var manager = try settings_module.SettingsManager.init(allocator, "");
    defer manager.deinit();

    const status = manager.getMasterPasswordStatus();

    try std.testing.expect(status.has_password == false);
    try std.testing.expect(status.unlocked == false);
    try std.testing.expect(status.locked_until == 0);
    try std.testing.expect(status.unlock_time == 0);
}

test "getMasterPasswordStatus 已解锁状态" {
    const allocator = std.testing.allocator;
    var manager = try settings_module.SettingsManager.init(allocator, "");
    defer manager.deinit();

    manager.backup_config.has_master_password = true;
    manager.backup_config.credentials_unlock_time = std.time.timestamp();
    manager.setCredentialPassword("secret");

    const status = manager.getMasterPasswordStatus();

    try std.testing.expect(status.has_password == true);
    try std.testing.expect(status.unlocked == true);
    try std.testing.expect(status.unlock_time > 0);
}

test "hasMasterPassword 无密码时返回false" {
    const allocator = std.testing.allocator;
    var manager = try settings_module.SettingsManager.init(allocator, "");
    defer manager.deinit();

    try std.testing.expect(manager.hasMasterPassword() == false);
}

test "hasMasterPassword 有密码时返回true" {
    const allocator = std.testing.allocator;
    var manager = try settings_module.SettingsManager.init(allocator, "");
    defer manager.deinit();

    manager.backup_config.has_master_password = true;

    try std.testing.expect(manager.hasMasterPassword() == true);
}

test "isUnlocked 未设置密码时返回false" {
    const allocator = std.testing.allocator;
    var manager = try settings_module.SettingsManager.init(allocator, "");
    defer manager.deinit();

    manager.backup_config.has_master_password = false;
    manager.backup_config.credentials_unlock_time = std.time.timestamp();
    manager.setCredentialPassword("secret");

    try std.testing.expect(manager.isUnlocked() == false);
}

test "isUnlocked 已锁定时返回false" {
    const allocator = std.testing.allocator;
    var manager = try settings_module.SettingsManager.init(allocator, "");
    defer manager.deinit();

    manager.backup_config.has_master_password = true;
    manager.backup_config.credential_locked_until = std.time.timestamp() + 3600;
    manager.setCredentialPassword("secret");

    try std.testing.expect(manager.isUnlocked() == false);
}

test "isUnlocked 已过期时返回false" {
    const allocator = std.testing.allocator;
    var manager = try settings_module.SettingsManager.init(allocator, "");
    defer manager.deinit();

    const ttl_seconds: i64 = 60 * 24 * 60 * 60;
    manager.backup_config.has_master_password = true;
    manager.backup_config.credentials_unlock_time = std.time.timestamp() - ttl_seconds - 1;
    manager.setCredentialPassword("secret");

    try std.testing.expect(manager.isUnlocked() == false);
}

test "isUnlocked 密码为空时返回false" {
    const allocator = std.testing.allocator;
    var manager = try settings_module.SettingsManager.init(allocator, "");
    defer manager.deinit();

    manager.backup_config.has_master_password = true;
    manager.backup_config.credentials_unlock_time = std.time.timestamp();
    manager.credential_unlock_password = null;

    try std.testing.expect(manager.isUnlocked() == false);
}

test "isUnlocked 已解锁且有效时返回true" {
    const allocator = std.testing.allocator;
    var manager = try settings_module.SettingsManager.init(allocator, "");
    defer manager.deinit();

    manager.backup_config.has_master_password = true;
    manager.backup_config.credentials_unlock_time = std.time.timestamp();
    manager.setCredentialPassword("secret");

    try std.testing.expect(manager.isUnlocked() == true);
}

test "unlockCredentials 成功解锁" {
    const allocator = std.testing.allocator;
    var manager = try settings_module.SettingsManager.init(allocator, "");
    defer manager.deinit();

    manager.setCredentialPassword("secret");

    const result = manager.unlockCredentials("secret");

    try std.testing.expect(result.success == true);
    try std.testing.expect(result.locked_until == 0);
    try std.testing.expect(manager.backup_config.credential_unlock_attempts == 0);
}

test "unlockCredentials 密码错误" {
    const allocator = std.testing.allocator;
    var manager = try settings_module.SettingsManager.init(allocator, "");
    defer manager.deinit();

    manager.setCredentialPassword("secret");

    const result = manager.unlockCredentials("wrong");

    try std.testing.expect(result.success == false);
    try std.testing.expect(manager.backup_config.credential_unlock_attempts == 1);
}

test "setMasterPassword 成功设置" {
    const allocator = std.testing.allocator;
    var manager = try settings_module.SettingsManager.init(allocator, "");
    defer manager.deinit();

    try manager.setMasterPassword("secret123");

    try std.testing.expect(manager.hasMasterPassword() == true);
    try std.testing.expect(manager.isUnlocked() == true);
}
