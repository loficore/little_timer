// Package app —— Wails v3 service 绑定。
//
// 本文件承载 Wails v3 Android 前端直接调用的四个 service。Wails v3 通过
// 反射绑定导出方法 —— 不需要注解 —— 所以这里的方法名和签名必须与
// `cmd/server/assets/bindings/little-timer/internal/app/wailsbindings.ts`
// 中生成的 WailsBindings 匹配（由 `assets/src/utils/wailsApiClient.ts` 驱动）。
//
// 每个 service 都是现有 *App 辅助函数的薄包装，外加少量与 HTTP handler
// 等价的逻辑，用于还没有直接 *App 方法的 endpoint。这里不放新业务逻辑。
//
// 设计：没有 //wails: 注解、没有工厂、没有 interface —— 每个 service 持有
// *App 并转发。Wails 反射层只需要公开类型上存在导出方法即可。
package app

import (
	"context"
	"errors"
	"time"

	"little-timer/internal/domain"
	"little-timer/internal/habits"
	"little-timer/internal/log"
)

// modeKey 把当前 clock 模式渲染成 JS 客户端期望的稳定字符串。
func modeKey(m domain.ModeEnum) string {
	if m == domain.CountdownMode {
		return "countdown"
	}
	return "stopwatch"
}

// TimerService

// TimerService 向 Wails v3 前端暴露 timer 状态 + 控制方法。所有导出方法
// 都经反射绑定。
type TimerService struct {
	app *App
}

// NewTimerService 构造绑定到给定 App 的 TimerService。
func NewTimerService(app *App) *TimerService { return &TimerService{app: app} }

func (s *TimerService) GetState() (any, error) {
	log.Debug("timer.get_state", "method", "TimerService.GetState")
	a := s.app
	state := a.Clock.Update()
	tz := a.Settings.Config().Basic.Timezone

	a.RLock()
	habitID := a.CurrentHabitID
	a.RUnlock()

	return map[string]any{
		"time":           state.GetTimeInfo(),
		"elapsed":        state.GetElapsedSeconds(),
		"mode":           modeKey(state.GetMode()),
		"is_running":     !state.IsPaused(),
		"is_finished":    state.IsFinished(),
		"in_rest":        state.InRest(),
		"loop_remaining": state.GetLoopRemaining(),
		"loop_total":     state.GetLoopTotal(),
		"rest_remaining": state.GetRestRemainingTime(),
		"timezone":       tz,
		"habit_id":       habitID,
	}, nil
}

// StartTimer 的参数可选；零值回退到默认（stopwatch、25 分钟工作）。
func (s *TimerService) StartTimer(habitID *int64, mode string, workDuration int64, restDuration int64, loopCount int64) (any, error) {
	log.Debug("timer.start", "method", "TimerService.StartTimer", "habit_id", habitID, "mode", mode)
	a := s.app

	m := "stopwatch"
	if mode == "countdown" {
		m = "countdown"
	}
	work := workDuration
	if work == 0 {
		work = 25 * 60
	}

	a.Lock()
	defer a.Unlock()

	// 已在运行的分支：保持同一 session，报告当前 habit id。
	if a.CurrentTimerSessionID != nil {
		row, err := a.SQLite.Timers().GetTimerSessionByID(*a.CurrentTimerSessionID)
		if err == nil {
			if row.IsRunning && !row.IsFinished && !row.IsPaused {
				a.CurrentHabitID = habitID
				if a.CurrentHabitID == nil && row.HabitID != nil {
					h := *row.HabitID
					a.CurrentHabitID = &h
				}
				return map[string]any{
					"status":     "already_running",
					"habit_id":   a.CurrentHabitID,
					"session_id": *a.CurrentTimerSessionID,
				}, nil
			}
			// 暂停分支 —— 恢复。
			cs := a.Clock.Update()
			if (cs.IsPaused() && !cs.IsFinished()) || row.IsPaused {
				pausedTotal := row.PausedTotalSeconds
				now := time.Now().Unix()
				if row.PauseStartedAt != nil && now > *row.PauseStartedAt {
					pausedTotal += now - *row.PauseStartedAt
				}
				a.Clock.HandleEvent(domain.UserStartTimerEvent{})
				_ = a.SQLite.Timers().UpdateTimerSession(
					row.ID, row.ElapsedSeconds, row.RemainingSeconds,
					pausedTotal, nil, &now,
					true, false, false,
					row.CurrentRound, row.InRest,
				)
				a.CurrentHabitID = habitID
				if a.CurrentHabitID == nil && row.HabitID != nil {
					h := *row.HabitID
					a.CurrentHabitID = &h
				}
				return map[string]any{
					"status":     "started",
					"habit_id":   a.CurrentHabitID,
					"session_id": row.ID,
				}, nil
			}
		}
		// 过期 session —— 清理。
		a.ResetTimerSession()
	}

	sessionID, err := a.CreateTimerSession(habitID, m, work, restDuration, loopCount)
	if err != nil {
		log.Error("timer.start failed", "method", "TimerService.StartTimer", "error", err.Error())
		return nil, err
	}
	a.CurrentHabitID = habitID
	a.Clock.HandleEvent(domain.UserStartTimerEvent{})
	log.Info("timer.start ok", "method", "TimerService.StartTimer", "session_id", sessionID, "habit_id", habitID, "mode", m)
	return map[string]any{
		"status":     "started",
		"habit_id":   habitID,
		"session_id": sessionID,
	}, nil
}

// FinishTimer 冻结 clock，并为当前 habit 写入一条每日 session 行。
func (s *TimerService) FinishTimer() (any, error) {
	log.Debug("timer.finish", "method", "TimerService.FinishTimer")
	a := s.app
	a.Lock()
	defer a.Unlock()

	habitID := a.CurrentHabitID
	sessionID := a.CurrentTimerSessionID

	elapsed, err := a.FinishTimerSession()
	if err != nil {
		log.Error("timer.finish failed", "method", "TimerService.FinishTimer", "error", err.Error())
		// 回退路径：发 user_finish_timer，从 clock 状态算 elapsed，
		// 若之前有 habit 则持久化一条每日 session。
		a.Clock.HandleEvent(domain.UserFinishTimerEvent{})
		state := a.Clock.Update()
		elapsedSeconds := state.GetElapsedSeconds()
		if habitID != nil && elapsedSeconds > 0 {
			_, _ = a.SQLite.Timers().CreateSession(*habitID, elapsedSeconds, 1, domain.TodayString(a.Settings.Config().Basic.Timezone))
		}
		a.ResetTimerSession()
		return map[string]any{
			"status":          "finished",
			"elapsed_seconds": elapsedSeconds,
		}, nil
	}

	if habitID != nil && elapsed > 0 {
		_, _ = a.SQLite.Timers().CreateSession(*habitID, elapsed, 1, domain.TodayString(a.Settings.Config().Basic.Timezone))
	}
	a.ResetTimerSession()

	log.Info("timer.finish ok", "method", "TimerService.FinishTimer", "elapsed_seconds", elapsed, "session_id", sessionID)
	return map[string]any{
		"status":          "finished",
		"elapsed_seconds": elapsed,
		"session_id":      sessionID,
	}, nil
}

// GetProgress 返回实时进度、模式与 paused/finished 标志；无活动 session
// 时惰性加载持久化 session。
func (s *TimerService) GetProgress() (any, error) {
	log.Debug("timer.get_progress", "method", "TimerService.GetProgress")
	a := s.app

	a.RLock()
	hasSession := a.CurrentTimerSessionID != nil
	a.RUnlock()
	if !hasSession {
		a.LoadTimerProgress()
	}

	a.RLock()
	sessionID := a.CurrentTimerSessionID
	habitID := a.CurrentHabitID
	a.RUnlock()

	state := a.Clock.Update()
	return map[string]any{
		"session_id":        sessionID,
		"habit_id":          habitID,
		"mode":              modeKey(state.GetMode()),
		"is_running":        !state.IsPaused(),
		"is_paused":         state.IsPaused(),
		"is_finished":       state.IsFinished(),
		"elapsed_seconds":   state.GetElapsedSeconds(),
		"remaining_seconds": state.GetRemainingSeconds(),
		"in_rest":           state.InRest(),
	}, nil
}

// StartRest 把 clock 切到 5 分钟倒计时并启动。
func (s *TimerService) StartRest() (any, error) {
	log.Debug("timer.start_rest", "method", "TimerService.StartRest")
	a := s.app
	const restSeconds uint64 = 5 * 60

	a.Clock.HandleEvent(domain.UserChangeConfigEvent{
		Config: domain.ClockTaskConfig{
			DefaultMode: domain.CountdownMode,
			Countdown: domain.CountdownConfig{
				DurationSeconds:     restSeconds,
				Loop:                false,
				LoopCount:           0,
				LoopIntervalSeconds: 0,
			},
			Stopwatch: domain.StopwatchConfig{
				MaxSeconds: 24 * 3600,
			},
		},
	})
	a.Clock.HandleEvent(domain.UserStartTimerEvent{})
	return map[string]any{
		"status":       "rest_started",
		"rest_seconds": restSeconds,
	}, nil
}

func (s *TimerService) PauseTimer() (any, error) {
	log.Debug("timer.pause", "method", "TimerService.PauseTimer")
	a := s.app
	a.Lock()
	defer a.Unlock()
	a.Clock.HandleEvent(domain.UserPauseTimerEvent{})
	a.SaveProgressLocked()
	log.Info("timer.pause ok", "method", "TimerService.PauseTimer")
	return map[string]any{"status": "paused"}, nil
}

func (s *TimerService) ResetTimer() (any, error) {
	log.Debug("timer.reset", "method", "TimerService.ResetTimer")
	a := s.app
	a.Lock()
	defer a.Unlock()
	a.ResetTimerSession()
	a.CurrentHabitID = nil
	a.Clock.HandleEvent(domain.UserResetTimerEvent{})
	return map[string]any{"status": "reset"}, nil
}

// HabitService

// truncStr 把 s 截断到 64 字符，避免刷屏日志。
func truncStr(s string) string {
	if len(s) > 64 {
		return s[:64]
	}
	return s
}

// HabitService 向 Wails v3 前端暴露 habit CRUD + session 查询。
type HabitService struct {
	app *App
}

// NewHabitService 构造绑定到给定 App 的 HabitService。
func NewHabitService(app *App) *HabitService { return &HabitService{app: app} }

func (s *HabitService) ListHabitSets() (any, error) {
	log.Debug("habit.list_sets", "method", "HabitService.ListHabitSets")
	rows, err := s.app.Habits.ListHabitSets(100, 0)
	if err != nil {
		log.Error("habit.list_sets failed", "method", "HabitService.ListHabitSets", "error", err.Error())
		return nil, err
	}
	return rows, nil
}

func (s *HabitService) CreateHabitSet(name string, description string, color string) (any, error) {
	log.Debug("habit.create_set", "method", "HabitService.CreateHabitSet", "name", truncStr(name), "color", color)
	res, err := s.app.Habits.CreateHabitSet(habits.CreateHabitSetInput{
		Name: name, Description: description, Color: color,
	})
	if err != nil {
		var he *habits.Error
		if errors.As(err, &he) {
			log.Warn("habit.create_set rejected", "method", "HabitService.CreateHabitSet", "code", he.Code, "error", he.Message)
			return map[string]any{"success": false, "code": he.Code, "error": he.Message}, nil
		}
		log.Error("habit.create_set failed", "method", "HabitService.CreateHabitSet", "name", truncStr(name), "error", err.Error())
		return nil, err
	}
	log.Info("habit.create_set ok", "method", "HabitService.CreateHabitSet", "id", res.ID)
	return map[string]any{
		"id":          res.ID,
		"name":        res.Name,
		"description": res.Description,
		"color":       res.Color,
	}, nil
}

func (s *HabitService) UpdateHabitSet(id int64, name string, description string, color string, wallpaper string) (any, error) {
	log.Debug("habit.update_set", "method", "HabitService.UpdateHabitSet", "id", id, "name", truncStr(name), "color", color)
	res, err := s.app.Habits.UpdateHabitSet(id, habits.UpdateHabitSetInput{
		Name: name, Description: description, Color: color, Wallpaper: wallpaper,
	})
	if err != nil {
		var he *habits.Error
		if errors.As(err, &he) {
			log.Warn("habit.update_set rejected", "method", "HabitService.UpdateHabitSet", "id", id, "code", he.Code, "error", he.Message)
			return map[string]any{"success": false, "code": he.Code, "error": he.Message}, nil
		}
		log.Error("habit.update_set failed", "method", "HabitService.UpdateHabitSet", "id", id, "error", err.Error())
		return nil, err
	}
	log.Info("habit.update_set ok", "method", "HabitService.UpdateHabitSet", "id", id)
	return map[string]any{
		"id":          res.ID,
		"name":        res.Name,
		"description": res.Description,
		"color":       res.Color,
		"wallpaper":   res.Wallpaper,
	}, nil
}

func (s *HabitService) DeleteHabitSet(id int64) (any, error) {
	log.Debug("habit.delete_set", "method", "HabitService.DeleteHabitSet", "id", id)
	if err := s.app.Habits.DeleteHabitSet(id); err != nil {
		log.Error("habit.delete_set failed", "method", "HabitService.DeleteHabitSet", "id", id, "error", err.Error())
		return nil, err
	}
	log.Info("habit.delete_set ok", "method", "HabitService.DeleteHabitSet", "id", id)
	return map[string]any{"success": true}, nil
}

// ListHabits 在 setID 为 nil 时返回所有 habit，否则限定在请求的 set 内。
func (s *HabitService) ListHabits(setID *int64) (any, error) {
	log.Debug("habit.list", "method", "HabitService.ListHabits", "set_id", setID)
	const limit, offset = 100, 0
	rows, err := s.app.Habits.ListHabits(setID, limit, offset)
	if err != nil {
		log.Error("habit.list failed", "method", "HabitService.ListHabits", "error", err.Error())
		return nil, err
	}
	return rows, nil
}

func (s *HabitService) CreateHabit(setID int64, name string, goalSeconds int64, color string) (any, error) {
	a := s.app
	log.Debug("habit.create", "method", "HabitService.CreateHabit", "set_id", setID, "name", truncStr(name), "goal_seconds", goalSeconds)
	res, err := a.Habits.CreateHabit(habits.CreateHabitInput{
		SetID: setID, Name: name, GoalSeconds: goalSeconds, Color: color,
	})
	if err != nil {
		var he *habits.Error
		if errors.As(err, &he) {
			log.Warn("habit.create rejected", "method", "HabitService.CreateHabit", "code", he.Code, "error", he.Message)
			return map[string]any{"success": false, "code": he.Code, "error": he.Message}, nil
		}
		log.Error("habit.create failed", "method", "HabitService.CreateHabit", "name", truncStr(name), "error", err.Error())
		return nil, err
	}
	log.Info("habit.create ok", "method", "HabitService.CreateHabit", "id", res.ID)
	return map[string]any{
		"id":           res.ID,
		"set_id":       res.SetID,
		"name":         res.Name,
		"goal_seconds": res.GoalSeconds,
		"color":        res.Color,
	}, nil
}

func (s *HabitService) UpdateHabit(id int64, name string, goalSeconds int64, color string, wallpaper string) (any, error) {
	a := s.app
	log.Debug("habit.update", "method", "HabitService.UpdateHabit", "id", id, "name", truncStr(name), "goal_seconds", goalSeconds, "color", color)
	res, err := a.Habits.UpdateHabit(id, habits.UpdateHabitInput{
		Name: name, GoalSeconds: goalSeconds, Color: color, Wallpaper: wallpaper,
	})
	if err != nil {
		var he *habits.Error
		if errors.As(err, &he) {
			log.Warn("habit.update rejected", "method", "HabitService.UpdateHabit", "id", id, "code", he.Code, "error", he.Message)
			return map[string]any{"success": false, "code": he.Code, "error": he.Message}, nil
		}
		log.Error("habit.update failed", "method", "HabitService.UpdateHabit", "id", id, "error", err.Error())
		return nil, err
	}
	log.Info("habit.update ok", "method", "HabitService.UpdateHabit", "id", id)
	return map[string]any{
		"id":           res.ID,
		"name":         res.Name,
		"goal_seconds": res.GoalSeconds,
		"color":        res.Color,
		"wallpaper":    res.Wallpaper,
	}, nil
}

func (s *HabitService) DeleteHabit(id int64) (any, error) {
	log.Debug("habit.delete", "method", "HabitService.DeleteHabit", "id", id)
	if err := s.app.Habits.DeleteHabit(id); err != nil {
		log.Error("habit.delete failed", "method", "HabitService.DeleteHabit", "id", id, "error", err.Error())
		return nil, err
	}
	log.Info("habit.delete ok", "method", "HabitService.DeleteHabit", "id", id)
	return map[string]any{"success": true}, nil
}

// CreateSession 把空 `date` 视为今天。
func (s *HabitService) CreateSession(habitID int64, durationSeconds int64, count int64, date string) (any, error) {
	a := s.app
	log.Debug("habit.create_session", "method", "HabitService.CreateSession", "habit_id", habitID, "duration_seconds", durationSeconds, "count", count)
	id, err := a.Habits.CreateSession(habits.CreateSessionInput{
		HabitID: habitID, DurationSeconds: durationSeconds, Count: count, Date: date,
	})
	if err != nil {
		log.Error("habit.create_session failed", "method", "HabitService.CreateSession", "habit_id", habitID, "error", err.Error())
		return nil, err
	}
	log.Info("habit.create_session ok", "method", "HabitService.CreateSession", "id", id)
	return map[string]any{
		"id":               id,
		"habit_id":         habitID,
		"duration_seconds": durationSeconds,
		"date":             domain.TodayString(a.Settings.Config().Basic.Timezone),
	}, nil
}

// ListSessions 支持三种查询形态：单日、日期区间、或不带过滤（= 今天）。
func (s *HabitService) ListSessions(date string, startDate string, endDate string) (any, error) {
	log.Debug("habit.list_sessions", "method", "HabitService.ListSessions", "date", date, "start_date", startDate, "end_date", endDate)
	const limit, offset = 100, 0
	rows, err := s.app.Habits.ListSessions(date, startDate, endDate, limit, offset)
	if err != nil {
		log.Error("habit.list_sessions failed", "method", "HabitService.ListSessions", "error", err.Error())
		return nil, err
	}
	return rows, nil
}

// GetHabitDetail 返回单个 habit，附累计秒数（默认今天）与进度百分比。
func (s *HabitService) GetHabitDetail(id int64, date string) (any, error) {
	log.Debug("habit.detail", "method", "HabitService.GetHabitDetail", "id", id, "date", date)
	a := s.app
	detail, err := a.Habits.GetHabitDetail(id, date)
	if err != nil {
		log.Error("habit.detail failed", "method", "HabitService.GetHabitDetail", "id", id, "error", err.Error())
		return nil, err
	}
	return map[string]any{
		"id":               detail.ID,
		"name":             detail.Name,
		"goal_seconds":     detail.GoalSeconds,
		"color":            detail.Color,
		"today_seconds":    detail.TodaySeconds,
		"progress_percent": detail.ProgressPercent,
	}, nil
}

// SettingsService

// SettingsService 向 Wails v3 前端暴露 settings 配置。
type SettingsService struct {
	app *App
}

// NewSettingsService 构造绑定到给定 App 的 SettingsService。
func NewSettingsService(app *App) *SettingsService { return &SettingsService{app: app} }

func (s *SettingsService) GetSettings() (any, error) {
	log.Debug("settings.get", "method", "SettingsService.GetSettings")
	return s.app.Settings.Config(), nil
}

func (s *SettingsService) UpdateSettings(json string) (any, error) {
	log.Debug("settings.update", "method", "SettingsService.UpdateSettings")
	if err := s.app.Settings.HandleSettingsEvent(domain.SettingsChangeEvent{JSON: json}); err != nil {
		log.Error("settings.update failed", "method", "SettingsService.UpdateSettings", "error", err.Error())
		return map[string]any{"success": false, "error": err.Error()}, nil
	}
	log.Info("settings.update ok", "method", "SettingsService.UpdateSettings")
	return map[string]any{"status": "settings_updated"}, nil
}

// BackupService

// BackupService 向 Wails v3 前端暴露 backup 配置 + 主口令生命周期。
type BackupService struct {
	app *App
}

// NewBackupService 构造绑定到给定 App 的 BackupService。
func NewBackupService(app *App) *BackupService { return &BackupService{app: app} }

// GetBackupConfig 返回持久化的 BackupConfig，secret 已打码。
func (s *BackupService) GetBackupConfig() (any, error) {
	cfg := s.app.Settings.BackupConfig()
	log.Debug("backup.get_config", "method", "BackupService.GetBackupConfig", "target_type", cfg.TargetType.String())
	return map[string]any{
		"enabled":              cfg.Enabled,
		"auto_backup":          cfg.AutoBackup,
		"auto_backup_interval": cfg.AutoBackupSecs,
		"target_type":          cfg.TargetType.String(),
		"local_path":           cfg.LocalPath,
		"webdav_url":           cfg.WebDAVURL,
		"webdav_username":      cfg.WebDAVUsername,
		"webdav_password":      maskSecret(cfg.WebDAVPassword),
		"s3_endpoint":          cfg.S3Endpoint,
		"s3_bucket":            cfg.S3Bucket,
		"s3_region":            cfg.S3Region,
		"s3_access_key":        maskSecret(cfg.S3AccessKey),
		"s3_secret_key":        maskSecret(cfg.S3SecretKey),
		"s3_path_prefix":       cfg.S3PathPrefix,
	}, nil
}

func (s *BackupService) UpdateBackupConfig(json string) (any, error) {
	cfg := s.app.Settings.BackupConfig()
	log.Debug("backup.update_config", "method", "BackupService.UpdateBackupConfig", "target_type", cfg.TargetType.String())
	if err := s.app.Settings.UpdateBackupConfigFromJSON(json); err != nil {
		log.Error("backup.update_config failed", "method", "BackupService.UpdateBackupConfig", "error", err.Error())
		return map[string]any{"success": false, "error": err.Error()}, nil
	}
	// 配置已持久化；据此重建 manager。重建失败绝不能让调用失败 ——
	// 配置已存下，manager 会在下次配置变更或启动时重建。
	if err := s.app.RebuildBackup(context.Background()); err != nil {
		log.Error("UpdateBackupConfig: rebuild failed", "error", err.Error())
	}
	log.Info("backup.update_config ok", "method", "BackupService.UpdateBackupConfig", "target_type", cfg.TargetType.String())
	return map[string]any{"success": true}, nil
}

func (s *BackupService) CreateBackup() (any, error) {
	a := s.app
	cfg := a.Settings.BackupConfig()
	log.Debug("backup.create", "method", "BackupService.CreateBackup", "target_type", cfg.TargetType.String())
	bm := a.BackupManager()
	if bm == nil {
		return map[string]any{"success": false, "error": "backup not configured"}, nil
	}
	if !cfg.Enabled {
		return map[string]any{"success": false, "error": "backup not enabled"}, nil
	}
	name, err := bm.CreateBackup()
	if err != nil {
		log.Error("backup.create failed", "method", "BackupService.CreateBackup", "error", err.Error())
		return map[string]any{"success": false, "error": err.Error()}, nil
	}
	log.Info("backup.create ok", "method", "BackupService.CreateBackup", "target_type", cfg.TargetType.String(), "backup_name", name)
	return map[string]any{"success": true, "backup_path": name}, nil
}

func (s *BackupService) RestoreBackup(name string) (any, error) {
	a := s.app
	cfg := a.Settings.BackupConfig()
	log.Debug("backup.restore", "method", "BackupService.RestoreBackup", "target_type", cfg.TargetType.String())
	bm := a.BackupManager()
	if bm == nil {
		return map[string]any{"success": false, "error": "backup not configured"}, nil
	}
	if err := bm.RestoreFromBackup(name); err != nil {
		log.Error("backup.restore failed", "method", "BackupService.RestoreBackup", "error", err.Error())
		return map[string]any{"success": false, "error": err.Error()}, nil
	}
	log.Info("backup.restore ok", "method", "BackupService.RestoreBackup", "target_type", cfg.TargetType.String(), "backup_name", name)
	return map[string]any{"success": true}, nil
}

func (s *BackupService) DeleteBackup(name string) (any, error) {
	a := s.app
	cfg := a.Settings.BackupConfig()
	log.Debug("backup.delete", "method", "BackupService.DeleteBackup", "target_type", cfg.TargetType.String())
	bm := a.BackupManager()
	if bm == nil {
		return map[string]any{"success": false, "error": "backup not configured"}, nil
	}
	if err := bm.DeleteBackup(name); err != nil {
		log.Error("backup.delete failed", "method", "BackupService.DeleteBackup", "error", err.Error())
		return map[string]any{"success": false, "error": err.Error()}, nil
	}
	log.Info("backup.delete ok", "method", "BackupService.DeleteBackup", "target_type", cfg.TargetType.String(), "backup_name", name)
	return map[string]any{"success": true}, nil
}

func (s *BackupService) VerifyBackup() (any, error) {
	a := s.app
	cfg := a.Settings.BackupConfig()
	log.Debug("backup.verify", "method", "BackupService.VerifyBackup", "target_type", cfg.TargetType.String())
	bm := a.BackupManager()
	if bm == nil {
		return map[string]any{"success": false, "error": "backup not configured"}, nil
	}
	if !cfg.Enabled {
		return map[string]any{"success": false, "error": "backup not enabled"}, nil
	}
	if err := bm.TestConnection(); err != nil {
		log.Error("backup.verify failed", "method", "BackupService.VerifyBackup", "error", err.Error())
		return map[string]any{"success": false, "error": err.Error()}, nil
	}
	log.Info("backup.verify ok", "method", "BackupService.VerifyBackup", "target_type", cfg.TargetType.String())
	return map[string]any{"success": true}, nil
}

func (s *BackupService) ListBackups() (any, error) {
	a := s.app
	cfg := a.Settings.BackupConfig()
	log.Debug("backup.list", "method", "BackupService.ListBackups", "target_type", cfg.TargetType.String())
	bm := a.BackupManager()
	if bm == nil {
		return map[string]any{"success": true, "backups": []any{}}, nil
	}
	items, err := bm.ListBackups()
	if err != nil {
		log.Error("backup.list failed", "method", "BackupService.ListBackups", "error", err.Error())
		return map[string]any{"success": true, "backups": []any{}}, nil
	}
	log.Info("backup.list ok", "method", "BackupService.ListBackups", "target_type", cfg.TargetType.String(), "count", len(items))
	return map[string]any{"success": true, "backups": items}, nil
}

func (s *BackupService) GetMasterPasswordStatus() (any, error) {
	status := s.app.GetMasterPasswordStatus()
	log.Debug("backup.master_status", "method", "BackupService.GetMasterPasswordStatus", "has_cred", status.HasPassword)
	return status, nil
}

func (s *BackupService) SetMasterPassword(password string) (any, error) {
	log.Debug("backup.set_master", "method", "BackupService.SetMasterPassword", "has_cred", password != "")
	if password == "" {
		return map[string]any{"success": false, "error": "missing password"}, nil
	}
	if len(password) < 4 {
		return map[string]any{"success": false, "error": "password too short (minimum 4 characters)"}, nil
	}
	if err := s.app.SetMasterPassword(password); err != nil {
		log.Error("backup.set_master failed", "method", "BackupService.SetMasterPassword", "error", err.Error())
		return map[string]any{"success": false, "error": err.Error()}, nil
	}
	log.Info("backup.set_master ok", "method", "BackupService.SetMasterPassword")
	return map[string]any{"success": true}, nil
}

func (s *BackupService) UnlockCredentials(password string) (any, error) {
	log.Debug("backup.unlock", "method", "BackupService.UnlockCredentials")
	res := s.app.UnlockCredentials(password)
	log.Info("backup.unlock ok", "method", "BackupService.UnlockCredentials", "success", res.Success)
	return map[string]any{
		"success":      res.Success,
		"locked_until": res.LockedUntil,
	}, nil
}

func (s *BackupService) LockCredentials() (any, error) {
	log.Debug("backup.lock", "method", "BackupService.LockCredentials")
	s.app.LockCredentials()
	log.Info("backup.lock ok", "method", "BackupService.LockCredentials")
	return map[string]any{"success": true}, nil
}

// 共享辅助函数 —— 从 handlers/ 复制而来，保持本文件自包含。

// maskSecret 对非空 secret 返回 "******"，否则返回 ""。
func maskSecret(s string) string {
	if s == "" {
		return ""
	}
	return "******"
}
