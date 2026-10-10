// Package http —— 集成测试用内存 keystore:让凭据 store 在无 keyring daemon
// 的 CI 上也能真实 Seal/Unseal,从而驱动 settings/backup 的配置保存路径而不
// 依赖 dbus。
package http

import "little-timer/internal/keystore"

// fakeKeystore 满足 keystore.Keystore 全集,可注入 app.App.Keystore,并被
// crypto.SecretStorage 以子集接口消费。Seal/Unseal 用固定前缀做可逆变换。
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
