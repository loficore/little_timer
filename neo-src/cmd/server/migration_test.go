// Package main —— Task 11 启动接入 keystore 迁移的测试。
//
// 覆盖两件事：
//
//  1. 正常模式：keystore 就绪后启动序列要真的跑一次 MigrateAll，并在其之后
//     确保 master key。顺序是 MigrateAll → EnsureMasterKey——MigrateAll 内部
//     自己也会 EnsureMasterKey（幂等），但启动序列显式调一次能让"密钥没建起来"
//     这件事在启动阶段就炸，而不是拖到第一次读凭据时。
//
//  2. 迁移等待模式：Task 6 留的 501 占位路由要换成 Task 9 的真实 handler。
//     用户输对主口令后，进程内 Secrets 必须立即可用、哨兵必须已写——否则
//     migrate 端点返回 204 却什么都没发生，用户还得重启一遍。
package main

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"little-timer/internal/crypto"
	"little-timer/internal/http/app"
	"little-timer/internal/keystore"
	"little-timer/internal/keystore/keystoretest"
	"little-timer/internal/settings"
	"little-timer/internal/storage"
	"little-timer/internal/storage/backup"
	"little-timer/internal/domain"
)

// writeV1SecretForMigration 造一个 v1 secret.db（口令 old-pass，内含一条
// llm_api_key 条目）。走 crypto.EncryptWithPassword 是 v1 的真实算法，不手搓密文。
// 条目里的 base64 值对应一个纯测试字面量，不是任何真实凭据。
func writeV1SecretForMigration(t *testing.T, dir, password string) string {
	t.Helper()
	path := filepath.Join(dir, "secret.db")
	plain := []byte(`{"llm_api_key":"c2stbGVnYWN5"}`) // base64("sk-legacy")
	sealed, err := crypto.EncryptWithPassword(plain, []byte(password))
	if err != nil {
		t.Fatalf("EncryptWithPassword: %v", err)
	}
	if err := os.WriteFile(path, append([]byte("LTMSECv1"), sealed...), 0o600); err != nil {
		t.Fatalf("写 v1 secret: %v", err)
	}
	return path
}

// newMigrationApp 构造带真 SQLite + settings + 内存 keystore 的 App，
// 供迁移等待模式的端点测试使用。
func newMigrationApp(t *testing.T, dir string, ks keystore.Keystore) *app.App {
	t.Helper()
	dbPath := filepath.Join(dir, "test.db")
	sqlite := storage.NewSqliteManager().Init(dbPath)
	if err := sqlite.Open(); err != nil {
		t.Fatalf("sqlite open: %v", err)
	}
	t.Cleanup(func() { _ = sqlite.Close() })
	if err := sqlite.Migrate(); err != nil {
		t.Fatalf("sqlite migrate: %v", err)
	}
	sm, err := settings.NewFromSqliteManager(sqlite, dbPath)
	if err != nil {
		t.Fatalf("settings: %v", err)
	}
	bm, err := backup.NewLocal(sqlite, dbPath, filepath.Join(dir, "backups"))
	if err != nil {
		t.Fatalf("backup manager: %v", err)
	}
	a := app.NewApp(domain.NewClockManager(domain.NewDefaultClockTaskConfig()), sm, sqlite, bm, dbPath)
	a.Keystore = ks
	_ = a.Secrets()
	return a
}

// TestRunKeystoreMigration_InNormalMode_RunsMigrateAndEnsuresKey 正常模式下
// 启动序列必须跑一次 MigrateAll 并确保 master key。
func TestRunKeystoreMigration_InNormalMode_RunsMigrateAndEnsuresKey(t *testing.T) {
	dir := t.TempDir()
	writeV1SecretForMigration(t, dir, "old-pass")
	dbPath := filepath.Join(dir, "test.db")

	ks := &fakeKeystore{}
	// 有 v1 文件就必须给口令——真实启动拿不到口令（用户当年自己设的），
	// 那条路径由 TestRunKeystoreMigration_NoPassword_ReportsErrUnseal 覆盖。
	if err := runKeystoreMigration(ks, dbPath, nil, []byte("old-pass")); err != nil {
		t.Fatalf("runKeystoreMigration: %v", err)
	}
	if ks.ensureCalls == 0 {
		t.Error("启动序列没有确保 master key")
	}

	// 迁移应已生效：哨兵落盘、secret.db 变成 v2。
	if _, err := os.Stat(filepath.Join(dir, keystore.SentinelName)); err != nil {
		t.Errorf("哨兵未落盘: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "secret.db"))
	if err != nil {
		t.Fatalf("读 secret: %v", err)
	}
	if !strings.HasPrefix(string(raw), "LTMSEKv2") {
		t.Errorf("secret magic = %.8q, want LTMSEKv2（未迁成 v2）", raw)
	}
}

// TestRunKeystoreMigration_NoPassword_ReportsErrUnseal 真实启动路径：磁盘上有
// v1 secret 文件但用户没给口令（本进程不知道当年那串）。此时 MigrateAll 必须
// 报 ErrUnseal 让调用方 warn 后继续，而不是 panic 或半途写坏文件——旧凭据
// 保持原样，用户事后可跑 migrate-keystore CLI 补。
func TestRunKeystoreMigration_NoPassword_ReportsErrUnseal(t *testing.T) {
	dir := t.TempDir()
	secretPath := writeV1SecretForMigration(t, dir, "old-pass")
	before, err := os.ReadFile(secretPath)
	if err != nil {
		t.Fatalf("读前: %v", err)
	}
	dbPath := filepath.Join(dir, "test.db")

	err = runKeystoreMigration(&fakeKeystore{}, dbPath, nil, nil)
	if !errors.Is(err, keystore.ErrUnseal) {
		t.Fatalf("err = %v，want ErrUnseal", err)
	}
	after, err2 := os.ReadFile(secretPath)
	if err2 != nil {
		t.Fatalf("读后: %v", err2)
	}
	if string(before) != string(after) {
		t.Error("缺口令时 secret.db 被改写，必须保持原样")
	}
}

// TestRunKeystoreMigration_AlreadyMigrated_NoOp 已迁移过（有哨兵）时不应
// 重复迁移，也不该报错——启动必须能反复重启。
func TestRunKeystoreMigration_AlreadyMigrated_NoOp(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	if err := os.WriteFile(filepath.Join(dir, keystore.SentinelName), nil, 0o600); err != nil {
		t.Fatalf("预置哨兵: %v", err)
	}

	// 口令故意给错：若真去解密就会失败，从而证明命中哨兵短路。
	if err := runKeystoreMigration(&fakeKeystore{}, dbPath, nil, []byte("WRONG")); err != nil {
		t.Fatalf("已迁移时不应报错: %v", err)
	}
}

// TestMigrationWaitRouter_ServesRealHandlers 迁移等待模式必须挂真实 handler，
// 不再是 Task 6 留的 501 占位——那是 Task 6 明确留下的面包屑。
func TestMigrationWaitRouter_ServesRealHandlers(t *testing.T) {
	dir := t.TempDir()
	writeV1SecretForMigration(t, dir, "old-pass")
	ks := keystoretest.New("secret-service", true)
	a := newMigrationApp(t, dir, ks)

	r := newMigrationWaitRouter(a, "*")

	// 三条端点都不能是 501。
	for _, tc := range []struct{ method, path string }{
		{http.MethodGet, "/api/credentials/status"},
		{http.MethodPost, "/api/credentials/reset"},
		{http.MethodPost, "/api/credentials/migrate"},
	} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(tc.method, tc.path, strings.NewReader(`{}`)))
		if w.Code == http.StatusNotImplemented {
			t.Errorf("%s %s 仍是 501 占位，Task 9 的真实 handler 未接上", tc.method, tc.path)
		}
	}
}

// TestMigrationWaitRouter_MigrateSuccess_ThenSecretsUsable 核心断言：迁移端点
// 返回 204 之后，进程内 Secrets 必须**立即**可用、哨兵已写。
// 若只回 204 而不真正迁移，用户还得重启一遍才知道成没成。
func TestMigrationWaitRouter_MigrateSuccess_ThenSecretsUsable(t *testing.T) {
	dir := t.TempDir()
	secretPath := writeV1SecretForMigration(t, dir, "old-pass")
	ks := keystoretest.New("secret-service", true)
	a := newMigrationApp(t, dir, ks)
	r := newMigrationWaitRouter(a, "*")

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/credentials/migrate",
		strings.NewReader(`{"password":"old-pass"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNoContent {
		t.Fatalf("code = %d, want 204; body=%s", w.Code, w.Body.String())
	}

	if _, err := os.Stat(filepath.Join(dir, keystore.SentinelName)); err != nil {
		t.Fatalf("迁移成功后哨兵未写: %v", err)
	}
	// 凭据必须真能读回——读回的是新建实例，模拟重启后的磁盘状态。
	fresh := crypto.NewSecretStorage(secretPath, ks)
	if lf := fresh.LoadFailure(); lf != nil {
		t.Fatalf("迁移后 store 载入失败: %v", lf)
	}
	got, err := fresh.Retrieve([]byte("llm_api_key"))
	if err != nil {
		t.Fatalf("读回凭据: %v", err)
	}
	if string(got) != "sk-legacy" {
		t.Errorf("llm_api_key = %q, want sk-legacy", got)
	}
}

// TestMigrationWaitRouter_MigrateWrongPassword_403AndNoSideEffect 口令错返
// 403 且磁盘零改动。
func TestMigrationWaitRouter_MigrateWrongPassword_403AndNoSideEffect(t *testing.T) {
	dir := t.TempDir()
	secretPath := writeV1SecretForMigration(t, dir, "old-pass")
	before, err := os.ReadFile(secretPath)
	if err != nil {
		t.Fatalf("读前: %v", err)
	}
	ks := keystoretest.New("secret-service", true)
	a := newMigrationApp(t, dir, ks)
	r := newMigrationWaitRouter(a, "*")

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/credentials/migrate",
		strings.NewReader(`{"password":"WRONG"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("code = %d, want 403", w.Code)
	}
	after, err := os.ReadFile(secretPath)
	if err != nil {
		t.Fatalf("读后: %v", err)
	}
	if string(before) != string(after) {
		t.Error("口令错时 secret.db 被改写")
	}
	if _, err := os.Stat(filepath.Join(dir, keystore.SentinelName)); err == nil {
		t.Error("失败却写了哨兵")
	}
}

var _ = gin.SetMode