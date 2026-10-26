package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"

	"little-timer/internal/crypto"
	"little-timer/internal/domain"
	"little-timer/internal/http/app"
	"little-timer/internal/keystore/keystoretest"
	"little-timer/internal/settings"
	"little-timer/internal/storage"
	"little-timer/internal/storage/backup"
)

// setupCredentialsRouter 注册 credentials 两条路由（与生产 router 对齐的最小
// 引擎）。路由注册本身的断言在 internal/http 包（那里能用 NewRouter）；
// 本文件只验 handler 行为。
func setupCredentialsRouter(t *testing.T, a *app.App) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("app", a)
		c.Next()
	})
	r.GET("/api/credentials/status", CredentialsStatus)
	r.POST("/api/credentials/reset", CredentialsReset)
	return r
}

// newCredentialsTestApp 构造带真 SQLite + settings + 内存 keystore 的 App。
// hasKey 控制 keystore 是否视 master key 为存在，用于造"磁盘有 blob 但 key
// 不在托管中"的降级态。
func newCredentialsTestApp(t *testing.T, hasKey bool) (*app.App, string) {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

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
	bm, err := backup.NewLocal(sqlite, dbPath, filepath.Join(tmpDir, "backups"))
	if err != nil {
		t.Fatalf("backup manager: %v", err)
	}
	a := app.NewApp(domain.NewClockManager(domain.NewDefaultClockTaskConfig()), sm, sqlite, bm, dbPath)
	a.Keystore = keystoretest.New("secret-service", hasKey)
	_ = a.Secrets()
	return a, tmpDir
}

// openRawDB 开一条独立的只读连接，用于断言 SQLite 里的标记列
// （SettingsManager 没有暴露 DB，那属于生产 API 面，不为测试开口子）。
func openRawDB(t *testing.T, dbPath string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("open raw db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

// ---- status ----

// TestCredentialsStatus_Normal 正常态：backend 取 keystore 的 Name()。
func TestCredentialsStatus_Normal(t *testing.T) {
	a, _ := newCredentialsTestApp(t, true)
	w := doJSON(t, setupCredentialsRouter(t, a), http.MethodGet, "/api/credentials/status", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200", w.Code)
	}
	var st domain.CredentialStatus
	if err := json.Unmarshal(w.Body.Bytes(), &st); err != nil {
		t.Fatalf("响应不是 CredentialStatus JSON: %v", err)
	}
	if st.Backend != "secret-service" {
		t.Errorf("backend = %q, want secret-service", st.Backend)
	}
	if st.HasSecret {
		t.Error("has_secret = true，尚未存入任何凭据")
	}
}

// TestCredentialsStatus_HasSecret 存过凭据后 has_secret 反映为 true。
func TestCredentialsStatus_HasSecret(t *testing.T) {
	a, _ := newCredentialsTestApp(t, true)
	if err := a.Secrets().Store([]byte("llm_api_key"), []byte("sk-x")); err != nil {
		t.Fatalf("store: %v", err)
	}
	w := doJSON(t, setupCredentialsRouter(t, a), http.MethodGet, "/api/credentials/status", nil)
	var st domain.CredentialStatus
	if err := json.Unmarshal(w.Body.Bytes(), &st); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !st.HasSecret {
		t.Error("has_secret = false，store 里明明有条目")
	}
}

// TestCredentialsStatus_BrokenPairReportsUnavailable 覆盖 Review Focus #2：
// keystore item 被用户删除 / keyring 被重置，而磁盘上的 secret.db 还在。
// 此时**不得**静默报成"没有凭据"——那会让前端引导用户重新填写，掩盖掉真正的
// 环境问题。必须 backend=unavailable 且 detail 非空。
func TestCredentialsStatus_BrokenPairReportsUnavailable(t *testing.T) {
	a, tmpDir := newCredentialsTestApp(t, true)
	// 先在正常态写入凭据，留下磁盘 blob。
	if err := a.Secrets().Store([]byte("llm_api_key"), []byte("sk-x")); err != nil {
		t.Fatalf("store: %v", err)
	}
	secretPath := filepath.Join(tmpDir, "secret.db")
	if _, err := os.Stat(secretPath); err != nil {
		t.Fatalf("前提不成立：store 后应有 secret.db: %v", err)
	}

	// 模拟 keyring 被重置：master key 不在了，磁盘 secret.db 仍在。
	// 换 App 重建以绕过 Secrets() 的 sync.Once（单 App 生命周期内不可重注入）。
	a2 := app.NewApp(a.Clock, a.Settings, a.SQLite, a.Backup, a.DBPath)
	a2.Keystore = keystoretest.New("secret-service", false)
	_ = a2.Secrets()

	w := doJSON(t, setupCredentialsRouter(t, a2), http.MethodGet, "/api/credentials/status", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200（降级态也返 200）", w.Code)
	}
	var st domain.CredentialStatus
	if err := json.Unmarshal(w.Body.Bytes(), &st); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if st.Backend != "unavailable" {
		t.Errorf("backend = %q, want unavailable", st.Backend)
	}
	if st.Detail == "" {
		t.Error("detail 为空：不可用必须给出可操作原因")
	}
	if !st.HasSecret {
		t.Error("has_secret = false，磁盘上确实还有 secret.db，应反映存在")
	}
}

// ---- reset ----

// TestCredentialsReset_ClearsSecretsAndFlags reset 必须真把凭据抹干净：
// secret store 空了、settings 的 has_* 归 0。否则重置后配置仍声称"有凭据"，
// 下次保存会把凭据写进一把与旧数据无关的 store。
func TestCredentialsReset_ClearsSecretsAndFlags(t *testing.T) {
	a, tmpDir := newCredentialsTestApp(t, true)
	if err := a.Secrets().Store([]byte("llm_api_key"), []byte("sk-x")); err != nil {
		t.Fatalf("store: %v", err)
	}
	// 预置 has_webdav_password=1（走真实写路径，不裸改 DB）。
	if err := a.Settings.UpdateBackupConfigFromJSON(
		`{"target_type":"webdav","webdav_username":"u","webdav_password":"p"}`); err != nil {
		t.Fatalf("seed webdav creds: %v", err)
	}
	raw := openRawDB(t, a.DBPath)
	var has int
	if err := raw.QueryRow(`SELECT has_webdav_password FROM backup_config WHERE id = 1;`).Scan(&has); err != nil {
		t.Fatalf("读 has_webdav_password: %v", err)
	}
	if has != 1 {
		t.Fatalf("前提不成立：has_webdav_password = %d, want 1", has)
	}

	w := doJSON(t, setupCredentialsRouter(t, a), http.MethodPost, "/api/credentials/reset", nil)
	if w.Code != http.StatusNoContent {
		t.Fatalf("code = %d, want 204; body=%s", w.Code, w.Body.String())
	}

	// 重开 store 读磁盘，不复用可能已被污染的 cache。
	fresh := crypto.NewSecretStorage(filepath.Join(tmpDir, "secret.db"), a.Keystore)
	_, err := fresh.Retrieve([]byte("llm_api_key"))
	if !errors.Is(err, crypto.ErrSecretNotFound) {
		t.Errorf("Retrieve err = %v，want ErrSecretNotFound（重置后确实不该有）", err)
	}

	if err := raw.QueryRow(`SELECT has_webdav_password FROM backup_config WHERE id = 1;`).Scan(&has); err != nil {
		t.Fatalf("重读 has_webdav_password: %v", err)
	}
	if has != 0 {
		t.Errorf("has_webdav_password = %d, want 0（重置后必须清标记）", has)
	}
}

// TestCredentialsReset_WithLoadFailurePurgesFile 覆盖 Review Focus #1：
// secret.db 拷自另一台机器/账户（master key 不在托管中而磁盘 blob 仍在）
// 的损坏对状态下，reset 原先返回 500，用户被永久锁死。现在必须走 Reset
// 逃生通道返回 **204**，且 secret.db 被物理清盘、store 回到干净态。
func TestCredentialsReset_WithLoadFailurePurgesFile(t *testing.T) {
	a, tmpDir := newCredentialsTestApp(t, true)
	if err := a.Secrets().Store([]byte("llm_api_key"), []byte("sk-x")); err != nil {
		t.Fatalf("store: %v", err)
	}
	secretPath := filepath.Join(tmpDir, "secret.db")
	if _, err := os.Stat(secretPath); err != nil {
		t.Fatalf("前提不成立：store 后应有 secret.db: %v", err)
	}

	// 构造损坏对：master key 不托管（hasKey=false）但磁盘 blob 仍在。
	// 换 App 重建以绕过 Secrets() 的 sync.Once（与 status 侧同款手法）。
	a2 := app.NewApp(a.Clock, a.Settings, a.SQLite, a.Backup, a.DBPath)
	a2.Keystore = keystoretest.New("secret-service", false)
	_ = a2.Secrets()
	if lf := a2.Secrets().LoadFailure(); lf == nil {
		t.Fatal("前提不成立：期望载入失败态，LoadFailure() 却为 nil")
	}

	w := doJSON(t, setupCredentialsRouter(t, a2), http.MethodPost, "/api/credentials/reset", nil)
	if w.Code != http.StatusNoContent {
		t.Fatalf("载入失败态下 reset = %d, want 204; body=%s", w.Code, w.Body.String())
	}
	if _, err := os.Stat(secretPath); !os.IsNotExist(err) {
		t.Errorf("reset 后 secret.db 仍在盘上: stat = %v", err)
	}
	if lf := a2.Secrets().LoadFailure(); lf != nil {
		t.Errorf("reset 后 LoadFailure() = %v, want nil（失败态已被逃生通道清除）", lf)
	}
	names, err := a2.Secrets().Items()
	if err != nil {
		t.Fatalf("reset 后 Items: %v", err)
	}
	if len(names) != 0 {
		t.Errorf("reset 后 Items = %v, want 空", names)
	}
}
