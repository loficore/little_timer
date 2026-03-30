//! 习惯追踪 CRUD 操作模块
const std = @import("std");
const zqlite = @import("zqlite");

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
};

pub const HabitRow = struct {
    id: i64,
    set_id: i64,
    name: []const u8,
    goal_seconds: i64,
    color: []const u8,
};

pub const SessionRow = struct {
    id: i64,
    habit_id: i64,
    duration_seconds: i64,
    count: i64,
    started_at: []const u8,
    date: []const u8,
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
        const db = self.db orelse return HabitError.QueryFailed;
        var rows = try db.rows("SELECT id, name, description, color FROM habit_sets ORDER BY created_at DESC;", .{});
        defer rows.deinit();

        var list = std.ArrayList(HabitSetRow){};
        errdefer list.deinit(self.allocator);

        while (rows.next()) |row| {
            try list.append(self.allocator, .{
                .id = row.get(i64, 0),
                .name = try self.allocator.dupe(u8, row.get([]const u8, 1)),
                .description = try self.allocator.dupe(u8, row.get([]const u8, 2)),
                .color = try self.allocator.dupe(u8, row.get([]const u8, 3)),
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
    pub fn updateHabitSet(self: *HabitCrudManager, id: i64, name: []const u8, description: []const u8, color: []const u8) !void {
        const db = self.db orelse return HabitError.QueryFailed;
        try db.exec(
            "UPDATE habit_sets SET name = ?, description = ?, color = ? WHERE id = ?;",
            .{ name, description, color, id },
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
        var rows = try db.rows("SELECT id, set_id, name, goal_seconds, color FROM habits ORDER BY created_at DESC;", .{});
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
        var rows = try db.rows("SELECT id, set_id, name, goal_seconds, color FROM habits WHERE set_id = ? ORDER BY created_at DESC;", .{set_id});
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
    pub fn updateHabit(self: *HabitCrudManager, id: i64, name: []const u8, goal_seconds: i64, color: []const u8) !void {
        const db = self.db orelse return HabitError.QueryFailed;
        try db.exec(
            "UPDATE habits SET name = ?, goal_seconds = ?, color = ? WHERE id = ?;",
            .{ name, goal_seconds, color, id },
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
        errdefer list.deinit(self.allocator);

        while (rows.next()) |row| {
            try list.append(self.allocator, .{
                .id = row.get(i64, 0),
                .habit_id = row.get(i64, 1),
                .duration_seconds = row.get(i64, 2),
                .count = row.get(i64, 3),
                .started_at = try self.allocator.dupe(u8, row.get([]const u8, 4)),
                .date = try self.allocator.dupe(u8, row.get([]const u8, 5)),
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
        errdefer list.deinit(self.allocator);

        while (rows.next()) |row| {
            try list.append(self.allocator, .{
                .id = row.get(i64, 0),
                .habit_id = row.get(i64, 1),
                .duration_seconds = row.get(i64, 2),
                .count = row.get(i64, 3),
                .started_at = try self.allocator.dupe(u8, row.get([]const u8, 4)),
                .date = try self.allocator.dupe(u8, row.get([]const u8, 5)),
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
        }
        self.allocator.free(habits);
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
