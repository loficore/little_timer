// Package keystoretest —— 测试支撑专用包(非生产代码),为 internal/http 下
// app / handlers / http 三个测试包提供共享的内存 FakeKeystore,替代此前
// 三份逐字重复的本地 fake。
//
// FakeKeystore 不依赖 keyring daemon 与 dbus:master key 由 hasKey 开关
// 控制,Seal/Unseal 用固定前缀做可逆变换,足以驱动 SecretStorage v2 的真实
// 磁盘 round-trip(Seal → 落盘 → 重建实例 → Unseal)。
//
// 可配置行为(与三份原副本的功能并集一致):
//   - name:成为 CredentialStatus.Backend 的来源,测试惯用 "secret-service"。
//   - hasKey=false:Seal/Unseal 返回 keystore.ErrNotFound、HasMasterKey 报
//     false,模拟“主密钥缺失但磁盘已有 blob”导致的不可用态。
//   - Unseal 对非本 fake 产出的 blob 返回 keystore.ErrUnseal。
package keystoretest

import "little-timer/internal/keystore"

// FakeKeystore 满足 keystore.Keystore 全集,可注入 app.App.Keystore,并被
// crypto.SecretStorage 以子集接口消费。零值无名字且无密钥,请用 New 构造。
type FakeKeystore struct {
	name   string
	hasKey bool
}

var _ keystore.Keystore = FakeKeystore{}

// New 返回名为 name 的内存 fake;hasKey 控制 master key 是否视为存在。
func New(name string, hasKey bool) FakeKeystore {
	return FakeKeystore{name: name, hasKey: hasKey}
}

// Name 返回注入的后端标识。
func (k FakeKeystore) Name() string { return k.name }

// Available 恒为就绪(内存实现无外部通道)。
func (k FakeKeystore) Available() error { return nil }

// EnsureMasterKey 恒成功(内存实现不落 keyring)。
func (k FakeKeystore) EnsureMasterKey() error { return nil }

// DeleteMasterKey 恒成功(内存实现不落 keyring)。
func (k FakeKeystore) DeleteMasterKey() error { return nil }

// HasMasterKey 返回构造时注入的 hasKey 开关。
func (k FakeKeystore) HasMasterKey() (bool, error) { return k.hasKey, nil }

// Seal 在 hasKey 时给明文加 "k:" 前缀;否则返回 keystore.ErrNotFound。
func (k FakeKeystore) Seal(plaintext []byte) ([]byte, error) {
	if !k.hasKey {
		return nil, keystore.ErrNotFound
	}
	out := make([]byte, 0, len(plaintext)+2)
	out = append(out, 'k', ':')
	return append(out, plaintext...), nil
}

// Unseal 逆 Seal;hasKey=false 返回 keystore.ErrNotFound,blob 非本 fake
// 产出时返回 keystore.ErrUnseal。
func (k FakeKeystore) Unseal(blob []byte) ([]byte, error) {
	if !k.hasKey {
		return nil, keystore.ErrNotFound
	}
	if len(blob) < 2 || blob[0] != 'k' || blob[1] != ':' {
		return nil, keystore.ErrUnseal
	}
	return append([]byte(nil), blob[2:]...), nil
}
