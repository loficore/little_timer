//! 习惯追踪 CRUD 操作模块
const std = @import("std");
const zqlite = @import("zqlite");
const logger = @import("../core/logger.zig");

pub const HabitError = error{
    InsertFailed,
    UpdateFailed,
    DeleteFailed,
    QueryFailed,
    NotFound,
};

pub const HabitSetRow = struct {
    id: i64,
    name: []const u8,
    description: []const u8,
    color: []const u8,
    wallpaper: []const u8,
};

pub const HabitRow = struct {
    id: i64,
    set_id: i64,
    name: []const u8,
    goal_seconds: i64,
    color: []const u8,
    wallpaper: []const u8,
};

pub const SessionRow = struct {
    id: i64,
    habit_id: i64,
    duration_seconds: i64,
    count: i64,
    started_at: []const u8,
    date: []const u8,
};

pub const TimerSessionRow = struct {
    id: i64,
    habit_id: ?i64,
    mode: []const u8,
    started_at: i64,
    updated_at: i64,
    is_running: bool,
    is_finished: bool,
    is_paused: bool,
    elapsed_seconds: i64,
    paused_total_seconds: i64,
    pause_started_at: ?i64,
    last_synced_at: ?i64,
    remaining_seconds: ?i64,
    work_duration: i64,
    rest_duration: i64,
    loop_count: i64,
    current_round: i64,
    in_rest: bool,
};

/// 习惯 CRUD 管理器
pub const HabitCrudManager = struct {
    db: ?zqlite.Conn,
    allocator: std.mem.Allocator,

    /// 初始化管理器
    /// 参数：
    /// - **allocator**：内存分配器，用于动态分配内存
    /// - **db**：可选的数据库连接，如果为 null 则所有操作都会失败
    /// 返回：
    /// - **HabitCrudManager** ： 返回一个新的 HabitCrudManager 实例
    pub fn init(allocator: std.mem.Allocator, db: ?zqlite.Conn) HabitCrudManager {
        return .{
            .db = db,
            .allocator = allocator,
        };
    }

    // === 习惯集 CRUD ===

    /// 创建新的习惯集
    /// 参数：
    /// - **name**：习惯集名称
    /// - **description**：习惯集描述
    /// - **color**：习惯集颜色（字符串格式，如 "#FF0000"）
    /// 返回：
    /// - **i64**：新创建的习惯集 ID
    /// 错误：
    /// - **HabitError.QueryFailed**：如果数据库查询失败
    pub fn createHabitSet(self: *HabitCrudManager, name: []const u8, description: []const u8, color: []const u8) !i64 {
        const db = self.db orelse return HabitError.QueryFailed;
        try db.exec(
            "INSERT INTO habit_sets (name, description, color) VALUES (?, ?, ?);",
            .{ name, description, color },
        );
        var rows = try db.rows("SELECT last_insert_rowid();", .{});
        defer rows.deinit();
        const row = rows.next() orelse return HabitError.QueryFailed;
        return row.get(i64, 0);
    }

    /// 获取所有习惯集信息
    /// 返回：
    /// - **[]HabitSetRow**：包含所有习惯集信息的数组
    /// 错误：
    /// - **HabitError.QueryFailed**：如果数据库查询失败
    pub fn getAllHabitSets(self: *HabitCrudManager) ![]HabitSetRow {
        const db = self.db orelse {
            logger.global_logger.err("❌ getAllHabitSets: db is null", .{});
            return HabitError.QueryFailed;
        };
        var rows = db.rows("SELECT id, name, description, color, COALESCE(wallpaper, '') FROM habit_sets ORDER BY created_at DESC;", .{}) catch |err| {
            logger.global_logger.err("❌ getAllHabitSets 查询失败: {any}", .{err});
            return HabitError.QueryFailed;
        };
        defer rows.deinit();

        var list = std.ArrayList(HabitSetRow){};
        errdefer list.deinit(self.allocator);

        while (rows.next()) |row| {
            try list.append(self.allocator, .{
                .id = row.get(i64, 0),
                .name = try self.allocator.dupe(u8, row.get([]const u8, 1)),
                .description = try self.allocator.dupe(u8, row.get([]const u8, 2)),
                .color = try self.allocator.dupe(u8, row.get([]const u8, 3)),
                .wallpaper = try self.allocator.dupe(u8, row.get([]const u8, 4)),
            });
        }
        return list.toOwnedSlice(self.allocator);
    }

    /// 更新习惯集信息
    /// 参数：
    /// - **id**：要更新的习惯集 ID
    /// - **name**：新的习惯集名称
    /// - **description**：新的习惯集描述
    /// - **color**：新的习惯集颜色（字符串格式，如 "#FF0000"）
    /// 错误：
    /// - **HabitError.QueryFailed**：如果数据库查询失败
    pub fn updateHabitSet(self: *HabitCrudManager, id: i64, name: []const u8, description: []const u8, color: []const u8, wallpaper: []const u8) !void {
        const db = self.db orelse return HabitError.QueryFailed;
        try db.exec(
            "UPDATE habit_sets SET name = ?, description = ?, color = ?, wallpaper = ? WHERE id = ?;",
            .{ name, description, color, wallpaper, id },
        );
    }

    /// 删除习惯集
    /// 参数：
    /// - **id**：要删除的习惯集 ID
    /// 错误：
    /// - **HabitError.QueryFailed**：如果数据库查询失败
    pub fn deleteHabitSet(self: *HabitCrudManager, id: i64) !void {
        const db = self.db orelse return HabitError.QueryFailed;
        try db.exec("DELETE FROM habit_sets WHERE id = ?;", .{id});
    }

    // === 习惯 CRUD ===
    /// 创建新的习惯
    /// 参数：
    /// - **set_id**：所属习惯集 ID
    /// - **name**：习惯名称
    /// - **goal_seconds**：目标时长（秒）
    /// - **color**：习惯颜色（字符串格式，如 "#FF0000"）
    /// 返回：
    /// - **i64**：新创建的习惯 ID
    /// 错误：
    /// - **HabitError.QueryFailed**：如果数据库查询失败
    pub fn createHabit(self: *HabitCrudManager, set_id: i64, name: []const u8, goal_seconds: i64, color: []const u8) !i64 {
        const db = self.db orelse return HabitError.QueryFailed;
        try db.exec(
            "INSERT INTO habits (set_id, name, goal_seconds, color) VALUES (?, ?, ?, ?);",
            .{ set_id, name, goal_seconds, color },
        );
        var rows = try db.rows("SELECT last_insert_rowid();", .{});
        defer rows.deinit();
        const row = rows.next() orelse return HabitError.QueryFailed;
        return row.get(i64, 0);
    }

    /// 获取所有习惯信息
    /// 返回：
    /// - **[]HabitRow**：包含所有习惯信息的数组
    /// 错误：
    /// - **HabitError.QueryFailed**：如果数据库查询失败
    pub fn getAllHabits(self: *HabitCrudManager) ![]HabitRow {
        const db = self.db orelse return HabitError.QueryFailed;
        var rows = try db.rows("SELECT id, set_id, name, goal_seconds, color, COALESCE(wallpaper, '') FROM habits ORDER BY created_at DESC;", .{});
        defer rows.deinit();

        var list = std.ArrayList(HabitRow){};
        errdefer list.deinit(self.allocator);

        while (rows.next()) |row| {
            try list.append(self.allocator, .{
                .id = row.get(i64, 0),
                .set_id = row.get(i64, 1),
                .name = try self.allocator.dupe(u8, row.get([]const u8, 2)),
                .goal_seconds = row.get(i64, 3),
                .color = try self.allocator.dupe(u8, row.get([]const u8, 4)),
                .wallpaper = try self.allocator.dupe(u8, row.get([]const u8, 5)),
            });
        }
        return list.toOwnedSlice(self.allocator);
    }

    /// 根据习惯集 ID 获取习惯信息
    /// 参数：
    /// - **set_id**：习惯集 ID
    /// 返回：
    /// - **[]HabitRow**：包含指定习惯集下所有习惯信息的数组
    /// 错误：
    /// - **HabitError.QueryFailed**：如果数据库查询失败
    pub fn getHabitsBySet(self: *HabitCrudManager, set_id: i64) ![]HabitRow {
        const db = self.db orelse return HabitError.QueryFailed;
        var rows = try db.rows("SELECT id, set_id, name, goal_seconds, color, COALESCE(wallpaper, '') FROM habits WHERE set_id = ? ORDER BY created_at DESC;", .{set_id});
        defer rows.deinit();

        var list = std.ArrayList(HabitRow){};
        errdefer list.deinit(self.allocator);

        while (rows.next()) |row| {
            try list.append(self.allocator, .{
                .id = row.get(i64, 0),
                .set_id = row.get(i64, 1),
                .name = try self.allocator.dupe(u8, row.get([]const u8, 2)),
                .goal_seconds = row.get(i64, 3),
                .color = try self.allocator.dupe(u8, row.get([]const u8, 4)),
                .wallpaper = try self.allocator.dupe(u8, row.get([]const u8, 5)),
            });
        }
        return list.toOwnedSlice(self.allocator);
    }

    /// 更新习惯信息
    /// 参数：
    /// - **id**：要更新的习惯 ID
    /// - **name**：新的习惯名称
    /// - **goal_seconds**：新的目标时长（秒）
    /// - **color**：新的习惯颜色（字符串格式，如 "#FF0000"）
    /// 错误：
    /// - **HabitError.QueryFailed**：如果数据库查询失败
    pub fn updateHabit(self: *HabitCrudManager, id: i64, name: []const u8, goal_seconds: i64, color: []const u8, wallpaper: []const u8) !void {
        const db = self.db orelse return HabitError.QueryFailed;
        try db.exec(
            "UPDATE habits SET name = ?, goal_seconds = ?, color = ?, wallpaper = ? WHERE id = ?;",
            .{ name, goal_seconds, color, wallpaper, id },
        );
    }

    /// 删除习惯
    /// 参数：
    /// - **id**：要删除的习惯 ID
    /// 错误：
    /// - **HabitError.QueryFailed**：如果数据库查询失败
    pub fn deleteHabit(self: *HabitCrudManager, id: i64) !void {
        const db = self.db orelse return HabitError.QueryFailed;
        try db.exec("DELETE FROM habits WHERE id = ?;", .{id});
    }

    /// 获取指定习惯的详情
    /// 参数：
    /// - **id**：习惯 ID
    /// 返回：
    /// - **?HabitRow**：习惯信息，如果不存在则返回 null
    /// 错误：
    /// - **HabitError.QueryFailed**：如果数据库查询失败
    pub fn getHabitById(self: *HabitCrudManager, id: i64) !?HabitRow {
        const db = self.db orelse return HabitError.QueryFailed;
        var rows = try db.rows("SELECT id, set_id, name, goal_seconds, color, COALESCE(wallpaper, '') FROM habits WHERE id = ?;", .{id});
        defer rows.deinit();

        if (rows.next()) |row| {
            const name = try self.allocator.dupe(u8, row.get([]const u8, 2));
            errdefer self.allocator.free(name);
            const color = try self.allocator.dupe(u8, row.get([]const u8, 4));
            errdefer self.allocator.free(color);
            const wallpaper = try self.allocator.dupe(u8, row.get([]const u8, 5));
            errdefer self.allocator.free(wallpaper);

            return .{
                .id = row.get(i64, 0),
                .set_id = row.get(i64, 1),
                .name = name,
                .goal_seconds = row.get(i64, 3),
                .color = color,
                .wallpaper = wallpaper,
            };
        }
        return null;
    }

    // === 记录 CRUD ===
    /// 创建新的记录
    /// 参数：
    /// - **habit_id**：所属习惯 ID
    /// - **duration_seconds**：持续时长（秒）
    /// - **count**：完成次数（默认为 1）
    /// - **date**：记录日期（字符串格式，如 "2024-06-01"）
    /// 返回：
    /// - **i64**：新创建的记录 ID
    /// 错误：
    /// - **HabitError.QueryFailed**：如果数据库查询失败
    pub fn createSession(self: *HabitCrudManager, habit_id: i64, duration_seconds: i64, count: i64, date: []const u8) !i64 {
        const db = self.db orelse return HabitError.QueryFailed;
        try db.exec(
            "INSERT INTO sessions (habit_id, duration_seconds, count, date) VALUES (?, ?, ?, ?);",
            .{ habit_id, duration_seconds, count, date },
        );
        var rows = try db.rows("SELECT last_insert_rowid();", .{});
        defer rows.deinit();
        const row = rows.next() orelse return HabitError.QueryFailed;
        return row.get(i64, 0);
    }

    /// 获取指定日期的所有记录
    /// 参数：
    /// - **date**：记录日期（字符串格式，如 "2024-06-01"）
    /// 返回：
    /// - **[]SessionRow**：包含指定日期所有记录信息的数组
    /// 错误：
    /// - **HabitError.QueryFailed**：如果数据库查询失败
    pub fn getSessionsByDate(self: *HabitCrudManager, date: []const u8) ![]SessionRow {
        const db = self.db orelse return HabitError.QueryFailed;
        var rows = try db.rows("SELECT id, habit_id, duration_seconds, count, started_at, date FROM sessions WHERE date = ? ORDER BY started_at DESC;", .{date});
        defer rows.deinit();

        var list = std.ArrayList(SessionRow){};
        errdefer {
            for (list.items) |s| {
                self.allocator.free(s.started_at);
                self.allocator.free(s.date);
            }
            list.deinit(self.allocator);
        }

        while (rows.next()) |row| {
            const started_at = try self.allocator.dupe(u8, row.get([]const u8, 4));
            errdefer self.allocator.free(started_at);
            const row_date = try self.allocator.dupe(u8, row.get([]const u8, 5));
            errdefer self.allocator.free(row_date);

            try list.append(self.allocator, .{
                .id = row.get(i64, 0),
                .habit_id = row.get(i64, 1),
                .duration_seconds = row.get(i64, 2),
                .count = row.get(i64, 3),
                .started_at = started_at,
                .date = row_date,
            });
        }
        return list.toOwnedSlice(self.allocator);
    }

    /// 获取指定日期范围内的所有记录
    /// 参数：
    /// - **start_date**：开始日期（字符串格式，如 "2024-06-01"）
    /// - **end_date**：结束日期（字符串格式，如 "2024-06-30"）
    /// 返回：
    /// - **[]SessionRow**：包含指定日期范围内所有记录信息的数组
    /// 错误：
    /// - **HabitError.QueryFailed**：如果数据库查询失败
    pub fn getSessionsByDateRange(self: *HabitCrudManager, start_date: []const u8, end_date: []const u8) ![]SessionRow {
        const db = self.db orelse return HabitError.QueryFailed;
        var rows = try db.rows("SELECT id, habit_id, duration_seconds, count, started_at, date FROM sessions WHERE date >= ? AND date <= ? ORDER BY date DESC;", .{ start_date, end_date });
        defer rows.deinit();

        var list = std.ArrayList(SessionRow){};
        errdefer {
            for (list.items) |s| {
                self.allocator.free(s.started_at);
                self.allocator.free(s.date);
            }
            list.deinit(self.allocator);
        }

        while (rows.next()) |row| {
            const started_at = try self.allocator.dupe(u8, row.get([]const u8, 4));
            errdefer self.allocator.free(started_at);
            const row_date = try self.allocator.dupe(u8, row.get([]const u8, 5));
            errdefer self.allocator.free(row_date);

            try list.append(self.allocator, .{
                .id = row.get(i64, 0),
                .habit_id = row.get(i64, 1),
                .duration_seconds = row.get(i64, 2),
                .count = row.get(i64, 3),
                .started_at = started_at,
                .date = row_date,
            });
        }
        return list.toOwnedSlice(self.allocator);
    }

    /// 获取指定习惯在指定日期的总时长
    /// 参数：
    /// - **habit_id**：习惯 ID
    /// - **date**：记录日期（字符串格式，如 "2024-06-01"）
    /// 返回：
    /// - **i64**：指定习惯在指定日期的总时长（秒）
    /// 错误：
    /// - **HabitError.QueryFailed**：如果数据库查询失败
    pub fn getHabitTodaySeconds(self: *HabitCrudManager, habit_id: i64, date: []const u8) !i64 {
        const db = self.db orelse return HabitError.QueryFailed;
        var rows = try db.rows("SELECT COALESCE(SUM(duration_seconds), 0) FROM sessions WHERE habit_id = ? AND date = ?;", .{ habit_id, date });
        defer rows.deinit();

        if (rows.next()) |row| {
            return row.get(i64, 0);
        }
        return 0;
    }

    /// 获取指定习惯的连续完成天数（习惯连胜）
    /// 参数：
    /// - **habit_id**：习惯 ID
    /// - **goal_seconds**：完成目标时长（秒）
    /// 返回：
    /// - **i64**：指定习惯的连续完成天数
    /// 错误：
    /// - **HabitError.QueryFailed**：如果数据库查询失败
    pub fn getHabitStreak(self: *HabitCrudManager, habit_id: i64, goal_seconds: i64) !i64 {
        const db = self.db orelse return HabitError.QueryFailed;

        var rows = try db.rows("SELECT date, SUM(duration_seconds) as total_seconds FROM sessions WHERE habit_id = ? GROUP BY date ORDER BY date DESC LIMIT 365;", .{habit_id});
        defer rows.deinit();

        var streak: i64 = 0;
        var prev_date: ?[]const u8 = null;
        _ = std.time.timestamp();

        while (rows.next()) |row| {
            const date = row.get([]const u8, 0);
            const total_seconds = row.get(i64, 1);

            if (total_seconds < goal_seconds) break;

            if (prev_date) |pd| {
                const prev_y = std.fmt.parseInt(i64, pd[0..4], 10) catch break;
                const prev_m = std.fmt.parseInt(i64, pd[5..7], 10) catch break;
                const prev_d = std.fmt.parseInt(i64, pd[8..10], 10) catch break;

                const curr_y = std.fmt.parseInt(i64, date[0..4], 10) catch break;
                const curr_m = std.fmt.parseInt(i64, date[5..7], 10) catch break;
                const curr_d = std.fmt.parseInt(i64, date[8..10], 10) catch break;

                const prev_days = prev_y * 365 + prev_m * 31 + prev_d;
                const curr_days = curr_y * 365 + curr_m * 31 + curr_d;

                if (curr_days != prev_days - 1) break;
            }
            streak += 1;
            prev_date = date;
        }

        return streak;
    }

    // === Timer Session CRUD ===

    /// 创建新的计时会话
    pub fn createTimerSession(
        self: *HabitCrudManager,
        habit_id: ?i64,
        mode: []const u8,
        work_duration: i64,
        rest_duration: i64,
        loop_count: i64,
    ) !i64 {
        const db = self.db orelse return HabitError.QueryFailed;
        const now_ts: i64 = @intCast(std.time.timestamp());

        // 处理 nullable habit_id
        const hid_null: ?i64 = if (habit_id != null) habit_id else null;

        try db.exec(
            \\INSERT INTO timer_sessions 
            \\(habit_id, mode, started_at, updated_at, is_running, is_finished, is_paused, elapsed_seconds, paused_total_seconds, pause_started_at, last_synced_at, remaining_seconds, work_duration, rest_duration, loop_count, current_round, in_rest)
            \\VALUES (?, ?, ?, ?, 1, 0, 0, 0, 0, NULL, ?, ?, ?, ?, ?, 0, 0);
        , .{
            hid_null,
            mode,
            now_ts,
            now_ts,
            now_ts,
            work_duration,
            work_duration,
            rest_duration,
            loop_count,
        });

        var rows = try db.rows("SELECT last_insert_rowid();", .{});
        defer rows.deinit();
        const row = rows.next() orelse return HabitError.QueryFailed;
        return row.get(i64, 0);
    }

    /// 更新计时会话状态
    pub fn updateTimerSession(
        self: *HabitCrudManager,
        session_id: i64,
        elapsed_seconds: i64,
        remaining_seconds: ?i64,
        paused_total_seconds: i64,
        pause_started_at: ?i64,
        last_synced_at: ?i64,
        is_running: bool,
        is_paused: bool,
        is_finished: bool,
        current_round: i64,
        in_rest: bool,
    ) !void {
        const db = self.db orelse return HabitError.QueryFailed;
        const now = std.time.timestamp();

        try db.exec(
            \\UPDATE timer_sessions 
            \\SET updated_at = ?, elapsed_seconds = ?, remaining_seconds = ?, paused_total_seconds = ?, pause_started_at = ?, last_synced_at = ?, is_running = ?, is_paused = ?, is_finished = ?, current_round = ?, in_rest = ?
            \\WHERE id = ?;
        , .{
            now,
            elapsed_seconds,
            remaining_seconds,
            paused_total_seconds,
            pause_started_at,
            last_synced_at,
            if (is_running) @as(i64, 1) else @as(i64, 0),
            if (is_paused) @as(i64, 1) else @as(i64, 0),
            if (is_finished) @as(i64, 1) else @as(i64, 0),
            current_round,
            if (in_rest) @as(i64, 1) else @as(i64, 0),
            session_id,
        });
    }

    /// 获取当前活跃的计时会话
    pub fn getActiveTimerSession(self: *HabitCrudManager) !?TimerSessionRow {
        const db = self.db orelse return HabitError.QueryFailed;

        var rows = try db.rows(
            \\SELECT id, habit_id, mode, started_at, updated_at, is_running, is_finished, is_paused, 
            \\elapsed_seconds, paused_total_seconds, pause_started_at, last_synced_at, remaining_seconds, work_duration, rest_duration, loop_count, current_round, in_rest
            \\FROM timer_sessions 
            \\WHERE is_finished = 0 
            \\ORDER BY updated_at DESC 
            \\LIMIT 1;
        , .{});
        defer rows.deinit();

        if (rows.next()) |row| {
            return .{
                .id = row.get(i64, 0),
                .habit_id = row.get(?i64, 1),
                .mode = try self.allocator.dupe(u8, row.get([]const u8, 2)),
                .started_at = row.get(i64, 3),
                .updated_at = row.get(i64, 4),
                .is_running = row.get(i64, 5) == 1,
                .is_finished = row.get(i64, 6) == 1,
                .is_paused = row.get(i64, 7) == 1,
                .elapsed_seconds = row.get(i64, 8),
                .paused_total_seconds = row.get(i64, 9),
                .pause_started_at = row.get(?i64, 10),
                .last_synced_at = row.get(?i64, 11),
                .remaining_seconds = row.get(?i64, 12),
                .work_duration = row.get(i64, 13),
                .rest_duration = row.get(i64, 14),
                .loop_count = row.get(i64, 15),
                .current_round = row.get(i64, 16),
                .in_rest = row.get(i64, 17) == 1,
            };
        }
        return null;
    }

    /// 根据会话 ID 获取计时会话
    pub fn getTimerSessionById(self: *HabitCrudManager, session_id: i64) !?TimerSessionRow {
        const db = self.db orelse return HabitError.QueryFailed;

        var rows = try db.rows(
            \\SELECT id, habit_id, mode, started_at, updated_at, is_running, is_finished, is_paused,
            \\elapsed_seconds, paused_total_seconds, pause_started_at, last_synced_at, remaining_seconds, work_duration, rest_duration, loop_count, current_round, in_rest
            \\FROM timer_sessions
            \\WHERE id = ?
            \\LIMIT 1;
        , .{session_id});
        defer rows.deinit();

        if (rows.next()) |row| {
            return .{
                .id = row.get(i64, 0),
                .habit_id = row.get(?i64, 1),
                .mode = try self.allocator.dupe(u8, row.get([]const u8, 2)),
                .started_at = row.get(i64, 3),
                .updated_at = row.get(i64, 4),
                .is_running = row.get(i64, 5) == 1,
                .is_finished = row.get(i64, 6) == 1,
                .is_paused = row.get(i64, 7) == 1,
                .elapsed_seconds = row.get(i64, 8),
                .paused_total_seconds = row.get(i64, 9),
                .pause_started_at = row.get(?i64, 10),
                .last_synced_at = row.get(?i64, 11),
                .remaining_seconds = row.get(?i64, 12),
                .work_duration = row.get(i64, 13),
                .rest_duration = row.get(i64, 14),
                .loop_count = row.get(i64, 15),
                .current_round = row.get(i64, 16),
                .in_rest = row.get(i64, 17) == 1,
            };
        }
        return null;
    }

    /// 删除计时会话
    pub fn deleteTimerSession(self: *HabitCrudManager, session_id: i64) !void {
        const db = self.db orelse return HabitError.QueryFailed;
        try db.exec("DELETE FROM timer_sessions WHERE id = ?;", .{session_id});
    }

    /// 完成计时会话（标记为完成）
    pub fn finishTimerSession(self: *HabitCrudManager, session_id: i64) !void {
        const db = self.db orelse return HabitError.QueryFailed;
        const now = std.time.timestamp();

        try db.exec(
            \\UPDATE timer_sessions 
            \\SET updated_at = ?, is_running = 0, is_finished = 1, is_paused = 0
            \\WHERE id = ?;
        , .{ now, session_id });
    }

    // === 清理内存 ===
    /// 释放习惯集信息占用的内存
    /// 参数：
    /// - **sets**：要释放的习惯集信息数组
    /// 返回：
    /// - **void**：无返回值
    pub fn freeHabitSets(self: *HabitCrudManager, sets: []HabitSetRow) void {
        for (sets) |s| {
            self.allocator.free(s.name);
            self.allocator.free(s.description);
            self.allocator.free(s.color);
            self.allocator.free(s.wallpaper);
        }
        self.allocator.free(sets);
    }

    /// 释放习惯信息占用的内存
    /// 参数：
    /// - **habits**：要释放的习惯信息数组
    /// 返回：
    /// - **void**：无返回值
    pub fn freeHabits(self: *HabitCrudManager, habits: []HabitRow) void {
        for (habits) |h| {
            self.allocator.free(h.name);
            self.allocator.free(h.color);
            self.allocator.free(h.wallpaper);
        }
        self.allocator.free(habits);
    }

    /// 释放单个习惯信息占用的内存
    /// 参数：
    /// - **habit**：要释放的习惯信息
    /// 返回：
    /// - **void**：无返回值
    pub fn freeHabit(self: *HabitCrudManager, habit: HabitRow) void {
        self.allocator.free(habit.name);
        self.allocator.free(habit.color);
        self.allocator.free(habit.wallpaper);
    }

    /// 释放记录信息占用的内存
    /// 参数：
    /// - **sessions**：要释放的记录信息数组
    /// 返回：
    /// - **void**：无返回值
    pub fn freeSessions(self: *HabitCrudManager, sessions: []SessionRow) void {
        for (sessions) |s| {
            self.allocator.free(s.started_at);
            self.allocator.free(s.date);
        }
        self.allocator.free(sessions);
    }
};
