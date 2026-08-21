package backup

// LocalAdapter 与 BackupManager.cleanupOldBackups 的测试。
//
// 保留策略测试驱动 BackupManager（cleanupOldBackups 的唯一持有者），
// 但底下实际走的是 LocalAdapter 的 Delete 路径 —— 那才是真正要守住的。

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"little-timer/internal/storage"
)

// writeBytes 以 0600 权限把内容写入 path，出错则测试失败。既用来播种
// "src" 文件（供 Backup），也用来预置已存在的备份文件（供 List / 保留策略测试）。
func writeBytes(t *testing.T, path string, content []byte) {
	t.Helper()
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("WriteFile %s: %v", path, err)
	}
}

// readBytes 是读侧对应物 —— 方便断言往返完整性，省去 io.ReadFile 噪音。
func readBytes(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile %s: %v", path, err)
	}
	return b
}

// backupName 构造规范文件名：`presets_backup_<ts>.db`。格式集中一处，
// 约定将来变化时测试能保持同步。
func backupName(ts int64) string {
	return filenamePrefix + strconv.FormatInt(ts, 10) + filenameSuffix
}

// makeAdapter 创建根在 t.TempDir()/backups 的 LocalAdapter，同时返回
// adapter 和底层目录（需要检查文件的测试直接用后者）。
func makeAdapter(t *testing.T) (*LocalAdapter, string) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "backups")
	return NewLocalAdapter(dir), dir
}

// newManager 是保留策略测试最小可用的 BackupManager：直接调用
// cleanupOldBackups，不需要活的 sqlite 连接。adapter 用真 LocalAdapter，
// 所以 Delete 会真实走文件系统。
func newManager(t *testing.T, dir string, maxBackups int) *BackupManager {
	t.Helper()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("MkdirAll %s: %v", dir, err)
	}
	m := &BackupManager{
		backupDir:  dir,
		maxBackups: maxBackups,
		adapter:    NewLocalAdapter(dir),
	}
	return m
}

func TestLocalBackupRestoreRoundTrip(t *testing.T) {
	adapter, dir := makeAdapter(t)

	// 播种一个内容确定性的 "database" 文件。
	src := filepath.Join(t.TempDir(), "presets.db")
	payload := []byte("SQLite-format-3\x00fake-presets-bytes")
	writeBytes(t, src, payload)

	name := backupName(time.Now().Unix())
	if err := adapter.Backup(src, name); err != nil {
		t.Fatalf("Backup: %v", err)
	}

	// 备份文件现在必须存在于配置的目录里。
	backupPath := filepath.Join(dir, name)
	if _, err := os.Stat(backupPath); err != nil {
		t.Fatalf("expected backup at %s: %v", backupPath, err)
	}

	// 恢复到全新目的地并确认逐字节一致的拷贝。
	dst := filepath.Join(t.TempDir(), "restored.db")
	if err := adapter.Restore(name, dst); err != nil {
		t.Fatalf("Restore: %v", err)
	}
	if got := readBytes(t, dst); string(got) != string(payload) {
		t.Errorf("restored content mismatch:\n got: %q\nwant: %q", got, payload)
	}
}

// TestLocalBackupPathTraversalEscapesDir 记录一个已知限制：
// filepath.Join 会折叠 `..` 段，所以形如 "../../../tmp/x" 的 backupName
// 会逃出配置的目录。我们并不断言规范里的 "should reject" 预期（代码
// 没拒绝）—— 断言实际发生的行为，未来的修复会表现为 diff 里的行为变化，
// 而不是无声回归。
func TestLocalBackupPathTraversalEscapesDir(t *testing.T) {
	adapter, dir := makeAdapter(t)

	// 在备份目录外放一个 "src" 文件；让 Backup 指向它，并借助 `..` 把它
	// 落到配置目录之外。
	src := filepath.Join(t.TempDir(), "src.db")
	writeBytes(t, src, []byte("traversal"))

	// 上跳两级 + 一个同级临时目录。
	outsideDir := t.TempDir()
	outsideFile := filepath.Base(outsideDir) + "_" + strconv.FormatInt(time.Now().UnixNano(), 10) + ".db"
	backupName := filepath.Join("..", "..", filepath.Base(outsideDir), outsideFile)

	if err := adapter.Backup(src, backupName); err != nil {
		t.Fatalf("Backup with traversal name: %v", err)
	}

	// 文件不应写进备份目录内。
	if entries, err := os.ReadDir(dir); err != nil {
		t.Fatalf("ReadDir %s: %v", dir, err)
	} else if len(entries) != 0 {
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("backup dir should be empty, got: %v", names)
	}

	// 而且它应该落在了外面（filepath.Join 清理了 traversal 段）。
	resolved := filepath.Join(outsideDir, outsideFile)
	if _, err := os.Stat(resolved); err != nil {
		t.Fatalf("expected escape at %s: %v", resolved, err)
	}

	// Restore 的解析方式相同（filepath.Join + Clean），所以它照样读到
	// 逃出的文件。记录这个对称行为 —— 两侧同等地接受 traversal。
	dst := filepath.Join(t.TempDir(), "dst.db")
	if err := adapter.Restore(backupName, dst); err != nil {
		t.Errorf("Restore with traversal name should mirror Backup: %v", err)
	} else if got := readBytes(t, dst); string(got) != "traversal" {
		t.Errorf("traversal Restore returned %q, want %q", got, "traversal")
	}
}

// TestLocalBackupUnwritableDir 确认目标目录无法创建时 Backup 失败。
// 我们把 adapter 指向一个确定不可能存在的路径（/proc 的子路径，Linux 上
// 只读），MkdirAll 和随后的拷贝都无法成功。
//
// root 用户不受文件系统权限约束 —— 跳过，以免 root CI 上出现无谓失败，
// 同时保持对非 root 用户的意义。
func TestLocalBackupUnwritableDir(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("permission test is unreliable as root")
	}

	bogus := filepath.Join("/proc", "self", "definitely-no-such-subdir", "backups")
	adapter := NewLocalAdapter(bogus)

	src := filepath.Join(t.TempDir(), "src.db")
	writeBytes(t, src, []byte("payload"))

	if err := adapter.Backup(src, backupName(time.Now().Unix())); err == nil {
		t.Errorf("Backup to %s should fail", bogus)
	}
}

func TestLocalBackupOverwritesExisting(t *testing.T) {
	adapter, dir := makeAdapter(t)

	src1 := filepath.Join(t.TempDir(), "src1.db")
	writeBytes(t, src1, []byte("first-payload"))
	src2 := filepath.Join(t.TempDir(), "src2.db")
	writeBytes(t, src2, []byte("second-payload-much-longer"))

	name := backupName(1700000000)
	if err := adapter.Backup(src1, name); err != nil {
		t.Fatalf("Backup #1: %v", err)
	}
	if err := adapter.Backup(src2, name); err != nil {
		t.Fatalf("Backup #2: %v", err)
	}

	// 备份目录里恰好一个文件，且是第二个 payload。
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 1 {
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Fatalf("expected 1 file, got %d: %v", len(entries), names)
	}

	dst := filepath.Join(t.TempDir(), "out.db")
	if err := adapter.Restore(name, dst); err != nil {
		t.Fatalf("Restore: %v", err)
	}
	if got := readBytes(t, dst); string(got) != "second-payload-much-longer" {
		t.Errorf("Restore returned stale content: %q", got)
	}
}

func TestLocalRestoreNotFound(t *testing.T) {
	adapter, _ := makeAdapter(t)

	dst := filepath.Join(t.TempDir(), "dst.db")
	err := adapter.Restore("presets_backup_does_not_exist.db", dst)
	if err == nil {
		t.Fatalf("Restore of missing backup should fail")
	}
	if !errors.Is(err, ErrFileNotFound) {
		t.Errorf("Restore error: got %v, want errors.Is ErrFileNotFound", err)
	}
	if _, statErr := os.Stat(dst); !os.IsNotExist(statErr) {
		t.Errorf("destination file should not exist after failed Restore (stat err: %v)", statErr)
	}
}

// TestLocalRestoreCorruptOrEmptyFile 确认 Restore 不校验 payload 内容 ——
// 它只拷字节。我们防的是 panic，不是数据语义（语义归上层那个把恢复文件当
// SQLite DB 打开的角色）。三种“垃圾”覆盖实际边缘情况：空、残缺 SQLite
// 头、纯随机字节。
func TestLocalRestoreCorruptOrEmptyFile(t *testing.T) {
	adapter, dir := makeAdapter(t)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	cases := []struct {
		name    string
		content []byte
	}{
		{"empty", []byte{}},
		{"partial_header", []byte("SQLite-form")},
		{"random_garbage", []byte{0x00, 0xff, 0x7f, 0x42, 0x13, 0x37}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			name := backupName(time.Now().UnixNano())
			writeBytes(t, filepath.Join(dir, name), tc.content)

			dst := filepath.Join(t.TempDir(), "dst.db")

			// 不得 panic；结果取决于内容，但 Restore 本身对坏字节从不报错 ——
			// 它是裸拷贝。
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("Restore panicked on %s content: %v", tc.name, r)
				}
			}()
			if err := adapter.Restore(name, dst); err != nil {
				t.Fatalf("Restore on %s payload: unexpected error %v", tc.name, err)
			}
			if got := readBytes(t, dst); string(got) != string(tc.content) {
				t.Errorf("Restore on %s payload: bytes mismatch\n got %v\nwant %v", tc.name, got, tc.content)
			}
		})
	}
}

func TestLocalListEmptyDir(t *testing.T) {
	adapter, dir := makeAdapter(t)

	// adapter 已配置但目录还不存在 —— 文档行为是“无条目、无错误”，
	// 而不是向调用方抛 ErrFileNotFound。
	got, err := adapter.List()
	if err != nil {
		t.Fatalf("List on missing dir: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("List on missing dir: got %d entries, want 0: %+v", len(got), got)
	}

	// 现在创建空目录；List 也应返回空 slice（返回的 slice 必须可安全 range）。
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	got, err = adapter.List()
	if err != nil {
		t.Fatalf("List on empty dir: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("List on empty dir: got %d entries, want 0: %+v", len(got), got)
	}
}

func TestLocalListMultipleBackups(t *testing.T) {
	adapter, dir := makeAdapter(t)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	// 三个格式正确的备份 + 几个诱饵（错前缀 / 错后缀 / 错 ts）来考验过滤器。
	good := []struct {
		name string
		ts   int64
		size int
	}{
		{backupName(1000), 1000, 5},
		{backupName(2000), 2000, 7},
		{backupName(3000), 3000, 9},
	}
	for _, g := range good {
		writeBytes(t, filepath.Join(dir, g.name), make([]byte, g.size))
	}
	// 诱饵 —— 必须被忽略。
	writeBytes(t, filepath.Join(dir, "unrelated.txt"), []byte("nope"))
	writeBytes(t, filepath.Join(dir, "presets_backup_notanumber.db"), []byte("garbage"))
	writeBytes(t, filepath.Join(dir, "presets_backup_4000"), []byte("missing suffix"))
	// 名字正确的目录也应被跳过。
	if err := os.Mkdir(filepath.Join(dir, backupName(5000)), 0o700); err != nil {
		t.Fatalf("mkdir decoy: %v", err)
	}

	got, err := adapter.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != len(good) {
		t.Fatalf("List len: got %d, want %d (entries: %+v)", len(got), len(good), got)
	}

	// 按时间戳排序以获得稳定比较 —— 契约不保证 List 的顺序。
	sort.Slice(got, func(i, j int) bool { return got[i].Timestamp < got[j].Timestamp })
	for i, g := range good {
		if got[i].Name != g.name {
			t.Errorf("entry %d: name = %q, want %q", i, got[i].Name, g.name)
		}
		if got[i].Timestamp != g.ts {
			t.Errorf("entry %d: ts = %d, want %d", i, got[i].Timestamp, g.ts)
		}
		if got[i].SizeBytes != uint64(g.size) {
			t.Errorf("entry %d: size = %d, want %d", i, got[i].SizeBytes, g.size)
		}
	}
}

func TestLocalDeleteExisting(t *testing.T) {
	adapter, dir := makeAdapter(t)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	name := backupName(time.Now().Unix())
	path := filepath.Join(dir, name)
	writeBytes(t, path, []byte("doomed"))

	if err := adapter.Delete(name); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("file still present after Delete: stat err = %v", err)
	}
}

func TestLocalDeleteNonexistentIsIdempotent(t *testing.T) {
	adapter, _ := makeAdapter(t)

	// 规范称之为“幂等” —— 删除缺失文件绝不能返回错误：
	// delete 时 not-found 视为 OK。
	if err := adapter.Delete(backupName(9999)); err != nil {
		t.Errorf("Delete on missing file: got %v, want nil", err)
	}
}

// TestRetentionKeepZero 在上限为 0 时删掉所有备份。我们绕过
// SetMaxBackups，因为它拒绝非正值（生产中合理的护栏，但恰恰是本测试钉住
// 的情况）。测试与生产代码同包，所以可以直接改字段。
func TestRetentionKeepZero(t *testing.T) {
	dir := t.TempDir()
	m := newManager(t, dir, 5)
	m.maxBackups = 0 // 直接改字段 —— SetMaxBackups 会拒绝 0。

	// 放三个备份。
	for _, ts := range []int64{10, 20, 30} {
		writeBytes(t, filepath.Join(dir, backupName(ts)), []byte("x"))
	}

	if err := m.cleanupOldBackups(); err != nil {
		t.Fatalf("cleanupOldBackups: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 0 {
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("keepCount=0: expected 0 backups, got %d: %v", len(entries), names)
	}
}

func TestRetentionKeepOne(t *testing.T) {
	dir := t.TempDir()
	m := newManager(t, dir, 1)

	for _, ts := range []int64{10, 20, 30, 40} {
		writeBytes(t, filepath.Join(dir, backupName(ts)), []byte("x"))
	}

	if err := m.cleanupOldBackups(); err != nil {
		t.Fatalf("cleanupOldBackups: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 1 {
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Fatalf("keepCount=1: expected 1 backup, got %d: %v", len(entries), names)
	}
	if entries[0].Name() != backupName(40) {
		t.Errorf("survivor: got %s, want %s", entries[0].Name(), backupName(40))
	}
}

func TestRetentionKeepTwoOfFive(t *testing.T) {
	dir := t.TempDir()
	m := newManager(t, dir, 2)

	for _, ts := range []int64{100, 200, 300, 400, 500} {
		writeBytes(t, filepath.Join(dir, backupName(ts)), []byte("x"))
	}

	if err := m.cleanupOldBackups(); err != nil {
		t.Fatalf("cleanupOldBackups: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 2 {
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Fatalf("keepCount=2: expected 2 backups, got %d: %v", len(entries), names)
	}

	// cleanupOldBackups 按时间戳升序排序并从头修剪，所以存活的两个
	// 必须是最新的两个。
	gotNames := make(map[string]bool, 2)
	for _, e := range entries {
		gotNames[e.Name()] = true
	}
	for _, want := range []string{backupName(400), backupName(500)} {
		if !gotNames[want] {
			t.Errorf("missing survivor %s; have %v", want, gotNames)
		}
	}
}

// TestRetentionMixedGroupsKeepsNewest 对应规范里“presetA 有 3 份、
// presetB 有 5 份”的想法，适配到真实的扁平命名空间模型：备份在同一个
// 目录里，但聚成两个明显不同的时间戳区间。清理逻辑是统一的 —— 全局保留
// 最新 N 份 —— 所以测试断言这一行为，而不是虚构一个不存在的按 preset
// 分组的机制。
func TestRetentionMixedGroupsKeepsNewest(t *testing.T) {
	dir := t.TempDir()
	m := newManager(t, dir, 3)

	// “A 组”：低时间戳；“B 组”：高时间戳。
	groupA := []int64{100, 200, 300}
	groupB := []int64{1_000_000, 1_000_100, 1_000_200, 1_000_300, 1_000_400}
	for _, ts := range append(append([]int64{}, groupA...), groupB...) {
		writeBytes(t, filepath.Join(dir, backupName(ts)), []byte("x"))
	}

	if err := m.cleanupOldBackups(); err != nil {
		t.Fatalf("cleanupOldBackups: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 3 {
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Fatalf("expected 3 backups, got %d: %v", len(entries), names)
	}

	// 最新三个是 B 组里 ts 最大的三个。
	sort.Slice(entries, func(i, j int) bool {
		ti, _ := strconv.ParseInt(strings.TrimSuffix(strings.TrimPrefix(entries[i].Name(), filenamePrefix), filenameSuffix), 10, 64)
		tj, _ := strconv.ParseInt(strings.TrimSuffix(strings.TrimPrefix(entries[j].Name(), filenamePrefix), filenameSuffix), 10, 64)
		return ti < tj
	})
	wantNewest := []string{backupName(1_000_200), backupName(1_000_300), backupName(1_000_400)}
	for i, w := range wantNewest {
		if entries[i].Name() != w {
			t.Errorf("survivor[%d] = %s, want %s", i, entries[i].Name(), w)
		}
	}
}

// TestRetentionSkipsCorruptFiles 验证规范点名的韧性：损坏 / 不可解析的
// 条目绝不能中断保留流程，也不计入保留上限（cleanupOldBackups 根本看不到
// 它们 —— List 已过滤）。
func TestRetentionSkipsCorruptFiles(t *testing.T) {
	dir := t.TempDir()
	m := newManager(t, dir, 2)

	// 3 个有效备份 + 2 个垃圾名字（时间戳解析不了）。
	valid := []int64{1000, 2000, 3000}
	for _, ts := range valid {
		writeBytes(t, filepath.Join(dir, backupName(ts)), []byte("x"))
	}
	writeBytes(t, filepath.Join(dir, "presets_backup_NOTANUMBER.db"), []byte("junk"))
	writeBytes(t, filepath.Join(dir, "presets_backup_also_bad.db"), []byte("junk"))

	if err := m.cleanupOldBackups(); err != nil {
		t.Fatalf("cleanupOldBackups: %v", err)
	}

	// 清理后应恰好剩 2 个存活文件（两个最新的有效备份），且两个损坏文件
	// 必须仍在磁盘上 —— 它们不在解析出的集合里，清理从未碰过它们。
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 4 {
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Fatalf("expected 4 files (2 newest + 2 corrupt), got %d: %v", len(entries), names)
	}

	// 验证两个最新的存活了下来。
	have := map[string]bool{}
	for _, e := range entries {
		have[e.Name()] = true
	}
	if !have[backupName(2000)] || !have[backupName(3000)] {
		t.Errorf("missing survivors; have %v", have)
	}
	if have[backupName(1000)] {
		t.Errorf("oldest valid backup should have been pruned; have %v", have)
	}
	if !have["presets_backup_NOTANUMBER.db"] || !have["presets_backup_also_bad.db"] {
		t.Errorf("corrupt files should be left untouched; have %v", have)
	}
}

// newRealManager 创建由真实 SqliteManager 支撑的 BackupManager，
// 供需要 VACUUM INTO（要求活的 SQLite 连接）的测试使用。
func newRealManager(t *testing.T) (*BackupManager, string, *storage.SqliteManager) {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	backupDir := filepath.Join(tmpDir, "backups")

	sqlite := storage.NewSqliteManager().Init(dbPath)
	if err := sqlite.Open(); err != nil {
		t.Fatalf("sqlite open: %v", err)
	}
	t.Cleanup(func() { _ = sqlite.Close() })
	if err := sqlite.Migrate(); err != nil {
		t.Fatalf("sqlite migrate: %v", err)
	}

	// 插入一行，让备份有真实内容。
	if _, err := sqlite.DB().Exec(`CREATE TABLE IF NOT EXISTS test_items (id INTEGER PRIMARY KEY, val TEXT);`); err != nil {
		t.Fatalf("create table: %v", err)
	}
	if _, err := sqlite.DB().Exec(`INSERT INTO test_items (val) VALUES ('hello-vacuum');`); err != nil {
		t.Fatalf("insert: %v", err)
	}

	mgr, err := NewLocal(sqlite, dbPath, backupDir)
	if err != nil {
		t.Fatalf("NewLocal: %v", err)
	}
	return mgr, backupDir, sqlite
}

// TestCreateBackup_VacuumInto 走通 VACUUM INTO：建真实 SQLite 数据库、
// 插一行、调用 CreateBackup。断言备份文件存在、临时文件已消失、
// manifest.json 已写入。
func TestCreateBackup_VacuumInto(t *testing.T) {
	mgr, backupDir, _ := newRealManager(t)

	name, err := mgr.CreateBackup()
	if err != nil {
		t.Fatalf("CreateBackup: %v", err)
	}
	if name == "" {
		t.Fatal("CreateBackup returned empty name")
	}

	// 备份文件必须存在于 backupDir。
	backupPath := filepath.Join(backupDir, name)
	if _, err := os.Stat(backupPath); err != nil {
		t.Fatalf("expected backup file at %s: %v", backupPath, err)
	}

	// 临时文件 `lt_backup_*.db` 必须已消失（defer os.Remove）。
	ltGlob := filepath.Join(filepath.Dir(mgr.dbPath), "lt_backup_*.db")
	matches, err := filepath.Glob(ltGlob)
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(matches) != 0 {
		t.Errorf("temp file(s) still present: %v", matches)
	}

	// manifest.json 必须存在于 backupDir。
	manifestPath := filepath.Join(backupDir, "manifest.json")
	if _, err := os.Stat(manifestPath); err != nil {
		t.Fatalf("expected manifest.json at %s: %v", manifestPath, err)
	}
}

// TestCreateBackup_SHA256Match 验证 manifest.json 里的 SHA-256 与独立
// 计算出的备份文件摘要一致。
func TestCreateBackup_SHA256Match(t *testing.T) {
	mgr, backupDir, _ := newRealManager(t)

	name, err := mgr.CreateBackup()
	if err != nil {
		t.Fatalf("CreateBackup: %v", err)
	}

	// 读 manifest.json 并取出 sha256 字段。
	manifestPath := filepath.Join(backupDir, "manifest.json")
	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}

	var manifest struct {
		Version int `json:"version"`
		Backups []struct {
			Name      string `json:"name"`
			Timestamp int64  `json:"timestamp"`
			SizeBytes uint64 `json:"size_bytes"`
			SHA256    string `json:"sha256"`
		} `json:"backups"`
	}
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		t.Fatalf("unmarshal manifest: %v", err)
	}

	// 找到我们这个备份名对应的条目。
	var entrySHA256 string
	for _, b := range manifest.Backups {
		if b.Name == name {
			entrySHA256 = b.SHA256
			break
		}
	}
	if entrySHA256 == "" {
		t.Fatalf("manifest has no entry for backup %q", name)
	}

	// 独立计算备份文件的 SHA-256。
	backupPath := filepath.Join(backupDir, name)
	computed, err := sha256File(backupPath)
	if err != nil {
		t.Fatalf("sha256File: %v", err)
	}

	if entrySHA256 != computed {
		t.Errorf("SHA-256 mismatch:\n manifest: %s\ncomputed: %s", entrySHA256, computed)
	}
}

// TestCreateBackup_SingleQuotePath 是 VACUUM INTO 单引号转义修复的回归
// 测试：临时文件与 DB 同目录，所以 dbPath 位于名字含单引号的目录（如
// `O'Brien`）时，旧代码会产生破碎 SQL 并让备份失败。SQLite 的 VACUUM INTO
// 不支持绑定参数，路径里的引号必须翻倍转义。
func TestCreateBackup_SingleQuotePath(t *testing.T) {
	quoteDir := filepath.Join(t.TempDir(), "O'Brien")
	if err := os.MkdirAll(quoteDir, 0o700); err != nil {
		t.Fatalf("MkdirAll %s: %v", quoteDir, err)
	}
	dbPath := filepath.Join(quoteDir, "test.db")
	backupDir := filepath.Join(quoteDir, "backups")

	sqlite := storage.NewSqliteManager().Init(dbPath)
	if err := sqlite.Open(); err != nil {
		t.Fatalf("sqlite open: %v", err)
	}
	t.Cleanup(func() { _ = sqlite.Close() })
	if err := sqlite.Migrate(); err != nil {
		t.Fatalf("sqlite migrate: %v", err)
	}
	if _, err := sqlite.DB().Exec(`CREATE TABLE IF NOT EXISTS test_items (id INTEGER PRIMARY KEY, val TEXT);`); err != nil {
		t.Fatalf("create table: %v", err)
	}
	if _, err := sqlite.DB().Exec(`INSERT INTO test_items (val) VALUES ('single-quote-path');`); err != nil {
		t.Fatalf("insert: %v", err)
	}

	mgr, err := NewLocal(sqlite, dbPath, backupDir)
	if err != nil {
		t.Fatalf("NewLocal: %v", err)
	}

	name, err := mgr.CreateBackup()
	if err != nil {
		t.Fatalf("CreateBackup in O'Brien dir: %v", err)
	}
	if name == "" {
		t.Fatal("CreateBackup returned empty name")
	}

	// 备份文件必须存在于 backupDir。
	if _, err := os.Stat(filepath.Join(backupDir, name)); err != nil {
		t.Fatalf("expected backup file at %s: %v", filepath.Join(backupDir, name), err)
	}

	// manifest.json 必须存在于 backupDir。
	if _, err := os.Stat(filepath.Join(backupDir, "manifest.json")); err != nil {
		t.Fatalf("expected manifest.json at %s: %v", filepath.Join(backupDir, "manifest.json"), err)
	}

	// 临时文件 `lt_backup_*.db` 必须已消失（defer os.Remove）。
	matches, err := filepath.Glob(filepath.Join(quoteDir, "lt_backup_*.db"))
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(matches) != 0 {
		t.Errorf("temp file(s) still present: %v", matches)
	}
}

// TestCreateBackup_TempCleanup 验证即使 adapter 的 Backup() 失败，
// 临时文件也会被清理（defer os.Remove）。
func TestCreateBackup_TempCleanup(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	backupDir := filepath.Join(tmpDir, "backups")

	sqlite := storage.NewSqliteManager().Init(dbPath)
	if err := sqlite.Open(); err != nil {
		t.Fatalf("sqlite open: %v", err)
	}
	t.Cleanup(func() { _ = sqlite.Close() })
	if err := sqlite.Migrate(); err != nil {
		t.Fatalf("sqlite migrate: %v", err)
	}
	if _, err := sqlite.DB().Exec(`CREATE TABLE IF NOT EXISTS test_items (id INTEGER PRIMARY KEY, val TEXT);`); err != nil {
		t.Fatalf("create table: %v", err)
	}
	if _, err := sqlite.DB().Exec(`INSERT INTO test_items (val) VALUES ('temp-cleanup');`); err != nil {
		t.Fatalf("insert: %v", err)
	}

	fake := NewFakeLocalAdapter(backupDir)
	fake.SetBackupError(errors.New("injected backup failure"))

	mgr := NewManagerWithAdapter(sqlite, dbPath, backupDir, fake)

	_, err := mgr.CreateBackup()
	if err == nil {
		t.Fatal("CreateBackup should fail with injected error")
	}

	// 临时文件 `lt_backup_*.db` 必须已消失（CreateBackup 里的 defer os.Remove）。
	ltGlob := filepath.Join(filepath.Dir(dbPath), "lt_backup_*.db")
	matches, err := filepath.Glob(ltGlob)
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(matches) != 0 {
		t.Errorf("temp file(s) leaked after failed backup: %v", matches)
	}
}

// TestCreateBackup_ManifestWritten 验证 CreateBackup 之后 manifest.json
// 的结构：version、backups 数组含 name/timestamp/sha256/size_bytes。
func TestCreateBackup_ManifestWritten(t *testing.T) {
	mgr, backupDir, _ := newRealManager(t)

	name, err := mgr.CreateBackup()
	if err != nil {
		t.Fatalf("CreateBackup: %v", err)
	}

	manifestPath := filepath.Join(backupDir, "manifest.json")
	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}

	var manifest struct {
		Version   int    `json:"version"`
		DBVersion string `json:"db_version"`
		Backups   []struct {
			Name      string `json:"name"`
			Timestamp int64  `json:"timestamp"`
			SHA256    string `json:"sha256"`
			SizeBytes uint64 `json:"size_bytes"`
		} `json:"backups"`
	}
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		t.Fatalf("unmarshal manifest: %v", err)
	}

	if manifest.Version != 1 {
		t.Errorf("version = %d, want 1", manifest.Version)
	}

	if len(manifest.Backups) == 0 {
		t.Fatal("manifest has no backups")
	}

	// 找到我们这个备份的条目。
	var found bool
	for _, b := range manifest.Backups {
		if b.Name == name {
			found = true
			if b.Name == "" {
				t.Error("backup name is empty")
			}
			if b.Timestamp == 0 {
				t.Error("backup timestamp is zero")
			}
			if b.SHA256 == "" {
				t.Error("backup sha256 is empty")
			}
			if b.SizeBytes == 0 {
				t.Error("backup size_bytes is zero")
			}
			break
		}
	}
	if !found {
		t.Errorf("manifest has no entry for backup %q", name)
	}
}
