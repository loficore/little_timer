package storage

import (
	"testing"
)

// TestLogSweep_HasSweepToday 验证 daily_sweep_log 的同日去重:
//   - 新库无日志 → HasSweepToday=false;
//   - LogSweep(today,"manual") 后 → HasSweepToday=true;
//   - 同日异 source 的 INSERT OR REPLACE 覆盖(让手动重跑刷新日志)。
func TestLogSweep_HasSweepToday(t *testing.T) {
	m := openTempSqlite(t)
	c := m.DailySweep()
	today := "2026-10-08"

	if ok, err := c.HasSweepToday(today); err != nil || ok {
		t.Errorf("新库 HasSweepToday(%q)=(%v,%v), want (false,nil)", today, ok, err)
	}
	if err := c.LogSweep(today, "manual"); err != nil {
		t.Fatalf("LogSweep manual: %v", err)
	}
	if ok, err := c.HasSweepToday(today); err != nil || !ok {
		t.Errorf("LogSweep 后 HasSweepToday(%q)=(%v,%v), want (true,nil)", today, ok, err)
	}
	if err := c.LogSweep(today, "cron"); err != nil {
		t.Fatalf("LogSweep cron(覆盖): %v", err)
	}
	got, err := c.GetLog(today)
	if err != nil {
		t.Fatalf("GetLog: %v", err)
	}
	if got == nil || got.Source != "cron" {
		t.Errorf("GetLog=%+v, want source=cron", got)
	}
}

// TestDailySweepSettings_RoundTrip 验证 settings 的 daily_sweep_* 两列读写。
func TestDailySweepSettings_RoundTrip(t *testing.T) {
	m := openTempSqlite(t)
	c := m.DailySweep()

	en, at, err := c.GetSettings()
	if err != nil {
		t.Fatalf("默认 GetSettings: %v", err)
	}
	if !en || at != "08:00" {
		t.Fatalf("默认=(enabled=%v, time=%q), want (true, 08:00)", en, at)
	}

	off := false
	t1030 := "10:30"
	if err := c.SetSettings(&off, &t1030); err != nil {
		t.Fatalf("SetSettings(off,10:30): %v", err)
	}
	en, at, err = c.GetSettings()
	if err != nil {
		t.Fatalf("读更新后: %v", err)
	}
	if en || at != "10:30" {
		t.Fatalf("更新后=(enabled=%v, time=%q), want (false, 10:30)", en, at)
	}

	// 仅 time 改回 → enabled 应保持 false(nil=不动)
	t0800 := "08:00"
	if err := c.SetSettings(nil, &t0800); err != nil {
		t.Fatalf("SetSettings(nil,08:00): %v", err)
	}
	en, at, _ = c.GetSettings()
	if en || at != "08:00" {
		t.Fatalf("仅 time 后=(enabled=%v, time=%q), want (false, 08:00)", en, at)
	}

	// 全 nil → no-op,不强写
	if err := c.SetSettings(nil, nil); err != nil {
		t.Fatalf("SetSettings 全 nil: %v", err)
	}
}
