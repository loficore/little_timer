package log

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFormatLine(t *testing.T) {
	got := formatLine("2026-08-02T15:04:05Z", "INFO", "msg")
	want := "[2026-08-02T15:04:05Z] [INFO]  msg"
	if got != want {
		t.Errorf("formatLine = %q, want %q", got, want)
	}
}

func TestOpenLogDirRotation(t *testing.T) {
	// fixture (a)：文件 < 10MB 时重新打开并追加。
	t.Run("reopen_append_small_file", func(t *testing.T) {
		dir := t.TempDir()
		seedName := time.Now().Format("2006-01-02") + ".log"
		seedPath := filepath.Join(dir, seedName)
		if err := os.WriteFile(seedPath, make([]byte, 1024*1024), 0644); err != nil {
			t.Fatal(err)
		}
		f, err := openLogDir(dir)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if filepath.Base(f.Name()) != seedName {
			t.Errorf("expected same file, got %s", f.Name())
		}
	})

	// fixture (b)：文件 >= 10MB + 1 字节时轮转。
	t.Run("rotate_large_file", func(t *testing.T) {
		dir := t.TempDir()
		seedName := time.Now().Format("2006-01-02") + ".log"
		seedPath := filepath.Join(dir, seedName)
		// 创建一个 10MB + 1 字节的稀疏文件。
		sf, err := os.Create(seedPath)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := sf.Seek(10*1024*1024, 0); err != nil {
			sf.Close()
			t.Fatal(err)
		}
		if _, err := sf.Write([]byte{0}); err != nil {
			sf.Close()
			t.Fatal(err)
		}
		sf.Close()

		f, err := openLogDir(dir)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if filepath.Base(f.Name()) == seedName {
			t.Errorf("expected new file, got same seed file %s", f.Name())
		}
		// 新文件应为空（或接近空）。
		info, err := f.Stat()
		if err != nil {
			t.Fatal(err)
		}
		if info.Size() > 10 {
			t.Errorf("new file size %d, expected near-empty", info.Size())
		}
	})
}

func TestOpenLogDirEmptyReturnsStderr(t *testing.T) {
	f, err := openLogDir("")
	if err != nil {
		t.Fatal(err)
	}
	if f != os.Stderr {
		t.Errorf("openLogDir(\"\") = %v, want os.Stderr", f)
	}
}

func TestInitSinkNonAndroid(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.log")
	file, err := os.Create(filePath)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	handler := initSink(file)

	// 类型断言：必须是 *textHandler，而非裸的 *slog.TextHandler。
	th, ok := handler.(*textHandler)
	if !ok {
		t.Fatalf("initSink returned %T, expected *textHandler", handler)
	}

	// 写入一条记录，验证文件输出包含自定义格式。
	r := slog.NewRecord(time.Date(2026, 8, 2, 15, 4, 5, 0, time.UTC), slog.LevelInfo, "test message", 0)
	r.Add("key", "value")
	if err := th.Handle(context.Background(), r); err != nil {
		t.Fatal(err)
	}

	// 读回文件内容。
	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatal(err)
	}
	got := string(content)

	// textHandler.Handle 先写 attrs，再写 formatLine + 换行。
	// 期望：" key=value[2026-08-02T15:04:05Z] [INFO]  test message\n"
	wantSubstr := "[2026-08-02T15:04:05Z] [INFO]  test message"
	if !contains(got, wantSubstr) {
		t.Errorf("file content %q does not contain %q", got, wantSubstr)
	}
	if !contains(got, "key=value") {
		t.Errorf("file content %q does not contain attr key=value", got)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// 每日轮转测试。

func TestOpenLogDirDailyNaming(t *testing.T) {
	dir := t.TempDir()
	today := time.Now()
	expectedName := today.Format("2006-01-02") + ".log"

	f, err := openLogDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	got := filepath.Base(f.Name())
	if got != expectedName {
		t.Errorf("openLogDir(empty dir) created file %q, want %q", got, expectedName)
	}
}

func TestOpenLogDirDailyReopen(t *testing.T) {
	dir := t.TempDir()
	today := time.Now()
	yesterday := today.Add(-24 * time.Hour)

	todayName := today.Format("2006-01-02") + ".log"
	yesterdayName := yesterday.Format("2006-01-02") + ".log"

	// 写入昨天的文件 1MB —— 比今天的小，旧代码会选中它
	if err := os.WriteFile(filepath.Join(dir, yesterdayName), make([]byte, 1024*1024), 0644); err != nil {
		t.Fatal(err)
	}
	// 写入今天的文件 1MB + 100 字节 —— 更大，旧代码会偏好昨天的
	if err := os.WriteFile(filepath.Join(dir, todayName), make([]byte, 1024*1024+100), 0644); err != nil {
		t.Fatal(err)
	}

	f, err := openLogDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	got := filepath.Base(f.Name())
	if got != todayName {
		t.Errorf("openLogDir re-opened %q, want today's file %q", got, todayName)
	}
}

func TestOpenLogDirDailyRotate(t *testing.T) {
	dir := t.TempDir()
	today := time.Now()
	yesterday := today.Add(-24 * time.Hour)

	yesterdayName := yesterday.Format("2006-01-02") + ".log"
	todayName := today.Format("2006-01-02") + ".log"

	// 写入昨天的文件 1MB
	if err := os.WriteFile(filepath.Join(dir, yesterdayName), make([]byte, 1024*1024), 0644); err != nil {
		t.Fatal(err)
	}

	f, err := openLogDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	got := filepath.Base(f.Name())
	if got != todayName {
		t.Errorf("openLogDir created %q, want today's date-based file %q", got, todayName)
	}
}

func TestOpenLogDirDailySizeOverflow(t *testing.T) {
	dir := t.TempDir()
	today := time.Now()
	prefix := today.Format("2006-01-02")
	baseName := prefix + ".log"
	expectedName := prefix + ".2.log"

	// 创建今天的基础文件，大小 10MB + 1 字节
	seedPath := filepath.Join(dir, baseName)
	sf, err := os.Create(seedPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := sf.Seek(10*1024*1024, 0); err != nil {
		sf.Close()
		t.Fatal(err)
	}
	if _, err := sf.Write([]byte{0}); err != nil {
		sf.Close()
		t.Fatal(err)
	}
	sf.Close()

	f, err := openLogDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	got := filepath.Base(f.Name())
	if got != expectedName {
		t.Errorf("openLogDir created %q, want overflow suffix file %q", got, expectedName)
	}
}

func TestOpenLogDirDailyMultipleOverflow(t *testing.T) {
	dir := t.TempDir()
	today := time.Now()
	prefix := today.Format("2006-01-02")
	baseName := prefix + ".log"
	suffix2Name := prefix + ".2.log"
	expectedName := prefix + ".3.log"

	// 基础文件和 .2.log 都创建为 10MB + 1
	for _, name := range []string{baseName, suffix2Name} {
		seedPath := filepath.Join(dir, name)
		sf, err := os.Create(seedPath)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := sf.Seek(10*1024*1024, 0); err != nil {
			sf.Close()
			t.Fatal(err)
		}
		if _, err := sf.Write([]byte{0}); err != nil {
			sf.Close()
			t.Fatal(err)
		}
		sf.Close()
	}

	f, err := openLogDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	got := filepath.Base(f.Name())
	if got != expectedName {
		t.Errorf("openLogDir created %q, want next overflow suffix file %q", got, expectedName)
	}
}

func TestOpenLogDirDailyBasePreferred(t *testing.T) {
	dir := t.TempDir()
	today := time.Now()
	prefix := today.Format("2006-01-02")
	baseName := prefix + ".log"
	suffix2Name := prefix + ".2.log"

	// 同时写入基础文件（1MB + 100 字节，较大）和 .2.log（1MB，较小）。
	// 旧代码按大小降序排序、选中最小的（.2.log）—— 错误行为。
	if err := os.WriteFile(filepath.Join(dir, baseName), make([]byte, 1024*1024+100), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, suffix2Name), make([]byte, 1024*1024), 0644); err != nil {
		t.Fatal(err)
	}

	f, err := openLogDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	got := filepath.Base(f.Name())
	if got != baseName {
		t.Errorf("openLogDir opened %q, want base file %q (preferred when under cap)", got, baseName)
	}
}

func TestOpenLogDirDailyIgnoresMalformed(t *testing.T) {
	dir := t.TempDir()
	today := time.Now()
	prefix := today.Format("2006-01-02")
	baseName := prefix + ".log"

	// 创建有效的 1MB 基础文件
	if err := os.WriteFile(filepath.Join(dir, baseName), make([]byte, 1024*1024), 0644); err != nil {
		t.Fatal(err)
	}
	// 后缀格式非法（非整数）
	if err := os.WriteFile(filepath.Join(dir, prefix+".abc.log"), make([]byte, 1024), 0644); err != nil {
		t.Fatal(err)
	}
	// 扩展名错误
	if err := os.WriteFile(filepath.Join(dir, prefix+".txt"), make([]byte, 1024), 0644); err != nil {
		t.Fatal(err)
	}
	// 双重扩展名（非 .log）
	if err := os.WriteFile(filepath.Join(dir, prefix+".log.bak"), make([]byte, 1024), 0644); err != nil {
		t.Fatal(err)
	}

	// 不得 panic，且必须返回有效的基础文件。
	f, err := openLogDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	got := filepath.Base(f.Name())
	if got != baseName {
		t.Errorf("openLogDir opened %q, want valid base file %q (malformed files ignored)", got, baseName)
	}
}
