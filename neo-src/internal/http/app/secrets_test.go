package app

import (
	"os"
	"path/filepath"
	"testing"

	"little-timer/internal/domain"
	"little-timer/internal/keystore"
	"little-timer/internal/keystore/keystoretest"
	"little-timer/internal/settings"
	"little-timer/internal/storage"
)

// 测试用内存 keystore 已去重到 keystoretest(master key 由 hasKey 开关控制,
// Seal/Unseal 用固定前缀做可逆变换,不依赖任何 keyring daemon,足以驱动
// SecretStorage v2 的真实磁盘 round-trip)。
//
// 编译期断言:满足 keystore.Keystore 全集,从而也能隐式传入 crypto.Keystore 子集。
var _ keystore.Keystore = keystoretest.FakeKeystore{}

// newSecretTestApp 构造一个接了真实 SQLite + SettingsManager 的 App,并把
// 注入用的 keystore 放进 a.Keystore,以便 Secrets()/CredentialStatus 惰性建
// store 时读取它。tdf 路径下的 secret.db 与 test.db 同级,t.TempDir 自动清理。
func newSecretTestApp(t *testing.T, ks keystore.Keystore) *App {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
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
	a := NewApp(domain.NewClockManager(domain.NewDefaultClockTaskConfig()), sm, sqlite, nil, dbPath)
	a.Keystore = ks
	return a
}

// 覆盖 round-trip:Store → Retrieve(内存)→ 从磁盘重建的第二实例(真实 Unseal)。
func TestSecrets_LoadsV2StoreRoundTrip(t *testing.T) {
	ks := keystoretest.New("secret-service", true)
	a := newSecretTestApp(t, ks)

	store := a.Secrets()
	if store == nil {
		t.Fatal("Secrets() = nil")
	}
	if err := store.Store([]byte("llm_api_key"), []byte("sk-secret-value")); err != nil {
		t.Fatalf("Store: %v", err)
	}
	got, err := store.Retrieve([]byte("llm_api_key"))
	if err != nil {
		t.Fatalf("Retrieve: %v", err)
	}
	if string(got) != "sk-secret-value" {
		t.Fatalf("Retrieve = %q, want sk-secret-value", got)
	}

	// 第二实例从同一磁盘 blob 读出(走真实 Unseal,非内存错觉)。
	reloaded := NewApp(a.Clock, a.Settings, a.SQLite, nil, a.DBPath)
	reloaded.Keystore = ks
	if _, err := os.Stat(filepath.Join(filepath.Dir(a.DBPath), "secret.db")); err != nil {
		t.Fatalf("secret.db 未落盘: %v", err)
	}
	got2, err := reloaded.Secrets().Retrieve([]byte("llm_api_key"))
	if err != nil {
		t.Fatalf("reloaded Retrieve: %v", err)
	}
	if string(got2) != "sk-secret-value" {
		t.Fatalf("reloaded Retrieve = %q, want sk-secret-value", got2)
	}
}

// Secrets() 接线 settings 的凭据流:App 建 store 后,Settings 保存能落 secrets。
func TestSecrets_WiresSettingsStore(t *testing.T) {
	ks := keystoretest.New("secret-service", true)
	a := newSecretTestApp(t, ks)
	_ = a.Secrets() // 触发 ensureSecrets → Settings.SetSecretStore 接线

	if err := a.Settings.UpdateBackupConfigFromJSON(
		`{"target_type":"webdav","webdav_url":"https://e/dav","webdav_password":"pw"}`,
	); err != nil {
		t.Fatalf("UpdateBackupConfigFromJSON(接线后应成功): %v", err)
	}
	cfg := a.Settings.BackupConfig()
	if cfg.WebDAVPassword != "pw" {
		t.Fatalf("BackupConfig().WebDAVPassword = %q, want pw(回填未生效?)", cfg.WebDAVPassword)
	}
}

func TestCredentialStatus_Normal(t *testing.T) {
	ks := keystoretest.New("secret-service", true)
	a := newSecretTestApp(t, ks)

	cs := a.CredentialStatus()
	if cs.Backend != "secret-service" {
		t.Errorf("Backend = %q, want secret-service(健康态取 keystore.Name())", cs.Backend)
	}
	if cs.HasSecret {
		t.Errorf("HasSecret = true, want false(新装无 secret)")
	}
}

func TestCredentialStatus_HasSecret(t *testing.T) {
	ks := keystoretest.New("secret-service", true)
	a := newSecretTestApp(t, ks)
	if err := a.Secrets().Store([]byte("llm_api_key"), []byte("sk-1")); err != nil {
		t.Fatalf("Store: %v", err)
	}

	cs := a.CredentialStatus()
	if cs.Backend != "secret-service" {
		t.Errorf("Backend = %q, want secret-service", cs.Backend)
	}
	if !cs.HasSecret {
		t.Errorf("HasSecret = false, want true(已存 secret)")
	}
}

// Review Focus #2:keystore item 缺失但 secret.db 文件存在 —— 必须报
// Backend="unavailable" + 非空 Detail,绝不假装"没有凭据"。
func TestCredentialStatus_UnavailableWhenFileExistsWithoutKey(t *testing.T) {
	ks := keystoretest.New("secret-service", true)
	a := newSecretTestApp(t, ks)
	// 先用有 key 的 keystore 落一个真实 blob。
	if err := a.Secrets().Store([]byte("llm_api_key"), []byte("sk-1")); err != nil {
		t.Fatalf("Store: %v", err)
	}

	// 换一个 key 缺失的 keystore,并让 App 重新构造 store(新建 App 复用同一 DBPath)。
	broke := keystoretest.New("secret-service", false)
	a2 := NewApp(a.Clock, a.Settings, a.SQLite, nil, a.DBPath)
	a2.Keystore = broke

	if _, err := os.Stat(filepath.Join(filepath.Dir(a.DBPath), "secret.db")); err != nil {
		t.Fatalf("前置条件:secret.db 应存在: %v", err)
	}
	cs := a2.CredentialStatus()
	if cs.Backend != "unavailable" {
		t.Errorf("Backend = %q, want unavailable", cs.Backend)
	}
	if cs.Detail == "" {
		t.Error("Detail 为空,应透出不可用原因(never pretend empty)")
	}
}
