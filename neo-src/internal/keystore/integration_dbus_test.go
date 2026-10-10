//go:build linux && integration_dbus

// integration_dbus_test.go —— 真实 org.freedesktop.secrets daemon 的集成测试。
//
// 需要运行中的密钥服务(session bus 上可达,或可由 dbus-daemon 激活)。
// 无 daemon 时 skip 并打印原因,绝不伪造通过。daemon 重启持久性子测试具有
// 破坏性(pkill gnome-keyring-daemon),默认 skip,仅当显式设置
// LT_IT_KEYRING_PASSWORD(登录密钥环解锁口令,经 stdin 传给 --unlock)时执行。
//
// 运行:go test -tags "linux integration_dbus" ./internal/keystore/
// 裁定(spec §12/§14):本测试只验证协议链路与持久性;item 落盘是否加密
// 由 keyring daemon 负责,不在 Go 测试断言。

package keystore

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"
)

// liveKeystoreOrSkip 探测 Available();daemon 不可用时 skip 并打印原因。
func liveKeystoreOrSkip(t *testing.T) Keystore {
	t.Helper()
	ks := Current()
	if err := ks.Available(); err != nil {
		t.Skipf("无可用的 org.freedesktop.secrets daemon,跳过集成测试: %v", err)
	}
	return ks
}

// TestIntegrationSecretServiceRoundtrip 真实 daemon 往返:
// Ensure → Has(true) → Seal/Unseal → Delete → Has(false)。
func TestIntegrationSecretServiceRoundtrip(t *testing.T) {
	ks := liveKeystoreOrSkip(t)

	// 清理可能的历史残留,并保证测试后不污染密钥环。
	if err := ks.DeleteMasterKey(); err != nil {
		t.Fatalf("前置清理 DeleteMasterKey = %v", err)
	}
	t.Cleanup(func() {
		if err := ks.DeleteMasterKey(); err != nil {
			t.Errorf("清理 DeleteMasterKey = %v", err)
		}
	})

	if has, err := ks.HasMasterKey(); err != nil || has {
		t.Fatalf("HasMasterKey() = (%v, %v), want (false, nil)", has, err)
	}

	if err := ks.EnsureMasterKey(); err != nil {
		t.Fatalf("EnsureMasterKey = %v", err)
	}
	if has, err := ks.HasMasterKey(); err != nil || !has {
		t.Fatalf("Ensure 后 HasMasterKey() = (%v, %v), want (true, nil)", has, err)
	}

	secret := []byte("集成测试:master key 托管往返 ✓")
	blob, err := ks.Seal(secret)
	if err != nil {
		t.Fatalf("Seal = %v", err)
	}
	// blob 布局:nonce(12)||ct||tag(16)。
	if got, want := len(blob), 12+len(secret)+16; got != want {
		t.Fatalf("blob 长度 = %d, want %d", got, want)
	}
	plain, err := ks.Unseal(blob)
	if err != nil {
		t.Fatalf("Unseal = %v", err)
	}
	if !bytes.Equal(plain, secret) {
		t.Fatalf("Unseal(Seal(x)) != x")
	}

	// 二次 Ensure 幂等:不得更换已托管的 key(旧 blob 仍可解)。
	if err := ks.EnsureMasterKey(); err != nil {
		t.Fatalf("二次 EnsureMasterKey = %v", err)
	}
	if _, err := ks.Unseal(blob); err != nil {
		t.Fatalf("二次 Ensure 后 Unseal = %v, want nil(key 应未变)", err)
	}

	// 篡改 1 字节 → ErrUnseal。
	bad := bytes.Clone(blob)
	bad[len(bad)-1] ^= 0x01
	if _, err := ks.Unseal(bad); !errors.Is(err, ErrUnseal) {
		t.Fatalf("篡改后 Unseal err = %v, want errors.Is(ErrUnseal)", err)
	}

	// Delete → Has(false) → Seal 报 ErrNotFound;Delete 幂等。
	if err := ks.DeleteMasterKey(); err != nil {
		t.Fatalf("DeleteMasterKey = %v", err)
	}
	if has, err := ks.HasMasterKey(); err != nil || has {
		t.Fatalf("Delete 后 HasMasterKey() = (%v, %v), want (false, nil)", has, err)
	}
	if _, err := ks.Seal([]byte("x")); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Delete 后 Seal err = %v, want errors.Is(ErrNotFound)", err)
	}
	if err := ks.DeleteMasterKey(); err != nil {
		t.Fatalf("幂等 DeleteMasterKey = %v", err)
	}
}

// TestIntegrationKeyPersistsDaemonRestart 验证 daemon 重启后 master key
// 仍在(持久化由 keyring daemon 负责落盘)。破坏性:会 pkill 当前会话的
// gnome-keyring-daemon 并用 LT_IT_KEYRING_PASSWORD 重新 --unlock,故默认 skip。
func TestIntegrationKeyPersistsDaemonRestart(t *testing.T) {
	password := os.Getenv("LT_IT_KEYRING_PASSWORD")
	if password == "" {
		t.Skip("跳过 daemon 重启持久性验证:需设置 LT_IT_KEYRING_PASSWORD(登录密钥环口令)" +
			"并确认本会话的 gnome-keyring-daemon 可被安全 kill/重启")
	}
	ks := liveKeystoreOrSkip(t)

	if err := ks.DeleteMasterKey(); err != nil {
		t.Fatalf("前置清理 DeleteMasterKey = %v", err)
	}
	t.Cleanup(func() {
		if err := ks.DeleteMasterKey(); err != nil {
			t.Errorf("清理 DeleteMasterKey = %v", err)
		}
	})

	if err := ks.EnsureMasterKey(); err != nil {
		t.Fatalf("EnsureMasterKey = %v", err)
	}
	secret := []byte("重启前后我都在")
	blob, err := ks.Seal(secret)
	if err != nil {
		t.Fatalf("Seal = %v", err)
	}

	// kill 当前 daemon(名字消失 = 确认已退出)。
	// 注意:daemon 进程 comm 被截断为 15 字符("gnome-keyring-d"),
	// 故用 -f 匹配完整命令行而非 -x 匹配进程名。
	if out, err := exec.Command("pkill", "-f", "^gnome-keyring-daemon").CombinedOutput(); err != nil {
		t.Skipf("pkill gnome-keyring-daemon 失败,跳过重启验证: %v: %s", err, out)
	}
	if !waitBusName(t, false, 5*time.Second) {
		t.Fatal("pkill 后 org.freedesktop.secrets 名称未消失")
	}

	// 用口令经 stdin 重新解锁启动 daemon(不经 shell,口令不作为参数暴露)。
	start := exec.Command("gnome-keyring-daemon", "--unlock", "--daemonize")
	start.Stdin = strings.NewReader(password)
	start.Env = os.Environ()
	if out, err := start.CombinedOutput(); err != nil {
		t.Fatalf("重启 gnome-keyring-daemon 失败: %v: %s", err, out)
	}
	if !waitBusName(t, true, 10*time.Second) {
		t.Fatal("重启后 org.freedesktop.secrets 名称未回归")
	}

	// key 仍在且可解出同一 master key(旧 blob 认证通过)。
	if has, err := ks.HasMasterKey(); err != nil || !has {
		t.Fatalf("重启后 HasMasterKey() = (%v, %v), want (true, nil)", has, err)
	}
	plain, err := ks.Unseal(blob)
	if err != nil {
		t.Fatalf("重启后 Unseal = %v, want nil(key 应持久且未变)", err)
	}
	if !bytes.Equal(plain, secret) {
		t.Fatalf("重启后 Unseal 结果 != 原文")
	}
}

// waitBusName 轮询 session bus 上 org.freedesktop.secrets 名称的有/无。
// 只查询 ListNames,绝不触发 StartServiceByName 激活(避免绕过 --unlock)。
func waitBusName(t *testing.T, wantPresent bool, timeout time.Duration) bool {
	t.Helper()
	conn, err := dbus.SessionBus()
	if err != nil {
		t.Fatalf("dbus.SessionBus = %v", err)
	}
	deadline := time.Now().Add(timeout)
	for {
		var names []string
		if err := conn.BusObject().Call("org.freedesktop.DBus.ListNames", 0).Store(&names); err == nil {
			present := false
			for _, n := range names {
				if n == secretBusName {
					present = true
					break
				}
			}
			if present == wantPresent {
				return true
			}
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(100 * time.Millisecond)
	}
}
