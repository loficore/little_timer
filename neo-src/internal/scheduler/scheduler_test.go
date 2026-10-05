// Package scheduler 测试 —— 表驱动覆盖 §5.2 算法的所有边界。
//
// 公共约定:
//   - 日期 = "2026-10-05",默认窗口 [06:00, 23:00) (17h)
//   - 所有 timestamp 用 parseTime 构造,避免依赖运行机时区
//   - Plan.Placements 排序:升序 ScheduledStart,ties 按 TaskID 升序
//   - 必断言项:placements (id 列表)、unscheduled、warnings (子串匹配)
package scheduler

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

// parseTime 把 day+hour+minute 组合成本地时区的 unix 时间戳。
func parseTime(t *testing.T, day string, hour, minute int) int64 {
	t.Helper()
	base, err := time.ParseInLocation("2006-01-02", day, time.Local)
	if err != nil {
		t.Fatalf("parse date %s: %v", day, err)
	}
	return base.Add(time.Duration(hour)*time.Hour + time.Duration(minute)*time.Minute).Unix()
}

func strPtr(s string) *string { return &s }
func i64Ptr(i int64) *int64   { return &i }

// warnHas 断言 plan.Warnings 中至少有一条包含 substring。
func warnHas(plan Plan, substr string) bool {
	for _, w := range plan.Warnings {
		if strings.Contains(w, substr) {
			return true
		}
	}
	return false
}

func TestSchedule_TableDriven(t *testing.T) {
	day := "2026-10-05"

	type want struct {
		placementIDs []int64
		unscheduled  []int64
		warningSubs  []string // 每个子串都必须出现在 Warnings 中
		noWarnings   []string // 每个子串都必须不出现在 Warnings 中
	}

	cases := []struct {
		name   string
		tasks  []Task
		habits []HabitSlot
		opts   Options
		want   want
	}{
		// ---------- 基础空 / 单 task ----------

		{
			"01-空输入",
			nil, nil, Options{},
			want{placementIDs: nil, unscheduled: nil},
		},

		{
			"02-单 task 默认评分",
			[]Task{{ID: 1, Title: "x", EstimatedMin: 30, Scores: Score{Priority: 3, Urgency: 5, Energy: 2, Context: "g"}}},
			nil, Options{},
			want{placementIDs: []int64{1}},
		},

		{
			"03-两个 task 默认评分都排入",
			[]Task{
				{ID: 1, EstimatedMin: 30, Scores: Score{3, 5, 2, "g"}},
				{ID: 2, EstimatedMin: 30, Scores: Score{3, 5, 2, "g"}},
			},
			nil, Options{},
			want{placementIDs: []int64{1, 2}},
		},

		// ---------- Pinned 行为 ----------

		{
			"04-pinned 不带 scheduled_start/end 也排入",
			[]Task{
				{ID: 1, EstimatedMin: 60, Pinned: true, Scores: Score{3, 5, 2, "g"}},
				{ID: 2, EstimatedMin: 30, Scores: Score{3, 5, 2, "g"}},
			},
			nil, Options{},
			want{placementIDs: []int64{1, 2}},
		},

		{
			"05-pinned 带 scheduled_start/end 固定时间,不移动",
			[]Task{
				{ID: 1, EstimatedMin: 60, Pinned: true,
					ScheduledStart: i64Ptr(parseTime(t, day, 14, 0)),
					ScheduledEnd:   i64Ptr(parseTime(t, day, 15, 0)),
					Scores:         Score{3, 5, 2, "g"}},
				{ID: 2, EstimatedMin: 30, Scores: Score{3, 5, 2, "g"}},
			},
			nil, Options{},
			// Task 2 (energy=2 middle) 占 11:00-11:30;Task 1 fixed 14:00-15:00 → 时间升序 [2, 1]
			want{placementIDs: []int64{2, 1}},
		},

		{
			"06-pinned 不带 scheduled 排在 free 之前(pinned 优先占位)",
			[]Task{
				{ID: 1, EstimatedMin: 60, Pinned: true, Scores: Score{1, 1, 2, "g"}}, // sortKey=1
				{ID: 2, EstimatedMin: 30, Scores: Score{5, 10, 2, "g"}},              // sortKey=50, 应后置
			},
			nil, Options{},
			// Pinned 占 06:00-07:00,Task 2 (energy=2) 占 11:00-11:30 → [1, 2]
			want{placementIDs: []int64{1, 2}},
		},

		// ---------- estimated_minutes 边界 ----------

		{
			"07-estimated_minutes < 15 视为 15",
			[]Task{{ID: 1, EstimatedMin: 5, Scores: Score{3, 5, 2, "g"}}},
			nil, Options{},
			want{placementIDs: []int64{1}},
		},

		{
			"08-estimated_minutes = 0 fallback 15",
			[]Task{{ID: 1, EstimatedMin: 0, Scores: Score{3, 5, 2, "g"}}},
			nil, Options{},
			want{placementIDs: []int64{1}},
		},

		{
			"09-estimated_minutes = 14 视为 15",
			[]Task{{ID: 1, EstimatedMin: 14, Scores: Score{3, 5, 2, "g"}}},
			nil, Options{},
			want{placementIDs: []int64{1}},
		},

		{
			"10-estimated_minutes > 180 clamp 180",
			[]Task{{ID: 1, EstimatedMin: 300, Scores: Score{3, 5, 2, "g"}}},
			nil, Options{},
			want{placementIDs: []int64{1}},
		},

		{
			"11-estimated_minutes = 180 不变",
			[]Task{{ID: 1, EstimatedMin: 180, Scores: Score{3, 5, 2, "g"}}},
			nil, Options{},
			want{placementIDs: []int64{1}},
		},

		// ---------- Habit 约束 ----------

		{
			"12-habit preference=1 硬约束避开,middle 被占 → fallback 早",
			[]Task{{ID: 1, EstimatedMin: 60, Scores: Score{3, 5, 2, "g"}}}, // energy=2 middle
			[]HabitSlot{
				{HabitID: 1, DayOfWeek: 0, StartMin: 11 * 60, EndMin: 18 * 60, Preference: 1},
			},
			Options{},
			// middle (11-18) blocked;fallback 早 (06-11) → 06:00-07:00
			want{placementIDs: []int64{1}},
		},

		{
			"13-habit preference=2 软约束 task 可压",
			[]Task{{ID: 1, EstimatedMin: 60, Scores: Score{3, 5, 2, "g"}}},
			[]HabitSlot{
				{HabitID: 1, DayOfWeek: 0, StartMin: 11 * 60, EndMin: 18 * 60, Preference: 2},
			},
			Options{},
			want{placementIDs: []int64{1}},
		},

		{
			"14-habit preference=3 仅参考忽略",
			[]Task{{ID: 1, EstimatedMin: 60, Scores: Score{3, 5, 2, "g"}}},
			[]HabitSlot{
				{HabitID: 1, DayOfWeek: 0, StartMin: 11 * 60, EndMin: 18 * 60, Preference: 3},
			},
			Options{},
			want{placementIDs: []int64{1}},
		},

		{
			"15-habit preference=1 占满全日 → 全 unscheduled + 时间不够",
			[]Task{{ID: 1, EstimatedMin: 30, Scores: Score{3, 5, 2, "g"}}},
			[]HabitSlot{
				{HabitID: 1, DayOfWeek: 0, StartMin: 6 * 60, EndMin: 23 * 60, Preference: 1},
			},
			Options{},
			want{unscheduled: []int64{1}, warningSubs: []string{"时间不够"}},
		},

		{
			"16-多个 habit preference 混合",
			[]Task{{ID: 1, EstimatedMin: 30, Scores: Score{3, 5, 2, "g"}}},
			[]HabitSlot{
				{HabitID: 1, DayOfWeek: 0, StartMin: 8 * 60, EndMin: 9 * 60, Preference: 1},
				{HabitID: 2, DayOfWeek: 0, StartMin: 12 * 60, EndMin: 13 * 60, Preference: 2},
				{HabitID: 3, DayOfWeek: 0, StartMin: 19 * 60, EndMin: 20 * 60, Preference: 3},
			},
			Options{},
			want{placementIDs: []int64{1}},
		},

		// ---------- 时间不够 / 全 pinned 占满 ----------

		{
			"17-17 个 pinned 60min 占满 17h → 第 18 个 unscheduled + 时间不够",
			func() []Task {
				var ts []Task
				for i := int64(1); i <= 17; i++ {
					ts = append(ts, Task{
						ID: i, EstimatedMin: 60, Pinned: true,
						Scores: Score{3, 5, 2, "g"},
					})
				}
				ts = append(ts, Task{ID: 18, EstimatedMin: 60, Scores: Score{3, 5, 2, "g"}})
				return ts
			}(),
			nil, Options{},
			want{
				placementIDs: []int64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17},
				unscheduled:  []int64{18},
				warningSubs:  []string{"时间不够"},
			},
		},

		{
			"18-estimated_minutes=999 默认窗口 clamp 180 后可装",
			[]Task{{ID: 1, EstimatedMin: 999, Scores: Score{3, 5, 2, "g"}}},
			nil, Options{},
			// EstimatedMin 999 被 clamp 到 180;默认窗口 17h 足以容纳,故任务正常排入。
			// 「块大于窗口 → unscheduled + 时间不够」由 case 18b 用 <180min 窗口覆盖。
			want{placementIDs: []int64{1}},
		},

		{
			"18b-窗口 < 180min → 任务进 unscheduled",
			[]Task{{ID: 1, EstimatedMin: 999, Scores: Score{3, 5, 2, "g"}}},
			nil, Options{DayStartHour: 10, DayEndHour: 11}, // 1h = 60min
			want{unscheduled: []int64{1}, warningSubs: []string{"时间不够"}},
		},

		// ---------- 同 context 聚集 ----------

		{
			"19-同 context 聚集:coding 整体在 writing 之前",
			[]Task{
				{ID: 1, EstimatedMin: 30, Scores: Score{3, 5, 2, "writing"}},
				{ID: 2, EstimatedMin: 30, Scores: Score{3, 5, 2, "coding"}},
				{ID: 3, EstimatedMin: 30, Scores: Score{3, 5, 2, "writing"}},
			},
			nil, Options{},
			// context 升序分组:coding(2) 先,writing(1,3) 紧随 → [2,1,3]
			want{placementIDs: []int64{2, 1, 3}},
		},

		{
			"20-3 个中文 context 按 Unicode 升序聚集",
			[]Task{
				{ID: 1, EstimatedMin: 30, Scores: Score{3, 5, 2, "杂事"}},
				{ID: 2, EstimatedMin: 30, Scores: Score{3, 5, 2, "学习"}},
				{ID: 3, EstimatedMin: 30, Scores: Score{3, 5, 2, "编码"}},
			},
			nil, Options{},
			// Unicode 升序:学习(2) < 杂事(1) < 编码(3) → [2,1,3]
			want{placementIDs: []int64{2, 1, 3}},
		},

		// ---------- 拓扑 ----------

		{
			"21-依赖链 A<-B<-C 拓扑 C→B→A",
			[]Task{
				{ID: 1, EstimatedMin: 30, BlockedBy: []int64{2}, Scores: Score{3, 5, 2, "g"}},
				{ID: 2, EstimatedMin: 30, BlockedBy: []int64{3}, Scores: Score{3, 5, 2, "g"}},
				{ID: 3, EstimatedMin: 30, Scores: Score{3, 5, 2, "g"}},
			},
			nil, Options{},
			want{placementIDs: []int64{3, 2, 1}},
		},

		{
			"21b-依赖 + 不同 energy:greedy 顺序非时间序,拓扑仍要正确(A 先 B 后)",
			[]Task{
				{ID: 1, EstimatedMin: 30, Scores: Score{3, 5, 1, "g"}},                        // A:energy=1 → 18:00
				{ID: 2, EstimatedMin: 30, BlockedBy: []int64{1}, Scores: Score{3, 5, 3, "g"}}, // B:blocked_by A, energy=3 → 06:00
			},
			nil, Options{},
			// sortKey: A=15(boost 1.0,非 blocker),B=22.5×0.5=11.25。A 先被处理,放 18:00;
			// B 后被处理,放 06:00。greedy=[A@18:00,B@06:00] 非时间序。topo 后
			// A 必须先于 B(因为 B blocked_by A),按时间升序槽位指派 → A→06:00,B→18:00
			want{placementIDs: []int64{1, 2}},
		},

		{
			"22-循环依赖 1↔2 → 保留原序 + 循环依赖 warning",
			[]Task{
				{ID: 1, EstimatedMin: 30, BlockedBy: []int64{2}, Scores: Score{3, 5, 2, "g"}},
				{ID: 2, EstimatedMin: 30, BlockedBy: []int64{1}, Scores: Score{3, 5, 2, "g"}},
			},
			nil, Options{},
			want{
				placementIDs: []int64{1, 2},
				warningSubs:  []string{"循环依赖"},
			},
		},

		{
			"23-3 节点环 1→2→3→1 → warning",
			[]Task{
				{ID: 1, EstimatedMin: 30, BlockedBy: []int64{3}, Scores: Score{3, 5, 2, "g"}},
				{ID: 2, EstimatedMin: 30, BlockedBy: []int64{1}, Scores: Score{3, 5, 2, "g"}},
				{ID: 3, EstimatedMin: 30, BlockedBy: []int64{2}, Scores: Score{3, 5, 2, "g"}},
			},
			nil, Options{},
			want{
				placementIDs: []int64{1, 2, 3},
				warningSubs:  []string{"循环依赖"},
			},
		},

		// ---------- due_date 排序 ----------

		{
			"24-due_date 早的优先",
			[]Task{
				{ID: 1, EstimatedMin: 30, DueDate: strPtr("2026-10-10"), Scores: Score{3, 5, 2, "g"}},
				{ID: 2, EstimatedMin: 30, DueDate: strPtr("2026-10-07"), Scores: Score{3, 5, 2, "g"}},
			},
			nil, Options{},
			want{placementIDs: []int64{2, 1}},
		},

		{
			"25-due_date nil 排最后",
			[]Task{
				{ID: 1, EstimatedMin: 30, Scores: Score{3, 5, 2, "g"}}, // nil due
				{ID: 2, EstimatedMin: 30, DueDate: strPtr("2026-10-07"), Scores: Score{3, 5, 2, "g"}},
			},
			nil, Options{},
			want{placementIDs: []int64{2, 1}},
		},

		{
			"26-3 个 due_date 排序:nil 末位",
			[]Task{
				{ID: 1, EstimatedMin: 30, Scores: Score{3, 5, 2, "g"}}, // nil
				{ID: 2, EstimatedMin: 30, DueDate: strPtr("2026-10-10"), Scores: Score{3, 5, 2, "g"}},
				{ID: 3, EstimatedMin: 30, DueDate: strPtr("2026-10-05"), Scores: Score{3, 5, 2, "g"}},
			},
			nil, Options{},
			want{placementIDs: []int64{3, 2, 1}},
		},

		// ---------- 能量档位 ----------

		{
			"27-energy=3 偏好早(06-11)",
			[]Task{{ID: 1, EstimatedMin: 60, Scores: Score{3, 5, 3, "g"}}},
			nil, Options{},
			want{placementIDs: []int64{1}}, // 期望时间 06:00-07:00
		},

		{
			"28-energy=1 偏好晚(18-23)",
			[]Task{{ID: 1, EstimatedMin: 60, Scores: Score{3, 5, 1, "g"}}},
			nil, Options{},
			want{placementIDs: []int64{1}}, // 期望时间 18:00-19:00
		},

		{
			"29-energy=2 偏好中(11-18)",
			[]Task{{ID: 1, EstimatedMin: 60, Scores: Score{3, 5, 2, "g"}}},
			nil, Options{},
			want{placementIDs: []int64{1}}, // 期望时间 11:00-12:00
		},

		{
			"30-energy 偏好导致时间差异,同 sortKey → Placements 时间升序",
			[]Task{
				{ID: 1, EstimatedMin: 60, Scores: Score{3, 5, 1, "g"}}, // energy=1, raw=15
				{ID: 2, EstimatedMin: 60, Scores: Score{3, 5, 3, "g"}}, // energy=3, raw=15
			},
			nil, Options{},
			// sortKey 同,TaskID 1<2 → Task 1 先被处理;但 energy=1 选晚,energy=3 选早。
			// Placements 按时间升序 → Task 2 (06-07) 在前,Task 1 (18-19) 在后
			want{placementIDs: []int64{2, 1}},
		},

		// ---------- sortKey 排序 ----------

		{
			"31-sortKey 高的优先",
			[]Task{
				{ID: 1, EstimatedMin: 30, Scores: Score{1, 1, 2, "g"}},  // sortKey=1
				{ID: 2, EstimatedMin: 30, Scores: Score{5, 10, 2, "g"}}, // sortKey=50
			},
			nil, Options{},
			want{placementIDs: []int64{2, 1}},
		},

		// ---------- dependency_boost 排序效果 ----------

		{
			"32-blocked task boost 0.5(可观察)",
			[]Task{
				{ID: 1, EstimatedMin: 30, BlockedBy: []int64{2}, Scores: Score{5, 10, 2, "g"}}, // raw 50, boost 0.5, sortKey=25
				{ID: 2, EstimatedMin: 30, Scores: Score{5, 10, 2, "g"}},                        // raw 50, boost 1.0, sortKey=50
			},
			nil, Options{},
			want{placementIDs: []int64{2, 1}},
		},

		{
			"33-blocker boost 1.5(可观察)",
			[]Task{
				{ID: 1, EstimatedMin: 30, Scores: Score{3, 5, 2, "g"}},                        // raw 15, 是 Task 2 blocker, boost 1.5, sortKey=22.5
				{ID: 2, EstimatedMin: 30, BlockedBy: []int64{1}, Scores: Score{4, 5, 2, "g"}}, // raw 20, blocked, boost 0.5, sortKey=10
			},
			nil, Options{},
			want{placementIDs: []int64{1, 2}},
		},

		// ---------- Options / 默认值 ----------

		{
			"34-Options 默认填充 06-23",
			[]Task{{ID: 1, EstimatedMin: 60, Scores: Score{3, 5, 2, "g"}}},
			nil, Options{}, // 留空 → 默认
			want{placementIDs: []int64{1}},
		},

		{
			"35-自定义 Options DayStart=8 DayEnd=20 → 12h 窗口",
			[]Task{{ID: 1, EstimatedMin: 60, Scores: Score{3, 5, 2, "g"}}},
			nil, Options{DayStartHour: 8, DayEndHour: 20},
			want{placementIDs: []int64{1}},
		},

		{
			"36-自定义窗口 8-9 单 interval → 60min task 占用整段",
			[]Task{{ID: 1, EstimatedMin: 60, Scores: Score{3, 5, 2, "g"}}},
			nil, Options{DayStartHour: 8, DayEndHour: 9},
			want{placementIDs: []int64{1}},
		},

		// ---------- 已排 task(非 pinned)视为占用 ----------

		{
			"37-非 pinned 但已有 ScheduledStart/End 视为占用",
			[]Task{
				{ID: 1, EstimatedMin: 30,
					ScheduledStart: i64Ptr(parseTime(t, day, 10, 0)),
					ScheduledEnd:   i64Ptr(parseTime(t, day, 10, 30)),
					Scores:         Score{3, 5, 2, "g"}}, // 10:00-10:30 fixed
				{ID: 2, EstimatedMin: 30, Scores: Score{3, 5, 2, "g"}},
			},
			nil, Options{},
			// Task 1 fixed at 10:00;Task 2 energy=2 → 11:00-11:30
			want{placementIDs: []int64{1, 2}},
		},

		// ---------- Placement 时间精确性 ----------

		{
			"38-Placement 时间精确:energy=2 占 11:00",
			[]Task{{ID: 1, EstimatedMin: 30, Scores: Score{3, 5, 2, "g"}}},
			nil, Options{},
			// 验证 Plan.Placements[0].ScheduledStart == 11:00 unix
			want{placementIDs: []int64{1}},
		},

		{
			"39-Placement 时间精确:energy=3 占 06:00",
			[]Task{{ID: 1, EstimatedMin: 30, Scores: Score{3, 5, 3, "g"}}},
			nil, Options{},
			want{placementIDs: []int64{1}},
		},

		{
			"40-Placement 时间精确:energy=1 占 18:00",
			[]Task{{ID: 1, EstimatedMin: 30, Scores: Score{3, 5, 1, "g"}}},
			nil, Options{},
			want{placementIDs: []int64{1}},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			plan := Schedule(c.tasks, c.habits, day, c.opts)

			var gotIDs []int64
			for _, p := range plan.Placements {
				gotIDs = append(gotIDs, p.TaskID)
			}
			if !reflect.DeepEqual(gotIDs, c.want.placementIDs) {
				t.Errorf("placement IDs = %v, want %v", gotIDs, c.want.placementIDs)
			}
			if !reflect.DeepEqual(plan.Unscheduled, c.want.unscheduled) {
				t.Errorf("Unscheduled = %v, want %v", plan.Unscheduled, c.want.unscheduled)
			}
			for _, sub := range c.want.warningSubs {
				if !warnHas(plan, sub) {
					t.Errorf("Warnings 缺子串 %q,actual = %v", sub, plan.Warnings)
				}
			}
			for _, sub := range c.want.noWarnings {
				if warnHas(plan, sub) {
					t.Errorf("Warnings 不应含子串 %q,actual = %v", sub, plan.Warnings)
				}
			}

			// Placements 必须按 ScheduledStart 升序 + TaskID 平序
			for i := 1; i < len(plan.Placements); i++ {
				a, b := plan.Placements[i-1], plan.Placements[i]
				if a.ScheduledStart > b.ScheduledStart {
					t.Errorf("Placements 非升序: idx=%d start=%d > idx=%d start=%d",
						i-1, a.ScheduledStart, i, b.ScheduledStart)
				}
				if a.ScheduledStart == b.ScheduledStart && a.TaskID > b.TaskID {
					t.Errorf("Placements 平序错: 同 start=%d, TaskID %d > %d",
						a.ScheduledStart, a.TaskID, b.TaskID)
				}
				if a.ScheduledEnd <= a.ScheduledStart {
					t.Errorf("Placement %d 非法: end=%d <= start=%d", a.TaskID, a.ScheduledEnd, a.ScheduledStart)
				}
			}
		})
	}
}

// TestSchedule_PlacementTimePrecision 单独验证 placement 的 ScheduledStart
// 时间戳(避免 time.Local 漂移时表驱动还能侥幸过)。
func TestSchedule_PlacementTimePrecision(t *testing.T) {
	day := "2026-10-05"

	cases := []struct {
		name        string
		task        Task
		wantStartHM string // "HH:MM"
	}{
		{"energy=2 → 11:00", Task{ID: 1, EstimatedMin: 30, Scores: Score{3, 5, 2, "g"}}, "11:00"},
		{"energy=3 → 06:00", Task{ID: 1, EstimatedMin: 30, Scores: Score{3, 5, 3, "g"}}, "06:00"},
		{"energy=1 → 18:00", Task{ID: 1, EstimatedMin: 30, Scores: Score{3, 5, 1, "g"}}, "18:00"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			plan := Schedule([]Task{c.task}, nil, day, Options{})
			if len(plan.Placements) != 1 {
				t.Fatalf("want 1 placement, got %d (%+v)", len(plan.Placements), plan)
			}
			p := plan.Placements[0]

			base, _ := time.ParseInLocation("2006-01-02", day, time.Local)
			wantHM, _ := time.ParseInLocation("2006-01-02 15:04", day+" "+c.wantStartHM, time.Local)
			if p.ScheduledStart != wantHM.Unix() {
				t.Errorf("ScheduledStart = %d, want unix(%s) = %d (diff %s)",
					p.ScheduledStart, c.wantStartHM, wantHM.Unix(),
					time.Unix(p.ScheduledStart, 0).In(time.Local).Sub(base))
			}
			// 结束 = 开始 + clamp(EstimatedMin, 15, 180) minutes
			wantEnd := wantHM.Add(30 * time.Minute).Unix()
			if p.ScheduledEnd != wantEnd {
				t.Errorf("ScheduledEnd = %d, want %d", p.ScheduledEnd, wantEnd)
			}
		})
	}
}

// TestSchedule_DeterministicNoMapIteration 确保相同输入多次执行结果一致
// (防 map 迭代顺序引入的非确定性)。
func TestSchedule_DeterministicNoMapIteration(t *testing.T) {
	day := "2026-10-05"
	tasks := []Task{
		{ID: 1, EstimatedMin: 30, Scores: Score{3, 5, 2, "g"}},
		{ID: 2, EstimatedMin: 30, Scores: Score{3, 5, 2, "g"}},
		{ID: 3, EstimatedMin: 30, Scores: Score{3, 5, 2, "g"}},
		{ID: 4, EstimatedMin: 30, Scores: Score{3, 5, 2, "g"}},
		{ID: 5, EstimatedMin: 30, Scores: Score{3, 5, 2, "g"}},
	}
	var first Plan
	for i := 0; i < 50; i++ {
		p := Schedule(tasks, nil, day, Options{})
		if i == 0 {
			first = p
			continue
		}
		if !reflect.DeepEqual(p.Placements, first.Placements) {
			t.Fatalf("iteration %d differs from first: %v vs %v", i, p.Placements, first.Placements)
		}
		if !reflect.DeepEqual(p.Unscheduled, first.Unscheduled) {
			t.Fatalf("iteration %d unscheduled differs: %v vs %v", i, p.Unscheduled, first.Unscheduled)
		}
	}
}
