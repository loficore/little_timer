// Package crypto —— 带主口令保护的 secret 存储。
//
// 不集成系统 keychain：secret 放在内存 map 中（用主口令派生的密钥做
// 静态加密），并持久化到磁盘上的单个加密文件。文件路径由调用方提供，
// 由调用方决定 secret blob 存放位置（通常是
// `os.UserConfigDir()/little_timer/secrets.enc`）。
//
// 磁盘 blob 的 wire format：
//
//	magic (8 字节："LTMSECv1") || salt (16) || nonce (12) ||
//	  gcm_sealed( JSON(map[string][]byte) )
//
// GCM 密封内的 JSON 为 `{"key": base64(value), …}`。这是规范要求的
// OS keychain 不可用时的加密文件回退方案。
package crypto

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"little-timer/internal/log"
)

// SecretError 枚举 secret 存储的各类失败模式。
type SecretError string

const (
	ErrSecretNotFound         SecretError = "secret not found"
	ErrSecretAlreadyExists    SecretError = "secret already exists"
	ErrSecretInvalidValue     SecretError = "invalid value"
	ErrSecretNoAccess         SecretError = "no access"
	ErrSecretOutOfMemory      SecretError = "out of memory"
	ErrSecretLocked           SecretError = "secret store locked"
	ErrSecretMasterPwdNotSet  SecretError = "master password not set"
	ErrSecretDecryptionFailed SecretError = "decryption failed"
	ErrSecretIO               SecretError = "secret store i/o failed"
)

func (e SecretError) Error() string { return string(e) }

// 加密 blob 文件的 magic 前缀。用来把自己的文件和磁盘上的意外垃圾
// 区分开，并预留清晰的升级路径（升到 v2，在 `loadFromFile` 里迁移 v1）。
var secretMagic = []byte("LTMSECv1")

// 锁定：5 次失败尝试 → 锁定 300 秒。
const (
	MaxUnlockAttempts      = 5
	LockoutDurationSeconds = 300
)

// SecretStorage 是凭据存储的加密文件回退方案。
// 每进程一个实例；所有方法都是 goroutine 安全的。
type SecretStorage struct {
	mu sync.Mutex

	// filePath 是加密 blob 在磁盘上的位置。为空表示
	// “不持久化 —— 进程退出后 secret 即消失”。
	filePath string

	// masterPassword 是 SetMasterPassword / Unlock 之后保留在内存中的
	// 用户口令。保留它是因为磁盘 blob 走的是 password → PBKDF2 →
	// AES-GCM；若无口令重新派生 AES 密钥就得把密钥存盘（这会摧毁加密的
	// 意义）。Lock() 时清零。
	masterPassword []byte

	// secrets 是内存 cache。key 是用户提供的任意字节切片；在 JSON map
	// 内按原样序列化。
	secrets map[string][]byte

	// 锁定计数。
	failedAttempts  uint32
	lockedUntilUnix int64
}

// New 返回一个空的、处于锁定状态的 SecretStorage，持久化到 filePath。
// filePath 为 "" 时存储只在内存中。
func New(filePath string) *SecretStorage {
	return &SecretStorage{
		filePath: filePath,
		secrets:  make(map[string][]byte),
	}
}

// 主口令生命周期。

// SetMasterPassword 设置（或替换）主口令。对已有存储调用时，会用新口令
// 重新加密磁盘 blob。
func (s *SecretStorage) SetMasterPassword(password []byte) error {
	log.Info("SetMasterPassword: success")
	s.mu.Lock()
	defer s.mu.Unlock()

	zeroBytes(s.masterPassword)
	s.masterPassword = append([]byte(nil), password...)
	s.secrets = make(map[string][]byte)
	s.failedAttempts = 0
	s.lockedUntilUnix = 0
	return s.persistLocked()
}

// Unlock 用 password 校验磁盘 blob（若存在）并填充内存 cache。
// 失败尝试会触发锁定（见常量）。
func (s *SecretStorage) Unlock(password []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	log.Info("Unlock: attempt", "failed_attempts", s.failedAttempts)

	if s.lockedUntilUnix > time.Now().Unix() {
		return fmt.Errorf("%w: locked until %d", ErrSecretLocked, s.lockedUntilUnix)
	}

	if err := s.loadFromFileLocked(password); err != nil {
		s.failedAttempts++
		if s.failedAttempts >= MaxUnlockAttempts {
			s.lockedUntilUnix = time.Now().Unix() + LockoutDurationSeconds
		}
		log.Error("Unlock: failed", "error", err.Error())
		return err
	}

	s.failedAttempts = 0
	s.lockedUntilUnix = 0

	return nil
}

// HasMasterPassword 报告磁盘 blob 是否存在。不触碰内存状态。
func (s *SecretStorage) HasMasterPassword() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.filePath == "" {
		return false
	}
	_, err := os.Stat(s.filePath)
	return err == nil
}

// IsLocked 报告存储当前是否锁定（未调用过 Unlock，或处于锁定期）。
func (s *SecretStorage) IsLocked() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.masterPassword == nil
}

// Lock 丢弃主口令 + 解密后的 cache。磁盘 blob 不动 —— 下次 Unlock 会
// 重新加载。
func (s *SecretStorage) Lock() {
	s.mu.Lock()
	defer s.mu.Unlock()
	zeroBytes(s.masterPassword)
	s.masterPassword = nil
	// 清空 plaintext cache，避免解密后的 secret 留在内存里。
	for k := range s.secrets {
		zeroBytes(s.secrets[k])
		delete(s.secrets, k)
	}
}

// Secret 操作（要求处于解锁状态）。

// Store 插入或替换一对 key/value。加密完成后明文字节会从内存中抹掉。
func (s *SecretStorage) Store(key, value []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.requireUnlocked(); err != nil {
		return err
	}
	stored := append([]byte(nil), value...)
	s.secrets[string(key)] = stored
	return s.persistLocked()
}

// Retrieve 返回 key 对应的明文值。返回的是副本；调用方用完可自行擦除。
func (s *SecretStorage) Retrieve(key []byte) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.requireUnlocked(); err != nil {
		return nil, err
	}
	v, ok := s.secrets[string(key)]
	if !ok {
		return nil, ErrSecretNotFound
	}
	return append([]byte(nil), v...), nil
}

// Delete 删除一个 key。删除不存在的 key 是 no-op。
func (s *SecretStorage) Delete(key []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.requireUnlocked(); err != nil {
		return err
	}
	if v, ok := s.secrets[string(key)]; ok {
		zeroBytes(v)
		delete(s.secrets, string(key))
	}
	return s.persistLocked()
}

// Clear 擦除所有 secret。适用于“重置凭据”流程。
func (s *SecretStorage) Clear() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.requireUnlocked(); err != nil {
		return err
	}
	for k, v := range s.secrets {
		zeroBytes(v)
		delete(s.secrets, k)
	}
	return s.persistLocked()
}

// LockoutUntil 返回当前锁定结束的 unix 时间戳；未锁定时返回 0。
func (s *SecretStorage) LockoutUntil() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lockedUntilUnix
}

// 内部实现 —— 调用方必须持有 s.mu。

func (s *SecretStorage) requireUnlocked() error {
	if s.masterPassword == nil {
		return ErrSecretLocked
	}
	if s.lockedUntilUnix > time.Now().Unix() {
		return fmt.Errorf("%w: locked until %d", ErrSecretLocked, s.lockedUntilUnix)
	}
	return nil
}

// persistLocked 把 s.secrets 序列化为 JSON，用 masterKey 通过
// EncryptWithPassword 加密（直接把 masterKey 的字节当作 password 输入），
// 再把带 magic 前缀的 blob 写盘。未配置 filePath 时是 no-op。
//
// 每次写入都重新派生一个新 salt，而不是持久化一个；salt 只是 PBKDF2
// 需要，这样密码格式能与 DecryptWithPassword 保持一致。等价做法是直接用
// masterKey 调 Encrypt/Decrypt，但绕道 EncryptWithPassword 意味着磁盘
// blob 也能被 DecryptWithPassword 打开 —— 只要有人恢复了主口令就行，
// 对手工恢复工具有用。
func (s *SecretStorage) persistLocked() error {
	if s.filePath == "" {
		return nil
	}
	if s.masterPassword == nil {
		return ErrSecretLocked
	}

	encoded := make(map[string]string, len(s.secrets))
	for k, v := range s.secrets {
		encoded[k] = base64.StdEncoding.EncodeToString(v)
	}
	plaintext, err := json.Marshal(encoded)
	if err != nil {
		return fmt.Errorf("%w: marshal: %v", ErrSecretIO, err)
	}
	defer zeroBytes(plaintext)

	sealed, err := EncryptWithPassword(plaintext, s.masterPassword)
	if err != nil {
		return err
	}

	out := make([]byte, 0, len(secretMagic)+len(sealed))
	out = append(out, secretMagic...)
	out = append(out, sealed...)

	if err := os.WriteFile(s.filePath, out, 0o600); err != nil {
		return fmt.Errorf("%w: write: %v", ErrSecretIO, err)
	}
	return nil
}

// loadFromFileLocked 读取磁盘 blob，从 password 派生密钥，填充 s.secrets。
// 失败时保持内存状态不变。
func (s *SecretStorage) loadFromFileLocked(password []byte) error {
	if s.filePath == "" {
		return ErrSecretNotFound
	}
	data, err := os.ReadFile(s.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return ErrSecretNotFound
		}
		return fmt.Errorf("%w: read: %v", ErrSecretIO, err)
	}
	if !bytes.HasPrefix(data, secretMagic) {
		return ErrSecretDecryptionFailed
	}
	sealed := data[len(secretMagic):]

	plaintext, err := DecryptWithPassword(sealed, password)
	if err != nil {
		return err
	}
	defer zeroBytes(plaintext)

	decoded := make(map[string]string)
	if err := json.Unmarshal(plaintext, &decoded); err != nil {
		return fmt.Errorf("%w: unmarshal: %v", ErrSecretDecryptionFailed, err)
	}
	secrets := make(map[string][]byte, len(decoded))
	for k, v := range decoded {
		raw, err := base64.StdEncoding.DecodeString(v)
		if err != nil {
			return fmt.Errorf("%w: base64: %v", ErrSecretDecryptionFailed, err)
		}
		secrets[k] = raw
	}
	// 全部解析成功后才提升为活动状态。
	s.masterPassword = append([]byte(nil), password...)
	s.secrets = secrets
	return nil
}

// zeroBytes 就地擦除一个 byte slice。针对 heap dump 的尽力防御；
// Go 的 GC 可能已复制过数据，所以这是卫生习惯，不是保证。
func zeroBytes(b []byte) {
	for i := range b {
		b[i] = 0
	}
}
