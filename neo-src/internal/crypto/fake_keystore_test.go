// fake_keystore_test.go —— SecretStorage v2 测试用的 keystore 替身。
//
// 镜像 Linux 后端语义:master key 存放在 map 里,Seal/Unseal 用取回的
// key 调 crypto.Encrypt/Decrypt(blob 布局同为 nonce(12)||ct||tag(16)),
// 与 keystore_linux.go 的 Go 内 AES-GCM 完全一致。
//
// 三个测试开关用于构造故障:
//   - unsealErr:非 nil 时 Unseal 恒返回该错误(模拟 keyring 被重置,
//     上层应透出 ErrKeystoreUnavailable)。
//   - hasErr:非 nil 时 HasMasterKey 返回 (false, hasErr),模拟
//     "密钥状态未知"(上层应透出 ErrKeystoreUnavailable 且可链到本错误)。
//   - unsealNoVerify:true 时 Unseal 跳过认证、原样返回传入 blob,
//     即"假装修不了密"。round-trip 测试在该开关打开时必须 fail ——
//     用来铁证测试路径确实在解密(反同义反复)。
package crypto_test

import (
	"fmt"
	"testing"

	"little-timer/internal/crypto"
	"little-timer/internal/keystore"
)

// fakeMasterKeyItem 与 keystore 实现的 item 名对应。
const fakeMasterKeyItem = "master_key"

type fakeKeystore struct {
	// items 托管 master key:"master_key" -> 32 字节随机 key。
	items map[string][]byte

	// unsealErr 非 nil 时 Unseal 直接返回它。
	unsealErr error

	// hasErr 非 nil 时 HasMasterKey 直接返回 (false, hasErr),
	// 模拟"master key 状态未知"(如 keystore daemon 不可达)。
	hasErr error

	// unsealNoVerify 打开后 Unseal 不校验 tag,恒返回原 blob。
	unsealNoVerify bool
}

// 编译期断言:替身既满足完整 keystore.Keystore(与真实后端同形状),
// 也满足 SecretStorage 的消费侧最小接口 crypto.Keystore。
var (
	_ keystore.Keystore = (*fakeKeystore)(nil)
	_ crypto.Keystore   = (*fakeKeystore)(nil)
)

// newFakeKeystore 返回已托管 master key 的替身。
func newFakeKeystore(t *testing.T) *fakeKeystore {
	t.Helper()
	f := &fakeKeystore{items: make(map[string][]byte)}
	if err := f.EnsureMasterKey(); err != nil {
		t.Fatalf("fake EnsureMasterKey: %v", err)
	}
	return f
}

// newFakeKeystoreNoKey 返回尚未托管 master key 的替身(模拟新装用户
// 或 keyring 被清空)。
func newFakeKeystoreNoKey(t *testing.T) *fakeKeystore {
	t.Helper()
	return &fakeKeystore{items: make(map[string][]byte)}
}

func (f *fakeKeystore) Name() string { return "fake" }

func (f *fakeKeystore) Available() error { return nil }

func (f *fakeKeystore) EnsureMasterKey() error {
	if _, ok := f.items[fakeMasterKeyItem]; !ok {
		f.items[fakeMasterKeyItem] = crypto.GenerateKey()
	}
	return nil
}

func (f *fakeKeystore) HasMasterKey() (bool, error) {
	if f.hasErr != nil {
		return false, f.hasErr
	}
	_, ok := f.items[fakeMasterKeyItem]
	return ok, nil
}

func (f *fakeKeystore) Seal(plaintext []byte) ([]byte, error) {
	key, ok := f.items[fakeMasterKeyItem]
	if !ok {
		return nil, fmt.Errorf("%w: fake master key 不存在", keystore.ErrNotFound)
	}
	return crypto.Encrypt(plaintext, key, crypto.GenerateNonce())
}

func (f *fakeKeystore) Unseal(blob []byte) ([]byte, error) {
	if f.unsealErr != nil {
		return nil, f.unsealErr
	}
	if f.unsealNoVerify {
		// 铁证开关:不校验 tag、不解密,原样返回。
		return append([]byte(nil), blob...), nil
	}
	key, ok := f.items[fakeMasterKeyItem]
	if !ok {
		return nil, fmt.Errorf("%w: fake master key 不存在", keystore.ErrNotFound)
	}
	return crypto.Decrypt(blob, key)
}

func (f *fakeKeystore) DeleteMasterKey() error {
	delete(f.items, fakeMasterKeyItem)
	return nil
}
