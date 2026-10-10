//go:build linux && integration_dbus

// integration_dbus_test.go —— 真实 org.freedesktop.secrets daemon 的集成测试。
//
// 需要运行中的密钥服务(session bus 上可达,或可由 dbus-daemon 激活)。
// 无 daemon 时 skip 并打印原因,绝不伪造通过。
//
// 破坏性保护(opt-in 环境变量):
//   - LT_IT_DESTROY_MASTER_KEY=1:授权删除密钥环中「已存在」的 little_timer
//     master key(前置/后置清理)。若未设置且 HasMasterKey() 为真,所有会删除
//     既有 key 的测试一律 skip——防止在本机(装有正式应用真实 keyring)上
//     运行 `go test -tags "linux integration_dbus"` 时悄悄使历史密封数据不可恢复。
//   - LT_IT_KEYRING_PASSWORD=<登录密钥环口令>:额外授权 daemon 重启子测试
//     (pkill + --unlock 重启,破坏性)。口令经 stdin 传给 --unlock,不经 shell。
//     重启子测试在 pkill 之前注册 t.Cleanup,任何失败路径都会尝试用口令复活
//     本会话 daemon,绝不把用户留在 daemon 已死的状态。
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

// envAllowDestroy 是否显式授权删除密钥环中已存在的 master key。
func envAllowDestroy() bool {
	return os.Getenv("LT_IT_DESTROY_MASTER_KEY") == "1"
}

// liveKeystoreOrSkip 探测 Available();daemon 不可用时 skip 并打印原因。
func liveKeystoreOrSkip(t *testing.T) Keystore {
	t.Helper()
	ks := Current()
	if err := ks.Available(); err != nil {
		t.Skipf("无可用的 org.freedesktop.secrets daemon,跳过集成测试: %v", err)
	}
	return ks
}

// clearMasterKeyForTest 执行前置清理 DeleteMasterKey;若 keyring 中已有
// master key 且未授权破坏性删除,则 skip 本测试(绝不触碰既有 key)。
// key 不存在时 DeleteMasterKey 幂等,任何情况下都可安全执行。
func clearMasterKeyForTest(t *testing.T, ks Keystore) {
	t.Helper()
	has, err := ks.HasMasterKey()
	if err != nil {
		t.Fatalf("前置检查 HasMasterKey() = (%v, %v)", has, err)
	}
	if has && !envAllowDestroy() {
		t.Skip("密钥环已存在 little_timer master key(可能是正式应用托管的真实 key)," +
			"前置删除会使其历史密封数据不可恢复;如确认要销毁请设置 LT_IT_DESTROY_MASTER_KEY=1 重跑")
	}
	if err := ks.DeleteMasterKey(); err != nil {
		t.Fatalf("前置清理 DeleteMasterKey = %v", err)
	}
	t.Cleanup(func() {
		if err := ks.DeleteMasterKey(); err != nil {
			t.Errorf("清理 DeleteMasterKey = %v", err)
		}
	})
}

// TestIntegrationSecretServiceRoundtrip 真实 daemon 往返:
// Ensure → Has(true) → Seal/Unseal → Delete → Has(false)。
// 前置清理仅在 LT_IT_DESTROY_MASTER_KEY=1 时允许删除既有 key,否则 skip。
func TestIntegrationSecretServiceRoundtrip(t *testing.T) {
	ks := liveKeystoreOrSkip(t)

	// 清理可能的历史残留,并保证测试后不污染密钥环(破坏性部分受 opt-in 门控)。
	clearMasterKeyForTest(t, ks)

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
// 仍在(持久化由 keyring daemon 负责落盘)。双重 opt-in:
//   - LT_IT_DESTROY_MASTER_KEY=1:允许前置删除既有 master key;
//   - LT_IT_KEYRING_PASSWORD:登录密钥环解锁口令,授权 pkill + --unlock 重启
//     daemon(本会话破坏性操作)。
//
// pkill 之前即注册复活 cleanup:无论测试在哪条路径失败,t.Cleanup 都会尝试
// 用口令 --unlock 重启 daemon,不留死 daemon 给会话。
func TestIntegrationKeyPersistsDaemonRestart(t *testing.T) {
	password := os.Getenv("LT_IT_KEYRING_PASSWORD")
	if password == "" {
		t.Skip("跳过 daemon 重启持久性验证:需设置 LT_IT_KEYRING_PASSWORD(登录密钥环口令)" +
			"并确认本会话的 gnome-keyring-daemon 可被安全 kill/重启")
	}
	if !envAllowDestroy() {
		t.Skip("跳过 daemon 重启持久性验证:前置清理会删除既有 master key," +
			"需同时设置 LT_IT_DESTROY_MASTER_KEY=1 明确授权销毁")
	}
	ks := liveKeystoreOrSkip(t)

	clearMasterKeyForTest(t, ks)

	// 在任何 pkill 之前注册 daemon 复活 cleanup:本测试对 daemon 的 kill/重启
	// 均由此 cleanup 兜底,失败路径(t.Fatal)同样执行(LIFO:先复活 daemon,
	// 再清理测试 key)。
	t.Cleanup(func() {
		reviveKeyringDaemon(t, password)
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
	startKeyringDaemon(t, password)
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

// startKeyringDaemon 用口令经 stdin 以 --unlock --daemonize 启动
// gnome-keyring-daemon(不经 shell,口令不作为参数暴露)。失败时 Fatalf,
// 仅限测试主体在已授权重启的语境下调用。
func startKeyringDaemon(t *testing.T, password string) {
	t.Helper()
	start := exec.Command("gnome-keyring-daemon", "--unlock", "--daemonize")
	start.Stdin = strings.NewReader(password)
	start.Env = os.Environ()
	if out, err := start.CombinedOutput(); err != nil {
		t.Fatalf("重启 gnome-keyring-daemon 失败: %v: %s", err, out)
	}
}

// reviveKeyringDaemon 尽力(daemon 语义下非断言)复活本会话 keyring daemon:
// 名字仍在则无需动作;否则用口令 --unlock 重启并在 10s 内轮询名字回归。
// 仅打日志报错(t.Logf),绝不在 cleanup 里 Fatal——它存在的意义就是不把
// 会话留在 daemon 已死的状态。
func reviveKeyringDaemon(t *testing.T, password string) {
	t.Helper()
	conn, err := dbus.SessionBus()
	if err != nil {
		t.Logf("复活 daemon 失败(无法连接 session bus,daemon 可能已死): %v", err)
		return
	}
	namePresent := func() bool {
		var names []string
		if err := conn.BusObject().Call("org.freedesktop.DBus.ListNames", 0).Store(&names); err != nil {
			return false
		}
		for _, n := range names {
			if n == secretBusName {
				return true
			}
		}
		return false
	}
	if namePresent() {
		return
	}
	start := exec.Command("gnome-keyring-daemon", "--unlock", "--daemonize")
	start.Stdin = strings.NewReader(password)
	start.Env = os.Environ()
	if out, err := start.CombinedOutput(); err != nil {
		t.Logf("cleanup 复活 gnome-keyring-daemon 失败(daemon 可能已死,需手工重启): %v: %s", err, out)
		return
	}
	deadline := time.Now().Add(10 * time.Second)
	for !namePresent() {
		if time.Now().After(deadline) {
			t.Logf("cleanup 已执行 --unlock 但 org.freedesktop.secrets 未回归,daemon 可能未就绪")
			return
		}
		time.Sleep(100 * time.Millisecond)
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
