// secret_v2_test.go —— SecretStorage v2(keystore envelope 薄壳)行为测试。
//
// 覆盖 brief Step 1 的场景矩阵:Store/Retrieve/Delete/Clear/Items 往返
// (含跨实例重新加载,即真实 Unseal 路径)、文件不存在→空 cache 且首次
// Store 落盘、magic 前缀逐字节 "LTMSEKv2"、落盘权限 0o600、Unseal 失败
// 透出 ErrKeystoreUnavailable、master key 缺失时的不可用行为,以及
// "假 Unseal 不校验 tag" 的铁证测试。
package crypto_test

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"

	"little-timer/internal/crypto"
	"little-timer/internal/keystore"
)

// v2 磁盘 blob 的 magic,按 brief 要求逐字节断言。
const wantMagic = "LTMSEKv2"

type secretKV struct{ key, value string }

var roundTripPairs = []secretKV{
	{"api_key", "sk-live-abcdef"},
	{"webdav_password", "p@ss/含中文与符号!?"},
	{"s3_access_key", "AKIA0123456789"},
	{"empty_value", ""},
}

// TestV2SecretRoundTrip 表驱动覆盖五类操作的往返,并在每一步后用新实例
// 从磁盘重新加载验证(新实例构造 → loadV2 → Unseal,证明落盘的确实
// 是 keystore 密封后的数据,而不是内存错觉)。
func TestV2SecretRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "secret.db")
	f := newFakeKeystore(t)
	store := crypto.NewSecretStorage(path, f)

	steps := []struct {
		name string
		run  func(t *testing.T)
	}{
		{"StoreRetrieve", func(t *testing.T) {
			for _, p := range roundTripPairs {
				if err := store.Store([]byte(p.key), []byte(p.value)); err != nil {
					t.Fatalf("Store %q: %v", p.key, err)
				}
			}
			for _, p := range roundTripPairs {
				got, err := store.Retrieve([]byte(p.key))
				if err != nil {
					t.Fatalf("Retrieve %q: %v", p.key, err)
				}
				if string(got) != p.value {
					t.Fatalf("Retrieve %q = %q, want %q", p.key, got, p.value)
				}
			}
			// Retrieve 返回副本:改写返回值不能污染存储。
			got, err := store.Retrieve([]byte("api_key"))
			if err != nil {
				t.Fatalf("Retrieve api_key: %v", err)
			}
			got[0] = 'X'
			again, err := store.Retrieve([]byte("api_key"))
			if err != nil {
				t.Fatalf("Retrieve api_key again: %v", err)
			}
			if string(again) != "sk-live-abcdef" {
				t.Fatalf("stored value mutated by caller copy: %q", again)
			}
		}},
		{"Overwrite", func(t *testing.T) {
			if err := store.Store([]byte("api_key"), []byte("sk-rotated")); err != nil {
				t.Fatalf("Store overwrite: %v", err)
			}
			got, err := store.Retrieve([]byte("api_key"))
			if err != nil || string(got) != "sk-rotated" {
				t.Fatalf("Retrieve after overwrite = %q, %v", got, err)
			}
		}},
		{"Items", func(t *testing.T) {
			want := []string{"api_key", "empty_value", "s3_access_key", "webdav_password"}
			got, err := store.Items()
			if err != nil {
				t.Fatalf("Items: %v", err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("Items = %v, want %v", got, want)
			}
		}},
		{"ReloadViaUnseal", func(t *testing.T) {
			reloaded := crypto.NewSecretStorage(path, f)
			for _, want := range []secretKV{
				{"api_key", "sk-rotated"},
				{"webdav_password", "p@ss/含中文与符号!?"},
				{"s3_access_key", "AKIA0123456789"},
				{"empty_value", ""},
			} {
				got, err := reloaded.Retrieve([]byte(want.key))
				if err != nil {
					t.Fatalf("reloaded Retrieve %q: %v", want.key, err)
				}
				if string(got) != want.value {
					t.Fatalf("reloaded Retrieve %q = %q, want %q", want.key, got, want.value)
				}
			}
		}},
		{"Delete", func(t *testing.T) {
			if err := store.Delete([]byte("s3_access_key")); err != nil {
				t.Fatalf("Delete: %v", err)
			}
			if _, err := store.Retrieve([]byte("s3_access_key")); !errors.Is(err, crypto.ErrSecretNotFound) {
				t.Fatalf("Retrieve after Delete = %v, want ErrSecretNotFound", err)
			}
			// 删除不存在的 key 是 no-op。
			if err := store.Delete([]byte("never_existed")); err != nil {
				t.Fatalf("Delete missing key: %v", err)
			}
			reloaded := crypto.NewSecretStorage(path, f)
			if _, err := reloaded.Retrieve([]byte("s3_access_key")); !errors.Is(err, crypto.ErrSecretNotFound) {
				t.Fatalf("reloaded Retrieve deleted key = %v, want ErrSecretNotFound", err)
			}
		}},
		{"Clear", func(t *testing.T) {
			if err := store.Clear(); err != nil {
				t.Fatalf("Clear: %v", err)
			}
			got, err := store.Items()
			if err != nil {
				t.Fatalf("Items after Clear: %v", err)
			}
			if len(got) != 0 {
				t.Fatalf("Items after Clear = %v, want empty", got)
			}
			reloaded := crypto.NewSecretStorage(path, f)
			names, err := reloaded.Items()
			if err != nil {
				t.Fatalf("reloaded Items after Clear: %v", err)
			}
			if len(names) != 0 {
				t.Fatalf("reloaded Items after Clear = %v, want empty", names)
			}
		}},
	}

	for _, st := range steps {
		t.Run(st.name, st.run)
	}
}

// TestV2FirstStoreWritesMagicAndPerms 验证:文件不存在 → 构造出空 cache
// (不报错);首次 Store 落盘;磁盘 blob 前缀逐字节等于 "LTMSEKv2";
// 权限 0o600;原子写的 .tmp 中间文件不残留。
func TestV2FirstStoreWritesMagicAndPerms(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "secret.db")
	f := newFakeKeystore(t)

	store := crypto.NewSecretStorage(path, f)
	names, err := store.Items()
	if err != nil {
		t.Fatalf("Items on fresh store: %v", err)
	}
	if len(names) != 0 {
		t.Fatalf("fresh Items = %v, want empty", names)
	}
	if _, err := store.Retrieve([]byte("anything")); !errors.Is(err, crypto.ErrSecretNotFound) {
		t.Fatalf("Retrieve on fresh store = %v, want ErrSecretNotFound", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("构造不应创建文件,stat = %v", err)
	}

	if err := store.Store([]byte("k"), []byte("v")); err != nil {
		t.Fatalf("first Store: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !bytes.HasPrefix(data, []byte(wantMagic)) {
		t.Fatalf("磁盘 blob 前缀 = %q, want %q", data[:8], wantMagic)
	}
	if bytes.HasPrefix(data, []byte("LTMSECv1")) {
		t.Fatal("磁盘 blob 仍是 v1 magic")
	}
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if perm := fi.Mode().Perm(); perm != 0o600 {
		t.Fatalf("文件权限 = %o, want 600", perm)
	}
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Fatalf("临时文件 %s.tmp 未清理: %v", path, err)
	}
}

// TestV2InMemoryOnly 验证 filePath=="" 的纯内存模式:操作正常、无磁盘副作用。
func TestV2InMemoryOnly(t *testing.T) {
	f := newFakeKeystore(t)
	store := crypto.NewSecretStorage("", f)
	if err := store.Store([]byte("k"), []byte("v")); err != nil {
		t.Fatalf("Store in-memory: %v", err)
	}
	got, err := store.Retrieve([]byte("k"))
	if err != nil || string(got) != "v" {
		t.Fatalf("Retrieve in-memory = %q, %v", got, err)
	}
}

// TestV2UnsealFailureSurfacesKeystoreUnavailable 覆盖 Review Focus #2:
// 文件在、但 keystore 解不开(哨兵错误 / 认证失败 / key 被换)时,
// Retrieve 与 Items 必须透出 ErrKeystoreUnavailable,不得静默返回空。
func TestV2UnsealFailureSurfacesKeystoreUnavailable(t *testing.T) {
	cases := []struct {
		name string
		// 用写文件的正常 fake 之外的第二个 fake 重新加载同一文件。
		hook func(t *testing.T) crypto.Keystore
		// storeFails=true 表示该 fake 连 Seal 也会失败(无 master key),
		// Store 必须透出 ErrKeystoreUnavailable 而不是静默写盘。
		storeFails bool
	}{
		{
			name: "UnsealSentinelErrUnseal",
			hook: func(t *testing.T) crypto.Keystore {
				f := newFakeKeystore(t)
				f.unsealErr = fmt.Errorf("%w: 模拟 keyring 重置", keystore.ErrUnseal)
				return f
			},
		},
		{
			// 新 key 解旧 blob:走真实 crypto.Decrypt 的认证失败路径。
			name: "MasterKeyReplaced",
			hook: func(t *testing.T) crypto.Keystore {
				return newFakeKeystore(t)
			},
		},
		{
			name:       "HasMasterKeyFalse",
			hook:       func(t *testing.T) crypto.Keystore { return newFakeKeystoreNoKey(t) },
			storeFails: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "secret.db")
			writer := newFakeKeystore(t)
			store := crypto.NewSecretStorage(path, writer)
			if err := store.Store([]byte("api_key"), []byte("sk-1")); err != nil {
				t.Fatalf("Store: %v", err)
			}

			broken := tc.hook(t)
			reloaded := crypto.NewSecretStorage(path, broken)
			if _, err := reloaded.Retrieve([]byte("api_key")); !errors.Is(err, crypto.ErrKeystoreUnavailable) {
				t.Fatalf("Retrieve = %v, want ErrKeystoreUnavailable", err)
			}
			if _, err := reloaded.Items(); !errors.Is(err, crypto.ErrKeystoreUnavailable) {
				t.Fatalf("Items = %v, want ErrKeystoreUnavailable", err)
			}
			if tc.storeFails {
				if err := reloaded.Store([]byte("k2"), []byte("v2")); !errors.Is(err, crypto.ErrKeystoreUnavailable) {
					t.Fatalf("Store 在无 master key 时 = %v, want ErrKeystoreUnavailable", err)
				}
			}
		})
	}
}

// TestV2CorruptFileIsNotSilentlyEmpty 验证:magic 不匹配 / 垃圾文件不能
// 被当成"没有 secret",必须透出解密失败。
func TestV2CorruptFileIsNotSilentlyEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "secret.db")
	// 模拟 v1 遗留文件:前缀 LTMSECv1 —— v2 薄壳拒绝读取(迁移归 Task 8)。
	if err := os.WriteFile(path, []byte("LTMSECv1-some-legacy-bytes"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	f := newFakeKeystore(t)
	store := crypto.NewSecretStorage(path, f)
	if _, err := store.Retrieve([]byte("k")); !errors.Is(err, crypto.ErrSecretDecryptionFailed) {
		t.Fatalf("Retrieve on v1 file = %v, want ErrSecretDecryptionFailed", err)
	}
	if _, err := store.Items(); !errors.Is(err, crypto.ErrSecretDecryptionFailed) {
		t.Fatalf("Items on v1 file = %v, want ErrSecretDecryptionFailed", err)
	}
}

// TestV2NoVerifyUnsealIsDetected —— 铁证 / 反同义反复:把 fake 的 Unseal
// 换成"不校验 tag、恒返回原 blob"后,跨实例 round-trip 必须失败。
// 若本测试在 noVerify 下仍然"通过",说明 round-trip 测试根本没走解密。
func TestV2NoVerifyUnsealIsDetected(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "secret.db")
	writer := newFakeKeystore(t)
	store := crypto.NewSecretStorage(path, writer)
	if err := store.Store([]byte("api_key"), []byte("sk-secret-value")); err != nil {
		t.Fatalf("Store: %v", err)
	}

	lazy := newFakeKeystore(t)
	lazy.unsealNoVerify = true // 同一个 master key,但 Unseal 假装成功、原样返回
	reloaded := crypto.NewSecretStorage(path, lazy)
	got, err := reloaded.Retrieve([]byte("api_key"))
	if err == nil {
		t.Fatalf("noVerify fake 下 Retrieve 竟然成功(%q)—— round-trip 没有真正解密", got)
	}
	if errors.Is(err, crypto.ErrKeystoreUnavailable) {
		t.Fatalf("noVerify fake 的 Unseal 没报错,失败原因应是解密/解析而非 keystore 不可用: %v", err)
	}
	if !errors.Is(err, crypto.ErrSecretDecryptionFailed) {
		t.Fatalf("Retrieve = %v, want ErrSecretDecryptionFailed(密封 blob 当 JSON 解析必炸)", err)
	}
}

// TestSecretStorage_ConcurrentStoreRetrieve —— Review Focus #4:worker 与
// HTTP 同进程交错 Store/Retrieve。断言:
//  1. 每次 Retrieve 拿到的都是本 goroutine 最后一次写入的完整值(GCM +
//     JSON 原子性保证读到的永不是半包,nonce 每次新生成不复用);
//  2. 结束后内存 map 完整(两个 key 都在,值为末次写入);
//  3. 磁盘文件可用同 key 的干净 fake 重新解开且内容一致。
//
// 以 -race 运行(go test -race ./internal/crypto/)。
func TestSecretStorage_ConcurrentStoreRetrieve(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "secret.db")
	f := newFakeKeystore(t)
	store := crypto.NewSecretStorage(path, f)

	const iters = 1000
	var wg sync.WaitGroup
	for g := 0; g < 2; g++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			// 每 goroutine 独占一个 key:交错只发生在锁与写盘上,
			// 因此 Store 后 Retrieve 必须拿到刚写入的完整值。
			key := fmt.Sprintf("g%d_key", id)
			for n := 0; n < iters; n++ {
				val := fmt.Sprintf("v%d-%05d", id, n)
				if err := store.Store([]byte(key), []byte(val)); err != nil {
					t.Errorf("goroutine %d Store #%d: %v", id, n, err)
					return
				}
				got, err := store.Retrieve([]byte(key))
				if err != nil {
					t.Errorf("goroutine %d Retrieve #%d: %v", id, n, err)
					return
				}
				if string(got) != val {
					t.Errorf("goroutine %d Retrieve #%d = %q, want %q(疑似撕裂读)", id, n, got, val)
					return
				}
			}
		}(g)
	}
	wg.Wait()

	// (2) 最终 map 完整:key 恰为两个,值可解析为 v{g}-***** 格式。
	names, err := store.Items()
	if err != nil {
		t.Fatalf("Items after concurrent run: %v", err)
	}
	if !reflect.DeepEqual(names, []string{"g0_key", "g1_key"}) {
		t.Fatalf("Items = %v, want [g0_key g1_key]", names)
	}

	// (3) 文件可解:用持有同一 master key 的 fake 重新加载 → 两个 key 都
	// 在,值形如 v%d-%05d(末次写入顺序由交错决定,只断言格式完整)。
	reloaded := crypto.NewSecretStorage(path, f)
	for _, key := range names {
		got, err := reloaded.Retrieve([]byte(key))
		if err != nil {
			t.Fatalf("reloaded Retrieve %q: %v", key, err)
		}
		var id, n int
		if _, err := fmt.Sscanf(string(got), "v%d-%05d", &id, &n); err != nil {
			t.Fatalf("reloaded value %q not a complete write: %v", got, err)
		}
		if id < 0 || id > 1 || n < 0 || n >= iters {
			t.Fatalf("reloaded value out of range: %q", got)
		}
	}
}
