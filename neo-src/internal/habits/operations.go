// Package habits 是 habit 域的用例层。
//
// 设计（见 .omo/plans/habit-vault-extraction.md）：
//   - HTTP handler 与 Wails service 都只做 transport 绑定和错误映射；
//   - 业务规则（name 校验、默认值、唯一性、日期默认、进度计算）集中在本包；
//   - Adapter 不允许直接 import internal/storage 子包。
//
// Phase 2：含 name 校验、默认值、唯一性检查、日期默认、进度计算。
// 11 个纯 pass-through 方法（List*/Delete*/GetHabitStats/TimerSession*）仍由
// adapter 直连 storage；Wave 3（Phase 2b）把它们的入口也搬到本包。
package habits

import (
	"strings"

	"little-timer/internal/domain"
	"little-timer/internal/settings"
	"little-timer/internal/storage"
)

// Error 是 habit 域的类型化错误。Adapter 用 errors.As 把它映射成 transport
// 响应；Code 是稳定的、前端可依赖的字符串。
type Error struct {
	Code    string
	Message string
}

func (e *Error) Error() string { return e.Message }

// 哨兵错误。规则方法返回这些。
var (
	ErrNameRequired = &Error{Code: "habit.name_required", Message: "missing name"}
	ErrNameExists   = &Error{Code: "habit.name_exists", Message: "habit name already exists in this set"}
	ErrNotFound     = &Error{Code: "habit.not_found", Message: "habit not found"}
	ErrSetNotFound  = &Error{Code: "habit.set_not_found", Message: "habit set not found"}
)

// 默认值。Phase 2 规则方法用它们。
const (
	DefaultColor       = domain.DefaultColor
	DefaultGoalSeconds = domain.DefaultGoalSeconds
)

// ---- 输入 / 输出类型 ----

// CreateHabitInput 是 CreateHabit 的入参。
type CreateHabitInput struct {
	SetID       int64
	Name        string
	GoalSeconds int64
	Color       string
}

// UpdateHabitInput 是 UpdateHabit 的入参。
type UpdateHabitInput struct {
	Name        string
	GoalSeconds int64
	Color       string
	Wallpaper   string
}

// CreateHabitSetInput 是 CreateHabitSet 的入参。
type CreateHabitSetInput struct {
	Name        string
	Description string
	Color       string
}

// UpdateHabitSetInput 是 UpdateHabitSet 的入参。
type UpdateHabitSetInput struct {
	Name        string
	Description string
	Color       string
	Wallpaper   string
}

// CreateSessionInput 是 CreateSession 的入参。Date / Count 留空走默认。
type CreateSessionInput struct {
	HabitID         int64
	DurationSeconds int64
	Count           int64
	Date            string
}

// HabitResult 是 Create/Update Habit 的返回。
type HabitResult struct {
	ID          int64
	SetID       int64
	Name        string
	GoalSeconds int64
	Color       string
	Wallpaper   string
}

// HabitSetResult 是 Create/Update HabitSet 的返回。
type HabitSetResult struct {
	ID          int64
	Name        string
	Description string
	Color       string
	Wallpaper   string
}

// HabitDetail 是 GetHabitDetail 的返回：基础字段 + 当天累计 + 进度百分比。
type HabitDetail struct {
	ID              int64
	Name            string
	GoalSeconds     int64
	Color           string
	TodaySeconds    int64
	ProgressPercent int64
}

// Operations 是 habit 域的用例层。
type Operations struct {
	sqlite   *storage.SqliteManager
	settings *settings.SettingsManager
}

// New 构造 Operations。
func New(sqlite *storage.SqliteManager, settings *settings.SettingsManager) *Operations {
	return &Operations{sqlite: sqlite, settings: settings}
}

// ---- HabitSet ----

// CreateHabitSet 校验 name 必填，color 缺省走 DefaultColor，建行。
func (o *Operations) CreateHabitSet(input CreateHabitSetInput) (HabitSetResult, error) {
	if strings.TrimSpace(input.Name) == "" {
		return HabitSetResult{}, ErrNameRequired
	}
	color := input.Color
	if color == "" {
		color = DefaultColor
	}
	id, err := o.sqlite.HabitSets().Create(input.Name, input.Description, color)
	if err != nil {
		return HabitSetResult{}, err
	}
	return HabitSetResult{
		ID:          id,
		Name:        input.Name,
		Description: input.Description,
		Color:       color,
	}, nil
}

// ListHabitSets 转发到 storage。
func (o *Operations) ListHabitSets(limit, offset int) ([]storage.HabitSetRow, error) {
	return o.sqlite.HabitSets().List(limit, offset)
}

// UpdateHabitSet 校验 name 必填，color 缺省走 DefaultColor，写行。
func (o *Operations) UpdateHabitSet(id int64, input UpdateHabitSetInput) (HabitSetResult, error) {
	if strings.TrimSpace(input.Name) == "" {
		return HabitSetResult{}, ErrNameRequired
	}
	color := input.Color
	if color == "" {
		color = DefaultColor
	}
	if err := o.sqlite.HabitSets().Update(id, input.Name, input.Description, color, input.Wallpaper); err != nil {
		return HabitSetResult{}, err
	}
	return HabitSetResult{
		ID:          id,
		Name:        input.Name,
		Description: input.Description,
		Color:       color,
		Wallpaper:   input.Wallpaper,
	}, nil
}

// DeleteHabitSet 转发到 storage。
func (o *Operations) DeleteHabitSet(id int64) error {
	return o.sqlite.HabitSets().Delete(id)
}

// ---- Habit ----

// CreateHabit 校验 name 必填、应用默认值、检查 set 内唯一性，建行。
func (o *Operations) CreateHabit(input CreateHabitInput) (HabitResult, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return HabitResult{}, ErrNameRequired
	}
	goal := input.GoalSeconds
	if goal == 0 {
		goal = DefaultGoalSeconds
	}
	color := input.Color
	if color == "" {
		color = DefaultColor
	}
	exists, err := o.sqlite.Habits().NameExistsInSet(input.SetID, name, nil)
	if err != nil {
		return HabitResult{}, err
	}
	if exists {
		return HabitResult{}, ErrNameExists
	}
	id, err := o.sqlite.Habits().Create(input.SetID, name, goal, color)
	if err != nil {
		return HabitResult{}, err
	}
	return HabitResult{
		ID:          id,
		SetID:       input.SetID,
		Name:        name,
		GoalSeconds: goal,
		Color:       color,
	}, nil
}

// ListHabits：setID 非 nil 时按 set 过滤。
func (o *Operations) ListHabits(setID *int64, limit, offset int) ([]storage.HabitRow, error) {
	if setID != nil {
		return o.sqlite.Habits().ListBySet(*setID, limit, offset)
	}
	return o.sqlite.Habits().List(limit, offset)
}

// UpdateHabit 校验 name 必填、应用默认值、检查 set 内唯一性（排除自己），写行。
//
// 取现有行（确认存在）；唯一性查询带 excludeID=&id，所以"改自己同名"不会触发
// ErrNameExists。
func (o *Operations) UpdateHabit(id int64, input UpdateHabitInput) (HabitResult, error) {
	if strings.TrimSpace(input.Name) == "" {
		return HabitResult{}, ErrNameRequired
	}
	goal := input.GoalSeconds
	if goal == 0 {
		goal = DefaultGoalSeconds
	}
	color := input.Color
	if color == "" {
		color = DefaultColor
	}
	existing, err := o.sqlite.Habits().GetByID(id)
	if err != nil {
		return HabitResult{}, ErrNotFound
	}
	exists, err := o.sqlite.Habits().NameExistsInSet(existing.SetID, input.Name, &id)
	if err != nil {
		return HabitResult{}, err
	}
	if exists {
		return HabitResult{}, ErrNameExists
	}
	if err := o.sqlite.Habits().Update(id, input.Name, goal, color, input.Wallpaper); err != nil {
		return HabitResult{}, err
	}
	return HabitResult{
		ID:          id,
		SetID:       existing.SetID,
		Name:        input.Name,
		GoalSeconds: goal,
		Color:       color,
		Wallpaper:   input.Wallpaper,
	}, nil
}

// DeleteHabit 转发到 storage。
func (o *Operations) DeleteHabit(id int64) error {
	return o.sqlite.Habits().Delete(id)
}

// GetHabitDetail 取 habit 行 + 当天累计 + 进度百分比。date 为空时按配置时区取今天。
func (o *Operations) GetHabitDetail(id int64, date string) (HabitDetail, error) {
	if date == "" {
		date = domain.TodayString(o.settings.Config().Basic.Timezone)
	}
	row, err := o.sqlite.Habits().GetByID(id)
	if err != nil {
		return HabitDetail{}, err
	}
	todaySeconds, _ := o.sqlite.Timers().TodaySecondsForHabit(id, date)
	var progress int64
	if row.GoalSeconds > 0 {
		progress = (todaySeconds * 100) / row.GoalSeconds
	}
	return HabitDetail{
		ID:              row.ID,
		Name:            row.Name,
		GoalSeconds:     row.GoalSeconds,
		Color:           row.Color,
		TodaySeconds:    todaySeconds,
		ProgressPercent: progress,
	}, nil
}

// GetHabit 按 ID 取 habit 行。HabitStats handler 用它做存在性检查。
func (o *Operations) GetHabit(id int64) (storage.HabitRow, error) {
	return o.sqlite.Habits().GetByID(id)
}

// GetHabitStats 转发到 storage。
func (o *Operations) GetHabitStats(id int64) (map[string]interface{}, error) {
	return o.sqlite.Timers().GetHabitStats(id)
}

// ---- Session ----

// CreateSession：date 缺省 = 今天，count 缺省 = 1。
func (o *Operations) CreateSession(input CreateSessionInput) (int64, error) {
	date := input.Date
	if date == "" {
		date = domain.TodayString(o.settings.Config().Basic.Timezone)
	}
	count := input.Count
	if count == 0 {
		count = 1
	}
	return o.sqlite.Timers().CreateSession(input.HabitID, input.DurationSeconds, count, date)
}

// ListSessions 支持三种查询形态：日期区间、单日、或（默认）今天。
func (o *Operations) ListSessions(date, startDate, endDate string, limit, offset int) ([]storage.SessionRow, error) {
	switch {
	case startDate != "" && endDate != "":
		return o.sqlite.Timers().ListSessionsByDateRange(startDate, endDate, limit, offset)
	case date != "":
		return o.sqlite.Timers().ListSessionsByDate(date, limit, offset)
	default:
		return o.sqlite.Timers().ListSessionsByDate(domain.TodayString(o.settings.Config().Basic.Timezone), limit, offset)
	}
}

// DeleteSession 转发到 storage。
func (o *Operations) DeleteSession(id int64) error {
	return o.sqlite.Timers().DeleteSession(id)
}

// ---- TimerSession（Phase 3 才会整体迁移到本包）----

// CreateTimerSession 转发到 storage。
func (o *Operations) CreateTimerSession(habitID *int64, mode string, workDuration, restDuration, loopCount int64) (int64, error) {
	return o.sqlite.Timers().CreateTimerSession(habitID, mode, workDuration, restDuration, loopCount)
}

// GetTimerSession：id 非 nil 时按 id 取，否则取当前活动 session。
func (o *Operations) GetTimerSession(id *int64) (storage.TimerSessionRow, error) {
	if id != nil {
		return o.sqlite.Timers().GetTimerSessionByID(*id)
	}
	return o.sqlite.Timers().GetActiveTimerSession()
}

// UpdateTimerSession 转发到 storage。
func (o *Operations) UpdateTimerSession(
	id int64,
	elapsedSeconds int64,
	remainingSeconds *int64,
	pausedTotalSeconds int64,
	pauseStartedAt *int64,
	lastSyncedAt *int64,
	isRunning, isPaused, isFinished bool,
	currentRound int64,
	inRest bool,
) error {
	return o.sqlite.Timers().UpdateTimerSession(
		id, elapsedSeconds, remainingSeconds,
		pausedTotalSeconds, pauseStartedAt, lastSyncedAt,
		isRunning, isPaused, isFinished,
		currentRound, inRest,
	)
}

// DeleteTimerSession 转发到 storage。
func (o *Operations) DeleteTimerSession(id int64) error {
	return o.sqlite.Timers().DeleteTimerSession(id)
}
