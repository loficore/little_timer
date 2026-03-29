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
    goal_count: i64,
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

pub const HabitCrudManager = struct {
    db: ?zqlite.Conn,
    allocator: std.mem.Allocator,

    pub fn init(allocator: std.mem.Allocator, db: ?zqlite.Conn) HabitCrudManager {
        return .{
            .db = db,
            .allocator = allocator,
        };
    }

    // === 习惯集 CRUD ===

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

    pub fn updateHabitSet(self: *HabitCrudManager, id: i64, name: []const u8, description: []const u8, color: []const u8) !void {
        const db = self.db orelse return HabitError.QueryFailed;
        try db.exec(
            "UPDATE habit_sets SET name = ?, description = ?, color = ? WHERE id = ?;",
            .{ name, description, color, id },
        );
    }

    pub fn deleteHabitSet(self: *HabitCrudManager, id: i64) !void {
        const db = self.db orelse return HabitError.QueryFailed;
        try db.exec("DELETE FROM habit_sets WHERE id = ?;", .{id});
    }

    // === 习惯 CRUD ===

    pub fn createHabit(self: *HabitCrudManager, set_id: i64, name: []const u8, goal_seconds: i64, goal_count: i64, color: []const u8) !i64 {
        const db = self.db orelse return HabitError.QueryFailed;
        try db.exec(
            "INSERT INTO habits (set_id, name, goal_seconds, goal_count, color) VALUES (?, ?, ?, ?, ?);",
            .{ set_id, name, goal_seconds, goal_count, color },
        );
        var rows = try db.rows("SELECT last_insert_rowid();", .{});
        defer rows.deinit();
        const row = rows.next() orelse return HabitError.QueryFailed;
        return row.get(i64, 0);
    }

    pub fn getAllHabits(self: *HabitCrudManager) ![]HabitRow {
        const db = self.db orelse return HabitError.QueryFailed;
        var rows = try db.rows("SELECT id, set_id, name, goal_seconds, goal_count, color FROM habits ORDER BY created_at DESC;", .{});
        defer rows.deinit();

        var list = std.ArrayList(HabitRow){};
        errdefer list.deinit(self.allocator);

        while (rows.next()) |row| {
            try list.append(self.allocator, .{
                .id = row.get(i64, 0),
                .set_id = row.get(i64, 1),
                .name = try self.allocator.dupe(u8, row.get([]const u8, 2)),
                .goal_seconds = row.get(i64, 3),
                .goal_count = row.get(i64, 4),
                .color = try self.allocator.dupe(u8, row.get([]const u8, 5)),
            });
        }
        return list.toOwnedSlice(self.allocator);
    }

    pub fn getHabitsBySet(self: *HabitCrudManager, set_id: i64) ![]HabitRow {
        const db = self.db orelse return HabitError.QueryFailed;
        var rows = try db.rows("SELECT id, set_id, name, goal_seconds, goal_count, color FROM habits WHERE set_id = ? ORDER BY created_at DESC;", .{set_id});
        defer rows.deinit();

        var list = std.ArrayList(HabitRow){};
        errdefer list.deinit(self.allocator);

        while (rows.next()) |row| {
            try list.append(self.allocator, .{
                .id = row.get(i64, 0),
                .set_id = row.get(i64, 1),
                .name = try self.allocator.dupe(u8, row.get([]const u8, 2)),
                .goal_seconds = row.get(i64, 3),
                .goal_count = row.get(i64, 4),
                .color = try self.allocator.dupe(u8, row.get([]const u8, 5)),
            });
        }
        return list.toOwnedSlice(self.allocator);
    }

    pub fn updateHabit(self: *HabitCrudManager, id: i64, name: []const u8, goal_seconds: i64, goal_count: i64, color: []const u8) !void {
        const db = self.db orelse return HabitError.QueryFailed;
        try db.exec(
            "UPDATE habits SET name = ?, goal_seconds = ?, goal_count = ?, color = ? WHERE id = ?;",
            .{ name, goal_seconds, goal_count, color, id },
        );
    }

    pub fn deleteHabit(self: *HabitCrudManager, id: i64) !void {
        const db = self.db orelse return HabitError.QueryFailed;
        try db.exec("DELETE FROM habits WHERE id = ?;", .{id});
    }

    // === 记录 CRUD ===

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

    pub fn getHabitTodaySeconds(self: *HabitCrudManager, habit_id: i64, date: []const u8) !i64 {
        const db = self.db orelse return HabitError.QueryFailed;
        var rows = try db.rows("SELECT COALESCE(SUM(duration_seconds), 0) FROM sessions WHERE habit_id = ? AND date = ?;", .{ habit_id, date });
        defer rows.deinit();

        if (rows.next()) |row| {
            return row.get(i64, 0);
        }
        return 0;
    }

    // === 清理内存 ===

    pub fn freeHabitSets(self: *HabitCrudManager, sets: []HabitSetRow) void {
        for (sets) |s| {
            self.allocator.free(s.name);
            self.allocator.free(s.description);
            self.allocator.free(s.color);
        }
        self.allocator.free(sets);
    }

    pub fn freeHabits(self: *HabitCrudManager, habits: []HabitRow) void {
        for (habits) |h| {
            self.allocator.free(h.name);
            self.allocator.free(h.color);
        }
        self.allocator.free(habits);
    }

    pub fn freeSessions(self: *HabitCrudManager, sessions: []SessionRow) void {
        for (sessions) |s| {
            self.allocator.free(s.started_at);
            self.allocator.free(s.date);
        }
        self.allocator.free(sessions);
    }
};
