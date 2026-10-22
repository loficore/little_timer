//go:build test_keystore

// 包 keystore 的**测试专用**内存 keystore（仅在 `-tags test_keystore` 下编译）。
//
// 为什么需要它：e2e / vrt 这类 job 验的是**应用行为**（计时、备份、设置流），
// 不是 keystore 的正确性。后者由 keystore-integration job 单独用真 daemon 验。
// 若让 e2e/vrt 也去启真 gnome-keyring，会引入一整套与被测目标无关的脆弱依赖：
// D-Bus 会话存活、gnome-keyring 版本差异、login 集合锁定时序……这些在真实
// 项目里是反复踩的坑（参考实现 libsecret 自己的测试也是用 mock Secret Service
// 而非真 daemon）。
//
// 为什么用 build tag 而不是环境变量：环境变量能被误设进生产，
// 一旦生产构建里启用了内存 keystore，**所有凭据将以明文强度存储**
// 且用户毫不知情。build tag 只能由构建命令显式开启，出现在 CI 配置里一眼可见，
// 生产构建不会带上 —— 这类安全相关的开关不该靠运行期字符串控制。
//
// 真 AES-GCM（stdlib，不引项目 crypto 包 —— 那会形成
// crypto → keystore → crypto 的导入环）。这一层保真度比 keystoretest 的
// "k:" 前缀高：能捕获密文损坏与 nonce 复用，而不是无条件往返成功。

package keystore

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"sync"
)

// memStore 是进程内 keystore。master key 只存在于堆内存，进程退出即消失。
type memStore struct {
	mu       sync.Mutex
	key      []byte
	hasKey   bool
	// perProcess 前缀让 blob 不可跨进程复用，避免测试间串味。
	perProcess []byte
}

func init() { currentPlatform = newMemKeystore() }

func newMemKeystore() Keystore {
	// 每个进程一份随机前缀：SecretStorage v2 的 seal 产物在本进程外必然解不开，
	// 与真实 keystore 的隔离语义一致。
	tag := make([]byte, 8)
	if _, err := rand.Read(tag); err != nil {
		panic(fmt.Errorf("keystore(mem): 读随机数失败: %w", err))
	}
	return &memStore{perProcess: tag}
}

func (m *memStore) Name() string { return "test-memory" }

// Available 恒为就绪 —— 内存实现没有外部通道可失败。
func (m *memStore) Available() error { return nil }

func (m *memStore) HasMasterKey() (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.hasKey, nil
}

func (m *memStore) EnsureMasterKey() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.hasKey {
		return nil
	}
	k := make([]byte, 32)
	if _, err := rand.Read(k); err != nil {
		return fmt.Errorf("%w: 读随机数失败: %v", ErrUnavailable, err)
	}
	m.key = k
	m.hasKey = true
	return nil
}

func (m *memStore) DeleteMasterKey() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.key = nil
	m.hasKey = false
	return nil
}

func (m *memStore) Seal(plaintext []byte) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.hasKey {
		return nil, ErrNotFound
	}
	aead, err := m.aeadLocked()
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("%w: 读 nonce 失败: %v", ErrUnavailable, err)
	}
	// 前缀让跨进程/跨构建的 blob 必然解不开，与真实 keystore 一致。
	sealed := aead.Seal(append([]byte{}, m.perProcess...), nonce, plaintext, nil)
	return append(nonce, sealed...), nil
}

func (m *memStore) Unseal(blob []byte) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.hasKey {
		return nil, ErrNotFound
	}
	aead, err := m.aeadLocked()
	if err != nil {
		return nil, err
	}
	if len(blob) < aead.NonceSize()+len(m.perProcess)+16 {
		return nil, ErrUnseal
	}
	nonce := blob[:aead.NonceSize()]
	body := blob[aead.NonceSize():]
	if len(body) < len(m.perProcess) || string(body[:len(m.perProcess)]) != string(m.perProcess) {
		// 别的进程/构建产生的 blob —— 内存 keystore 解不开，与真实实现同义。
		return nil, ErrUnseal
	}
	pt, err := aead.Open(nil, nonce, body[len(m.perProcess):], nil)
	if err != nil {
		return nil, ErrUnseal
	}
	return pt, nil
}

// aeadLocked 构造 GCM。调用方必须已持锁。
func (m *memStore) aeadLocked() (cipher.AEAD, error) {
	if len(m.key) != 32 {
		return nil, fmt.Errorf("%w: master key 未建立", ErrUnavailable)
	}
	block, err := aes.NewCipher(m.key)
	if err != nil {
		return nil, fmt.Errorf("%w: 构造 cipher 失败: %v", ErrUnavailable, err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("%w: 构造 GCM 失败: %v", ErrUnavailable, err)
	}
	return aead, nil
}
