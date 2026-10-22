//go:build !android

// keystore_androidcore_test.go —— Android KeystoreBridge 后端的纯逻辑单元测试。
//
// 设计:JNI 传输层(keystore_android.go,tag `android`)是 androidBridge
// 接口的一个薄适配器;全部可测逻辑(blob 布局校验、bridge 错误字符串 →
// 哨兵错误映射、Keystore 方法行为)位于无 tag 的
// keystore_android_core.go,注入 fake bridge 后可在 linux 主机上运行。
// 本测试文件 tag `!android`,与 keystore_android_stub.go 的编译面一致
// (android 真机构建用 JNI 传输,不需要 fake)。
//
// Java 侧(KeystoreBridge.java)在本仓库没有 Java 测试运行器,仅经
// javac/gradle 编译门 + 逐行自审;真机行为列入人工验收。

package keystore

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"testing"
)

// fakeAndroidBridge 是 androidBridge 的可注入替身。
type fakeAndroidBridge struct {
	reach  bool
	ensure error
	hasV   bool
	hasE   error
	sealV  []byte
	sealE  error
	sealIn []byte // 捕获最近一次 seal 入参
	unslV  []byte
	unslE  error
	unslIn []byte // 捕获最近一次 unseal 入参
	delE   error

	// 调用计数:断言幂等语义(hasKey 为 true 时不再请求生成)。
	ensureCalls int
	hasCalls    int
	sealCalls   int
	unsealCalls int
}

func (f *fakeAndroidBridge) reachable() bool { return f.reach }
func (f *fakeAndroidBridge) ensureKey() error {
	f.ensureCalls++
	return f.ensure
}
func (f *fakeAndroidBridge) hasKey() (bool, error) {
	f.hasCalls++
	return f.hasV, f.hasE
}
func (f *fakeAndroidBridge) seal(p []byte) ([]byte, error) {
	f.sealCalls++
	f.sealIn = append([]byte(nil), p...)
	return f.sealV, f.sealE
}
func (f *fakeAndroidBridge) unseal(b []byte) ([]byte, error) {
	f.unsealCalls++
	f.unslIn = append([]byte(nil), b...)
	return f.unslV, f.unslE
}
func (f *fakeAndroidBridge) deleteKey() error { return f.delE }

func newFake() *fakeAndroidBridge { return &fakeAndroidBridge{reach: true} }

// TestAndroidName 断言后端名与 brief 完全一致。
func TestAndroidName(t *testing.T) {
	if got := newAndroidKeystoreWithBridge(newFake()).Name(); got != "android-keystore" {
		t.Fatalf("Name() = %q, want %q", got, "android-keystore")
	}
}

// TestAndroidAvailable 断言 bridge 可达/不可达时的 Available 行为:
// 不可达必须包 ErrUnavailable 且带可执行的中文指引。
func TestAndroidAvailable(t *testing.T) {
	ok := newAndroidKeystoreWithBridge(newFake())
	if err := ok.Available(); err != nil {
		t.Fatalf("Available(reachable) = %v, want nil", err)
	}

	dead := &fakeAndroidBridge{reach: false}
	err := newAndroidKeystoreWithBridge(dead).Available()
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("Available(disconnected) err = %v, want ErrUnavailable", err)
	}
	if !containsCJK(err.Error()) {
		t.Fatalf("Available 错误信息应带中文指引,got %q", err.Error())
	}
}

// TestAndroidSplitBlob 断言 iv(12)||ct||tag 布局校验:合法 blob 拆出
// 12 字节 iv 与 ct||tag;长度不足 iv+tag(28B)返回 ErrUnseal。
func TestAndroidSplitBlob(t *testing.T) {
	blob := bytes.Repeat([]byte{0x11}, 12+5+16)
	iv, ctTag, err := splitAndroidBlob(blob)
	if err != nil {
		t.Fatalf("splitAndroidBlob = %v, want nil", err)
	}
	if len(iv) != 12 || !bytes.Equal(ctTag, blob[12:]) {
		t.Fatalf("iv/ctTag 拆分错误: len(iv)=%d", len(iv))
	}

	for _, short := range []int{0, 1, 12, 27} {
		if _, _, err := splitAndroidBlob(bytes.Repeat([]byte{0x22}, short)); !errors.Is(err, ErrUnseal) {
			t.Fatalf("splitAndroidBlob(len=%d) err = %v, want ErrUnseal", short, err)
		}
	}
	// 恰为 28B(iv+tag,空明文)合法。
	if _, _, err := splitAndroidBlob(bytes.Repeat([]byte{0x33}, 28)); err != nil {
		t.Fatalf("splitAndroidBlob(28) = %v, want nil", err)
	}
}

// TestAndroidMapBridgeError 断言 bridge 错误字符串到哨兵错误的映射表。
func TestAndroidMapBridgeError(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want error
	}{
		{"nil", nil, nil},
		{"not_found 标记", errors.New("android keystore bridge: java.security.KeyStoreException|KEY_NOT_FOUND: alias missing"), ErrNotFound},
		{"unseal 标记", errors.New("android keystore bridge: javax.crypto.AEADBadTagException|UNSEAL_FAILED: tag mismatch"), ErrUnseal},
		{"断连标记", errors.New("android keystore bridge: BRIDGE_DISCONNECTED|JVM unavailable"), ErrUnavailable},
		{"断连哨兵", errBridgeDisconnected, ErrUnavailable},
		// 不可恢复别名(alias 存在但密钥损坏)→ 独立标记,映射 ErrUnavailable
		// 而非 ErrNotFound,否则 hasKey 短路让 EnsureMasterKey 成功、Seal 永远
		// ErrNotFound 形成无恢复路径的死锁。
		{"不可恢复别名标记", errors.New("android keystore bridge: java.security.UnrecoverableKeyException|KEY_UNRECOVERABLE: keystore corrupted"), ErrUnavailable},
		{"未知异常 → 不可用", errors.New("android keystore bridge: java.lang.IllegalStateException|null"), ErrUnavailable},
		// 已包装哨兵的错误原样透传,不重复套壳。
		{"透传 ErrUnseal", fmtErrUnseal("blob 长度不足"), ErrUnseal},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := mapAndroidError(tc.err)
			if tc.err == nil {
				if got != nil {
					t.Fatalf("mapAndroidError(nil) = %v, want nil", got)
				}
				return
			}
			if !errors.Is(got, tc.want) {
				t.Fatalf("mapAndroidError(%v) = %v, want errors.Is(.., %v)", tc.err, got, tc.want)
			}
		})
	}
}

// TestAndroidMapErrorGuidanceTail 钉住两条 ErrUnavailable 路径都携带可执行的
// 中文指引尾(卸载/清除数据不可恢复):KEY_UNRECOVERABLE(不可恢复别名)与
// 未知 Java 异常(default 分支)。同时断言 KEY_UNRECOVERABLE 绝不映射为
// ErrNotFound(否则恢复路径被封死)。
func TestAndroidMapErrorGuidanceTail(t *testing.T) {
	const guidanceSub = "卸载/清除数据将不可恢复地丢失已存凭据"
	cases := map[string]error{
		"不可恢复别名标记": errors.New("android keystore bridge: java.security.UnrecoverableKeyException|KEY_UNRECOVERABLE: keystore corrupted"),
		"未知异常":     errors.New("android keystore bridge: java.lang.IllegalStateException|boom"),
	}
	for name, in := range cases {
		t.Run(name, func(t *testing.T) {
			got := mapAndroidError(in)
			if !errors.Is(got, ErrUnavailable) {
				t.Fatalf("mapAndroidError(%v) = %v, want errors.Is(ErrUnavailable)", in, got)
			}
			if errors.Is(got, ErrNotFound) {
				t.Fatalf("不可恢复/未知异常绝不应映射为 ErrNotFound,got %v", got)
			}
			if !strings.Contains(got.Error(), guidanceSub) {
				t.Fatalf("err text = %q, want 含指引子串 %q", got.Error(), guidanceSub)
			}
		})
	}
}

// TestAndroidSealUnseal 断言 Seal/Unseal 的入参出参透传与错误映射。
func TestAndroidSealUnseal(t *testing.T) {
	f := newFake()
	f.sealV = bytes.Repeat([]byte{0xaa}, 12+3+16)
	f.unslV = []byte("plain")
	ks := newAndroidKeystoreWithBridge(f)

	blob, err := ks.Seal([]byte("secret"))
	if err != nil {
		t.Fatalf("Seal = %v", err)
	}
	if !bytes.Equal(blob, f.sealV) {
		t.Fatal("Seal 应透传 bridge 返回的 iv||ct||tag blob")
	}
	if !bytes.Equal(f.sealIn, []byte("secret")) {
		t.Fatal("Seal 未把明文交给 bridge")
	}

	plain, err := ks.Unseal(blob)
	if err != nil {
		t.Fatalf("Unseal = %v", err)
	}
	if !bytes.Equal(plain, []byte("plain")) || !bytes.Equal(f.unslIn, blob) {
		t.Fatal("Unseal 透传错误")
	}

	// blob 过短:不发 JNI,直接 ErrUnseal。
	f2 := newFake()
	ks2 := newAndroidKeystoreWithBridge(f2)
	if _, err := ks2.Unseal([]byte{1, 2, 3}); !errors.Is(err, ErrUnseal) {
		t.Fatalf("短 blob Unseal err = %v, want ErrUnseal", err)
	}
	if f2.unslIn != nil {
		t.Fatal("短 blob 不应打到 bridge")
	}

	// bridge 报 KEY_NOT_FOUND → ErrNotFound;报 UNSEAL_FAILED → ErrUnseal。
	f.sealE = errors.New("android keystore bridge: java.security.KeyStoreException|KEY_NOT_FOUND: no alias")
	if _, err := ks.Seal([]byte("x")); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Seal KEY_NOT_FOUND err = %v, want ErrNotFound", err)
	}
	f.sealE = nil
	// KEY_UNRECOVERABLE(alias 存在但密钥损坏)→ ErrUnavailable 而非 ErrNotFound。
	f.sealE = errors.New("android keystore bridge: java.security.UnrecoverableKeyException|KEY_UNRECOVERABLE: corrupted")
	if _, err := ks.Seal([]byte("x")); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("Seal KEY_UNRECOVERABLE err = %v, want ErrUnavailable", err)
	}
	f.sealE = nil
	f.unslE = errors.New("android keystore bridge: javax.crypto.AEADBadTagException|UNSEAL_FAILED: bad tag")
	if _, err := ks.Unseal(blob); !errors.Is(err, ErrUnseal) {
		t.Fatalf("Unseal UNSEAL_FAILED err = %v, want ErrUnseal", err)
	}

	// bridge 断连:Seal/Unseal 均 ErrUnavailable。
	dead := newAndroidKeystoreWithBridge(&fakeAndroidBridge{reach: false})
	if _, err := dead.Seal([]byte("x")); !errors.Is(err, ErrUnavailable) {
		t.Fatal("断连 Seal 应 ErrUnavailable")
	}
	if _, err := dead.Unseal(blob); !errors.Is(err, ErrUnavailable) {
		t.Fatal("断连 Unseal 应 ErrUnavailable")
	}
}

// TestAndroidKeyLifecycle 断言 Ensure/Has/Delete 的转发与幂等语义。
func TestAndroidKeyLifecycle(t *testing.T) {
	f := newFake()
	ks := newAndroidKeystoreWithBridge(f)

	// alias 不存在:ensureKey 被请求恰好一次。
	if err := ks.EnsureMasterKey(); err != nil {
		t.Fatalf("EnsureMasterKey = %v", err)
	}
	if f.ensureCalls != 1 {
		t.Fatalf("alias 缺失时 EnsureMasterKey 应请求生成一次, ensureCalls = %d", f.ensureCalls)
	}

	// hasKey 已为 true:幂等短路,绝不重复请求生成(硬件 key 不会被重建)。
	f.hasV = true
	if err := ks.EnsureMasterKey(); err != nil {
		t.Fatalf("EnsureMasterKey(alias 已存在) = %v", err)
	}
	if f.ensureCalls != 1 {
		t.Fatalf("hasKey=true 时不应再生成, ensureCalls = %d", f.ensureCalls)
	}

	if has, err := ks.HasMasterKey(); err != nil || has != true {
		t.Fatalf("HasMasterKey = (%v, %v), want (true, nil)", has, err)
	}
	f.hasV = false
	if has, err := ks.HasMasterKey(); err != nil || has != false {
		t.Fatalf("HasMasterKey = (%v, %v), want (false, nil)", has, err)
	}

	// hasKey 自身报错经错误映射上抛(断连 → ErrUnavailable)。
	f.hasE = errors.New("android keystore bridge: BRIDGE_DISCONNECTED|keystore bridge not registered")
	if _, err := ks.HasMasterKey(); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("HasMasterKey(断连标记) err = %v, want ErrUnavailable", err)
	}

	// deleteKey 报 alias 不存在:DeleteMasterKey 幂等返回 nil。
	f.hasE = nil
	f.delE = errors.New("android keystore bridge: java.security.KeyStoreException|KEY_NOT_FOUND: no alias")
	if err := ks.DeleteMasterKey(); err != nil {
		t.Fatalf("DeleteMasterKey(alias 缺失) = %v, want nil(幂等)", err)
	}
	// 其它错误照常上抛。
	f.delE = errors.New("android keystore bridge: java.lang.IllegalStateException|boom")
	if err := ks.DeleteMasterKey(); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("DeleteMasterKey(未知) err = %v, want ErrUnavailable", err)
	}
	// ensureKey 失败经错误映射上抛(alias 缺失路径)。
	f.delE = nil
	f.hasV = false
	f.ensure = errors.New("android keystore bridge: BRIDGE_DISCONNECTED|JVM unavailable")
	if err := ks.EnsureMasterKey(); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("EnsureMasterKey(断连) err = %v, want ErrUnavailable", err)
	}
}

// TestAndroidSealValidatesBridgeBlob 断言 bridge 返回的 blob 布局不合
// iv(12)||ct||tag 时 Seal 直接 ErrUnseal(坏产物绝不落库)。
func TestAndroidSealValidatesBridgeBlob(t *testing.T) {
	f := newFake()
	f.sealV = bytes.Repeat([]byte{0x44}, 12+16-1) // 27B:不足 iv+tag
	ks := newAndroidKeystoreWithBridge(f)
	if _, err := ks.Seal([]byte("x")); !errors.Is(err, ErrUnseal) {
		t.Fatalf("Seal(坏 blob) err = %v, want ErrUnseal", err)
	}
}

// containsCJK 判断字符串是否含中日韩字符(指引文案存在性的轻量断言)。
func containsCJK(s string) bool {
	for _, r := range s {
		if r >= 0x4e00 && r <= 0x9fff {
			return true
		}
	}
	return false
}

// fmtErrUnseal 构造一个已包装 ErrUnseal 的错误(测试透传路径)。
func fmtErrUnseal(reason string) error {
	return fmt.Errorf("%w: %s", ErrUnseal, reason)
}
