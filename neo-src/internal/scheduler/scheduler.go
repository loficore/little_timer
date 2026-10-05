// Package scheduler 是确定性规则调度引擎(纯函数,不接触 DB / HTTP / 时钟)。
//
// 设计边界(参 spec §2「关键边界」):
//   - 只依赖 Go 标准库;不 import internal/domain,不 import net/http 或 storage
//   - 无全局可变状态、无 goroutine;相同输入必然产出相同 Plan(可重现)
//   - LLM 只负责「打分」,本包只负责「摆位」
//
// 输入是某一天的所有 task 与当天已过滤好的 habit slots,输出 Plan。
// 调用方负责按 date 的 DayOfWeek 预过滤 habits;本包不按 DayOfWeek 过滤。
package scheduler

// 默认作息窗口(小时)。Options 中为零值时回落到这些常量。
const (
	// DefaultDayStartHour 是一天的默认开始小时(06:00)。
	DefaultDayStartHour = 6
	// DefaultDayEndHour 是一天的默认结束小时(23:00)。
	DefaultDayEndHour = 23

	// minBlockMinutes / maxBlockMinutes 是时间块的 clamp 上下界。
	minBlockMinutes = 15
	maxBlockMinutes = 180
)

// Score 是 LLM / 用户给出的四维评分。
type Score struct {
	Priority int    // 1-5(重要)
	Urgency  int    // 1-10(紧急)
	Energy   int    // 1-3(1=低,2=中,3=高)
	Context  string // 上下文标签(写作/编码/沟通/学习/杂事/general)
}

// Task 是调度输入的单条任务。
type Task struct {
	ID             int64
	Title          string
	EstimatedMin   int     // 软提示,见 src spec §5.6
	Pinned         bool    // 用户显式钉住
	DueDate        *string // YYYY-MM-DD,可为 nil
	ScheduledStart *int64  // 已排(非 nil 表示已有固定时间)
	ScheduledEnd   *int64
	Scores         Score
	BlockedBy      []int64 // 拓扑依赖:这些 task 完成后本 task 才能开始
}

// HabitSlot 是某天的一个习惯时段(分钟表示,0-1440)。
//
// Preference 语义(参 spec §3.2):
//   - 1 = 硬约束:Scheduler 必须避开,当作 obstacle
//   - 2 = 软约束:尽量满足,冲突时 task 可压在区间上
//   - 3 = 仅参考:不参与调度
type HabitSlot struct {
	HabitID    int64
	DayOfWeek  int // 0=Sun;调用方负责按当天过滤,本包不判断
	StartMin   int
	EndMin     int
	Preference int
}

// Placement 是计划中一个被摆位的 task。
type Placement struct {
	TaskID         int64
	ScheduledStart int64  // unix 秒
	ScheduledEnd   int64  // unix 秒
	Reason         string // debug:为什么排到这个时段
}

// Plan 是一次调度的输出。
//
// 确定性排序约定(controller 决议):Placements 按 ScheduledStart 升序,
// 相同 start 时按 TaskID 升序。Unscheduled 按 TaskID 升序。
// Warnings 按产生顺序稳定追加。
type Plan struct {
	Date        string      // YYYY-MM-DD
	Placements  []Placement // 见上方排序约定
	Unscheduled []int64     // 排不下的 task id(冲突或时间不够)
	Warnings    []string
}

// Options 是调度参数。为零值时回落到默认值。
type Options struct {
	DayStartHour int // 默认 6(DefaultDayStartHour)
	DayEndHour   int // 默认 23(DefaultDayEndHour)
}

// Schedule 是核心入口(纯函数)。
//
// 输入:某天要排的所有 task + 当天 habit slots + 日期 + 选项;
// 输出:plan。Options 的零值会先被填充为默认作息(模拟 spec §5.2 默认)。
func Schedule(tasks []Task, habits []HabitSlot, date string, opts Options) Plan {
	if opts.DayStartHour == 0 {
		opts.DayStartHour = DefaultDayStartHour
	}
	if opts.DayEndHour == 0 {
		opts.DayEndHour = DefaultDayEndHour
	}
	return scheduleImpl(tasks, habits, date, opts)
}
