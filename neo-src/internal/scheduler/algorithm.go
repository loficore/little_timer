// algorithm.go —— 实现 spec §5.2 的「贪心 4 阶段 + 拓扑修正」算法。
//
// 阶段:
//  1. 解析日期 + 计算窗口(dayStart..dayEnd)
//  2. 收集「硬占位」:已排任务(ScheduledStart/End 都设) + preference=1 的 habit
//  3. 先把 Pinned 但没时间的 task 按 TaskID 顺序 earliest-fit 放进窗口
//     (它们一旦排进去,就不再被移动,与 spec「Pinned=true 不再排」一致)
//  4. 把剩余任务按 sortKey / due_date / context / TaskID 排序,
//     按 energy 档位(早/中/晚) 偏好贪心摆位
//  5. 对非锚点(greedy)placements 做一次拓扑修正;检测到环则保留原序 + warning
//
// 所有时间均为 unix 秒(本地时区,经 time.ParseInLocation("2006-01-02", ...))。
package scheduler

import (
	"fmt"
	"sort"
	"time"
)

// interval 表示一个 [start, end) 的时间区间(unix 秒)。
type interval struct{ start, end int64 }

// ledger 记录一个 placement 的元信息(用于拓扑修正阶段)。
type ledger struct {
	taskID int64
	start  int64
	end    int64
	reason string
}

// tierRange 把 (hourStart, hourEnd) 转成绝对 unix 区间(基于 dayBase)。
// 用于能量档位(早/中/晚)的硬编码区间;调用方负责将其与窗口求交。
func tierRange(dayBase time.Time, hourStart, hourEnd int) interval {
	return interval{
		start: dayBase.Add(time.Duration(hourStart) * time.Hour).Unix(),
		end:   dayBase.Add(time.Duration(hourEnd) * time.Hour).Unix(),
	}
}

// atMin 把「分钟 since midnight」转成 unix。
func atMin(base time.Time, minOfDay int) int64 {
	return base.Add(time.Duration(minOfDay) * time.Minute).Unix()
}

// scheduleImpl 是真正的算法核心;见文件头注释。
func scheduleImpl(tasks []Task, habits []HabitSlot, date string, opts Options) Plan {
	plan := Plan{Date: date}

	// 解析日期。失败 → 返回只带 warning 的 plan(让上层知道日期有问题)。
	base, err := time.ParseInLocation("2006-01-02", date, time.Local)
	if err != nil {
		plan.Warnings = append(plan.Warnings, fmt.Sprintf("日期解析失败: %s", err))
		return plan
	}

	// 窗口 = [dayStart, dayEnd);若 DayEnd <= DayStart 直接返回空 plan。
	dayStart := base.Add(time.Duration(opts.DayStartHour) * time.Hour).Unix()
	dayEnd := base.Add(time.Duration(opts.DayEndHour) * time.Hour).Unix()
	if dayEnd <= dayStart {
		plan.Warnings = append(plan.Warnings, fmt.Sprintf("无效窗口: DayStartHour=%d DayEndHour=%d",
			opts.DayStartHour, opts.DayEndHour))
		return plan
	}
	window := interval{start: dayStart, end: dayEnd}

	// 1) 收集「锚点」:ScheduledStart/End 都已设的任务;再加 preference=1 的 habit。
	var anchored []ledger
	var occupied []interval
	for _, tk := range tasks {
		if tk.ScheduledStart != nil && tk.ScheduledEnd != nil && *tk.ScheduledEnd > *tk.ScheduledStart {
			anchored = append(anchored, ledger{
				taskID: tk.ID, start: *tk.ScheduledStart, end: *tk.ScheduledEnd,
				reason: "已排:ScheduledStart/End 已固定,不移动",
			})
			occupied = append(occupied, interval{start: *tk.ScheduledStart, end: *tk.ScheduledEnd})
		}
	}
	for _, h := range habits {
		if h.Preference != 1 {
			continue
		}
		if h.EndMin <= h.StartMin {
			continue // 防御:异常输入
		}
		s := atMin(base, h.StartMin)
		e := atMin(base, h.EndMin)
		// 与窗口求交;若区间在窗口外,anchored habit 仍可能影响(此处仅作 obstacle)
		if e <= dayStart || s >= dayEnd {
			continue // 完全在窗口外,不影响
		}
		if s < dayStart {
			s = dayStart
		}
		if e > dayEnd {
			e = dayEnd
		}
		occupied = append(occupied, interval{start: s, end: e})
	}

	// 切割空闲区间
	free := cutIntervals(window, occupied)

	// 2) Pinned 但无 ScheduledStart/End 的 task:earliest-fit 顺序处理(TaskID asc),
	//    排进去之后视为锚点(不再移动)。
	pinnedUns := make([]Task, 0)
	remaining := make([]Task, 0)
	for _, tk := range tasks {
		hasScheduled := tk.ScheduledStart != nil && tk.ScheduledEnd != nil && *tk.ScheduledEnd > *tk.ScheduledStart
		if tk.Pinned && !hasScheduled {
			pinnedUns = append(pinnedUns, tk)
		} else if !hasScheduled {
			remaining = append(remaining, tk)
		}
	}
	sort.Slice(pinnedUns, func(i, j int) bool { return pinnedUns[i].ID < pinnedUns[j].ID })

	for _, tk := range pinnedUns {
		block := int64(clampBlock(tk.EstimatedMin)) * 60 // minutes → seconds
		start, end, ok, newFree := placeEarliest(block, free)
		if !ok {
			plan.Unscheduled = append(plan.Unscheduled, tk.ID)
			plan.Warnings = append(plan.Warnings,
				fmt.Sprintf("Pinned 任务 %d 时间不够,未能排入", tk.ID))
			continue
		}
		anchored = append(anchored, ledger{taskID: tk.ID, start: start, end: end, reason: "Pinned:earliest-fit"})
		free = newFree
	}

	// 3) 排序 remaining:sortKey desc,due_date asc(nil last),context asc,ID asc。
	sort.Slice(remaining, func(i, j int) bool {
		a, b := remaining[i], remaining[j]
		ka := computeSortKey(a, remaining)
		kb := computeSortKey(b, remaining)
		if ka != kb {
			return ka > kb // 降序
		}
		da, db := dueRank(a.DueDate), dueRank(b.DueDate)
		if da != db {
			return da < db // 早 due 在前
		}
		if a.Scores.Context != b.Scores.Context {
			return a.Scores.Context < b.Scores.Context // 升序:同 context 聚集
		}
		return a.ID < b.ID
	})

	// 4) 能量档位(绝对区间,跨窗口时由 cutIntervals 求交过滤)
	tiers := []interval{
		tierRange(base, 6, 11),  // early
		tierRange(base, 11, 18), // middle
		tierRange(base, 18, 23), // late
	}

	var greedy []ledger
	for _, tk := range remaining {
		block := int64(clampBlock(tk.EstimatedMin)) * 60 // minutes → seconds
		tierOrder := tierOrderFor(tk.Scores.Energy)
		start, end, ok, newFree := placeWithEnergy(block, free, tiers, tierOrder)
		if !ok {
			plan.Unscheduled = append(plan.Unscheduled, tk.ID)
			plan.Warnings = append(plan.Warnings,
				fmt.Sprintf("任务 %d 时间不够,未能排入", tk.ID))
			continue
		}
		greedy = append(greedy, ledger{
			taskID: tk.ID, start: start, end: end,
			reason: fmt.Sprintf("greedy:energy=%d 偏好档位 %d", tk.Scores.Energy, tierOrder[0]+1),
		})
		free = newFree
	}

	// 5) 拓扑修正(只对 greedy placements)
	fixed, cycle := topoFix(greedy, remaining)
	if cycle {
		plan.Warnings = append(plan.Warnings,
			fmt.Sprintf("任务循环依赖:未能完全拓扑修正,涉及 IDs %v", idsIn(fixed)))
	}
	greedy = fixed

	// 6) 合并 anchored + greedy,按 (ScheduledStart asc, TaskID asc) 排序输出
	all := append([]ledger(nil), anchored...)
	all = append(all, greedy...)
	sort.Slice(all, func(i, j int) bool {
		if all[i].start != all[j].start {
			return all[i].start < all[j].start
		}
		return all[i].taskID < all[j].taskID
	})
	plan.Placements = make([]Placement, 0, len(all))
	for _, l := range all {
		plan.Placements = append(plan.Placements, Placement{
			TaskID: l.taskID, ScheduledStart: l.start, ScheduledEnd: l.end, Reason: l.reason,
		})
	}

	// Unscheduled 排序保证确定性
	if plan.Unscheduled == nil {
		plan.Unscheduled = nil
	}
	sort.Slice(plan.Unscheduled, func(i, j int) bool { return plan.Unscheduled[i] < plan.Unscheduled[j] })
	return plan
}

// clampBlock 把 estimated_minutes clamp 到 [minBlockMinutes, maxBlockMinutes]。
//
// estimated_minutes 是软提示(见 spec §5.6),仅作块大小参考,不作硬目标;
// 这层 clamp 仅是保护性下限/上限,不是"必须耗时"。
func clampBlock(est int) int {
	if est <= 0 {
		return minBlockMinutes
	}
	if est < minBlockMinutes {
		return minBlockMinutes
	}
	if est > maxBlockMinutes {
		return maxBlockMinutes
	}
	return est
}

// dueRank 把 *string due_date 转成可比较的 int:
//
//	due == nil  → INT_MAX (排最后)
//	due 解析失败 → INT_MAX-1 (尽力,但放最后一批)
//	due 解析成功 → 当天 00:00 unix
func dueRank(due *string) int64 {
	if due == nil {
		return 1 << 62
	}
	t, err := time.ParseInLocation("2006-01-02", *due, time.Local)
	if err != nil {
		return (1 << 62) - 1
	}
	return t.Unix()
}

// dependencyBoost 计算 dependency_boost(spec §5.2 step 3):
//
//	本 task 是别人 blocker → 1.5
//	本 task 被别人 block  → 0.5
//	二者皆否            → 1.0
//	本 task 被 block 优先于本 task 是 blocker(避免双计数)。
func dependencyBoost(self Task, all []Task) float64 {
	blocked := len(self.BlockedBy) > 0
	if blocked {
		return 0.5
	}
	for _, other := range all {
		for _, b := range other.BlockedBy {
			if b == self.ID {
				return 1.5
			}
		}
	}
	return 1.0
}

// computeSortKey = urgency × priority × dependency_boost。
func computeSortKey(t Task, all []Task) float64 {
	return float64(t.Scores.Urgency) * float64(t.Scores.Priority) * dependencyBoost(t, all)
}

// tierOrderFor 返回某能量值的「tier 偏好顺序」。
// 档位下标:0=early(06-11),1=middle(11-18),2=late(18-23)。
func tierOrderFor(energy int) []int {
	switch energy {
	case 3: // 高精力 → 早
		return []int{0, 1, 2}
	case 1: // 低精力 → 晚
		return []int{2, 1, 0}
	default: // 2 或 0/未知 → 中
		return []int{1, 0, 2}
	}
}

// cutIntervals 从 window 减去所有 occupied,返回按 start 升序的剩余区间(可能为空)。
// 假定 occupied 与 window 时间已规范化(occupied ⊆ window 或可与 window 求交)。
func cutIntervals(window interval, occupied []interval) []interval {
	// 1) 规范化 occupied:裁到 window 内 + 按 start 排序 + 合并相邻
	norms := make([]interval, 0, len(occupied))
	for _, o := range occupied {
		s, e := o.start, o.end
		if e <= window.start || s >= window.end {
			continue
		}
		if s < window.start {
			s = window.start
		}
		if e > window.end {
			e = window.end
		}
		norms = append(norms, interval{start: s, end: e})
	}
	sort.Slice(norms, func(i, j int) bool { return norms[i].start < norms[j].start })
	merged := make([]interval, 0, len(norms))
	for _, n := range norms {
		if len(merged) == 0 || n.start > merged[len(merged)-1].end {
			merged = append(merged, n)
		} else if n.end > merged[len(merged)-1].end {
			merged[len(merged)-1].end = n.end
		}
	}

	// 2) 切割:从 window 头开始,跳过 merged 中的段
	free := make([]interval, 0)
	cur := window.start
	for _, m := range merged {
		if m.start > cur {
			free = append(free, interval{start: cur, end: m.start})
		}
		if m.end > cur {
			cur = m.end
		}
	}
	if cur < window.end {
		free = append(free, interval{start: cur, end: window.end})
	}
	return free
}

// placeEarliest 把 block 放在 free 的最早可容纳位置(first-fit)。
// 返回放置后的 (start, end, ok, newFree)。
func placeEarliest(block int64, free []interval) (int64, int64, bool, []interval) {
	for i, iv := range free {
		if iv.start+block <= iv.end {
			return placeInto(free, i, iv.start, block)
		}
	}
	return 0, 0, false, free
}

// placeWithEnergy 按 tierOrder 依次尝试:每个 tier 内找 candidateStart
// (max(iv.start, tier.start) 且能装下 block) 最小的 free 区间放置。
func placeWithEnergy(block int64, free []interval, tiers []interval, tierOrder []int) (int64, int64, bool, []interval) {
	for _, ti := range tierOrder {
		tier := tiers[ti]
		bestIdx := -1
		var bestStart int64
		for i, iv := range free {
			s := iv.start
			if tier.start > s {
				s = tier.start
			}
			if s >= tier.end { // 不与 tier 重叠
				continue
			}
			if s+block > iv.end { // 装不下
				continue
			}
			if bestIdx == -1 || s < bestStart {
				bestIdx = i
				bestStart = s
			}
		}
		if bestIdx >= 0 {
			return placeInto(free, bestIdx, bestStart, block)
		}
	}
	return 0, 0, false, free
}

// placeInto 在 free[idx] 内从 at 占用 block 分钟,返回 (at, at+block, true, 新 free)。
func placeInto(free []interval, idx int, at, block int64) (int64, int64, bool, []interval) {
	iv := free[idx]
	out := make([]interval, 0, len(free))
	out = append(out, free[:idx]...)
	if at > iv.start {
		out = append(out, interval{start: iv.start, end: at})
	}
	end := at + block
	if end < iv.end {
		out = append(out, interval{start: end, end: iv.end})
	}
	out = append(out, free[idx+1:]...)
	sort.Slice(out, func(i, j int) bool { return out[i].start < out[j].start })
	return at, end, true, out
}

// topoFix 对 greedy placements 做一次拓扑修正;时间槽按拓扑序重排(在 greedy 内部)。
// 返回 (fixed, cycle)。cycle=true 表示原样返回 + 调用方需添加 warning。
//
// 算法:
//  1. 在 greedy + remaining 中建立索引;若依赖目标不在 greedy 内则忽略(锚点无法被重排)
//  2. Kahn 算法;每步从入度 0 的节点里按索引(时间)升序 pop,保证确定性
//  3. 若所有节点都被消费 → 用拓扑序重排时间槽(槽集合不变,槽与任务重映射)
//  4. 否则 → 环,返回 (原序, true)
func topoFix(greedy []ledger, remaining []Task) ([]ledger, bool) {
	if len(greedy) <= 1 {
		return greedy, false
	}
	// 关键:greedy 的 append 顺序不一定是时间序(先摆 energy=1 拿到 18:00、
	// 再摆 energy=3 拿到 06:00),所有图节点/下标都以「时间序快照」为准。
	timeOrdered := make([]ledger, len(greedy))
	copy(timeOrdered, greedy)
	sort.Slice(timeOrdered, func(i, j int) bool { return timeOrdered[i].start < timeOrdered[j].start })

	idx := make(map[int64]int, len(timeOrdered))
	for i, g := range timeOrdered {
		idx[g.taskID] = i
	}
	indeg := make([]int, len(timeOrdered))
	adj := make([][]int, len(timeOrdered))
	for _, tk := range remaining {
		j, ok := idx[tk.ID]
		if !ok {
			continue // 不在 greedy(已锚点或未排入)
		}
		for _, b := range tk.BlockedBy {
			i, ok := idx[b]
			if !ok {
				continue // blocker 不在 greedy,跳过
			}
			adj[i] = append(adj[i], j) // i (blocker) → j (blocked)
			indeg[j]++
		}
	}

	// 拓扑序(每次从入度 0 中按时间索引升序选一个,确定性)。
	// 无依赖时 order 即 [0,1,2,...] → 时间槽与任务一一对应,原样保留。
	var order []int
	queue := make([]int, 0, len(timeOrdered))
	for i := range indeg {
		if indeg[i] == 0 {
			queue = append(queue, i)
		}
	}
	for len(queue) > 0 {
		mi := 0
		for k := 1; k < len(queue); k++ {
			if queue[k] < queue[mi] {
				mi = k
			}
		}
		n := queue[mi]
		queue = append(queue[:mi], queue[mi+1:]...)
		order = append(order, n)
		for _, m := range adj[n] {
			indeg[m]--
			if indeg[m] == 0 {
				queue = append(queue, m)
			}
		}
	}
	if len(order) != len(timeOrdered) {
		return greedy, true // 环
	}

	// order[k] = 应拿到第 k 早时间槽的 task 在 timeOrdered 中的下标。
	// 槽位不变(仍按时间升序),只是任务与槽位重新配对。
	out := make([]ledger, len(timeOrdered))
	for k, ti := range order {
		out[k] = ledger{
			taskID: timeOrdered[ti].taskID,
			start:  timeOrdered[k].start,
			end:    timeOrdered[k].end,
			reason: timeOrdered[ti].reason + "(拓扑修正)",
		}
	}
	return out, false
}

// idsIn 返回 ledger 列表中的 task id 列表(用于 warning)。
func idsIn(ls []ledger) []int64 {
	out := make([]int64, 0, len(ls))
	for _, l := range ls {
		out = append(out, l.taskID)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}
