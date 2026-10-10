// Package handlers —— 测试用内存 keystore:让凭据 store 在无 keyring daemon
// 的 CI 上也能真实 Seal/Unseal(落盘 round-trip),把 SecretStorage v2 的
// 磁盘路径跑通而不依赖 dbus。
package handlers

import "little-timer/internal/keystore"

// fakeKeystore 满足 keystore.Keystore 全集,从而可注入 app.App.Keystore,并被
// crypto.SecretStorage 以子集接口消费。Seal/Unseal 用固定前缀做可逆变换。
//
//   - hasKey=false:Seal/Unseal 返回 keystore.ErrNotFound,模拟“主密钥缺失
//     但磁盘已有 blob”导致的不可用态。
//   - name 会成为 CredentialStatus.Backend 的来源,默认用 "secret-service"。
type fakeKeystore struct {
	name   string
	hasKey bool
}

var _ keystore.Keystore = fakeKeystore{}

func (k fakeKeystore) Name() string                { return k.name }
func (k fakeKeystore) Available() error            { return nil }
func (k fakeKeystore) EnsureMasterKey() error      { return nil }
func (k fakeKeystore) DeleteMasterKey() error      { return nil }
func (k fakeKeystore) HasMasterKey() (bool, error) { return k.hasKey, nil }

func (k fakeKeystore) Seal(plaintext []byte) ([]byte, error) {
	if !k.hasKey {
		return nil, keystore.ErrNotFound
	}
	out := make([]byte, 0, len(plaintext)+2)
	out = append(out, 'k', ':')
	return append(out, plaintext...), nil
}

func (k fakeKeystore) Unseal(blob []byte) ([]byte, error) {
	if !k.hasKey {
		return nil, keystore.ErrNotFound
	}
	if len(blob) < 2 || blob[0] != 'k' || blob[1] != ':' {
		return nil, keystore.ErrUnseal
	}
	return append([]byte(nil), blob[2:]...), nil
}
