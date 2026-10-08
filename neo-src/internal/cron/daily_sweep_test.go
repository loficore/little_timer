package cron

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"little-timer/internal/storage"
)

// openTempSqlite 拉起一个全新且已迁移的 SqliteManager(测试用)。
func openTempSqlite(t *testing.T) *storage.SqliteManager {
	t.Helper()
	m := storage.NewSqliteManager().Init(filepath.Join(t.TempDir(), "test.db"))
	if err := m.Open(); err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := m.Migrate(); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	t.Cleanup(func() { _ = m.Close() })
	return m
}

// helper:把 YYYY-MM-DDTHH:MM:SS(本地)解析成 time.Time(本地时区)。
func mustParseTime(s string) time.Time {
	t, err := time.ParseInLocation("2006-01-02T15:04:05", s, time.Local)
	if err != nil {
		panic(err)
	}
	return t
}

// TestDailySweepCron_DisabledSkips 验证 disabled=true 不触发 sweep(即便到点)。
func TestDailySweepCron_DisabledSkips(t *testing.T) {
	m := openTempSqlite(t)
	off := false
	if err := m.DailySweep().SetSettings(&off, nil); err != nil {
		t.Fatal(err)
	}
	called := 0
	c := NewDailySweepCron(m.DailySweep(), func(context.Context) ([]int64, error) {
		called++
		return nil, nil
	})
	c.SetNow(func() time.Time { return mustParseTime("2026-10-08T08:00:00") })
	c.tick()
	if called != 0 {
		t.Errorf("disabled 仍触发 called=%d, want 0", called)
	}
}

// TestDailySweepCron_RespectsConfiguredTime 验证未到点不触发,到点才触发。
func TestDailySweepCron_RespectsConfiguredTime(t *testing.T) {
	m := openTempSqlite(t)
	at := "10:30"
	if err := m.DailySweep().SetSettings(nil, &at); err != nil {
		t.Fatal(err)
	}
	called := 0
	c := NewDailySweepCron(m.DailySweep(), func(context.Context) ([]int64, error) {
		called++
		return nil, nil
	})
	c.SetNow(func() time.Time { return mustParseTime("2026-10-08T08:00:00") })
	c.tick()
	if called != 0 {
		t.Errorf("未到点触发 called=%d, want 0", called)
	}
	c.SetNow(func() time.Time { return mustParseTime("2026-10-08T10:30:00") })
	c.tick()
	if called != 1 {
		t.Errorf("到点 called=%d, want 1", called)
	}
}

// TestDailySweepCron_NoStartupImmediateSweep 验证 Start 不立即 tick(避免"启动即弹窗")。
func TestDailySweepCron_NoStartupImmediateSweep(t *testing.T) {
	m := openTempSqlite(t) // 默认 enabled=true, time=08:00
	called := 0
	c := NewDailySweepCron(m.DailySweep(), func(context.Context) ([]int64, error) {
		called++
		return nil, nil
	})
	c.SetNow(func() time.Time { return mustParseTime("2026-10-08T03:00:00") })
	c.Start()
	time.Sleep(30 * time.Millisecond)
	c.Stop()
	if called != 0 {
		t.Errorf("启动即跑 called=%d, want 0", called)
	}
}

// TestDailySweepCron_SkipsIfAlreadySwept 验证同日已 sweep(manual 触发)→ cron 跳过。
func TestDailySweepCron_SkipsIfAlreadySwept(t *testing.T) {
	m := openTempSqlite(t)
	if err := m.DailySweep().LogSweep("2026-10-08", "manual"); err != nil {
		t.Fatal(err)
	}
	called := 0
	c := NewDailySweepCron(m.DailySweep(), func(context.Context) ([]int64, error) {
		called++
		return nil, nil
	})
	c.SetNow(func() time.Time { return mustParseTime("2026-10-08T08:00:00") })
	c.tick()
	if called != 0 {
		t.Errorf("同日已 sweep called=%d, want 0", called)
	}
}

// TestDailySweepCron_SettingsChangePicksUp 验证用户改 time 后下一个 tick 即生效
//(不重启进程)。铁证:去掉 tick 内的 GetSettings 重读 → 改 settings 后仍按旧值。
func TestDailySweepCron_SettingsChangePicksUp(t *testing.T) {
	m := openTempSqlite(t)
	called := 0
	c := NewDailySweepCron(m.DailySweep(), func(context.Context) ([]int64, error) {
		called++
		return nil, nil
	})
	// 第一分钟:默认 08:00,now=07:59 → 不触发
	c.SetNow(func() time.Time { return mustParseTime("2026-10-08T07:59:00") })
	c.tick()
	if called != 0 {
		t.Errorf("07:59 called=%d, want 0", called)
	}
	// 用户把 time 改成 07:30,now=07:30 → 触发
	at := "07:30"
	if err := m.DailySweep().SetSettings(nil, &at); err != nil {
		t.Fatal(err)
	}
	c.SetNow(func() time.Time { return mustParseTime("2026-10-08T07:30:00") })
	c.tick()
	if called != 1 {
		t.Errorf("改后到点 called=%d, want 1", called)
	}
}


