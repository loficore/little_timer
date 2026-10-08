// Package storage —— DailySweepCrud:daily_sweep_log 同日去重 + settings
// 的 daily_sweep_* 列读写(v12)。
//
//   - LogSweep / HasSweepToday / GetLog 操作 daily_sweep_log(cron 触发前查,
//     手动触发不查);
//   - GetSettings / SetSettings 直接读写 settings id=1 的两列(与
//     llm_settings.go 读法一致,不引入对 settings.SettingsManager 的依赖)。
package storage

import (
	"database/sql"
	"errors"

	"little-timer/internal/domain"
)

// DailySweepCrud 操作 daily_sweep_log 表与 settings 的 daily_sweep_* 列。
type DailySweepCrud struct {
	db *sql.DB
}

// NewDailySweepCrud 返回空的 DailySweepCrud。SetDB 由 SqliteManager 调用。
func NewDailySweepCrud() *DailySweepCrud { return &DailySweepCrud{} }

// SetDB 接入 *sql.DB。
func (c *DailySweepCrud) SetDB(db *sql.DB) { c.db = db }

// LogSweep 写入一日的 sweep 日志。date 已有 → INSERT OR REPLACE 覆盖(同 source
// 重跑刷新 swept_at;异 source 也允许,如手动触发的覆盖 cron)。
func (c *DailySweepCrud) LogSweep(date, source string) error {
	_, err := c.db.Exec(
		`INSERT OR REPLACE INTO daily_sweep_log (date, swept_at, source)
		 VALUES (?, CURRENT_TIMESTAMP, ?);`,
		date, source,
	)
	return err
}

// HasSweepToday 报告 today(date,本地 YYYY-MM-DD)是否已有 sweep 日志。
// cron 触发前调;手动触发不调(用户主动想再做一次)。
func (c *DailySweepCrud) HasSweepToday(today string) (bool, error) {
	var n int
	if err := c.db.QueryRow(
		`SELECT COUNT(*) FROM daily_sweep_log WHERE date = ?;`, today,
	).Scan(&n); err != nil {
		return false, err
	}
	return n > 0, nil
}

// GetLog 取一日的 sweep 日志。无行 → (nil, nil)(caller 据此判断"今天还没 sweep")。
func (c *DailySweepCrud) GetLog(date string) (*domain.DailySweepLog, error) {
	var l domain.DailySweepLog
	err := c.db.QueryRow(
		`SELECT date, swept_at, source FROM daily_sweep_log WHERE date = ?;`, date,
	).Scan(&l.Date, &l.SweptAt, &l.Source)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &l, nil
}

// GetSettings 读 settings id=1 的 daily_sweep_enabled / daily_sweep_time。
// 应用层假定 settings 行已被 initializeDefaultSettings 初始化(参见 migration_test)。
func (c *DailySweepCrud) GetSettings() (enabled bool, at string, err error) {
	var eInt int
	if err = c.db.QueryRow(
		`SELECT daily_sweep_enabled, daily_sweep_time FROM settings WHERE id = 1;`,
	).Scan(&eInt, &at); err != nil {
		return false, "", err
	}
	return eInt != 0, at, nil
}

// SetSettings 按指针语义更新 settings id=1 的两列(指针 nil = 不动)。
// 与 llm_settings.go 的 PUT handler 模式对齐 —— 只更新 body 里出现的字段。
func (c *DailySweepCrud) SetSettings(enabled *bool, at *string) error {
	sets, args := []string{}, []any{}
	if enabled != nil {
		sets = append(sets, "daily_sweep_enabled = ?")
		args = append(args, boolToInt(*enabled))
	}
	if at != nil {
		sets = append(sets, "daily_sweep_time = ?")
		args = append(args, *at)
	}
	if len(sets) == 0 {
		return nil
	}
	sets = append(sets, "updated_at = CURRENT_TIMESTAMP")
	q := "UPDATE settings SET " + joinComma(sets) + " WHERE id = 1;"
	_, err := c.db.Exec(q, args...)
	return err
}
