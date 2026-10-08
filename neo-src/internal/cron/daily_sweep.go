// Package cron —— GTD Daily Sweep 定时触发(每分钟检查 settings)。
//
// 设计:
//   - SweepStarter 是 cron 调用的"入队 review jobs"函数,与 cron 包解耦;
//     cmd/server 注入 handlers.StartDailySweep;
//   - 每分钟读一次 settings.daily_sweep_enabled + daily_sweep_time(用户改
//     配置无需重启进程),仅在 (now.Format("15:04") == settings.time) 且
//     今日未 sweep 时触发;
//   - **不在启动时立即 tick**(避免"启动即弹窗");启动后第一个 tick 由 1 分钟
//     ticker 决定,期间配置更改即时生效。
//
// 失败安全:settings 读失败 / SweepStarter 错误均只 warn log,跳过本分钟,下分
// 钟重试。
package cron

import (
	"context"
	"sync"
	"time"

	"little-timer/internal/log"
	"little-timer/internal/storage"
)

// SweepStarter 是 cron 触发的入队函数(cron 不直接知道如何入队 review jobs)。
type SweepStarter func(ctx context.Context) ([]int64, error)

// DailySweepCron 是 GTD Daily Sweep 的进程内定时器。
type DailySweepCron struct {
	daily *storage.DailySweepCrud
	start SweepStarter

	stop   chan struct{}
	stopOn sync.Once

	now func() time.Time // 默认 time.Now;测试注入 fake 时钟
}

// NewDailySweepCron 构造一个未启动的 cron。now 默认为 time.Now,测试用 SetNow。
func NewDailySweepCron(daily *storage.DailySweepCrud, start SweepStarter) *DailySweepCron {
	return &DailySweepCron{
		daily: daily,
		start: start,
		stop:  make(chan struct{}),
		now:   time.Now,
	}
}

// SetNow 注入 fake 时钟(测试用);nil 还原为 time.Now。
func (c *DailySweepCron) SetNow(fn func() time.Time) {
	if fn == nil {
		c.now = time.Now
		return
	}
	c.now = fn
}

// Start 启动 ticker goroutine(每分钟 tick);**不**在启动时立即触发。
func (c *DailySweepCron) Start() {
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-c.stop:
				return
			case <-ticker.C:
				c.tick()
			}
		}
	}()
}

// Stop 通知 goroutine 退出。可重复调用。
func (c *DailySweepCron) Stop() {
	c.stopOn.Do(func() { close(c.stop) })
}

// tick 跑一次检查 + (可能)触发 sweep。纯逻辑,Start 与测试共用。
func (c *DailySweepCron) tick() {
	enabled, at, err := c.daily.GetSettings()
	if err != nil {
		log.Warn("cron.daily_sweep settings read failed", "error", err.Error())
		return // 跳过本分钟,下分钟重试
	}
	if !enabled {
		return
	}
	now := c.now().In(time.Local)
	if now.Format("15:04") != at {
		return // 未到点
	}
	today := now.Format("2006-01-02")
	has, _ := c.daily.HasSweepToday(today)
	if has {
		return // 同日已 sweep(cron / manual 触发过)
	}
	if _, err := c.start(context.Background()); err != nil {
		log.Warn("cron.daily_sweep start failed", "error", err.Error())
		return
	}
	if err := c.daily.LogSweep(today, "cron"); err != nil {
		log.Warn("cron.daily_sweep log failed", "error", err.Error())
	}
}
