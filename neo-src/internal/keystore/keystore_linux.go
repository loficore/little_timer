//go:build linux && !android

// keystore_linux.go —— linux 平台的 Keystore 实现:master key(32B)托管在
// (tag 带 `!android`:Go 把 GOOS=android 也视为满足 `linux`,而
// GOOS=android 由 keystore_android.go 注册后端,此处必须显式排除,
// 否则两个 init() 会先后覆盖 currentPlatform。)
// XDG Secret Service(org.freedesktop.secrets,D-Bus),数据加解密在进程内
// 用取回的 key 做 AES-256-GCM(blob = nonce(12)||ct||tag(16))。
// D-Bus 协议细节见 secret_service.go,包文档见 keystore.go。
//
// master key 仅在调用栈内存中短暂存在;本包无任何日志输出,key 不落日志。

package keystore

import (
	"fmt"

	"little-timer/internal/crypto"
)

func init() { currentPlatform = newLinuxKeystore() }

// linuxStore 是 Secret Service 后端的 Keystore 实现。无状态:每个操作新建
// D-Bus 客户端句柄(godbus 的 SessionBus 共享连接会自动复用 socket),
// 不缓存 master key。
type linuxStore struct{}

// newLinuxKeystore 返回 linux 平台 Keystore 实例。
func newLinuxKeystore() Keystore { return linuxStore{} }

// Name 返回 "secret-service",供 UI/日志区分后端。
func (linuxStore) Name() string { return "secret-service" }

// Available 探测 session bus 上 org.freedesktop.secrets 是否可达
// (必要时尝试 D-Bus 激活)。失败带 §9 sway 指引文案。
func (linuxStore) Available() error {
	svc, err := dialSecretService()
	if err != nil {
		return err
	}
	// 只查 bus name 是不够的：daemon 在线但默认集合锁定时，bus name 明明在，
	// 可所有凭据操作都会失败。启动 fail-fast（spec §9）要拦的正是这种
	// "半可用"状态 —— 放它过去，报错会拖到第一次读/写凭据才炸，且文案
	// 指向用户而不是环境。
	return svc.probeCollectionUsable()
}

// HasMasterKey 报告密钥环中是否已存在 master key item;item 被锁定也算存在。
func (linuxStore) HasMasterKey() (bool, error) {
	svc, err := dialSecretService()
	if err != nil {
		return false, err
	}
	_, found, _, err := svc.searchMasterKey()
	if err != nil {
		return false, err
	}
	return found, nil
}

// EnsureMasterKey 幂等建立 item "application:little_timer/master_key":
// 已存在则跳过;否则生成 32 字节随机 key 存入默认集合。
func (l linuxStore) EnsureMasterKey() error {
	has, err := l.HasMasterKey()
	if err != nil {
		return err
	}
	if has {
		return nil
	}
	svc, err := dialSecretService()
	if err != nil {
		return err
	}
	return svc.storeMasterKey(crypto.GenerateKey())
}

// Seal 取回 master key 后加密明文,返回 nonce(12)||ct||tag(16)。
// key 缺失返回 ErrNotFound;通道不可用返回 ErrUnavailable。
func (linuxStore) Seal(plaintext []byte) ([]byte, error) {
	key, err := masterKeyBytes()
	if err != nil {
		return nil, err
	}
	return sealBlob(plaintext, key)
}

// Unseal 取回 master key 后解密 Seal 产物;认证失败返回 ErrUnseal。
func (linuxStore) Unseal(blob []byte) ([]byte, error) {
	key, err := masterKeyBytes()
	if err != nil {
		return nil, err
	}
	return unsealBlob(blob, key)
}

// DeleteMasterKey 移除 master key item;不存在时幂等返回 nil。
func (linuxStore) DeleteMasterKey() error {
	svc, err := dialSecretService()
	if err != nil {
		return err
	}
	return svc.deleteMasterKey()
}

// masterKeyBytes 经 D-Bus 取回并校验 master key(缺失 → ErrNotFound)。
func masterKeyBytes() ([]byte, error) {
	svc, err := dialSecretService()
	if err != nil {
		return nil, err
	}
	key, err := svc.fetchMasterKey()
	if err != nil {
		return nil, err
	}
	if err := decodeMasterKey(key); err != nil {
		return nil, err
	}
	return key, nil
}

// sealBlob 用 AES-256-GCM 加密(key 必须 32B,nonce 由 crypto 包生成)。
func sealBlob(plaintext, key []byte) ([]byte, error) {
	blob, err := crypto.Encrypt(plaintext, key, crypto.GenerateNonce())
	if err != nil {
		return nil, fmt.Errorf("%w: seal 加密失败: %v", ErrUnseal, err)
	}
	return blob, nil
}

// unsealBlob 解密 Seal 产物;GCM 认证失败/blob 过短均映射为 ErrUnseal。
func unsealBlob(blob, key []byte) ([]byte, error) {
	plaintext, err := crypto.Decrypt(blob, key)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUnseal, err)
	}
	return plaintext, nil
}

// decodeMasterKey 校验取回的 master key 长度。异常长度说明密钥环被外部
// 重置/污染,按 ErrUnseal 报(所有历史 blob 将不可解)。
func decodeMasterKey(raw []byte) error {
	if len(raw) != crypto.AES256GCMKeySize {
		return fmt.Errorf("%w: master key 应为 %d 字节,实际 %d 字节(密钥环可能已被重置)",
			ErrUnseal, crypto.AES256GCMKeySize, len(raw))
	}
	return nil
}
